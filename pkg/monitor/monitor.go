package monitor

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/utkin-tech/go-kvmtool/pkg/mux"
	"github.com/utkin-tech/go-kvmtool/virtio"
)

type AddTerminalRequest struct {
	ID uint `json:"id"`
}

func RunServer() {
	dialer, err := mux.NewMuxDialer(virtio.Terminals[0])
	if err != nil {
		log.Printf("Failed to create mux dialer: %v", err)
		return
	}
	defer dialer.Close()

	transport := &http.Transport{
		DialContext: dialer.DialContext,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	http.HandleFunc("/addPort", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req AddTerminalRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		virtio.AddPort(req.ID)

		w.WriteHeader(http.StatusCreated)
	})

	http.HandleFunc("/guestHello", func(w http.ResponseWriter, r *http.Request) {
		var resp *http.Response
		var err error

		resp, err = client.Get("http://dummy-host/hello")

		if err != nil {
			log.Printf("HTTP request error: %v", err)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		fmt.Fprintf(w, string(body))
	})

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
