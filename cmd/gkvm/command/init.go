package command

import (
	"fmt"

	"github.com/urfave/cli/v2"
	"github.com/utkin-tech/go-kvmtool"
)

var Init = cli.Command{
	Name: "init",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "bundle",
			Aliases: []string{"b"},
			Usage:   "path to the OCI bundle",
			Value:   ".",
		},
	},
	Action: func(c *cli.Context) error {
		root := c.String("root")
		if c.NArg() < 1 {
			return fmt.Errorf("container-id is required")
		}
		containerId := c.Args().First()
		bundle := c.String("bundle")

		fmt.Printf("init: id=%s bundle=%s\n", containerId, bundle)

		kvmtool.Run(bundle, root, containerId)
		return nil
	},
}
