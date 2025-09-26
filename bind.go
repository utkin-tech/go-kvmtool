package kvmtool

import "C"
import "github.com/utkin-tech/go-kvmtool/pkg/ociconfig"

//export get_rootfs_path
func get_rootfs_path() *C.char {
	return C.CString(ociconfig.RootfsPath)
}
