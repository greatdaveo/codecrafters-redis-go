package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

var _ = net.Listen
var _ = os.Exit

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
		fmt.Println(n)

		if err != nil {
			if err == io.EOF{
				fmt.Println("End of file: No more connection")			
			} else {
				fmt.Println("Error: ", err)			
			}
			return
		}
		
		input := string(buffer[:n])
		// fmt.Println(input)

		if strings.Contains(strings.ToUpper(input), "ECHO") {
			parts := strings.Split(input, "\r\n")

			message := parts[4]

			response := fmt.Sprintf("$%d\r\n%s\r\n", len(message), message)			
			
			conn.Write([]byte(response))
		} else {
			conn.Write([]byte("+PONG\r\n"))
		}

	}	
}