package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/hashicorp/yamux"
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"

	"github.com/utkin-tech/go-kvmtool/pkg/api/leonardo"
	guest_mux "github.com/utkin-tech/go-kvmtool/pkg/mux"
)

func runProcessCmd(cmd *exec.Cmd, port uint32) error {
	portName := fmt.Sprintf("/dev/vport1p%d", port)

	portFile, err := os.OpenFile(portName, os.O_RDWR, 0600)
	if err != nil {
		log.Printf("failed to open file: %v\n", err)
		return err
	}
	defer portFile.Close()

	time.Sleep(100 * time.Millisecond)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	defer ptmx.Close()

	go func() {
		_, err := io.Copy(ptmx, portFile)
		log.Printf("failed copy ptmx to portFile: %v\n", err)
	}()

	go func() {
		_, err := io.Copy(portFile, ptmx)
		log.Printf("failed copy portFile to ptmx: %v\n", err)
	}()

	return cmd.Wait()
}

func runProcess(command string, port uint32) error {
	cmd := exec.Command(command)
	return runProcessCmd(cmd, port)
}

func doMounts() {
	_ = unix.Mount("sysfs", "/sys", "sysfs", 0, "")
	_ = unix.Mount("proc", "/proc", "proc", 0, "")
	_ = unix.Mount("devtmpfs", "/dev", "devtmpfs", 0, "")
	_ = os.MkdirAll("/dev/pts", 0755)
	_ = unix.Mount("devpts", "/dev/pts", "devpts", 0, "")
}

func main() {
	fmt.Println("Mounting...")

	doMounts()

	leonardoStartSignal := make(chan leonardo.StartRequest)
	leonardoServerReadySignal := make(chan bool)

	f, err := os.OpenFile("/dev/vport1p0", os.O_RDWR, 0600)
	if err != nil {
		log.Printf("failed to open file: %v\n", err)
		panic(err)
	}
	defer f.Close()

	// wait until host will be in connected state https://github.com/torvalds/linux/blob/master/drivers/char/virtio_console.c#L747
	time.Sleep(100 * time.Millisecond)

	session, err := yamux.Server(f, nil)
	if err != nil {
		log.Panicf("failed to create yamux server: %v", err)
	}
	defer session.Close()

	dialer, err := guest_mux.NewMuxDialer(session)
	if err != nil {
		log.Printf("Failed to create mux dialer: %v", err)
		return
	}
	defer dialer.Close()

	transport := &http.Transport{
		DialContext: dialer.DialContext,
	}

	client := &http.Client{
		Transport: transport,
	}

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/exec", func(w http.ResponseWriter, r *http.Request) {
			var req leonardo.ExecRequest

			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				panic(err)
			}

			runProcess(req.Command, req.Port)
		})
		mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
			var req leonardo.StartRequest

			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				panic(err)
			}

			leonardoStartSignal <- req
			close(leonardoStartSignal)

			select {}
		})

		server := &http.Server{Handler: mux}
		fmt.Println("HTTP server starting on mux...")

		close(leonardoServerReadySignal)
		if err := server.Serve(session); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	if _, err := unix.Setsid(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: setsid failed: %v\n", err)
	}

	spec, err := loadSpec()
	if err != nil {
		panic(err)
	}

	for _, env := range spec.Process.Env {
		splittedEnv := strings.SplitN(env, "=", 2)
		envKey := splittedEnv[0]
		envValue := splittedEnv[1]
		os.Setenv(envKey, envValue)
	}

	cmd := exec.Command(spec.Process.Args[0])
	cmd.Dir = spec.Process.Cwd

	<-leonardoServerReadySignal

	_, err = client.Post("http://donatello/ready", "application/json; charset=utf-8", nil)

	if err != nil {
		log.Printf("donatello ready request error: %v", err)
		return
	}

	startReq := <-leonardoStartSignal

	runErr := runProcessCmd(cmd, startReq.Port)

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

func loadSpec() (*specs.Spec, error) {
	configPath := filepath.Join("/virt/config.json")
	var spec *specs.Spec

	f, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", configPath, err)
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&spec); err != nil {
		return nil, fmt.Errorf("failed to decode config.json: %w", err)
	}

	return spec, nil
}
