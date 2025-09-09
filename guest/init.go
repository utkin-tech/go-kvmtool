package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/utkin-tech/go-kvmtool/pkg/mux"
	"golang.org/x/sys/unix"
)

func runProcess(filename string) error {
	cmd := exec.Command(filename)
	cmd.Env = []string{"TERM=linux", "DISPLAY=192.168.33.1:0", "HOME=/virt/home"}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func doMounts() {
	_ = unix.Mount("hostfs", "/host", "9p", unix.MS_RDONLY, "trans=virtio,version=9p2000.L")

	_ = unix.Mount("sysfs", "/sys", "sysfs", 0, "")
	_ = unix.Mount("proc", "/proc", "proc", 0, "")
	_ = unix.Mount("devtmpfs", "/dev", "devtmpfs", 0, "")
	_ = os.MkdirAll("/dev/pts", 0755)
	_ = unix.Mount("devpts", "/dev/pts", "devpts", 0, "")
}

func main() {
	fmt.Println("Mounting...")

	doMounts()

	f, err := os.OpenFile("/dev/vport1p0", os.O_RDWR, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	go func() {
		listener, err := mux.NewMuxListener(f)
		if err != nil {
			log.Printf("Failed to create mux listener: %v", err)
			return
		}
		defer listener.Close()

		mux := http.NewServeMux()
		mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Hello from HTTP over yamux! Time: %s", time.Now().Format(time.RFC3339))
		})

		server := &http.Server{Handler: mux}
		fmt.Println("HTTP server starting on mux...")

		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	if _, err := unix.Setsid(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: setsid failed: %v\n", err)
	}

	runErr := runProcess("/bin/sh")

	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				fmt.Fprintf(os.Stderr, "child exited with status %d\n", status.ExitStatus())
			}
		} else {
			fmt.Fprintf(os.Stderr, "run error: %v\n", runErr)
		}
	}

	unix.Sync()
	if err := unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART); err != nil {
		fmt.Fprintf(os.Stderr, "reboot failed: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "Init failed: %v\n", runErr)
	os.Exit(1)
}
