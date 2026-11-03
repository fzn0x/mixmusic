package cast

import (
	"bytes"
	"testing"
)

// TestDNSQuery writes a query's questions in DNS's labels, of the
// Internet class, a dot that a label holds escaped.
func TestDNSQuery(t *testing.T) {
	q, err := dnsQuery(0x1234, []question{{service, typePTR}, {`a\.b.local.`, typeSRV}})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x12, 0x34, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0}
	want = append(want, "\x0b_googlecast\x04_tcp\x05local\x00\x00\x0c\x00\x01"...)
	want = append(want, "\x03a.b\x05local\x00\x00\x21\x00\x01"...)
	if !bytes.Equal(q, want) {
		t.Fatalf("query\n got %q\nwant %q", q, want)
	}
	if _, err := dnsQuery(1, []question{{"a..b.", typeA}}); err == nil {
		t.Error("a name with an empty label written")
	}
}

// TestParseDNS reads an answer's records, their names compressed, and
// leaves out the types browsing doesn't read; it turns down one cut short
// or whose pointers go round.
func TestParseDNS(t *testing.T) {
	msg := []byte{0, 7, 0x84, 0, 0, 0, 0, 3, 0, 0, 0, 1}
	// PTR _googlecast._tcp.local. -> Chromecast-1.<pointer to 12>
	msg = append(msg, "\x0b_googlecast\x04_tcp\x05local\x00"...)
	msg = append(msg, 0, 12, 0, 1, 0, 0, 0, 120, 0, 15)
	inst := byte(len(msg))
	msg = append(msg, "\x0cChromecast-1"...)
	msg = append(msg, 0xC0, 12)
	// SRV at <pointer to the instance>, port 8009, target h.local.
	msg = append(msg, 0xC0, inst, 0, 33, 0x80, 1, 0, 0, 0, 120, 0, 15, 0, 0, 0, 0, 0x1f, 0x49)
	msg = append(msg, "\x01h\x05local\x00"...)
	// AAAA, which browsing leaves out.
	msg = append(msg, 0xC0, 12, 0, 28, 0, 1, 0, 0, 0, 120, 0, 16)
	msg = append(msg, make([]byte, 16)...)
	// TXT among the additional records.
	msg = append(msg, 0xC0, inst, 0, 16, 0x80, 1, 0, 0, 0, 120, 0, 10, 4, 'i', 'd', '=', '1', 4, 'f', 'n', '=', 'K')
	id, response, records, err := parseDNS(msg)
	if err != nil || id != 7 || !response {
		t.Fatalf("parse: %d %v %v", id, response, err)
	}
	if len(records) != 3 {
		t.Fatalf("records %+v", records)
	}
	if r := records[0]; r.rtype != typePTR || r.name != service || r.target != "Chromecast-1."+service {
		t.Errorf("PTR %+v", r)
	}
	if r := records[1]; r.rtype != typeSRV || r.name != "chromecast-1."+service || r.port != 8009 || r.target != "h.local." {
		t.Errorf("SRV %+v", r)
	}
	if r := records[2]; r.rtype != typeTXT || len(r.txt) != 2 || r.txt[1] != "fn=K" {
		t.Errorf("TXT %+v", r)
	}
	if _, _, _, err := parseDNS(msg[:len(msg)-3]); err == nil {
		t.Error("an answer cut short read")
	}
	loop := []byte{0, 1, 0x84, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0xC0, 12, 0, 12, 0, 1}
	if _, _, _, err := parseDNS(loop); err == nil {
		t.Error("a name pointing at itself read")
	}
	if got := firstLabel(`Living\.Room.local.`); got != "Living.Room" {
		t.Errorf("first label %q", got)
	}
}
