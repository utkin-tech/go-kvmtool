package command

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/urfave/cli/v2"
	"github.com/utkin-tech/go-kvmtool/pkg/utils"
	"golang.org/x/term"
)

var Exec = cli.Command{
	Name:      "exec",
	Usage:     "exec to container",
	ArgsUsage: "<container-id>",
	Action: func(c *cli.Context) error {
		if c.NArg() < 1 {
			return fmt.Errorf("container-id is required")
		}
		root := c.String("root")
		containerId := c.Args().First()

		fmt.Printf("create: id=%s, root=%s\n", containerId, root)

		socketPath := utils.SocketPath(root, containerId)
		execFunc(socketPath)

		return nil
	},
}

func execFunc(socketPath string) {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("failed to make raw: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	d := utils.UnixWebsocketDialer(socketPath)

	url := "ws://raphael/exec"
	conn, resp, err := d.Dial(url, nil)
	if err != nil {
		if resp != nil {
			log.Fatalf("dial failed: %v (http status: %s)", err, resp.Status)
		}
		log.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	go func() {
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if mt == websocket.BinaryMessage || mt == websocket.TextMessage {
				_, _ = os.Stdout.Write(msg)
			}
		}
	}()

	buf := make([]byte, 4096)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Println("stdin read:", err)
			}
			break
		}
		if n == 1 && buf[0] == exitShortcut {
			fmt.Fprintln(os.Stderr, "\nExiting client...")
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "user exit"))
			time.Sleep(300 * time.Millisecond)
			break
		}
		if n > 0 {
			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				log.Println("write ws:", err)
				break
			}
		}
	}
}
