package command

import (
	"github.com/urfave/cli/v2"
)

var Init = cli.Command{
	Name: "init",
	Action: func(c *cli.Context) error {

		return nil
	},
}
