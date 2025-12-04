package donatello_server

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/utkin-tech/go-kvmtool/internal/state"
)

func RunServer(l net.Listener) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		state.Observable.Set(specs.StateCreated)
	})

	server := &http.Server{Handler: mux}
	fmt.Println("donatello server starting...")

	if err := server.Serve(l); err != nil && err != http.ErrServerClosed {
		log.Printf("donatello server error: %v\n", err)
	}
}
