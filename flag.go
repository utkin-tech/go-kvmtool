package main

import (
	"flag"
	"fmt"
)

type Config struct {
	guest_name        string
	nrcpus            int
	balloon           bool
	vnc               bool
	gtk               bool
	sdl               bool
	virtio_rng        bool
	nodefaults        bool
	console           string
	vsock_cid         uint64
	dev               string
	sandbox           string
	hugetlbfs_path    string
	virtio_transport  string
	kernel_filename   string
	initrd_filename   string
	kernel_cmdline    string
	firmware_filename string
	flash_filename    string
	no_dhcp           bool
	single_step       bool
	ioport_debug      bool
	mmio_debug        bool
	debug_iodelay     int
}

func BuildOptions(cfg *Config) {
	// Basic options
	flag.StringVar(&cfg.guest_name, "name", "", "A name for the guest")
	flag.IntVar(&cfg.nrcpus, "c", 0, "Number of CPUs")
	flag.IntVar(&cfg.nrcpus, "cpus", 0, "Number of CPUs")

	// For callbacks like mem_parser, we'd need to implement them separately
	flag.BoolVar(&cfg.balloon, "balloon", false, "Enable virtio balloon")
	flag.BoolVar(&cfg.vnc, "vnc", false, "Enable VNC framebuffer")
	flag.BoolVar(&cfg.gtk, "gtk", false, "Enable GTK framebuffer")
	flag.BoolVar(&cfg.sdl, "sdl", false, "Enable SDL framebuffer")
	flag.BoolVar(&cfg.virtio_rng, "rng", false, "Enable virtio Random Number Generator")
	flag.BoolVar(&cfg.nodefaults, "nodefaults", false, "Disable implicit configuration that cannot be disabled otherwise")
	flag.StringVar(&cfg.console, "console", "", "Console to use (serial, virtio or hv)")
	flag.Uint64Var(&cfg.vsock_cid, "vsock", 0, "Guest virtio socket CID")
	flag.StringVar(&cfg.dev, "dev", "", "KVM device file")
	flag.StringVar(&cfg.sandbox, "sandbox", "", "Run this script when booting into custom rootfs")
	flag.StringVar(&cfg.hugetlbfs_path, "hugetlbfs", "", "Hugetlbfs path")
	flag.StringVar(&cfg.virtio_transport, "virtio-transport", "", "Type of virtio transport")
	flag.Bool("virtio-legacy", false, "Use legacy virtio transport (Deprecated: Use --virtio-transport option instead)")

	// Kernel options
	flag.StringVar(&cfg.kernel_filename, "k", "", "Kernel to boot in virtual machine")
	flag.StringVar(&cfg.kernel_filename, "kernel", "", "Kernel to boot in virtual machine")
	flag.StringVar(&cfg.initrd_filename, "i", "", "Initial RAM disk image")
	flag.StringVar(&cfg.initrd_filename, "initrd", "", "Initial RAM disk image")
	flag.StringVar(&cfg.kernel_cmdline, "p", "", "Kernel command line arguments")
	flag.StringVar(&cfg.kernel_cmdline, "params", "", "Kernel command line arguments")
	flag.StringVar(&cfg.firmware_filename, "f", "", "Firmware image to boot in virtual machine")
	flag.StringVar(&cfg.firmware_filename, "firmware", "", "Firmware image to boot in virtual machine")
	flag.StringVar(&cfg.flash_filename, "F", "", "Flash image to present to virtual machine")
	flag.StringVar(&cfg.flash_filename, "flash", "", "Flash image to present to virtual machine")

	// Networking options
	flag.BoolVar(&cfg.no_dhcp, "no-dhcp", false, "Disable kernel DHCP in rootfs mode")

	// Debug options
	flag.BoolVar(&cfg.single_step, "debug-single-step", false, "Enable single stepping")
	flag.BoolVar(&cfg.ioport_debug, "debug-ioport", false, "Enable ioport debugging")
	flag.BoolVar(&cfg.mmio_debug, "debug-mmio", false, "Enable MMIO debugging")
	flag.IntVar(&cfg.debug_iodelay, "debug-iodelay", 0, "Delay IO by millisecond")
}

// Example callback functions (would need to be implemented)
func memParser(value string) error {
	// Implement memory parsing logic
	return nil
}

func imgNameParser(value string) error {
	// Implement image name parsing logic
	return nil
}

func parse() {
	cfg := &Config{}

	BuildOptions(cfg)

	flag.Parse()

	// Rest of your program
	fmt.Printf("Config: %+v\n", cfg)
}
