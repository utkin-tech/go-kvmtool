package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
	"github.com/utkin-tech/go-kvmtool/cmd/gkvm/command"
)

func main() {
	app := &cli.App{
		Name:  "gkvm",
		Usage: "GKVM OCI runtime CLI",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "root",
				Usage: "root directory for storage of container state (this should be located in tmpfs)",
				Value: "/run/gkvm",
			},
		},
		Commands: []*cli.Command{
			&command.Create,
			&command.Run,
			&command.Start,
			&command.State,
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
