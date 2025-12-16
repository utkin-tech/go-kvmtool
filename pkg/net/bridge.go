package net

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

func CreateBridge(name string) error {
	_, err := netlink.LinkByName(name)
	if err == nil {
		link, _ := netlink.LinkByName(name)
		return netlink.LinkDel(link)
	}

	bridge := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: name,
			MTU:  1500,
			// HardwareAddr: net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
		},
	}

	if err := netlink.LinkAdd(bridge); err != nil {
		return fmt.Errorf("failed LinkAdd: %w", err)
	}

	if err := netlink.LinkSetUp(bridge); err != nil {
		return fmt.Errorf("failed interface up: %w", err)
	}

	return nil
}

func AddInterfaceToBridge(ifaceName string, bridgeName string) error {
	iface, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("interface %s not found: %w", ifaceName, err)
	}

	bridge, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("bridge %s not found: %w", bridgeName, err)
	}

	if err := netlink.LinkSetDown(iface); err != nil {
		return fmt.Errorf("failed down interface: %w", err)
	}

	if err := netlink.LinkSetMaster(iface, bridge); err != nil {
		return fmt.Errorf("failed add interface in bridge: %w", err)
	}

	if err := netlink.LinkSetUp(iface); err != nil {
		return fmt.Errorf("failed interface up: %w", err)
	}

	return nil
}
