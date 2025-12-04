package command

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli/v2"
	"github.com/utkin-tech/go-kvmtool/pkg/config"
	"github.com/utkin-tech/go-kvmtool/pkg/ociconfig"
)

var Create = cli.Command{
	Name:      "create",
	Usage:     "create a container",
	ArgsUsage: "<container-id>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "bundle",
			Aliases: []string{"b"},
			Usage:   "path to the OCI bundle",
			Value:   ".",
		},
	},
	Action: func(c *cli.Context) error {
		if c.NArg() < 1 {
			return fmt.Errorf("container-id is required")
		}
		root := c.String("root")
		containerId := c.Args().First()
		bundle := c.String("bundle")

		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable: %w", err)
		}

		_, err = config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		err = ociconfig.Load(bundle)
		if err != nil {
			return fmt.Errorf("failed to load oci config: %w", err)
		}

		cmd := exec.Command(exe, "--root", root, "init", "--bundle", bundle, containerId)

		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid:  true,
			Setctty: false,
		}

		if ociconfig.HasNamespace(specs.NetworkNamespace) {
			cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWNET | syscall.CLONE_NEWNS
		}

		cmd.Dir = bundle

		logFileName := fmt.Sprintf("/tmp/daemon-%s.log", containerId)
		logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		defer logFile.Close()

		cmd.Stdout = logFile
		cmd.Stderr = logFile
		cmd.Stdin = nil

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start init process: %w", err)
		}

		fmt.Printf("Init started with PID: %d\n", cmd.Process.Pid)
		return nil
	},
}
