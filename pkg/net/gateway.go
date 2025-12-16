package net

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

func getDefaultGateway() (net.IP, error) {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("failed to get routes: %w", err)
	}

	for _, route := range routes {
		if route.Dst == nil {
			if route.Gw != nil {
				return route.Gw, nil
			}
		}
	}

	return nil, fmt.Errorf("default gateway not found")
}
