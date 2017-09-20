package main

import (
	"bytes"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
)

const (
	DEFAULT_ADDRESS   = "localhost:3333"
	CONN_TYPE         = "tcp"
	DEFAULT_SERVER_ID = "sample_server"
)

var serverAddress = flag.String(
	"address",
	DEFAULT_ADDRESS,
	"Comma separated addresses in host:port format that the server will bind to.",
)

var serverId = flag.String(
	"serverId",
	DEFAULT_SERVER_ID,
	"The Server id that is echoed back for each message.",
)

func main() {
	flag.Parse()
	addresses := strings.Split(*serverAddress, ",")
	includeServerAddress := len(addresses) > 1
	wg := sync.WaitGroup{}
	for _, address := range addresses {
		wg.Add(1)
		go launchServer(address, includeServerAddress, &wg)
	}
	wg.Wait()
}

func launchServer(address string, includeServerAddress bool, wg *sync.WaitGroup) {
	// Listen for incoming connections.
	listener, err := net.Listen(CONN_TYPE, address)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	// Close the listener when the application closes.
	defer listener.Close()
	fmt.Printf("%s:Listening on %s\n", *serverId, address)
	for {
		// Listen for an incoming connection.
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			wg.Done()
		}
		// Handle connections in a new goroutine.
		go handleRequest(conn, includeServerAddress, address)
	}
}

// Handles incoming requests.
func handleRequest(conn net.Conn, includeServerAddress bool, address string) {
	// Close the connection when you're done with it.
	defer conn.Close()
	// Make a buffer to hold incoming data.
	buff := make([]byte, 1024)
	// Continue to receive the data forever...
	for {
		// Read the incoming connection into the buffer.
		readBytes, err := conn.Read(buff)
		if err != nil {
			fmt.Println("Error on connection read:", err.Error())
			return
		}
		var writeBuffer bytes.Buffer
		writeBuffer.WriteString(*serverId)
		if includeServerAddress {
			writeBuffer.WriteString("(" + address + ")")
		}
		writeBuffer.WriteString(":")
		writeBuffer.Write(buff[0:readBytes])
		fmt.Println(writeBuffer.String())
		_, err = conn.Write(writeBuffer.Bytes())
		if err != nil {
			fmt.Println("Error on connection write:", err.Error())
			return
		}
	}
}
