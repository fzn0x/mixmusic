package portmap

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/portmap/portmaptest"
)

// newRouter starts a fake router of the internet address external, which
// the test stops: never a real one.
func newRouter(t *testing.T, external string) *portmaptest.Router {
	t.Helper()
	r, err := portmaptest.New(external)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	return r
}

// clientOf asks r alone: its PCP and NAT-PMP as the gateway of IPv4 and of
// IPv6, and its UPnP in place of the multicast group.
func clientOf(r *portmaptest.Router) *Client {
	return &Client{
		Gateways: func() ([]netip.Addr, error) {
			return []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("::1")}, nil
		},
		ServerPort: r.Port(), SSDP: r.SSDP(), Retry: 20 * time.Millisecond,
	}
}

func asked(r *portmaptest.Router, what string) int {
	n := 0
	for _, a := range r.Asked() {
		if a == what {
			n++
		}
	}
	return n
}

func forwardOf(t *testing.T, r *portmaptest.Router, external uint16) portmaptest.Forward {
	t.Helper()
	for _, f := range r.Forwards() {
		if f.External == external {
			return f
		}
	}
	t.Fatalf("the router forwards no port %d: %+v", external, r.Forwards())
	return portmaptest.Forward{}
}

// TestMapByPCP asks a router that speaks PCP: it forwards the port asked,
// tells its internet address, renews the forward and deletes it.
func TestMapByPCP(t *testing.T) {
	r := newRouter(t, "203.0.113.7")
	c := clientOf(r)
	ctx := context.Background()
	m, err := c.Map(ctx, 41234, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if m.Method != PCP || m.External != netip.MustParseAddrPort("203.0.113.7:41234") || m.Internal != 41234 || m.Lifetime != time.Hour {
		t.Fatalf("mapped %+v", m)
	}
	if f := forwardOf(t, r, 41234); f.Internal != 41234 || f.Client != netip.MustParseAddr("127.0.0.1") || f.Method != "PCP" {
		t.Fatalf("the router keeps %+v", f)
	}
	again, err := c.Renew(ctx, m, 2*time.Hour)
	if err != nil || again.External != m.External || again.Lifetime != 2*time.Hour || len(r.Forwards()) != 1 {
		t.Fatalf("renewed %+v, %v; the router keeps %+v", again, err, r.Forwards())
	}
	if err := c.Delete(ctx, again); err != nil {
		t.Fatal(err)
	}
	if len(r.Forwards()) != 0 || asked(r, "PCP delete") != 1 {
		t.Fatalf("after Delete the router keeps %+v, and was asked %v", r.Forwards(), r.Asked())
	}
}

// TestMapByNATPMP asks a router of NAT-PMP alone, which tells a PCP client
// it speaks another version: NAT-PMP forwards the port, and tells the
// router's address.
func TestMapByNATPMP(t *testing.T) {
	r := newRouter(t, "198.51.100.20")
	r.Set(func(r *portmaptest.Router) { r.PCP = false })
	c := clientOf(r)
	m, err := c.Map(context.Background(), 5000, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if m.Method != NATPMP || m.External != netip.MustParseAddrPort("198.51.100.20:5000") || m.Lifetime != 10*time.Minute {
		t.Fatalf("mapped %+v", m)
	}
	if asked(r, "NAT-PMP address") != 1 || asked(r, "NAT-PMP map") != 1 || asked(r, "UPnP search") != 0 {
		t.Fatalf("asked %v", r.Asked())
	}
	if err := c.Delete(context.Background(), m); err != nil || len(r.Forwards()) != 0 {
		t.Fatalf("Delete: %v; the router keeps %+v", err, r.Forwards())
	}
}

// TestTakenPortGetsAnother asks for a port another computer has: PCP and
// NAT-PMP give another, which the mapping tells, and UPnP tries another.
func TestTakenPortGetsAnother(t *testing.T) {
	for _, method := range []Method{PCP, NATPMP, UPnP} {
		t.Run(string(method), func(t *testing.T) {
			r := newRouter(t, "203.0.113.7")
			r.Set(func(r *portmaptest.Router) {
				r.Taken = []uint16{6000}
				r.PCP, r.NATPMP = method == PCP, method == NATPMP
			})
			m, err := clientOf(r).Map(context.Background(), 6000, time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			if m.Method != method || m.External.Port() == 6000 || m.Internal != 6000 {
				t.Fatalf("mapped %+v", m)
			}
			forwardOf(t, r, m.External.Port())
		})
	}
}

// TestMapByUPnP asks a router of UPnP alone: its search finds it, its
// description names its control, which forwards the port, tells the
// router's address, and deletes it.
func TestMapByUPnP(t *testing.T) {
	r := newRouter(t, "192.0.2.44")
	r.Set(func(r *portmaptest.Router) { r.PCP, r.NATPMP = false, false })
	c := clientOf(r)
	m, err := c.Map(context.Background(), 41234, 30*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if m.Method != UPnP || m.External != netip.MustParseAddrPort("192.0.2.44:41234") || m.Lifetime != 30*time.Minute {
		t.Fatalf("mapped %+v", m)
	}
	if f := forwardOf(t, r, 41234); f.Client != netip.MustParseAddr("127.0.0.1") || f.Lifetime != 30*time.Minute {
		t.Fatalf("the router keeps %+v", f)
	}
	for _, a := range []string{"UPnP search", "UPnP description", "AddPortMapping", "GetExternalIPAddress"} {
		if asked(r, a) == 0 {
			t.Errorf("never asked %s: %v", a, r.Asked())
		}
	}
	if err := c.Delete(context.Background(), m); err != nil || len(r.Forwards()) != 0 {
		t.Fatalf("Delete: %v; the router keeps %+v", err, r.Forwards())
	}
}

// TestUPnPForGoodIsRefused asks a router whose UPnP takes forwards for
// good alone: it is asked for none, as one left behind would stay.
func TestUPnPForGoodIsRefused(t *testing.T) {
	r := newRouter(t, "192.0.2.44")
	r.Set(func(r *portmaptest.Router) { r.PCP, r.NATPMP, r.Permanent = false, false, true })
	_, err := clientOf(r).Map(context.Background(), 41234, 30*time.Minute)
	var re *RefusedError
	if !errors.As(err, &re) || re.Code != 725 || re.Method != UPnP {
		t.Fatalf("Map: %v", err)
	}
	if len(r.Forwards()) != 0 || asked(r, "AddPortMapping") != 1 {
		t.Fatalf("the router keeps %+v, and was asked %v", r.Forwards(), r.Asked())
	}
}

// TestRefused asks a router that refuses every forward: Map tells it
// refused, by its code.
func TestRefused(t *testing.T) {
	r := newRouter(t, "203.0.113.7")
	r.Set(func(r *portmaptest.Router) { r.Refuse = true })
	_, err := clientOf(r).Map(context.Background(), 41234, time.Hour)
	var re *RefusedError
	if !errors.As(err, &re) || re.Code != 2 || re.Method != PCP {
		t.Fatalf("Map: %v", err)
	}
	if len(r.Forwards()) != 0 {
		t.Fatalf("the router keeps %+v", r.Forwards())
	}
}

// TestNoRouter asks where nothing answers: no gateway, and a search no
// router hears. Map tells no router answered, soon.
func TestNoRouter(t *testing.T) {
	quiet, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer quiet.Close()
	c := &Client{Gateways: func() ([]netip.Addr, error) { return nil, nil }, SSDP: quiet.LocalAddr().String(), Retry: 20 * time.Millisecond}
	start := time.Now()
	if _, err := c.Map(context.Background(), 41234, time.Hour); !errors.Is(err, ErrNoRouter) {
		t.Fatalf("Map: %v", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("Map took %v", d)
	}
}

// TestSearchAsksNoOtherAddress answers UPnP's search with a description at
// an address other than the one answering: it isn't asked.
func TestSearchAsksNoOtherAddress(t *testing.T) {
	liar, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer liar.Close()
	go func() {
		buf := make([]byte, 2048)
		for {
			_, from, err := liar.ReadFromUDPAddrPort(buf)
			if err != nil {
				return
			}
			liar.WriteToUDPAddrPort([]byte("HTTP/1.1 200 OK\r\nST: urn:schemas-upnp-org:device:InternetGatewayDevice:1\r\nLOCATION: http://192.0.2.1:80/rootDesc.xml\r\n\r\n"), from)
		}
	}()
	c := &Client{Gateways: func() ([]netip.Addr, error) { return nil, nil }, SSDP: liar.LocalAddr().String(), Retry: 20 * time.Millisecond}
	hosts, locs, err := c.search(context.Background())
	if err != nil || len(hosts) != 0 || len(locs) != 0 {
		t.Fatalf("search found %v %v, %v", hosts, locs, err)
	}
}

// TestOpenIPv6 asks the router of IPv6 by PCP to let a port of this
// computer's IPv6 address in: the address isn't translated.
func TestOpenIPv6(t *testing.T) {
	r := newRouter(t, "203.0.113.7")
	if !r.HasIPv6() {
		t.Skip("no IPv6 loopback here")
	}
	c := clientOf(r)
	m, err := c.Open(context.Background(), netip.MustParseAddr("::1"), 41234, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if m.Method != PCP || m.External != netip.MustParseAddrPort("[::1]:41234") {
		t.Fatalf("opened %+v", m)
	}
	if f := forwardOf(t, r, 41234); f.Client != netip.MustParseAddr("::1") {
		t.Fatalf("the router keeps %+v", f)
	}
	if err := c.Delete(context.Background(), m); err != nil || len(r.Forwards()) != 0 {
		t.Fatalf("Delete: %v; the router keeps %+v", err, r.Forwards())
	}
}

// TestShared tells the internet addresses of routers behind another: the
// providers' shared range and the private ones.
func TestShared(t *testing.T) {
	for _, a := range []string{"100.64.0.1", "100.127.255.254", "10.1.2.3", "172.16.0.9", "192.168.1.1", "0.0.0.0", "::ffff:100.70.1.1",
		"224.0.0.5", "239.255.255.250", "255.255.255.255", "240.1.2.3", "0.1.2.3", "127.0.0.1", "169.254.1.1"} {
		if !Shared(netip.MustParseAddr(a)) {
			t.Errorf("%s isn't shared", a)
		}
	}
	for _, a := range []string{"203.0.113.7", "100.128.0.1", "8.8.8.8", "2001:4860::8888"} {
		if Shared(netip.MustParseAddr(a)) {
			t.Errorf("%s is shared", a)
		}
	}
}

// TestReachableIPv6 keeps the global addresses of IPv6 alone.
func TestReachableIPv6(t *testing.T) {
	for a, want := range map[string]bool{
		"2a01:4f8::1": true, "2600:1f18::5": true,
		"fd00::1": false, "fe80::1": false, "::1": false, "2001:db8::1": false, "2001:0:1::1": false, "2002:c000:204::1": false,
		"::ffff:203.0.113.7": false, "203.0.113.7": false,
	} {
		if got := reachableIPv6(netip.MustParseAddr(a)); got != want {
			t.Errorf("reachableIPv6(%s) = %v", a, got)
		}
	}
	for _, a := range PublicIPv6() {
		if !reachableIPv6(a) || eui64(a) {
			t.Errorf("PublicIPv6 tells %s", a)
		}
	}
	// An address made of a hardware address tells it.
	for a, want := range map[string]bool{"2a01:4f8::21a:2bff:fe3c:4d5e": true, "2a01:4f8::9d3c:7a11:52e0:1b6f": false} {
		if got := eui64(netip.MustParseAddr(a)); got != want {
			t.Errorf("eui64(%s) = %v", a, got)
		}
	}
}

// TestMethodsAskedInOrder asks a router of all three: PCP alone answers,
// and nothing else is asked.
func TestMethodsAskedInOrder(t *testing.T) {
	r := newRouter(t, "203.0.113.7")
	if _, err := clientOf(r).Map(context.Background(), 41234, time.Hour); err != nil {
		t.Fatal(err)
	}
	if got := r.Asked(); !slices.Equal(got, []string{"PCP map"}) {
		t.Fatalf("asked %v", got)
	}
}

// TestUPnPOfTheGatewayAlone has a device that isn't the default gateway
// answer UPnP's search: it isn't asked to forward anything.
func TestUPnPOfTheGatewayAlone(t *testing.T) {
	r := newRouter(t, "192.0.2.44")
	r.Set(func(r *portmaptest.Router) { r.PCP, r.NATPMP = false, false })
	c := clientOf(r)
	// The gateway is another address of the loopback, which answers
	// nothing.
	c.Gateways = func() ([]netip.Addr, error) { return []netip.Addr{netip.MustParseAddr("127.0.0.2")}, nil }
	if _, err := c.Map(context.Background(), 41234, time.Hour); !errors.Is(err, ErrNoRouter) {
		t.Fatalf("Map: %v", err)
	}
	if asked(r, "AddPortMapping") != 0 || len(r.Forwards()) != 0 {
		t.Fatalf("the device was asked %v", r.Asked())
	}
}

// TestKeeperRefusesAnAddressThatIsntOne keeps a port where the router
// tells a multicast address as its own: it tells so, and leaves no
// forward.
func TestKeeperRefusesAnAddressThatIsntOne(t *testing.T) {
	r := newRouter(t, "224.0.0.5")
	k, _ := newKeeper(r)
	k.Start()
	defer k.Stop()
	got := waitReach(t, k, "the answer", func(r Reach) bool { return !r.Asking })
	var re *RefusedError
	if !errors.As(got.Err, &re) || got.Forward.IsValid() || !strings.Contains(re.Reason, "224.0.0.5") {
		t.Fatalf("reach %+v", got)
	}
	if len(r.Forwards()) != 0 {
		t.Fatalf("the router keeps %+v", r.Forwards())
	}
}
