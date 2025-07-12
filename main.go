package main

/*
#cgo CFLAGS: -Iinclude -Ix86/include
#cgo CFLAGS: -DCONFIG_GUEST_INIT -DCONFIG_GUEST_PRE_INIT -DCONFIG_X86_64 -DCONFIG_X86
#cgo CFLAGS: -D_FILE_OFFSET_BITS=64 -D_GNU_SOURCE
#cgo LDFLAGS: x86/bios/bios-rom.o guest/guest_init.o guest/guest_pre_init.o
#cgo LDFLAGS: -lz

#include "kvm/kvm.h"
#include <stdlib.h>
#include <stdio.h>
#include "kvm/builtin-run.h"

static int handle_kvm_command(int argc, char **argv, int fd_in, int fd_out) {
    return kvm_cmd_run(argc - 1, (const char **) &argv[1], NULL, fd_in, fd_out);
}

static void set_kvm_dir() {
    kvm__set_dir("%s/%s", HOME_DIR, KVM_PID_FILE_PATH);
}
*/
import "C"
import (
	"fmt"
	"os"
	"unsafe"

	"github.com/utkin-tech/go-kvmtool/demo"
	_ "github.com/utkin-tech/go-kvmtool/disk"
	_ "github.com/utkin-tech/go-kvmtool/hw"
	_ "github.com/utkin-tech/go-kvmtool/net/uip"
	_ "github.com/utkin-tech/go-kvmtool/util"
	_ "github.com/utkin-tech/go-kvmtool/vfio"
	_ "github.com/utkin-tech/go-kvmtool/virtio"
	_ "github.com/utkin-tech/go-kvmtool/x86"
)

const socketPath = "/tmp/example.sock"

func main() {
	socks, err := demo.CreateSockets()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer socks.Close()

	go demo.RunServer(socketPath, socks.ParentToChildWriter, socks.ChildToParentReader)

	C.set_kvm_dir()

	argc := len(os.Args) - 1
	argv := make([]*C.char, argc)

	for i, arg := range os.Args[1:] {
		argv[i] = C.CString(arg)
		defer C.free(unsafe.Pointer(argv[i]))
	}

	var argvPtr **C.char
	if argc > 0 {
		argvPtr = &argv[0]
	}

	fdIn := C.int(socks.ParentToChildReader.Fd())
	fdOut := C.int(socks.ChildToParentWriter.Fd())
	ret := C.handle_kvm_command(C.int(argc), argvPtr, fdIn, fdOut)

	os.Exit(int(ret))
}
