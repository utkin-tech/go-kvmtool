package raphael_server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/utkin-tech/go-kvmtool/internal/state"
	"github.com/utkin-tech/go-kvmtool/pkg/api/leonardo"
	guest_mux "github.com/utkin-tech/go-kvmtool/pkg/mux"
	donatello_server "github.com/utkin-tech/go-kvmtool/pkg/server/donatello"
	"github.com/utkin-tech/go-kvmtool/pkg/terminal"
	"github.com/utkin-tech/go-kvmtool/pkg/utils"
	"github.com/utkin-tech/go-kvmtool/virtio"
)

var LeonardoClient *http.Client

func RunServer(root string, containerId string) {
	socketPath := utils.SocketPath(root, containerId)

	err := os.MkdirAll(utils.SocketDir(root, containerId), 0755)
	if err != nil {
		fmt.Printf("Error when create directory: %v\n", err)
		return
	}

	session, err := yamux.Client(virtio.Terminals[0], nil)
	if err != nil {
		log.Panicf("failed to create yamux server: %v", err)
	}
	defer session.Close()

	dialer, err := guest_mux.NewMuxDialer(session)
	if err != nil {
		log.Printf("Failed to create mux dialer: %v", err)
		return
	}
	defer dialer.Close()

	transport := &http.Transport{
		DialContext: dialer.DialContext,
	}

	LeonardoClient = &http.Client{
		Transport: transport,
		// Timeout:   5 * time.Second,
	}

	portNum.Store(2)

	term := virtio.Terminals[1]
	go addSocket(socketPath, term)
	go donatello_server.RunServer(session)

	select {}
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
	r.HandleFunc("/exec", func(w http.ResponseWriter, r *http.Request) {
		serveExec(w, r)
	})
	r.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		serveStart(w, r)
	})

	server := &http.Server{
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	fmt.Println("Unix domain socket server listening on", socketPath)

	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

var upgrader = websocket.Upgrader{}

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time to wait before force close on connection.
	closeGracePeriod = 10 * time.Second
)

func pumpStdin(ws *websocket.Conn, w io.Writer) {
	for {
		_, msg, err := ws.ReadMessage()

		if err != nil {
			log.Println("ws reader error")
			return
		}

		w.Write(msg)
	}
}

func pumpStdout(ws *websocket.Conn, r io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if err != nil {
			log.Println("stdout reader error")
			break
		}

		if n > 0 {
			err := ws.WriteMessage(websocket.BinaryMessage, buf[:n])
			if err != nil {
				log.Println("ws writer error")
				break
			}
		}
	}

	ws.SetWriteDeadline(time.Now().Add(writeWait))
	ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	time.Sleep(closeGracePeriod)
	ws.Close()
}

func serveAttach(w http.ResponseWriter, r *http.Request, outr io.Reader, inw io.Writer) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}

	defer ws.Close()

	go pumpStdout(ws, outr)

	pumpStdin(ws, inw)
}

func serveInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"version": "0.0.0", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
}

func serveExec(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	defer ws.Close()

	command := r.URL.Query().Get("command")
	if command == "" {
		command = "/bin/sh"
	}

	nextPortNum := portNum.Add(1)
	term := virtio.NewTerminal(nextPortNum)

	go pumpStdout(ws, term.GuestToHost)

	go pumpStdin(ws, term.HostToGuest)

	req := leonardo.ExecRequest{
		Command: command,
		Port:    nextPortNum,
	}

	b, err := json.Marshal(req)
	if err != nil {
		log.Fatalf("Failed to Serialize to JSON from native Go struct type: %v", err)
	}

	reqBody := bytes.NewBuffer(b)

	resp, err := LeonardoClient.Post("http://leonardo/exec", "application/json; charset=utf-8", reqBody)

	if err != nil {
		log.Printf("HTTP request error: %v", err)
		return
	}
	defer resp.Body.Close()

	resBody, _ := io.ReadAll(resp.Body)

	fmt.Fprint(w, string(resBody))
}

func serveStart(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	defer ws.Close()

	stateObs := state.Observable.Subscribe()
	for {
		state := <-stateObs
		log.Printf("state: %s\n", state)
		if state == specs.StateCreated {
			break
		}
	}

	nextPortNum := portNum.Add(1)
	term := virtio.NewTerminal(nextPortNum)

	go pumpStdout(ws, term.GuestToHost)

	go pumpStdin(ws, term.HostToGuest)

	req := leonardo.StartRequest{
		Port: nextPortNum,
	}

	b, err := json.Marshal(req)
	if err != nil {
		log.Fatalf("Failed to Serialize to JSON from native Go struct type: %v", err)
	}

	reqBody := bytes.NewBuffer(b)

	resp, err := LeonardoClient.Post("http://leonardo/start", "application/json; charset=utf-8", reqBody)

	if err != nil {
		log.Printf("HTTP request error: %v", err)
		return
	}
	defer resp.Body.Close()

	resBody, _ := io.ReadAll(resp.Body)

	fmt.Fprint(w, string(resBody))
}

var portNum atomic.Uint32
