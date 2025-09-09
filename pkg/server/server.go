package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/utkin-tech/go-kvmtool/pkg/terminal"
	"github.com/utkin-tech/go-kvmtool/virtio"
)

const terminalNum = 2

func RunServer() {
	for i := 0; i < terminalNum; i++ {
		socketPath := fmt.Sprintf("/tmp/gkvm/term%d", i)
		term := virtio.Terminals[i]
		go addSocket(socketPath, term)
	}
}

func addSocket(socketPath string, term *terminal.Terminal) {
	if err := os.RemoveAll(socketPath); err != nil {
		log.Fatal(err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	defer listener.Close()

	if err := os.Chmod(socketPath, 0770); err != nil {
		log.Fatal("Chmod error:", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down server...")
		listener.Close()
		os.Remove(socketPath)
		os.Exit(0)
	}()

	fmt.Println("Unix domain socket server listening on", socketPath)

	parentToChildWriter := term.HostToGuest
	childToParentReader := term.GuestToHost

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Accept error:", err)
			continue
		}

		go handleConnection(conn, parentToChildWriter, childToParentReader)
	}
}

func handleConnection(conn net.Conn, parentToChildWriter io.Writer, childToParentReader io.Reader) {
	defer conn.Close()
	clientAddr := conn.RemoteAddr().String()
	fmt.Printf("Client connected: %s\n", clientAddr)

	go io.Copy(parentToChildWriter, conn)
	io.Copy(conn, childToParentReader)

	fmt.Printf("Client disconnected: %s\n", clientAddr)
}
