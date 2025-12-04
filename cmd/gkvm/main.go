package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/urfave/cli/v2"
	"github.com/utkin-tech/go-kvmtool/cmd/gkvm/command"
)

func main() {
	file, err := os.OpenFile("/tmp/gkvm.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	handler := slog.NewTextHandler(file, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	msg := fmt.Sprintf("application stars with args: %v", os.Args)
	logger.Info(msg)

	root := "/run/gkvm"

	app := &cli.App{
		Name:  "gkvm",
		Usage: "GKVM OCI runtime CLI",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "root",
				Usage: "root directory for storage of container state (this should be located in tmpfs)",
				Value: root,
			},
		},
		Commands: []*cli.Command{
			&command.Create,
			&command.Run,
			&command.Start,
			&command.State,
			&command.Init,
			&command.Attach,
			&command.Exec,
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
