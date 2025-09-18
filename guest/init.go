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

func doMounts9p() {
	newRoot := "/mnt/rootfs"
	putOld := "old_root"

	if err := os.MkdirAll(newRoot, 0755); err != nil {
		fmt.Printf("mkdir rootfs: %v", err)
		return
	}

	if err := unix.Mount("/dev/root", newRoot, "9p", 0,
		"trans=virtio,version=9p2000.L,cache=loose"); err != nil {
		fmt.Printf("mount 9p rootfs: %v", err)
		return
	}

	if err := unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		fmt.Printf("mount MS_PRIVATE: %v", err)
		return
	}

	if err := unix.Mount(newRoot, newRoot, "", unix.MS_BIND, ""); err != nil {
		fmt.Printf("mount MS_BIND on %s: %v", newRoot, err)
		return
	}

	putOldPath := newRoot + "/" + putOld
	if err := os.MkdirAll(putOldPath, 0777); err != nil {
		fmt.Printf("mkdir %s: %v", putOldPath, err)
		return
	}

	if err := os.Chdir(newRoot); err != nil {
		fmt.Printf("chdir %s: %v", newRoot, err)
		return
	}

	if err := unix.PivotRoot(newRoot, putOldPath); err != nil {
		fmt.Printf("pivot_root %s -> %s: %v", newRoot, putOldPath, err)
		return
	}

	if err := os.Chdir("/"); err != nil {
		fmt.Printf("chdir /: %v", err)
		return
	}

	// if err := os.Chdir("/"); err != nil {
	//     return fmt.Errorf("chdir /: %w", err)
	// }

	// if err := unix.Unmount("/old_root", unix.MNT_DETACH); err != nil {
	//     return fmt.Errorf("unmount old_root: %w", err)
	// }
	// _ = os.RemoveAll("/old_root")
}

func doMounts() {
	_ = unix.Mount("sysfs", "/sys", "sysfs", 0, "")
	_ = unix.Mount("proc", "/proc", "proc", 0, "")
	_ = unix.Mount("devtmpfs", "/dev", "devtmpfs", 0, "")
	_ = os.MkdirAll("/dev/pts", 0755)
	_ = unix.Mount("devpts", "/dev/pts", "devpts", 0, "")
}

func doAgent() {
	go func() {
		f, err := os.OpenFile("/dev/vport1p0", os.O_RDWR, 0600)
		if err != nil {
			panic(err)
		}
		defer f.Close()

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
}

func main() {
	fmt.Println("Mounting...")

	doMounts9p()

	doMounts()

	doAgent()

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
