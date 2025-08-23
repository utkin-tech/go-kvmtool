package virtio

/*
#cgo CFLAGS: -DCONFIG_GUEST_INIT -DCONFIG_GUEST_PRE_INIT -DCONFIG_X86_64 -DCONFIG_X86
#cgo CFLAGS: -D_FILE_OFFSET_BITS=64 -D_GNU_SOURCE
#cgo CFLAGS: -I../include -I../x86/include

#include <linux/virtio_console.h>
*/
import "C"
import (
	"sync"
)

var mu sync.Mutex
var ch = make(chan C.struct_virtio_console_control, 16)
var count int

//export has_config_event
func has_config_event() bool {
	mu.Lock()
	defer mu.Unlock()
	return count > 0
}

//export get_config_event
func get_config_event() C.struct_virtio_console_control {
	mu.Lock()
	defer mu.Unlock()
	count--
	event := <-ch
	return event
}

//export put_config_event
func put_config_event(event C.struct_virtio_console_control) {
	mu.Lock()
	defer mu.Unlock()
	count++
	ch <- event
}
