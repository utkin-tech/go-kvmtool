package main

import (
	_ "embed"
	"unsafe"
)
import "C"

//go:embed guest/init
var init_binary []byte

//export get_init_binary_size
func get_init_binary_size() unsafe.Pointer {
	return C.CBytes(init_binary)
}

//export get_init_binary
func get_init_binary() int {
	return len(init_binary)
}

//go:embed guest/pre_init
var pre_init_binary []byte

//export get_pre_init_binary_size
func get_pre_init_binary_size() unsafe.Pointer {
	return C.CBytes(pre_init_binary)
}

//export get_pre_init_binary
func get_pre_init_binary() int {
	return len(pre_init_binary)
}
