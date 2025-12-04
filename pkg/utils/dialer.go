package utils

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func UnixWebsocketDialer(socketPath string) *websocket.Dialer {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}

	wsDialer := websocket.Dialer{
		NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath)
		},
		HandshakeTimeout: 5 * time.Second,
	}

	return &wsDialer
}

func UnixHttpClient(socketPath string) *http.Client {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
		Timeout: 10 * time.Second,
	}

	return client
}
