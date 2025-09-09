package mux

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/hashicorp/yamux"
)

type MuxDialer struct {
	session *yamux.Session
}

func NewMuxDialer(conn io.ReadWriteCloser) (*MuxDialer, error) {
	config := yamux.DefaultConfig()
	config.EnableKeepAlive = true
	config.KeepAliveInterval = 30 * time.Second

	session, err := yamux.Client(conn, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create yamux client: %w", err)
	}

	return &MuxDialer{session: session}, nil
}

func (d *MuxDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return d.session.OpenStream()
}

func (d *MuxDialer) Close() error {
	return d.session.Close()
}
