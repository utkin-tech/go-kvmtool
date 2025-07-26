package x86

/*
#cgo CFLAGS: -DCONFIG_GUEST_INIT -DCONFIG_GUEST_PRE_INIT -DCONFIG_X86_64 -DCONFIG_X86
#cgo CFLAGS: -D_FILE_OFFSET_BITS=64 -D_GNU_SOURCE
#cgo CFLAGS: -I../include -Iinclude -Ibios
*/
import "C"
import (
	_ "embed"
	"unsafe"
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
