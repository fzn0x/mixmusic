package dnssd

import (
	"net"
	"testing"
)

// TestBrowseKeepsAFewAddressesOfAHost has something on the network answer
// for a host's name with a hundred addresses: a few are kept.
func TestBrowseKeepsAFewAddressesOfAHost(t *testing.T) {
	f := newFindings(testService)
	inst, host := "Desk."+testService, "desk.local."
	recs := []record{
		{name: testService, rtype: typePTR, ttl: 10, target: inst},
		{name: inst, rtype: typeSRV, ttl: 10, target: host, port: 50958},
	}
	for i := range 100 {
		recs = append(recs, record{name: host, rtype: typeA, ttl: 10, ip: net.IPv4(203, 0, 113, byte(i))})
	}
	reply, err := appendResponse(nil, 0, nil, recs, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.read(reply, net.IPv4(192, 168, 1, 12))
	got := f.instances(nil)
	if len(got) != 1 || len(got[0].Others) != maxHostAddrs-1 {
		t.Fatalf("found %+v", got)
	}
}
