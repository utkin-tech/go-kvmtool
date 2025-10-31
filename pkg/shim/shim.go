//go:build linux

package example

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	goruntime "runtime"
	"syscall"
	"time"

	taskAPI "github.com/containerd/containerd/api/runtime/task/v2"
	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/pkg/schedcore"
	"github.com/containerd/containerd/protobuf"
	ptypes "github.com/containerd/containerd/protobuf/types"
	"github.com/containerd/containerd/runtime/v2/shim"
	"github.com/containerd/fifo"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/proto"
)

var (
	// check to make sure the *service implements the GRPC API
	_ = (taskAPI.TaskService)(&service{})
)

// New returns a new shim service
func New(ctx context.Context, id string, publisher shim.Publisher, shutdown func()) (shim.Shim, error) {
	return &service{}, nil
}

type service struct {
	childPid uint32
	id       string
	bundle   string

	stdin  string
	stdout string
	stderr string
}

func newCommand(ctx context.Context, id, containerdAddress, containerdTTRPCAddress string) (*exec.Cmd, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	args := []string{
		"-namespace", ns,
		"-id", id,
		"-address", containerdAddress,
	}
	cmd := exec.Command(self, args...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "GOMAXPROCS=2")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	return cmd, nil
}

func (s *service) StartShim(ctx context.Context, opts shim.StartOpts) (_ string, retErr error) {
	// start logger
	file, err := os.OpenFile("/tmp/gkvm.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	handler := slog.NewTextHandler(file, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	pid := os.Getpid()
	ppid := os.Getppid()

	msg := fmt.Sprintf("StartShim: application stars with args: %v, pid: %d, ppid: %d", os.Args, pid, ppid)
	logger.Info(msg)
	// end logger

	cmd, err := newCommand(ctx, opts.ID, opts.Address, opts.TTRPCAddress)
	if err != nil {
		return "", err
	}
	address, err := shim.SocketAddress(ctx, opts.Address, opts.ID)
	if err != nil {
		return "", err
	}
	socket, err := shim.NewSocket(address)
	if err != nil {
		if !shim.SocketEaddrinuse(err) {
			return "", err
		}
		if err := shim.RemoveSocket(address); err != nil {
			return "", fmt.Errorf("remove already used socket: %w", err)
		}
		if socket, err = shim.NewSocket(address); err != nil {
			return "", err
		}
	}
	defer func() {
		if retErr != nil {
			socket.Close()
			_ = shim.RemoveSocket(address)
		}
	}()
	// make sure that reexec shim-v2 binary use the value if need
	if err := shim.WriteAddress("address", address); err != nil {
		return "", err
	}

	f, err := socket.File()
	if err != nil {
		return "", err
	}

	cmd.ExtraFiles = append(cmd.ExtraFiles, f)

	goruntime.LockOSThread()
	if os.Getenv("SCHED_CORE") != "" {
		if err := schedcore.Create(schedcore.ProcessGroup); err != nil {
			return "", fmt.Errorf("enable sched core support: %w", err)
		}
	}

	if err := cmd.Start(); err != nil {
		f.Close()
		return "", err
	}
	goruntime.UnlockOSThread()

	defer func() {
		if retErr != nil {
			cmd.Process.Kill()
		}
	}()
	// make sure to wait after start
	go cmd.Wait()
	if err := shim.WritePidFile("shim.pid", cmd.Process.Pid); err != nil {
		return "", err
	}

	msg = fmt.Sprintf("StartShim: child PID: %d", cmd.Process.Pid)
	logger.Info(msg)

	if data, err := io.ReadAll(os.Stdin); err == nil {
		if len(data) > 0 {
			var any ptypes.Any
			if err := proto.Unmarshal(data, &any); err != nil {
				return "", err
			}
			// v, err := typeurl.UnmarshalAny(&any)
			// if err != nil {
			// 	return "", err
			// }
			// if opts, ok := v.(*options.Options); ok {
			// 	if opts.ShimCgroup != "" {
			// 		cg, err := cgroup1.Load(cgroup1.StaticPath(opts.ShimCgroup))
			// 		if err != nil {
			// 			return "", fmt.Errorf("failed to load cgroup %s: %w", opts.ShimCgroup, err)
			// 		}
			// 		if err := cg.AddProc(uint64(cmd.Process.Pid)); err != nil {
			// 			return "", fmt.Errorf("failed to join cgroup %s: %w", opts.ShimCgroup, err)
			// 		}
			// 	}
			// }
		}
	}
	if err := shim.AdjustOOMScore(cmd.Process.Pid); err != nil {
		return "", fmt.Errorf("failed to adjust OOM score for shim: %w", err)
	}
	return address, nil
}

// Cleanup is a binary call that cleans up any resources used by the shim when the service crashes
func (s *service) Cleanup(ctx context.Context) (*taskAPI.DeleteResponse, error) {
	return nil, errdefs.ErrNotImplemented
}

// Create a new container
func (s *service) Create(ctx context.Context, r *taskAPI.CreateTaskRequest) (_ *taskAPI.CreateTaskResponse, err error) {
	// start logger
	file, err := os.OpenFile("/tmp/gkvm.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	handler := slog.NewTextHandler(file, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	pid := os.Getpid()
	ppid := os.Getppid()

	msg := fmt.Sprintf("Create: application stars with args: %v, pid: %d, ppid: %d", os.Args, pid, ppid)
	logger.Info(msg)
	// end logger

	cmd := exec.Command("/bin/bash")

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNET,
	}

	s.stdin = r.Stdin
	s.stdout = r.Stdout
	s.stderr = r.Stderr
	msg = fmt.Sprintf("Create: stdin: %s, stdout: %s, stderr: %s\n", s.stdin, s.stdout, s.stderr)
	logger.Info(msg)

	fifoStdin, err := fifo.OpenFifo(ctx, s.stdin, unix.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	fifoStdout, err := fifo.OpenFifo(ctx, s.stdout, unix.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}

	fifoStderr, err := fifo.OpenFifo(ctx, s.stdout, unix.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}

	cmd.Stdin = fifoStdin
	cmd.Stdout = fifoStdout
	cmd.Stderr = fifoStderr

	err = cmd.Start()
	if err != nil {
		fmt.Printf("Error starting process: %v\n", err)
		return
	}

	// Get the PID
	s.childPid = uint32(cmd.Process.Pid)
	msg = fmt.Sprintf("Create: Child process PID: %d\n", s.childPid)
	logger.Info(msg)

	// fifoStdin, err := fifo.OpenFifo(ctx, s.stdin, unix.O_RDONLY, 0)
	// if err != nil {
	// 	return nil, err
	// }

	s.id = r.ID
	s.bundle = r.Bundle

	return &taskAPI.CreateTaskResponse{
		Pid: s.childPid,
	}, nil
}

// Start the primary user process inside the container
func (s *service) Start(ctx context.Context, r *taskAPI.StartRequest) (*taskAPI.StartResponse, error) {
	// start logger
	file, err := os.OpenFile("/tmp/gkvm.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	handler := slog.NewTextHandler(file, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	pid := os.Getpid()
	ppid := os.Getppid()

	msg := fmt.Sprintf("Start: application stars with args: %v, pid: %d, ppid: %d", os.Args, pid, ppid)
	logger.Info(msg)
	// end logger

	msg = fmt.Sprintf("Start: Child process PID: %d\n", s.childPid)
	logger.Info(msg)

	return &taskAPI.StartResponse{
		Pid: s.childPid,
	}, nil
}

// Delete a process or container
func (s *service) Delete(ctx context.Context, r *taskAPI.DeleteRequest) (*taskAPI.DeleteResponse, error) {
	// start logger
	file, err := os.OpenFile("/tmp/gkvm.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	handler := slog.NewTextHandler(file, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	pid := os.Getpid()
	ppid := os.Getppid()

	msg := fmt.Sprintf("Delete: application stars with args: %v, pid: %d, ppid: %d", os.Args, pid, ppid)
	logger.Info(msg)
	// end logger

	return nil, errdefs.ErrNotImplemented
}

// Exec an additional process inside the container
func (s *service) Exec(ctx context.Context, r *taskAPI.ExecProcessRequest) (*ptypes.Empty, error) {
	// start logger
	file, err := os.OpenFile("/tmp/gkvm.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	handler := slog.NewTextHandler(file, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	pid := os.Getpid()
	ppid := os.Getppid()

	msg := fmt.Sprintf("Exec: application stars with args: %v, pid: %d, ppid: %d", os.Args, pid, ppid)
	logger.Info(msg)
	// end logger

	return nil, errdefs.ErrNotImplemented
}

// ResizePty of a process
func (s *service) ResizePty(ctx context.Context, r *taskAPI.ResizePtyRequest) (*ptypes.Empty, error) {
	// start logger
	file, err := os.OpenFile("/tmp/gkvm.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	handler := slog.NewTextHandler(file, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	pid := os.Getpid()
	ppid := os.Getppid()

	msg := fmt.Sprintf("ResizePty: application stars with args: %v, pid: %d, ppid: %d", os.Args, pid, ppid)
	logger.Info(msg)
	// end logger

	return nil, errdefs.ErrNotImplemented
}

// State returns runtime state of a process
func (s *service) State(ctx context.Context, r *taskAPI.StateRequest) (*taskAPI.StateResponse, error) {
	// start logger
	file, err := os.OpenFile("/tmp/gkvm.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	handler := slog.NewTextHandler(file, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	pid := os.Getpid()
	ppid := os.Getppid()

	msg := fmt.Sprintf("State: application stars with args: %v, pid: %d, ppid: %d", os.Args, pid, ppid)
	logger.Info(msg)
	// end logger

	return &taskAPI.StateResponse{
		ID:         s.id,
		Bundle:     s.bundle,
		Pid:        s.childPid,
		Status:     task.Status_RUNNING,
		Stdin:      "",
		Stdout:     "",
		Stderr:     "",
		Terminal:   false,
		ExitStatus: 0,
		ExitedAt:   protobuf.ToTimestamp(time.Now()),
	}, nil
}

// Pause the container
func (s *service) Pause(ctx context.Context, r *taskAPI.PauseRequest) (*ptypes.Empty, error) {
	return nil, errdefs.ErrNotImplemented
}

// Resume the container
func (s *service) Resume(ctx context.Context, r *taskAPI.ResumeRequest) (*ptypes.Empty, error) {
	return nil, errdefs.ErrNotImplemented
}

// Kill a process
func (s *service) Kill(ctx context.Context, r *taskAPI.KillRequest) (*ptypes.Empty, error) {
	return nil, errdefs.ErrNotImplemented
}

// Pids returns all pids inside the container
func (s *service) Pids(ctx context.Context, r *taskAPI.PidsRequest) (*taskAPI.PidsResponse, error) {
	return &taskAPI.PidsResponse{
		Processes: []*task.ProcessInfo{
			{
				Pid: s.childPid,
			},
		},
	}, nil
}

// CloseIO of a process
func (s *service) CloseIO(ctx context.Context, r *taskAPI.CloseIORequest) (*ptypes.Empty, error) {
	return nil, errdefs.ErrNotImplemented
}

// Checkpoint the container
func (s *service) Checkpoint(ctx context.Context, r *taskAPI.CheckpointTaskRequest) (*ptypes.Empty, error) {
	return nil, errdefs.ErrNotImplemented
}

// Connect returns shim information of the underlying service
func (s *service) Connect(ctx context.Context, r *taskAPI.ConnectRequest) (*taskAPI.ConnectResponse, error) {
	return &taskAPI.ConnectResponse{
		ShimPid: uint32(os.Getpid()),
		TaskPid: s.childPid,
	}, nil
}

// Shutdown is called after the underlying resources of the shim are cleaned up and the service can be stopped
func (s *service) Shutdown(ctx context.Context, r *taskAPI.ShutdownRequest) (*ptypes.Empty, error) {
	os.Exit(0)
	return &ptypes.Empty{}, nil
}

// Stats returns container level system stats for a container and its processes
func (s *service) Stats(ctx context.Context, r *taskAPI.StatsRequest) (*taskAPI.StatsResponse, error) {
	// start logger
	file, err := os.OpenFile("/tmp/gkvm.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	handler := slog.NewTextHandler(file, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	pid := os.Getpid()
	ppid := os.Getppid()

	msg := fmt.Sprintf("Stats: application stars with args: %v, pid: %d, ppid: %d", os.Args, pid, ppid)
	logger.Info(msg)
	// end logger

	return nil, errdefs.ErrNotImplemented
}

// Update the live container
func (s *service) Update(ctx context.Context, r *taskAPI.UpdateTaskRequest) (*ptypes.Empty, error) {
	return nil, errdefs.ErrNotImplemented
}

// Wait for a process to exit
func (s *service) Wait(ctx context.Context, r *taskAPI.WaitRequest) (*taskAPI.WaitResponse, error) {
	// start logger
	file, err := os.OpenFile("/tmp/gkvm.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	handler := slog.NewTextHandler(file, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	pid := os.Getpid()
	ppid := os.Getppid()

	msg := fmt.Sprintf("Wait: application stars with args: %v, pid: %d, ppid: %d", os.Args, pid, ppid)
	logger.Info(msg)
	// end logger

	return nil, errdefs.ErrNotImplemented
}
