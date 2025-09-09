package virtio

/*
#include "kvm/virtio-console.h"
*/
import "C"
import (
	"unsafe"

	"github.com/utkin-tech/go-kvmtool/pkg/ringbuffer"
	"github.com/utkin-tech/go-kvmtool/pkg/terminal"
)

const terminalNum = 2

var Terminals []*terminal.Terminal

func HostToGuestFunc(term int) func() {
	if term != 0 {
		term = (term + 1) << 1
	}

	return func() {
		C.virtio_console__inject_interrupt_vq(nil, C.int(term))
	}
}

func init() {
	Terminals = make([]*terminal.Terminal, terminalNum)
	for i := 0; i < terminalNum; i++ {
		Terminals[i] = &terminal.Terminal{
			HostToGuest: ringbuffer.NewRingBuffer[byte](1<<16, HostToGuestFunc(i)),
			GuestToHost: ringbuffer.NewRingBuffer[byte](1<<10, nil),
		}
	}
}

//export g_ringbuffer_write
func g_ringbuffer_write(base unsafe.Pointer, len C.int, term C.int) C.int {
	buffer := C.GoBytes(base, len)
	for _, b := range buffer {
		Terminals[term].GuestToHost.Push(b)
	}
	return len
}

//export g_term_getc
func g_term_getc(term C.int) C.int {
	b, ok := Terminals[term].HostToGuest.TryPop()
	if !ok {
		return -1
	}
	return C.int(b)
}

//export g_term_readable
func g_term_readable(term C.int) bool {
	return !Terminals[term].HostToGuest.IsEmpty()
}
