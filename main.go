package main

/*
#cgo CFLAGS: -Iinclude -Ix86/include
#cgo CFLAGS: -DCONFIG_GUEST_INIT -DCONFIG_X86_64 -DCONFIG_X86
#cgo CFLAGS: -D_FILE_OFFSET_BITS=64 -D_GNU_SOURCE
#cgo LDFLAGS: -lz

#include "kvm/kvm.h"
#include <stdlib.h>
#include <stdio.h>
#include "kvm/builtin-run.h"

static int handle_kvm_command(const char *kernel_filename) {
    return kvm_cmd_run(kernel_filename);
}

static void set_kvm_dir() {
    kvm__set_dir("%s/%s", HOME_DIR, KVM_PID_FILE_PATH);
}
*/
import "C"
import (
	"os"
	"unsafe"

	_ "github.com/utkin-tech/go-kvmtool/disk"
	_ "github.com/utkin-tech/go-kvmtool/hw"
	_ "github.com/utkin-tech/go-kvmtool/net/uip"
	"github.com/utkin-tech/go-kvmtool/pkg/monitor"
	"github.com/utkin-tech/go-kvmtool/pkg/server"
	_ "github.com/utkin-tech/go-kvmtool/util"
	_ "github.com/utkin-tech/go-kvmtool/vfio"
	_ "github.com/utkin-tech/go-kvmtool/x86"
)

const KernelFilename = "/home/user/go-kvmtool/tmp/bzImage2"

func main() {
	go server.RunServer()

	go monitor.RunServer()

	C.set_kvm_dir()

	kernelFilename := C.CString(KernelFilename)
	defer C.free(unsafe.Pointer(kernelFilename))

	ret := C.handle_kvm_command(kernelFilename)

	os.Exit(int(ret))
}
