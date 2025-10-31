package main

import (
	"github.com/containerd/containerd/runtime/v2/shim"
	gkvm_shim "github.com/utkin-tech/go-kvmtool/pkg/shim"
)

func main() {
	// init and execute the shim
	shim.Run("io.containerd.example.v1", gkvm_shim.New)
}
