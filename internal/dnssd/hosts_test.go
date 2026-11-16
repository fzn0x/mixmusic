package dnssd

import (
	"net"
	"reflect"
	"testing"
)

// TestNearestFirst puts the addresses of the networks this computer is on
// first, a VPN's interface of one address and the providers' shared range
// after, and keeps the order they came in otherwise.
func TestNearestFirst(t *testing.T) {
	ip := net.ParseIP
	_, lan, _ := net.ParseCIDR("192.168.1.0/24")
	_, vpn, _ := net.ParseCIDR("100.127.209.93/32")
	_, wide, _ := net.ParseCIDR("100.64.0.0/10")
	for _, tc := range []struct {
		nets []*net.IPNet
		in   []net.IP
		want []net.IP
	}{
		{[]*net.IPNet{lan, vpn}, []net.IP{ip("100.127.209.93"), ip("192.168.1.12")}, []net.IP{ip("192.168.1.12"), ip("100.127.209.93")}},
		{[]*net.IPNet{wide, lan}, []net.IP{ip("100.127.209.93"), ip("192.168.1.12")}, []net.IP{ip("192.168.1.12"), ip("100.127.209.93")}},
		{[]*net.IPNet{lan}, []net.IP{ip("203.0.113.7"), ip("198.51.100.1")}, []net.IP{ip("203.0.113.7"), ip("198.51.100.1")}},
		{nil, []net.IP{ip("100.127.209.93"), ip("192.168.1.12")}, []net.IP{ip("100.127.209.93"), ip("192.168.1.12")}},
	} {
		if got := nearestFirst(tc.in, tc.nets); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("on %v, %v: %v, want %v", tc.nets, tc.in, got, tc.want)
		}
	}
}

// TestBrowseKeepsEveryAddress has a host answer with its address on the
// network each question came on, a VPN's last: the instance found keeps
// both, of the network it's on first.
func TestBrowseKeepsEveryAddress(t *testing.T) {
	f := newFindings(testService)
	inst, host := "Desk."+testService, "desk.local."
	for _, addr := range []net.IP{net.IPv4(192, 168, 1, 12), net.IPv4(100, 127, 209, 93)} {
		reply, err := appendResponse(nil, 0, nil, []record{
			{name: testService, rtype: typePTR, ttl: 10, target: inst},
			{name: inst, rtype: typeSRV, ttl: 10, target: host, port: 50958},
			{name: inst, rtype: typeTXT, ttl: 10, txt: []string{"id=desk"}},
			{name: host, rtype: typeA, ttl: 10, ip: addr},
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		f.read(reply, addr)
	}
	_, lan, _ := net.ParseCIDR("192.168.1.0/24")
	_, vpn, _ := net.ParseCIDR("100.127.209.93/32")
	got := f.instances([]*net.IPNet{vpn, lan})
	if len(got) != 1 || got[0].Addr() != "192.168.1.12:50958" || !reflect.DeepEqual(got[0].Others, []string{"100.127.209.93"}) {
		t.Fatalf("found %+v", got)
	}
}
