//go:build linux

package example

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/containerd/api/events"
	taskAPI "github.com/containerd/containerd/api/runtime/task/v2"
	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/pkg/schedcore"
	"github.com/containerd/containerd/protobuf"
	ptypes "github.com/containerd/containerd/protobuf/types"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/runtime/v2/shim"
	"github.com/containerd/containerd/sys/reaper"
	"github.com/containerd/fifo"
	"github.com/containerd/go-runc"
	"github.com/containerd/log"
	"github.com/gorilla/websocket"
	"github.com/utkin-tech/go-kvmtool/pkg/ociconfig"
	"github.com/utkin-tech/go-kvmtool/pkg/utils"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	// check to make sure the *service implements the GRPC API
	_ = (taskAPI.TaskService)(&service{})
)

// New returns a new shim service
func New(ctx context.Context, id string, publisher shim.Publisher, shutdown func()) (shim.Shim, error) {
	s := &service{
		ec:      reaper.Default.Subscribe(),
		events:  make(chan interface{}, 128),
		context: ctx,
	}
	go s.processExits(ctx)
	go s.forward(ctx, publisher)
	return s, nil
}

type service struct {
	childPid uint32
	id       string
	bundle   string

	stdin  string
	stdout string
	stderr string

	ec chan runc.Exit

	eventSendMu sync.Mutex
	events      chan interface{}

	context context.Context
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
	pid := os.Getpid()
	ppid := os.Getppid()

	log.G(ctx).Infof("Create: application stars with args: %v, pid: %d, ppid: %d", os.Args, pid, ppid)

	bundle := r.Bundle
	containerId := r.ID

	err = ociconfig.Load(bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to load oci config: %w", err)
	}

	cmd := exec.Command("/home/user/go-kvmtool/bin/gkvm", "init", "--bundle", bundle, containerId)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNET | syscall.CLONE_NEWNS,
	}

	cmd.Dir = bundle

	logFileName := filepath.Join("/tmp", "log.json")
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil

	s.stdin = r.Stdin
	s.stdout = r.Stdout
	s.stderr = r.Stderr
	log.G(ctx).Infof("Create: stdin: %s, stdout: %s, stderr: %s\n", s.stdin, s.stdout, s.stderr)

	err = cmd.Start()
	if err != nil {
		fmt.Printf("Error starting process: %v\n", err)
		fmt.Fprintf(logFile, "Error starting process: %v\n", err)
		return
	}

	// Get the PID
	s.childPid = uint32(cmd.Process.Pid)
	log.G(ctx).Infof("Create: Child process PID: %d\n", s.childPid)

	s.id = r.ID
	s.bundle = r.Bundle

	return &taskAPI.CreateTaskResponse{
		Pid: s.childPid,
	}, nil
}

// Start the primary user process inside the container
func (s *service) Start(ctx context.Context, r *taskAPI.StartRequest) (*taskAPI.StartResponse, error) {
	pid := os.Getpid()
	ppid := os.Getppid()

	log.G(ctx).Infof("Start: application stars with args: %v, pid: %d, ppid: %d", os.Args, pid, ppid)
	log.G(ctx).Infof("Start: Child process PID: %d\n", s.childPid)

	fifoStdin, err := fifo.OpenFifo(ctx, s.stdin, unix.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	fifoStdout, err := fifo.OpenFifo(ctx, s.stdout, unix.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}

	socketPath := utils.SocketPath("/run/gkvm", r.ID)
	d := utils.UnixWebsocketDialer(socketPath)

	url := "ws://raphael/start"
	conn, resp, err := d.Dial(url, nil)
	if err != nil {
		if resp != nil {
			log.G(ctx).Errorf("dial failed: %v (http status: %s)", err, resp.Status)
			return nil, err
		}
	}

	go func() {
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if mt == websocket.BinaryMessage || mt == websocket.TextMessage {
				_, _ = fifoStdout.Write(msg)
			}
		}
	}()

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := fifoStdin.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.G(ctx).Errorf("stdin read: %v", err)
				}
				break
			}
			if n > 0 {
				if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
					log.G(ctx).Errorf("ws write error: %v", err)
					break
				}
			}
		}
	}()

	return &taskAPI.StartResponse{
		Pid: s.childPid,
	}, nil
}

// Delete a process or container
func (s *service) Delete(ctx context.Context, r *taskAPI.DeleteRequest) (*taskAPI.DeleteResponse, error) {
	pid := os.Getpid()
	ppid := os.Getppid()

	log.G(ctx).Infof("Delete: application stars with args: %v, pid: %d, ppid: %d", os.Args, pid, ppid)

	return nil, errdefs.ErrNotImplemented
}

// Exec an additional process inside the container
func (s *service) Exec(ctx context.Context, r *taskAPI.ExecProcessRequest) (*ptypes.Empty, error) {
	pid := os.Getpid()
	ppid := os.Getppid()

	log.G(ctx).Infof("Exec: application stars with args: %v, pid: %d, ppid: %d", os.Args, pid, ppid)

	return nil, errdefs.ErrNotImplemented
}

// ResizePty of a process
func (s *service) ResizePty(ctx context.Context, r *taskAPI.ResizePtyRequest) (*ptypes.Empty, error) {
	pid := os.Getpid()
	ppid := os.Getppid()

	log.G(ctx).Infof("ResizePty: application stars with args: %v, pid: %d, ppid: %d", os.Args, pid, ppid)

	return &emptypb.Empty{}, nil
}

// State returns runtime state of a process
func (s *service) State(ctx context.Context, r *taskAPI.StateRequest) (*taskAPI.StateResponse, error) {
	pid := os.Getpid()
	ppid := os.Getppid()

	log.G(ctx).Infof("State: application stars with args: %v, pid: %d, ppid: %d", os.Args, pid, ppid)

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
	log.G(ctx).Info("Pause: ok")

	return nil, errdefs.ErrNotImplemented
}

// Resume the container
func (s *service) Resume(ctx context.Context, r *taskAPI.ResumeRequest) (*ptypes.Empty, error) {
	log.G(ctx).Info("Resume: ok")

	return nil, errdefs.ErrNotImplemented
}

// Kill a process
func (s *service) Kill(ctx context.Context, r *taskAPI.KillRequest) (*ptypes.Empty, error) {
	log.G(ctx).Infof("Kill: %d", r.Signal)

	return &emptypb.Empty{}, nil
}

// Pids returns all pids inside the container
func (s *service) Pids(ctx context.Context, r *taskAPI.PidsRequest) (*taskAPI.PidsResponse, error) {
	log.G(ctx).Info("Pids: ok")

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
	log.G(ctx).Info("CloseIO: ok")

	return nil, errdefs.ErrNotImplemented
}

// Checkpoint the container
func (s *service) Checkpoint(ctx context.Context, r *taskAPI.CheckpointTaskRequest) (*ptypes.Empty, error) {
	log.G(ctx).Info("Checkpoint: ok")

	return nil, errdefs.ErrNotImplemented
}

// Connect returns shim information of the underlying service
func (s *service) Connect(ctx context.Context, r *taskAPI.ConnectRequest) (*taskAPI.ConnectResponse, error) {
	log.G(ctx).Info("Connect: ok")

	return &taskAPI.ConnectResponse{
		ShimPid: uint32(os.Getpid()),
		TaskPid: s.childPid,
	}, nil
}

// Shutdown is called after the underlying resources of the shim are cleaned up and the service can be stopped
func (s *service) Shutdown(ctx context.Context, r *taskAPI.ShutdownRequest) (*ptypes.Empty, error) {
	log.G(ctx).Info("Shutdown: ok")

	return &ptypes.Empty{}, nil
}

// Stats returns container level system stats for a container and its processes
func (s *service) Stats(ctx context.Context, r *taskAPI.StatsRequest) (*taskAPI.StatsResponse, error) {
	pid := os.Getpid()
	ppid := os.Getppid()

	log.G(ctx).Infof("Stats: application stars with args: %v, pid: %d, ppid: %d", os.Args, pid, ppid)

	return nil, errdefs.ErrNotImplemented
}

// Update the live container
func (s *service) Update(ctx context.Context, r *taskAPI.UpdateTaskRequest) (*ptypes.Empty, error) {
	return nil, errdefs.ErrNotImplemented
}

// Wait for a process to exit
func (s *service) Wait(ctx context.Context, r *taskAPI.WaitRequest) (*taskAPI.WaitResponse, error) {
	return nil, errdefs.ErrNotImplemented
}

func (s *service) sendL(evt interface{}) {
	s.eventSendMu.Lock()
	s.events <- evt
	s.eventSendMu.Unlock()
}

func (s *service) processExits(ctx context.Context) {
	for e := range s.ec {
		s.checkProcesses(ctx, e)
	}
}

func (s *service) checkProcesses(ctx context.Context, e runc.Exit) {
	log.G(ctx).Infof("checkProcesses: %d: %#v", s.childPid, e)

	if int(s.childPid) == e.Pid {
		s.sendL(&events.TaskExit{
			ContainerID: s.id,
			ID:          s.id,
			Pid:         uint32(e.Pid),
			ExitStatus:  uint32(e.Status),
			ExitedAt:    protobuf.ToTimestamp(time.Now()),
		})
		return
	}

}

func (s *service) forward(ctx context.Context, publisher shim.Publisher) {
	for e := range s.events {
		err := publisher.Publish(ctx, getTopic(e), e)
		if err != nil {
			// Should not happen.
			panic(fmt.Errorf("post event: %w", err))
		}
	}
}

func getTopic(e any) string {
	switch e.(type) {
	case *events.TaskCreate:
		return runtime.TaskCreateEventTopic
	case *events.TaskStart:
		return runtime.TaskStartEventTopic
	case *events.TaskOOM:
		return runtime.TaskOOMEventTopic
	case *events.TaskExit:
		return runtime.TaskExitEventTopic
	case *events.TaskDelete:
		return runtime.TaskDeleteEventTopic
	case *events.TaskExecAdded:
		return runtime.TaskExecAddedEventTopic
	case *events.TaskExecStarted:
		return runtime.TaskExecStartedEventTopic
	default:
		log.L.Infof("no topic for type %#v", e)
	}
	return runtime.TaskUnknownTopic
}
