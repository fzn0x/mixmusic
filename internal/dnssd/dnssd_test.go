package dnssd

import (
	"context"
	"net"
	"reflect"
	"testing"
	"time"
)

const testService = "_mixmusic._tcp.local."

// loopback is the address the tests' responders answer with: they never
// reach the network.
func loopback(net.IP) net.IP { return net.IPv4(127, 0, 0, 1) }

// serve has r answer on a port of 127.0.0.1 until the test ends, and
// returns its address.
func serve(t *testing.T, r *Responder) string {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- r.Serve(conn) }()
	t.Cleanup(func() {
		conn.Close()
		if err := <-done; err != nil {
			t.Errorf("serving: %v", err)
		}
	})
	return conn.LocalAddr().String()
}

func browse(t *testing.T, to string) []Instance {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	found, err := Browse(ctx, testService, BrowseOptions{Wait: 300 * time.Millisecond, To: to, From: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// TestBrowseFindsTheResponder finds an instance whose name has a dot and
// capitals, where it listens, with its TXT record.
func TestBrowseFindsTheResponder(t *testing.T) {
	r := &Responder{Service: testService, Instance: "Atka's Mac.mini", Port: 41234,
		Text: []string{"id=abcd", "name=Atka's Mac.mini", "v=1", "ID=second"}, AddrFor: loopback}
	found := browse(t, serve(t, r))
	want := []Instance{{Name: "Atka's Mac.mini", Host: "127.0.0.1", Port: 41234,
		Text: map[string]string{"id": "abcd", "name": "Atka's Mac.mini", "v": "1"}}}
	if !reflect.DeepEqual(found, want) {
		t.Errorf("found %+v, want %+v", found, want)
	}
	if got := found[0].Addr(); got != "127.0.0.1:41234" {
		t.Errorf("address %q", got)
	}
}

// TestResponderAnswersItsOwn answers nothing of another service, or of
// another instance.
func TestResponderAnswersItsOwn(t *testing.T) {
	r := &Responder{Service: testService, Instance: "Desktop", Port: 1, AddrFor: loopback}
	from := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 50000}
	for _, q := range []question{
		{name: "_googlecast._tcp.local.", qtype: typePTR},
		{name: "laptop._mixmusic._tcp.local.", qtype: typeSRV},
		{name: testService, qtype: typeA},
	} {
		msg, err := appendQuery(nil, 7, []question{q})
		if err != nil {
			t.Fatal(err)
		}
		if reply, _ := r.answer(msg, from); reply != nil {
			t.Errorf("answered %+v", q)
		}
	}
}

// TestAnswerWhere answers a one-shot querier directly, with its ID, its
// question and short times; mDNS's port in the group, without them; and
// mDNS's port directly where it asks so.
func TestAnswerWhere(t *testing.T) {
	r := &Responder{Service: testService, Instance: "Desktop", Port: 9, Text: []string{"v=1"}, AddrFor: loopback}
	ip := net.IPv4(192, 0, 2, 7)
	for _, c := range []struct {
		name    string
		port    int
		unicast bool
		group   bool
	}{
		{"one-shot", 50000, false, false},
		{"mdns", 5353, false, true},
		{"mdns asking directly", 5353, true, false},
	} {
		msg, err := appendQuery(nil, 77, []question{{name: testService, qtype: typePTR, unicast: c.unicast}})
		if err != nil {
			t.Fatal(err)
		}
		from := &net.UDPAddr{IP: ip, Port: c.port}
		reply, to := r.answer(msg, from)
		if reply == nil {
			t.Fatalf("%s: no answer", c.name)
		}
		if got := to.String(); (got == mdnsGroup.String()) != c.group {
			t.Errorf("%s: answered to %s", c.name, got)
		}
		m, err := parse(reply)
		if err != nil {
			t.Fatal(err)
		}
		legacy := c.port != 5353
		if (m.id == 77) != legacy || (len(m.questions) == 1) != legacy {
			t.Errorf("%s: ID %d, questions %+v", c.name, m.id, m.questions)
		}
		kinds := map[uint16]bool{}
		for _, rec := range m.records {
			kinds[rec.rtype] = true
			if legacy && rec.ttl > legacyTTL {
				t.Errorf("%s: %d's time %d", c.name, rec.rtype, rec.ttl)
			}
		}
		for _, k := range []uint16{typePTR, typeSRV, typeTXT, typeA} {
			if !kinds[k] {
				t.Errorf("%s: no record of type %d in %+v", c.name, k, m.records)
			}
		}
	}
}

// TestBrowseAsksForTheRest asks again for the SRV, TXT and A records of an
// instance whose responder answered the first question alone.
func TestBrowseAsksForTheRest(t *testing.T) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	inst := "Laptop._mixmusic._tcp.local."
	host := "laptop.local."
	go func() {
		buf := make([]byte, 9000)
		for {
			n, from, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			m, err := parse(buf[:n])
			if err != nil {
				continue
			}
			var answers []record
			for _, q := range m.questions {
				switch q.qtype {
				case typePTR:
					answers = append(answers, record{name: testService, rtype: typePTR, ttl: 10, target: inst})
				case typeSRV:
					answers = append(answers, record{name: inst, rtype: typeSRV, ttl: 10, target: host, port: 7000})
				case typeTXT:
					answers = append(answers, record{name: inst, rtype: typeTXT, ttl: 10, txt: []string{"id=lap"}})
				case typeA:
					answers = append(answers, record{name: host, rtype: typeA, ttl: 10, ip: net.IPv4(127, 0, 0, 1)})
				}
			}
			reply, err := appendResponse(nil, m.id, m.questions, answers, nil)
			if err == nil {
				conn.WriteToUDP(reply, from)
			}
		}
	}()
	found := browse(t, conn.LocalAddr().String())
	want := []Instance{{Name: "Laptop", Host: "127.0.0.1", Port: 7000, Text: map[string]string{"id": "lap"}}}
	if !reflect.DeepEqual(found, want) {
		t.Errorf("found %+v, want %+v", found, want)
	}
}

// TestNames writes names with dots and backslashes in a label, and reads
// them back escaped.
func TestNames(t *testing.T) {
	for _, name := range []string{`a\.b._x._tcp.local.`, `back\\slash.local.`, "plain.local."} {
		b, err := appendName(nil, name)
		if err != nil {
			t.Fatal(err)
		}
		got, end, err := readName(b, 0)
		if err != nil || got != name || end != len(b) {
			t.Errorf("%q came back as %q (%d of %d, %v)", name, got, end, len(b), err)
		}
	}
	if got := firstLabel(`Mac\.mini._x._tcp.local.`); got != "Mac.mini" {
		t.Errorf("first label %q", got)
	}
	if _, err := appendName(nil, "a..b"); err == nil {
		t.Error("an empty label was written")
	}
}

// TestParseShort reads messages cut short without reading past their
// end.
func TestParseShort(t *testing.T) {
	r := &Responder{Service: testService, Instance: "Desktop", Port: 9, Text: []string{"v=1"}, AddrFor: loopback}
	msg, _ := appendQuery(nil, 1, []question{{name: testService, qtype: typePTR}})
	reply, _ := r.answer(msg, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 50000})
	// Every cut reads, or fails, without reading past its end.
	for n := range len(reply) {
		_, _ = parse(reply[:n])
	}
	if _, err := parse(reply); err != nil {
		t.Fatal(err)
	}
	if _, err := parse([]byte{0, 1, 2}); err == nil {
		t.Error("a message of 3 bytes read")
	}
}

func TestHostLabel(t *testing.T) {
	for in, want := range map[string]string{
		"Atka's MacBook Pro": "Atkas-MacBook-Pro",
		"DESKTOP-4J2K":       "DESKTOP-4J2K",
		"  ":                 "mixmusic",
		"café.local":         "caf-local",
	} {
		if got := HostLabel(in); got != want {
			t.Errorf("HostLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
