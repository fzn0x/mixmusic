package portmap

import (
	"errors"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/portmap/portmaptest"
)

// waitReach waits for k to tell what ok wants.
func waitReach(t *testing.T, k *Keeper, what string, ok func(Reach) bool) Reach {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		r := k.Reach()
		if ok(r) {
			return r
		}
		if time.Now().After(deadline) {
			t.Fatalf("waiting for %s; the keeper tells %+v", what, r)
		}
	}
}

func newKeeper(r *portmaptest.Router, v6 ...netip.Addr) (*Keeper, *atomic.Int32) {
	var told atomic.Int32
	k := &Keeper{Client: clientOf(r), Port: 41234, Lifetime: time.Hour, IPv6: func() []netip.Addr { return v6 }, Changed: func() { told.Add(1) }}
	return k, &told
}

// TestKeeperForwardsAndRemoves keeps a port forwarded: the router forwards
// it, the keeper tells where the internet reaches it, and as the keeper
// stops, the router forwards it no more.
func TestKeeperForwardsAndRemoves(t *testing.T) {
	r := newRouter(t, "203.0.113.7")
	k, told := newKeeper(r)
	k.Start()
	got := waitReach(t, k, "the forward", func(r Reach) bool { return !r.Asking })
	if got.Forward != netip.MustParseAddrPort("203.0.113.7:41234") || got.Method != PCP || got.Err != nil {
		t.Fatalf("reach %+v", got)
	}
	if told.Load() == 0 || len(r.Forwards()) != 1 {
		t.Fatalf("told %d times; the router keeps %+v", told.Load(), r.Forwards())
	}
	k.Stop()
	if len(r.Forwards()) != 0 {
		t.Fatalf("after Stop the router keeps %+v", r.Forwards())
	}
}

// TestKeeperRenewsAndFollowsTheAddress renews the forward at half its
// lifetime, and tells the router's new internet address once it changed,
// as Poke asks again.
func TestKeeperRenewsAndFollowsTheAddress(t *testing.T) {
	r := newRouter(t, "203.0.113.7")
	k, _ := newKeeper(r)
	k.Lifetime = 2 * time.Second
	k.Start()
	defer k.Stop()
	waitReach(t, k, "the forward", func(r Reach) bool { return r.Forward.IsValid() })
	for deadline := time.Now().Add(10 * time.Second); asked(r, "PCP map") < 3; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("never renewed: asked %v", r.Asked())
		}
	}
	r.Set(func(r *portmaptest.Router) { r.External = netip.MustParseAddr("203.0.113.99") })
	k.Poke()
	waitReach(t, k, "the new address", func(r Reach) bool { return r.Forward == netip.MustParseAddrPort("203.0.113.99:41234") })
	if len(r.Forwards()) != 1 {
		t.Fatalf("the router keeps %+v", r.Forwards())
	}
}

// TestKeeperBehindAnotherRouter keeps a port where the router's internet
// address is the provider's shared one: it tells so, and leaves no
// forward.
func TestKeeperBehindAnotherRouter(t *testing.T) {
	r := newRouter(t, "100.70.1.2")
	k, _ := newKeeper(r)
	k.Start()
	defer k.Stop()
	got := waitReach(t, k, "the answer", func(r Reach) bool { return !r.Asking })
	var se *SharedError
	if !errors.As(got.Err, &se) || se.Addr != netip.MustParseAddr("100.70.1.2") || got.Forward.IsValid() {
		t.Fatalf("reach %+v", got)
	}
	if len(r.Forwards()) != 0 {
		t.Fatalf("the router keeps %+v", r.Forwards())
	}
}

// TestKeeperOpensIPv6 keeps the port of this computer's IPv6 address open
// at the router, as well as the forward of IPv4, and closes both.
func TestKeeperOpensIPv6(t *testing.T) {
	r := newRouter(t, "203.0.113.7")
	if !r.HasIPv6() {
		t.Skip("no IPv6 loopback here")
	}
	k, _ := newKeeper(r, netip.MustParseAddr("::1"))
	k.Start()
	got := waitReach(t, k, "the answer", func(r Reach) bool { return !r.Asking })
	if got.IPv6 != netip.MustParseAddr("::1") || !got.Opened || !got.Forward.IsValid() || len(r.Forwards()) != 2 {
		t.Fatalf("reach %+v; the router keeps %+v", got, r.Forwards())
	}
	k.Stop()
	if len(r.Forwards()) != 0 {
		t.Fatalf("after Stop the router keeps %+v", r.Forwards())
	}
}

// TestKeeperWithNoRouter tells that no router answered, and asks again
// later.
func TestKeeperWithNoRouter(t *testing.T) {
	r := newRouter(t, "203.0.113.7")
	r.Set(func(r *portmaptest.Router) { r.PCP, r.NATPMP, r.UPnP = false, false, false })
	k, _ := newKeeper(r)
	k.Lifetime = 2 * time.Second
	k.Start()
	defer k.Stop()
	got := waitReach(t, k, "the answer", func(r Reach) bool { return !r.Asking })
	if !errors.Is(got.Err, ErrNoRouter) {
		t.Fatalf("reach %+v", got)
	}
	r.Set(func(r *portmaptest.Router) { r.UPnP = true })
	waitReach(t, k, "the forward of UPnP", func(r Reach) bool { return r.Method == UPnP && r.Err == nil })
}
