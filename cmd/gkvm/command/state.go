package command

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

var State = cli.Command{
	Name:      "state",
	Usage:     "show the state of a container",
	ArgsUsage: "<container-id>",
	Action: func(c *cli.Context) error {
		if c.NArg() < 1 {
			return fmt.Errorf("container-id is required")
		}
		containerID := c.Args().First()
		fmt.Printf("state: id=%s\n", containerID)

		root := c.String("root")
		fmt.Printf("state: root=%s\n", root)
		return nil
	},
}
