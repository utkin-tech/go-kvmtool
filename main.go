package main

/*
#cgo CFLAGS: -Iinclude -Ix86/include
#cgo CFLAGS: -DCONFIG_GUEST_INIT -DCONFIG_GUEST_PRE_INIT -DCONFIG_X86_64 -DCONFIG_X86
#cgo CFLAGS: -D_FILE_OFFSET_BITS=64 -D_GNU_SOURCE
#cgo LDFLAGS: x86/bios/bios-rom.o
#cgo LDFLAGS: -lz

#include "kvm/kvm.h"
#include <stdlib.h>
#include <stdio.h>
#include "kvm/builtin-run.h"

static int handle_kvm_command(int fd_in, int fd_out, const char *kernel_filename) {
    return kvm_cmd_run(fd_in, fd_out, kernel_filename);
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
	cfg := ParseConfig()

	socks, err := demo.CreateSockets()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer socks.Close()

	go demo.RunServer(socketPath, socks.ParentToChildWriter, socks.ChildToParentReader)

	C.set_kvm_dir()

	fdIn := C.int(socks.ParentToChildReader.Fd())
	fdOut := C.int(socks.ChildToParentWriter.Fd())

	var kernelFilename *C.char
	if len(cfg.kernelFilename) > 0 {
		kernelFilename = C.CString(cfg.kernelFilename)
	}
	defer C.free(unsafe.Pointer(kernelFilename))

	ret := C.handle_kvm_command(fdIn, fdOut, kernelFilename)

	os.Exit(int(ret))
}
