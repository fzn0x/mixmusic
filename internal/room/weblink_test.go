package room

import (
	"bytes"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/remote"
)

func testInvite(t *testing.T, addrs ...string) Invite {
	t.Helper()
	id, err := remote.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	secret, _ := NewSecret()
	return Invite{Code: "ABC123", Host: id.Fingerprint(), Secret: secret, Addrs: addrs}
}

func sameInvite(a, b Invite) bool {
	return a.Code == b.Code && a.Host == b.Host && bytes.Equal(a.Secret, b.Secret) && slices.Equal(a.Addrs, b.Addrs)
}

// TestLinkCarriesAddresses writes an invite link of a room that friends
// outside join, with the router's address and an IPv6 one, and reads them
// back, in both of its forms; a link of the network alone carries none.
func TestLinkCarriesAddresses(t *testing.T) {
	in := testInvite(t, "203.0.113.7:41234", "[2001:db8::5]:41234", "desk.tail1234.ts.net:41234")
	link := in.Link()
	if !strings.Contains(link, "@203.0.113.7:41234,%5B2001:db8::5%5D:41234,desk.tail1234.ts.net:41234") {
		t.Fatalf("link %q", link)
	}
	if got, err := ParseLink(link); err != nil || !sameInvite(got, in) {
		t.Fatalf("%q reads as %+v, %v", link, got, err)
	}
	web := in.WebLink()
	if !strings.HasPrefix(web, "https://mixmusic.functionx.dev/join/#ABC123."+in.Host.String()+".") {
		t.Fatalf("web link %q", web)
	}
	u, err := url.Parse(web)
	if err != nil || u.RawQuery != "" || u.Path != "/join/" {
		t.Fatalf("web link %q: %+v, %v", web, u, err)
	}
	if got, err := ParseLink(web); err != nil || !sameInvite(got, in) {
		t.Fatalf("%q reads as %+v, %v", web, got, err)
	}
	// GitHub Pages' copy of the website opens it too.
	pages := strings.Replace(web, "mixmusic.functionx.dev/join/", "fzn0x.github.io/mixmusic/join/", 1)
	if got, err := ParseLink(pages); err != nil || !sameInvite(got, in) {
		t.Fatalf("%q reads as %+v, %v", pages, got, err)
	}
	lan := testInvite(t)
	if strings.Contains(lan.Link(), "@") {
		t.Fatalf("a link of the network alone: %q", lan.Link())
	}
	if got, err := ParseLink(lan.WebLink()); err != nil || !sameInvite(got, lan) {
		t.Fatalf("%q reads as %+v, %v", lan.WebLink(), got, err)
	}
}

// TestLinkRefusesOtherAddresses refuses links whose addresses aren't, or
// that are many, or of a page that isn't the website's.
func TestLinkRefusesOtherAddresses(t *testing.T) {
	in := testInvite(t)
	base := in.Link()
	for _, bad := range []string{
		base + "@203.0.113.7",
		base + "@203.0.113.7:0",
		base + "@203.0.113.7:70000",
		base + "@[fe80::1%25eth0]:41234",
		base + "@0.0.0.0:41234",
		base + "@bad_host:41234",
		base + "@-x.example:41234",
		base + "@1.1.1.1:1,1.1.1.2:1,1.1.1.3:1,1.1.1.4:1,1.1.1.5:1",
		strings.Replace(in.WebLink(), "mixmusic.functionx.dev", "example.com", 1),
		strings.Replace(in.WebLink(), "/join/", "/play/", 1),
		"https://mixmusic.functionx.dev/join/",
	} {
		if got, err := ParseLink(bad); err == nil {
			t.Errorf("%q reads as %+v", bad, got)
		}
	}
}

// TestInviteAddr reads addresses as people type them: with a port or not,
// IPv6 in brackets or not, names in any case.
func TestInviteAddr(t *testing.T) {
	for in, want := range map[string]string{
		"203.0.113.7":            "203.0.113.7:41234",
		" 203.0.113.7:5000 ":     "203.0.113.7:5000",
		"2001:db8::5":            "[2001:db8::5]:41234",
		"[2001:db8::5]":          "[2001:db8::5]:41234",
		"[2001:db8::5]:6000":     "[2001:db8::5]:6000",
		"Desk.Tail1234.ts.net":   "desk.tail1234.ts.net:41234",
		"desk.tail1234.ts.net.":  "desk.tail1234.ts.net:41234",
		"100.101.102.103:41234":  "100.101.102.103:41234",
		"::ffff:203.0.113.7":     "203.0.113.7:41234",
		"my-host.example.org:80": "my-host.example.org:80",
	} {
		if got, ok := InviteAddr(in, 41234); !ok || got != want {
			t.Errorf("InviteAddr(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "  ", "http://x", "a b", "x:y", "1.2.3.4:0", "host:65536", "fe80::1%eth0"} {
		if got, ok := InviteAddr(bad, 41234); ok {
			t.Errorf("InviteAddr(%q) = %q", bad, got)
		}
	}
}
