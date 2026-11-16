package room

import (
	"net"
	"net/netip"
	"slices"
	"testing"
)

// TestSharedRangeOfAModemIsItsOwn takes a network of the providers' shared
// range on an interface that isn't a tunnel, as a modem's that hands this
// computer the provider's address: its own prefix is near, the provider's
// other customers aren't. A tunnel's, of whatever mask, makes the whole
// range near.
func TestSharedRangeOfAModemIsItsOwn(t *testing.T) {
	modem := netsOf("192.168.1.0/24", "100.70.0.0/22")
	for addr, want := range map[string]bool{"100.70.1.9": true, "100.70.4.1": false, "100.101.102.103": false} {
		if got := modem.Of(addr); got != want {
			t.Errorf("behind a modem of 100.70.0.0/22, Of(%q) = %v", addr, got)
		}
	}
	_, wide, _ := net.ParseCIDR("100.70.1.2/10")
	tunnel := addrNets(&net.IPNet{IP: net.ParseIP("100.70.1.2").To4(), Mask: wide.Mask}, true)
	if !slices.Contains(tunnel, netip.MustParsePrefix("100.70.1.2/32")) {
		t.Fatalf("a tunnel's networks %v", tunnel)
	}
	if !NearbyOf(netip.MustParseAddr("100.101.102.103"), tunnel) {
		t.Error("a tunnel of the shared range doesn't take its VPN in")
	}
	lan := addrNets(&net.IPNet{IP: net.ParseIP("100.70.1.2").To4(), Mask: net.CIDRMask(22, 32)}, false)
	if len(lan) != 1 || NearbyOf(netip.MustParseAddr("100.101.102.103"), lan) {
		t.Errorf("a modem's network of the shared range %v takes the provider's others in", lan)
	}
}
