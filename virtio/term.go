package virtio

/*
#include "kvm/virtio-console.h"
*/
import "C"
import (
	"fmt"
	"unsafe"

	"github.com/utkin-tech/go-kvmtool/pkg/ringbuffer"
	"github.com/utkin-tech/go-kvmtool/pkg/terminal"
)

const terminalNum = 2

const kernelTerminalId = 1

var Terminals map[uint32]*terminal.Terminal

func HostToGuestFunc(term int) func() {
	vq := 0
	if term != 0 {
		vq = (term + 1) << 1
	}

	return func() {
		C.virtio_console__inject_interrupt_vq(nil, C.int(vq))
	}
}

func init() {
	Terminals = make(map[uint32]*terminal.Terminal, terminalNum)
	for i := uint32(0); i < terminalNum; i++ {
		Terminals[i] = &terminal.Terminal{
			HostToGuest: ringbuffer.NewRingBuffer[byte](1<<16, HostToGuestFunc(int(i))),
			GuestToHost: ringbuffer.NewRingBuffer[byte](1<<10, nil),
		}
	}
}

//export g_ringbuffer_write
func g_ringbuffer_write(base unsafe.Pointer, len C.int, term C.int) C.int {
	buffer := C.GoBytes(base, len)
	for _, b := range buffer {
		// TODO remove debug output and sink it to kernel.log
		if term == kernelTerminalId {
			fmt.Print(string(b))
		}
		Terminals[uint32(term)].GuestToHost.Push(b)
	}
	return len
}

//export g_term_getc
func g_term_getc(term C.int) C.int {
	b, ok := Terminals[uint32(term)].HostToGuest.TryPop()
	if !ok {
		return -1
	}
	return C.int(b)
}

//export g_term_readable
func g_term_readable(term C.int) bool {
	hostToGuest := Terminals[uint32(term)].HostToGuest
	return !hostToGuest.IsEmpty()
}

func NewTerminal(port uint32) *terminal.Terminal {
	term := &terminal.Terminal{
		HostToGuest: ringbuffer.NewRingBuffer[byte](1<<16, HostToGuestFunc(int(port))),
		GuestToHost: ringbuffer.NewRingBuffer[byte](1<<10, nil),
	}

	Terminals[port] = term

	addPort(uint(port))

	return term
}
