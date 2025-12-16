package net

import (
	"bytes"
	"encoding/json"
	"log"
	"net"
	"sync"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/utkin-tech/go-kvmtool/internal/state"
	"github.com/utkin-tech/go-kvmtool/pkg/api/leonardo"
	raphael_server "github.com/utkin-tech/go-kvmtool/pkg/server/raphael"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

type Interfaces struct {
	mu       sync.Mutex
	eth0init bool
	eth0addr *net.IPNet
	tap0init bool
	br0init  bool
	gw       net.IP
}

func HandleNewInterafaces() {
	ch := make(chan netlink.LinkUpdate)
	done := make(chan struct{})
	interfaces := Interfaces{}

	err := netlink.LinkSubscribe(ch, done)
	if err != nil {
		log.Fatalf("failed subscribe on events: %v", err)
		return
	}

	for update := range ch {
		handleLinkUpdate(update, &interfaces)
	}
}

func handleLinkUpdate(update netlink.LinkUpdate, interfaces *Interfaces) {
	interfaces.mu.Lock()
	defer interfaces.mu.Unlock()

	if update.Header.Type == unix.RTM_NEWLINK {
		if update.Link.Attrs().OperState == netlink.OperUp ||
			(update.Link.Attrs().Flags&net.FlagUp) != 0 {

			if update.Attrs().Name == "eth0" {
				initializeEth0(interfaces)
			}

			if update.Attrs().Name == "tap0" {
				initializeTap0(interfaces)
			}

			if !interfaces.br0init && interfaces.eth0init && interfaces.tap0init {
				initializeBr0(interfaces)

				go func() {
					stateObs := state.Observable.Subscribe()
					for {
						state := <-stateObs
						if state == specs.StateCreated {
							break
						}
					}

					req := leonardo.NetRequest{
						Addr: interfaces.eth0addr.String(),
						Gw:   interfaces.gw.String(),
					}

					b, err := json.Marshal(req)
					if err != nil {
						log.Fatalf("Failed to Serialize to JSON from native Go struct type: %v", err)
					}

					reqBody := bytes.NewBuffer(b)

					if raphael_server.LeonardoClient != nil {
						_, err = raphael_server.LeonardoClient.Post("http://leonardo/net", "application/json; charset=utf-8", reqBody)
					}

					if err != nil {
						log.Printf("HTTP request error: %v", err)
						return
					}
				}()
			}
		}
	}
}

func initializeEth0(interfaces *Interfaces) {
	if interfaces.eth0init {
		return
	}

	addrs, err := getInterfaceIPs("eth0")
	if err != nil {
		return
	}

	var ip4Addr net.IP = nil
	var ipAddr *net.IPNet = nil

	for _, a := range addrs {
		var ok bool
		ipAddr, ok = a.(*net.IPNet)
		if !ok {
			continue
		}
		ip4Addr = ipAddr.IP.To4()
		if ip4Addr != nil {
			break
		}
	}

	if ip4Addr == nil {
		return
	}

	interfaces.eth0addr = ipAddr
	interfaces.eth0init = true
}

func initializeTap0(interfaces *Interfaces) {
	if interfaces.tap0init {
		return
	}

	interfaces.tap0init = true
}

func initializeBr0(interfaces *Interfaces) error {
	gw, _ := getDefaultGateway()
	interfaces.gw = gw

	AddInterfaceToBridge("eth0", "br0")

	AddInterfaceToBridge("tap0", "br0")

	interfaces.br0init = true
	return nil
}

func getInterfaceIPs(ifaceName string) ([]net.Addr, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}

	return addrs, nil
}
