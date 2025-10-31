package server

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/utkin-tech/go-kvmtool/pkg/terminal"
	"github.com/utkin-tech/go-kvmtool/pkg/utils"
	"github.com/utkin-tech/go-kvmtool/virtio"
)

func RunServer(root string, containerId string) {
	socketPath := utils.SocketPath(root, containerId)

	err := os.MkdirAll(utils.SocketDir(root, containerId), 0755)
	if err != nil {
		fmt.Printf("Error when create directory: %v\n", err)
		return
	}

	term := virtio.Terminals[0]
	go addSocket(socketPath, term)
}

func addSocket(socketPath string, term *terminal.Terminal) {
	parentToChildWriter := term.HostToGuest
	childToParentReader := term.GuestToHost

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

	r := mux.NewRouter()

	r.HandleFunc("/info", serveInfo)
	r.HandleFunc("/attach", func(w http.ResponseWriter, r *http.Request) {
		serveAttach(w, r, childToParentReader, parentToChildWriter)
	})

	server := &http.Server{
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	fmt.Println("Unix domain socket server listening on", socketPath)

	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Ошибка сервера: %v", err)
	}
}

var upgrader = websocket.Upgrader{}

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Maximum message size allowed from peer.
	maxMessageSize = 8192

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Time to wait before force close on connection.
	closeGracePeriod = 10 * time.Second
)

func pumpStdin(ws *websocket.Conn, w io.Writer) {
	defer ws.Close()
	ws.SetReadLimit(maxMessageSize)
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			break
		}
		message = append(message, '\n')
		if _, err := w.Write(message); err != nil {
			break
		}
	}
}

func pumpStdout(ws *websocket.Conn, r io.Reader, done chan struct{}) {
	defer func() {
	}()
	s := bufio.NewScanner(r)
	for s.Scan() {
		ws.SetWriteDeadline(time.Now().Add(writeWait))
		if err := ws.WriteMessage(websocket.TextMessage, s.Bytes()); err != nil {
			ws.Close()
			break
		}
	}
	if s.Err() != nil {
		log.Println("scan:", s.Err())
	}
	close(done)

	ws.SetWriteDeadline(time.Now().Add(writeWait))
	ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	time.Sleep(closeGracePeriod)
	ws.Close()
}

func ping(ws *websocket.Conn, done chan struct{}) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait)); err != nil {
				log.Println("ping:", err)
			}
		case <-done:
			return
		}
	}
}

func serveAttach(w http.ResponseWriter, r *http.Request, outr io.Reader, inw io.Writer) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}

	defer ws.Close()

	stdoutDone := make(chan struct{})
	go pumpStdout(ws, outr, stdoutDone)
	go ping(ws, stdoutDone)

	pumpStdin(ws, inw)
}

func serveInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"version": "0.0.0", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
}
