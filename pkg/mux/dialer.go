package mux

import (
	"context"
	"net"

	"github.com/hashicorp/yamux"
)

type MuxDialer struct {
	session *yamux.Session
}

func NewMuxDialer(session *yamux.Session) (*MuxDialer, error) {
	return &MuxDialer{session: session}, nil
}

func (d *MuxDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return d.session.OpenStream()
}

func (d *MuxDialer) Close() error {
	return d.session.Close()
}
