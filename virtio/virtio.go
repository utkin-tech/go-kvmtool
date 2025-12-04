package virtio

/*
#cgo CFLAGS: -DCONFIG_GUEST_INIT -DCONFIG_X86_64 -DCONFIG_X86
#cgo CFLAGS: -D_FILE_OFFSET_BITS=64 -D_GNU_SOURCE
#cgo CFLAGS: -I../include -I../x86/include

#include <linux/virtio_console.h>
#include "kvm/virtio-console.h"
*/
import "C"
import "github.com/utkin-tech/go-kvmtool/pkg/ringbuffer"

func configEventsFunc() {
	C.virtio_console_config__inject_interrupt()
}

var configEvents = ringbuffer.NewRingBuffer[C.struct_virtio_console_control](1<<10, configEventsFunc)

//export has_config_event
func has_config_event() bool {
	return !configEvents.IsEmpty()
}

//export get_config_event
func get_config_event() C.struct_virtio_console_control {
	event, _ := configEvents.TryPop()
	return event
}

//export put_config_event
func put_config_event(event C.struct_virtio_console_control) {
	configEvents.Push(event)
}

func addPort(term uint) {
	var cpkt C.struct_virtio_console_control
	cpkt.id = C.__virtio32(term)
	cpkt.event = C.VIRTIO_CONSOLE_PORT_ADD
	cpkt.value = C.__virtio16(1)
	put_config_event(cpkt)
}
