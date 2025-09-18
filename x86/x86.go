package x86

/*
#cgo CFLAGS: -DCONFIG_GUEST_INIT -DCONFIG_X86_64 -DCONFIG_X86
#cgo CFLAGS: -D_FILE_OFFSET_BITS=64 -D_GNU_SOURCE
#cgo CFLAGS: -I../include -Iinclude -Ibios
#include <stdint.h>
*/
import "C"
import (
	_ "embed"
	"fmt"
	"unsafe"

	"github.com/utkin-tech/go-kvmtool/pkg/initrd"
)

//go:embed bios/bios.bin
var bios_rom2 []byte

//export bios_rom2_start
func bios_rom2_start() unsafe.Pointer {
	return C.CBytes(bios_rom2)
}

//export bios_rom2_size
func bios_rom2_size() int {
	return len(bios_rom2)
}

//export get_initrd_size
func get_initrd_size() int {
	fmt.Printf("get_initrd_size %d\n", len(initrd.Initrd))
	return len(initrd.Initrd)
}

//export read_in_full_initrd
func read_in_full_initrd(p *C.uint8_t) int {
	fmt.Println("read_in_full_initrd")
	length := len(initrd.Initrd)
	slice := unsafe.Slice(p, length)
	for i := 0; i < length; i++ {
		slice[i] = C.uint8_t(initrd.Initrd[i])
	}
	return length
}
