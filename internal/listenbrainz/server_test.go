package listenbrainz

import (
	"context"
	"errors"
	"net/netip"
	"testing"
)

func TestParseServer(t *testing.T) {
	for _, c := range []struct {
		in, want string
		plain    bool
	}{
		{"", DefaultServer, false},
		{"  ", DefaultServer, false},
		{"https://api.listenbrainz.org/", DefaultServer, false},
		{"https://api.listenbrainz.org/1/", DefaultServer, false},
		{"listenbrainz.org", DefaultServer, false},
		{"lb.example.com", "https://lb.example.com", false},
		{"HTTPS://LB.Example.com:8100/", "https://lb.example.com:8100", false},
		{"https://music.example.com/apis/listenbrainz", "https://music.example.com/apis/listenbrainz", false},
		{"https://music.example.com/apis/listenbrainz/1", "https://music.example.com/apis/listenbrainz", false},
		{"http://192.168.1.20:42010/apis/listenbrainz/", "http://192.168.1.20:42010/apis/listenbrainz", true},
	} {
		got, plain, err := ParseServer(c.in)
		if err != nil || got != c.want || plain != c.plain {
			t.Errorf("ParseServer(%q) = %q, %v, %v; want %q, %v", c.in, got, plain, err, c.want, c.plain)
		}
	}
	for _, in := range []string{"ftp://lb.example.com", "https://", "https://user:pass@lb.example.com", "https://lb.example.com/?token=x", "https://lb.example.com/#x", "not a server at all"} {
		if got, _, err := ParseServer(in); !errors.Is(err, ErrNotServer) {
			t.Errorf("ParseServer(%q) = %q, %v", in, got, err)
		}
	}
}

func TestLocal(t *testing.T) {
	resolve := func(_ context.Context, host string) ([]netip.Addr, error) {
		switch host {
		case "nas.example.com":
			return []netip.Addr{netip.MustParseAddr("192.168.1.5")}, nil
		case "split.example.com":
			return []netip.Addr{netip.MustParseAddr("10.0.0.2"), netip.MustParseAddr("93.184.215.14")}, nil
		case "public.example.com":
			return []netip.Addr{netip.MustParseAddr("93.184.215.14")}, nil
		}
		return nil, errors.New("no such host")
	}
	for _, c := range []struct {
		server string
		local  bool
	}{
		{"http://127.0.0.1:42010", true},
		{"http://[::1]:42010", true},
		{"http://localhost:42010", true},
		{"http://10.1.2.3", true},
		{"http://172.16.0.9", true},
		{"http://192.168.1.20:42010/apis/listenbrainz", true},
		{"http://169.254.10.10", true},
		{"http://[fd12:3456::1]", true},
		{"http://[fe80::1]", true},
		{"http://nas", true},
		{"http://maloja.local", true},
		{"http://maloja.lan", true},
		{"http://maloja.home.arpa", true},
		{"http://nas.example.com", true},
		{"http://split.example.com", false},
		{"http://public.example.com", false},
		{"http://gone.example.com", false},
		{"http://93.184.215.14", false},
		{"http://100.101.102.103", false},
		{"http://172.32.0.1", false},
	} {
		if got := Local(context.Background(), c.server, resolve); got != c.local {
			t.Errorf("Local(%q) = %v", c.server, got)
		}
	}
}

func TestPlausible(t *testing.T) {
	for _, c := range []struct {
		server, token string
		ok            bool
	}{
		{DefaultServer, "0f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0", true},
		{DefaultServer, "0F1E2D3C-4B5A-6978-8796-A5B4C3D2E1F0", true},
		{DefaultServer, "0f1e2d3c4b5a69788796a5b4c3d2e1f0", false},
		{DefaultServer, "my-password", false},
		{DefaultServer, "", false},
		{"http://192.168.1.20:42010/apis/listenbrainz", "maloja-key", true},
		{"http://192.168.1.20:42010/apis/listenbrainz", "", false},
	} {
		if got := Plausible(c.server, c.token); got != c.ok {
			t.Errorf("Plausible(%q, %q) = %v", c.server, c.token, got)
		}
	}
}
