//go:build !linux && !darwin && !windows

package portmap

import (
	"errors"
	"net/netip"
)

// systemGateways: this system's routes aren't read, so PCP and NAT-PMP ask
// no router, and UPnP, which needs none, still asks.
func systemGateways() ([]netip.Addr, error) {
	return nil, errors.New("portmap: this system's default gateways aren't read")
}
