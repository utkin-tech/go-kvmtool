package terminal

import (
	"fmt"
	"io"

	"github.com/utkin-tech/go-kvmtool/pkg/ringbuffer"
)

var _ io.ReadWriteCloser = (*Terminal)(nil)

type Terminal struct {
	HostToGuest *ringbuffer.RingBuffer[byte]
	GuestToHost *ringbuffer.RingBuffer[byte]
}

func (t *Terminal) Read(p []byte) (n int, err error) {
	return t.GuestToHost.Read(p)
}

func (t *Terminal) Write(p []byte) (n int, err error) {
	return t.HostToGuest.Write(p)
}

func (t *Terminal) Close() error {
	fmt.Println("Close unimplemented for Terminal")
	return nil
}
