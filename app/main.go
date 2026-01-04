package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
)

var _ = net.Listen
var _ = os.Exit
var (
	storage = make(map[string]string)
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
		
		// GET Input: *3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n
		// SET Input: *2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n
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
			key := parts[4]
			value := parts[6]

			mu.Lock()
			storage[key] = value
			mu.Unlock()

			conn.Write([]byte("+OK\r\n"))
		case "GET":
			key := parts[4]

			mu.RLock()
			value, exists := storage[key]
			mu.RUnlock()

			if exists {
				// Send Bulk String: "$length\r\nvalue\r\n"
				response := fmt.Sprintf("$%d\r\n%s\r\n", len(value), value)			
				conn.Write([]byte(response))
			} else {
				// Redis Null Bull String 
				conn.Write([]byte("$-1\r\n"))
			}
			
		default:
			fmt.Println("Unknown command:", command)
		}
		

	}	
}