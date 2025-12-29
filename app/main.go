package main

import (
	"fmt"
	"net"
	"os"
	"io"

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

	buffer := make([]byte, 1024)

	for {		
		_, err := conn.Read(buffer)

		if err != nil {
			if err == io.EOF{
				fmt.Println("End of file: No more connection")			
			} else {
				fmt.Println("Error: ", err)			
			}
			return
		}

		conn.Write([]byte("+PONG\r\n"))
	}	
}