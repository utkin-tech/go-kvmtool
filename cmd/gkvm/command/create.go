package command

import (
	"fmt"

	"github.com/urfave/cli/v2"
	"github.com/utkin-tech/go-kvmtool"
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
		if c.NArg() < 1 {
			return fmt.Errorf("container-id is required")
		}
		root := c.String("root")
		containerId := c.Args().First()
		bundle := c.String("bundle")
		pidFile := c.String("pid-file")
		consoleSocket := c.String("console-socket")

		fmt.Printf("create: id=%s bundle=%s pidFile=%s consoleSocket=%s\n",
			containerId, bundle, pidFile, consoleSocket)

		kvmtool.Run(bundle, root, containerId)
		return nil
	},
}
