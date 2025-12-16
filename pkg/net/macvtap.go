package net

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

// net.CreateMacVTap("eth0", "macvtap0", netlink.MACVLAN_MODE_BRIDGE)
func CreateMacVTap(parentInterface string, tapName string, mode netlink.MacvlanMode) error {
	parentLink, err := netlink.LinkByName(parentInterface)
	if err != nil {
		return fmt.Errorf("failed to find parent interface %s: %v", parentInterface, err)
	}

	macvtap := &netlink.Macvtap{
		Macvlan: netlink.Macvlan{
			LinkAttrs: netlink.LinkAttrs{
				Name:        tapName,
				ParentIndex: parentLink.Attrs().Index,
			},
			Mode: mode,
		},
	}

	if err := netlink.LinkAdd(macvtap); err != nil {
		return fmt.Errorf("failed to create macvtap: %v", err)
	}

	if err := netlink.LinkSetUp(macvtap); err != nil {
		return fmt.Errorf("failed to set macvtap up: %v", err)
	}

	return nil
}
