package room

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
)

// TestFarOK dials the addresses links and hosts tell, but this computer's
// and those of no one.
func TestFarOK(t *testing.T) {
	for addr, want := range map[string]bool{
		"203.0.113.7": true, "192.168.1.12": true, "10.0.0.5": true, "100.101.102.103": true, "2001:db8::5": true, "fd7a:115c:a1e0::1": true,
		"127.0.0.1": false, "127.9.9.9": false, "::1": false, "::ffff:127.0.0.1": false, "0.0.0.0": false, "::": false,
		"169.254.1.1": false, "fe80::1": false, "fe80::1%eth0": false, "224.0.0.251": false, "ff02::fb": false, "255.255.255.255": false,
	} {
		if got := FarOK(netip.MustParseAddr(addr)); got != want {
			t.Errorf("FarOK(%s) = %v", addr, got)
		}
	}
}

// TestFarDialerRefusesThisComputer dials, as a link or a host told, a room
// that listens on this computer: by its address, and by a name that
// resolves to it, each is refused before it connects.
func TestFarDialerRefusesThisComputer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	for _, addr := range []string{ln.Addr().String(), net.JoinHostPort("localhost", port)} {
		c, err := FarDialer().DialContext(context.Background(), "tcp", addr)
		if err == nil {
			c.Close()
		}
		if !errors.Is(err, ErrFarAddr) {
			t.Errorf("dialling %s: %v", addr, err)
		}
	}
}
