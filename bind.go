package kvmtool

import "C"
import "github.com/utkin-tech/go-kvmtool/pkg/ociconfig"

//export g_get_rootfs_path
func g_get_rootfs_path() *C.char {
	rootfsPath := ociconfig.Spec.Root.Path
	return C.CString(rootfsPath)
}

//export g_get_ram_size
func g_get_ram_size() uint64 {
	memory := ociconfig.Spec.Linux.Resources.Memory
	if memory == nil {
		return 0
	}

	limit := memory.Limit
	if memory == nil {
		return 0
	}

	return uint64(*limit)
}

//export g_get_nrcpus
func g_get_nrcpus() int {
	cpu := ociconfig.Spec.Linux.Resources.CPU
	if cpu == nil {
		return 0
	}

	period := cpu.Period
	if period == nil {
		return 0
	}

	quota := cpu.Quota
	if quota == nil {
		return 0
	}

	nrcpus := int(*quota / int64(*period))

	return nrcpus
}
