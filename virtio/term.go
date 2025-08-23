package virtio

/*
#include "kvm/virtio-console.h"
*/
import "C"
import (
	"fmt"
	"unsafe"

	"github.com/utkin-tech/go-kvmtool/pkg/ringbuffer"
)

var HostToGuest = ringbuffer.NewRingBuffer[byte](1<<16, HostToGuestFunc)
var GuestToHost = ringbuffer.NewRingBuffer[byte](1<<10, nil)

func HostToGuestFunc() {
	C.virtio_console__inject_interrupt(nil)
}

//export g_ringbuffer_write
func g_ringbuffer_write(base unsafe.Pointer, len C.int) C.int {
	buffer := C.GoBytes(base, len)
	for _, b := range buffer {
		GuestToHost.Push(b)
	}
	return len
}

//export g_term_getc
func g_term_getc(term C.int) C.int {
	fmt.Println("g_term_getc")

	b, ok := HostToGuest.TryPop()
	if !ok {
		return -1
	}
	return C.int(b)
}

//export g_term_readable
func g_term_readable(term C.int) bool {
	return !HostToGuest.IsEmpty()
}
