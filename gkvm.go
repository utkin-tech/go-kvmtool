package kvmtool

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
	setvbuf(stdout, NULL, _IONBF, 0);
    return kvm_cmd_run(kernel_filename);
}

static void set_kvm_dir() {
    kvm__set_dir("%s/%s", HOME_DIR, KVM_PID_FILE_PATH);
}
*/
import "C"
import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	_ "github.com/utkin-tech/go-kvmtool/disk"
	_ "github.com/utkin-tech/go-kvmtool/hw"
	_ "github.com/utkin-tech/go-kvmtool/net/uip"
	"github.com/utkin-tech/go-kvmtool/pkg/config"
	gkvm_net "github.com/utkin-tech/go-kvmtool/pkg/net"
	"github.com/utkin-tech/go-kvmtool/pkg/ociconfig"
	raphael_server "github.com/utkin-tech/go-kvmtool/pkg/server/raphael"
	"github.com/utkin-tech/go-kvmtool/pkg/utils"
	_ "github.com/utkin-tech/go-kvmtool/util"
	_ "github.com/utkin-tech/go-kvmtool/vfio"
	_ "github.com/utkin-tech/go-kvmtool/x86"
	"golang.org/x/sys/unix"
)

const (
	virtDir        = "virt"
	agentFileName  = "init"
	configFileName = "config.json"
)

func prepareRootfs(cfg *config.Config, bundle string) error {
	virtPath := filepath.Join(ociconfig.RootfsPath, virtDir)

	agentFileSource := cfg.Agent
	agentFileTarget := filepath.Join(virtPath, agentFileName)

	configFileSource := filepath.Join(bundle, configFileName)
	configFileTarget := filepath.Join(virtPath, configFileName)

	err := os.MkdirAll(virtPath, 0775)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	err = unix.Mount("tmpfs", virtPath, "tmpfs", 0, "")
	if err != nil {
		return fmt.Errorf("failed to create tmpfs: %w", err)
	}

	utils.CopyFile(agentFileSource, agentFileTarget)
	utils.CopyFile(configFileSource, configFileTarget)

	return nil
}

func Run(bundle string, root string, containerId string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("failed to load config: %v\n", err)
		return
	}

	err = ociconfig.Load(bundle)
	if err != nil {
		fmt.Printf("failed to load oci config: %v\n", err)
		return
	}

	err = prepareRootfs(cfg, bundle)
	if err != nil {
		fmt.Printf("failed to prepare rootfs: %v\n", err)
		return
	}

	go raphael_server.RunServer(root, containerId)

	C.set_kvm_dir()

	kernelFilename := C.CString(cfg.Kernel)
	defer C.free(unsafe.Pointer(kernelFilename))

	gkvm_net.CreateBridge("br0")
	go gkvm_net.HandleNewInterafaces()

	ret := C.handle_kvm_command(kernelFilename)

	os.Exit(int(ret))
}
