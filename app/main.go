package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var _ = net.Listen
var _ = os.Exit

type Item struct {
	Value string
	ExpiresAt time.Time
	HasExpiry bool
}

var (
	storage = make(map[string]Item) //FOR SET and GET
	lists = make(map[string][]string) // For RPUSH, LPUSH and LRANGE
	mu 	sync.RWMutex
)


func main() {	
	l, err := net.Listen("tcp", "0.0.0.0:6379")

	if err != nil {
		fmt.Println("Failed to bind to port 6379")
		os.Exit(1)
	}

	for {
		conn, err := l.Accept()

		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}

		go multipleConn(conn)
	}
}



func multipleConn(conn net.Conn) {
	defer conn.Close()
		
	for {		
		buffer := make([]byte, 1024)

		n, err := conn.Read(buffer)
		// fmt.Println(n)

		if err != nil {
			if err == io.EOF{
				fmt.Println("End of file: No more connection")			
			} else {
				fmt.Println("Error: ", err)			
			}
			return
		}
		
		input := string(buffer[:n])
		parts := strings.Split(input, "\r\n")
		command := strings.ToUpper(parts[2])

		switch command {
		case "PING":
			conn.Write([]byte("+PONG\r\n"))			
		case "ECHO":
			message := parts[4]
			response := fmt.Sprintf("$%d\r\n%s\r\n", len(message), message)			
			conn.Write([]byte(response))
		case "SET":
			// SET foo bar PX 100: *5\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n$2\r\nPX\r\n$3\r\n100\r\n
			key := parts[4]
			value := parts[6]
			
			item := Item{
				Value: value,
				HasExpiry: false,
			}
			
			if len(parts) > 8 && strings.ToUpper(parts[8]) == "PX"{
				ms, err := strconv.Atoi(parts[10])
				expiryDuration := time.Duration(ms) * time.Millisecond
				expiresAt := time.Now().Add(expiryDuration)

				if err == nil {
					item.HasExpiry = true
					item.ExpiresAt = expiresAt					
				}
				
			}
			
			mu.Lock()
			storage[key] = item			
			mu.Unlock()

			conn.Write([]byte("+OK\r\n"))
		case "GET":
			// GET foo: *2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n
			key := parts[4]

			mu.RLock()
			item, exists := storage[key]
			mu.RUnlock()

			if exists && item.HasExpiry && time.Now().After(item.ExpiresAt){
				// if it has expired
				mu.Lock()
				delete(storage, key) //cleanup from memory
				mu.Unlock()

				exists = false
			}

			if exists {
				// Send Bulk String: "$length\r\nvalue\r\n"
				response := fmt.Sprintf("$%d\r\n%s\r\n", len(item.Value), item.Value)			
				conn.Write([]byte(response))
			} else {
				// Redis Null Bull String 
				conn.Write([]byte("$-1\r\n"))
			}
		case "LPUSH":
			key := parts[4]
			var newValues []string

			for i := 6; i < len(parts); i += 2 {
				if parts[i] != "" {
					newValues = append(newValues, parts[i])
				}
			}

			mu.Lock()
			currentList := lists[key]

			// Reverse prepend 
			for _, val := range newValues {
				currentList = append([]string{val}, currentList...)
			}

			lists[key] = currentList
			listLength := len(lists[key])
			mu.Unlock()

			// :<length>\r\n
			conn.Write([]byte(fmt.Sprintf(":%d\r\n", listLength)))

		case "RPUSH":
			// RPUSH colors red blue: *4\r\n$5\r\nRPUSH\r\n$6\r\ncolors\r\n$3\r\nred\r\n$4\r\nblue\r\n
			key := parts[4]
			var newValues []string

			for i := 6; i < len(parts); i += 2 {
				if parts[i] != "" {
					newValues = append(newValues, parts[i])
				}
			}

			mu.Lock()
			lists[key] = append(lists[key], newValues...)
			listLength := len(lists[key])
			mu.Unlock()

			// :<length>\r\n
			response := fmt.Sprintf(":%d\r\n", listLength)
			conn.Write([]byte(response))
		
		case "LRANGE":
			// LRANGE key start stop: *4\r\n$6\r\nLRANGE\r\n$6\r\nmylist\r\n$1\r\n0\r\n$1\r\n2\r\n
			key := parts[4]
			start, _ := strconv.Atoi(parts[6])
			stop, _ := strconv.Atoi(parts[8])

			mu.RLock()
			fullList, exists := lists[key]
			mu.RUnlock()

			listLegth := len(fullList)

			if !exists || listLegth == 0 {
				conn.Write([]byte("*0\r\n"))
				continue
			}

			// For negative start or stop
			if start < 0 {
				start = listLegth + start
			}

			if stop < 0 {
				stop = listLegth + stop
			}

			// after conversion
			if start < 0 {
				start = 0
			}
			if stop >= listLegth {
				stop = listLegth - 1 // cap to the last index
			}

			if stop >= len(fullList){
				stop = len(fullList) - 1
			}

			if start > stop {
				conn.Write([]byte("*0\r\n"))
				continue
			}

			//Extract sub slice
			result := fullList[start : stop + 1] // because stop is exclusive
			// Array header - count of items in the array
			response := fmt.Sprintf("*%d\r\n", len(result))

			// Add each item as a bulk string to the array
			for _, item := range result {
				response += fmt.Sprintf("$%d\r\n%s\r\n", len(item), item)
			}

			conn.Write([]byte(response))
		
		case "LLEN":
			// LLEN key: *2\r\n$4\r\nLLEN\r\n$6\r\nmylist\r\n
			key := parts[4]

			mu.RLock()
			currentLists, exists := lists[key]
			mu.RUnlock()

			length := 0

			if exists {
				length = len(currentLists)
			}

			// :count\r\n
			response := fmt.Sprintf(":%d\r\n", length)
			
			conn.Write([]byte(response))


			
		default:
			fmt.Println("Unknown command:", command)
		}
		

	}	
}