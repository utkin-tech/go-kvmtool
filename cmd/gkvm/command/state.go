package command

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v2"
	raphael_api "github.com/utkin-tech/go-kvmtool/pkg/api/raphael"
	"github.com/utkin-tech/go-kvmtool/pkg/utils"
)

var State = cli.Command{
	Name:      "state",
	Usage:     "show the state of a container",
	ArgsUsage: "<container-id>",
	Action: func(c *cli.Context) error {
		if c.NArg() < 1 {
			return fmt.Errorf("container-id is required")
		}
		root := c.String("root")
		containerId := c.Args().First()

		socketPath := utils.SocketPath(root, containerId)
		client := utils.UnixHttpClient(socketPath)

		url := "http://raphael/state"
		resp, err := client.Get(url)
		if err != nil {
			if resp != nil {
				log.Fatalf("dial failed: %v (http status: %s)", err, resp.Status)
			}
			log.Fatalf("dial failed: %v", err)
		}

		var state raphael_api.StateResponse

		if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
			panic(err)
		}
		data, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			return err
		}
		os.Stdout.Write(data)
		return nil
	},
}
