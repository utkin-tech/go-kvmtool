package main

import (
	"fmt"
	"log"
	"net"

	"github.com/vishvananda/netlink"
)

func AddNet(ip *net.IPNet, gateway net.IP) {
	link, err := netlink.LinkByName("eth0")
	if err != nil {
		log.Fatalf("Failed to get link: %v", err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		log.Fatalf("Failed to set link up: %v", err)
	}
	fmt.Println("Interface eth0 set to UP")

	addr := &netlink.Addr{
		IPNet: ip,
	}

	if err := netlink.AddrAdd(link, addr); err != nil {
		log.Fatalf("Failed to add address: %v", err)
	}

	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       nil,
		Gw:        gateway,
	}

	if err := netlink.RouteAdd(route); err != nil {
		log.Fatalf("Failed to add route: %v", err)
	}
}
