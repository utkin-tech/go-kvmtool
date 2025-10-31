package command

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/urfave/cli/v2"
	"github.com/utkin-tech/go-kvmtool/pkg/utils"
	"golang.org/x/term"
)

var Attach = cli.Command{
	Name:      "attach",
	Usage:     "attach to container",
	ArgsUsage: "<container-id>",
	Action: func(c *cli.Context) error {
		if c.NArg() < 1 {
			return fmt.Errorf("container-id is required")
		}
		root := c.String("root")
		containerId := c.Args().First()

		fmt.Printf("create: id=%s, root=%s\n", containerId, root)

		socketPath := utils.SocketPath(root, containerId)
		attach(socketPath)

		return nil
	},
}

const exitShortcut = 0x1d // Ctrl + ]

func unixDialer(path string) *websocket.Dialer {
	d := websocket.DefaultDialer
	d.NetDial = func(network, addr string) (net.Conn, error) {
		return net.Dial("unix", path)
	}
	d.HandshakeTimeout = 5 * time.Second
	return d
}

func attach(socketPath string) {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("failed to make raw: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	url := "ws://unix/attach"
	d := unixDialer(socketPath)

	conn, resp, err := d.Dial(url, http.Header{})
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
