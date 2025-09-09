package mux

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/hashicorp/yamux"
)

type MuxListener struct {
	session *yamux.Session
}

var _ net.Listener = (*MuxListener)(nil)

func NewMuxListener(conn io.ReadWriteCloser) (*MuxListener, error) {
	config := yamux.DefaultConfig()
	config.EnableKeepAlive = true
	config.KeepAliveInterval = 30 * time.Second

	session, err := yamux.Server(conn, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create yamux server: %w", err)
	}

	return &MuxListener{session: session}, nil
}

func (l *MuxListener) Accept() (net.Conn, error) {
	return l.session.Accept()
}

func (l *MuxListener) Close() error {
	return l.session.Close()
}

func (l *MuxListener) Addr() net.Addr {
	return l.session.Addr()
}
