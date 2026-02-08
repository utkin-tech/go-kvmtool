package kvmtool

import (
	"errors"
	"strings"
)

type Console int

const (
	Console8250   Console = 1
	ConsoleVirtio Console = 2
	ConsoleHv     Console = 3
)

const (
	ConsoleStrVirtio = "virtio"
	ConsoleStrSerial = "serial"
	ConsoleStrHv     = "hv"
)

const DefaultConsole = ConsoleStrSerial

type KvmConfig struct {
	RamSize       uint64
	KernelCmdline *string
	Nodefaults    bool
	ActiveConsole Console
	Console       *string
	Network       *string
	HostIp        *string
	GuestIp       *string
	GuestMac      *string
	HostMac       *string
	Script        *string
	RealCmdline   *string
	Vnc           bool
	Gtk           bool
	Sdl           bool
	Balloon       bool
	UsingRootfs   bool
	CustomRootfs  bool
}

func (k *KvmConfig) ParseConsole() error {
	console := DefaultConsole
	if c := k.Console; c != nil {
		console = *c
	}

	if console == "virtio" {
		k.ActiveConsole = ConsoleVirtio
	} else if console == "serial" {
		k.ActiveConsole = Console8250
	} else if console == "hv" {
		k.ActiveConsole = ConsoleHv
	} else {
		return errors.New("No console!")
	}
	return nil
}

var DefaultHostAddr = "192.168.33.1"
var DefaultHostMac = "02:01:01:01:01:01"

var DefaultGuestAddr = "192.168.33.15"
var DefaultGuestMac = "02:15:15:15:15:15"

var DefaultNetwork = "user"
var DefaultScript = "none"

func (k *KvmConfig) ParseNetwork() error {
	if k.HostIp == nil {
		k.HostIp = &DefaultHostAddr
	}

	if k.GuestIp == nil {
		k.GuestIp = &DefaultGuestAddr
	}

	if k.HostMac == nil {
		k.HostMac = &DefaultHostMac
	}

	if k.GuestMac == nil {
		k.GuestMac = &DefaultGuestMac
	}

	if k.Script == nil {
		k.Script = &DefaultScript
	}

	if k.Network == nil {
		k.Network = &DefaultNetwork
	}

	return nil
}

func (k *KvmConfig) ParseCmdline() {
	if k.Nodefaults {
		k.RealCmdline = k.KernelCmdline
	} else {
		k.SetRealCmdline()
	}
}

func (k *KvmConfig) SetRealCmdline() {
	var realCmdline string
	var video bool

	video = k.Vnc || k.Sdl || k.Gtk
	realCmdline += ArchSetCmdline(video)

	if video {
		realCmdline += " console=tty0"
	} else {
		switch k.ActiveConsole {
		case ConsoleHv:
		case ConsoleVirtio:
			realCmdline += " console=hvc0"
		case Console8250:
			realCmdline += " console=ttyS0"
		}
	}

	if k.UsingRootfs {
		realCmdline += " rw rootflags=trans=virtio,version=9p2000.L,cache=loose rootfstype=9p"
		if k.CustomRootfs {
			realCmdline += " init=/virt/init"
		}
	} else if k.KernelCmdline == nil || !strings.Contains(*k.KernelCmdline, "root=") {
		realCmdline += " root=/dev/vda rw"
	}

	if k.KernelCmdline != nil {
		realCmdline += " "
		realCmdline += *k.KernelCmdline
	}

	k.RealCmdline = &realCmdline
}

// for kvm__arch_set_cmdline
func ArchSetCmdline(video bool) string {
	cmdline := "noapic noacpi pci=conf1 reboot=k panic=1 i8042.direct=1 i8042.dumbkbd=1 i8042.nopnp=1"
	if video {
		cmdline += " video=vesafb"
	} else {
		cmdline += " earlyprintk=serial i8042.noaux=1"
	}
	return cmdline
}
