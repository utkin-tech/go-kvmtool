package command

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

var Start = cli.Command{
	Name:      "start",
	Usage:     "start a created container",
	ArgsUsage: "<container-id>",
	Action: func(c *cli.Context) error {
		if c.NArg() < 1 {
			return fmt.Errorf("container-id is required")
		}
		containerID := c.Args().First()
		fmt.Printf("start: id=%s\n", containerID)
		return nil
	},
}
