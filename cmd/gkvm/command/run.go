package command

import (
	"fmt"

	"github.com/urfave/cli/v2"
	"github.com/utkin-tech/go-kvmtool"
)

var Run = cli.Command{
	Name:      "run",
	Usage:     "create and run a container",
	ArgsUsage: "<container-id>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "bundle",
			Aliases: []string{"b"},
			Usage:   "path to the OCI bundle",
			Value:   ".",
		},
		&cli.StringFlag{
			Name:  "pid-file",
			Usage: "write the container's pid to the file",
		},
		&cli.StringFlag{
			Name:  "console-socket",
			Usage: "path to an AF_UNIX socket for the console",
		},
	},
	Action: func(c *cli.Context) error {
		root := c.String("root")
		if c.NArg() < 1 {
			return fmt.Errorf("container-id is required")
		}
		containerId := c.Args().First()
		bundle := c.String("bundle")
		pidFile := c.String("pid-file")
		consoleSocket := c.String("console-socket")

		fmt.Printf("run: id=%s bundle=%s pidFile=%s consoleSocket=%s\n",
			containerId, bundle, pidFile, consoleSocket)

		kvmtool.Run(bundle, root, containerId)

		return nil
	},
}
