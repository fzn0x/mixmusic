package main

import (
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
)

// listenOn returns the option that has the server of overlays listen on
// 127.0.0.1 at port, 0 for one the system picks, whatever the settings
// ask, as tests listen nowhere else.
func listenOn(port int) func(o *app.Options) {
	return func(o *app.Options) {
		o.OverlayListen = func(network, addr string) (net.Listener, error) {
			return net.Listen(network, "127.0.0.1:"+strconv.Itoa(port))
		}
	}
}

// TestOverlayRows shows the switch of Now playing for overlays off at
// first, with nothing else; on, it shows the port and the addresses to
// copy, and copies them (#344).
func TestOverlayRows(t *testing.T) {
	const title, label = "Now playing for overlays", "Serve what plays to overlays"
	s := newTestShellOf(t, listenOn(0))
	tt := testerOf(t, s, 1280, 2800)
	show(s, tt, "/settings/desktop")
	if !tt.HasText(title) {
		t.Fatalf("Settings › Desktop doesn't show the row: %q", tt.Texts())
	}
	if tt.HasText("Browser source address") || tt.HasText("Overlay port") {
		t.Error("off, the rows of the port and the addresses show")
	}
	if s.svc.System.OverlayStatus().On {
		t.Fatal("the server listens at first")
	}
	click(t, s, tt, label)
	s.drain()
	tt.Frame()
	st := s.svc.System.OverlayStatus()
	if !s.svc.Preferences.Get().OverlayServer || !st.On {
		t.Fatalf("the switch didn't turn the server on: %+v, set %v", st, s.svc.Preferences.Get().OverlayServer)
	}
	for _, text := range []string{"Overlay port", "Browser source address", "http://" + st.Addr + "/overlay",
		"Data address", "http://" + st.Addr + "/now-playing"} {
		if !tt.HasText(text) {
			t.Errorf("on, the page doesn't show %q: %q", text, tt.Texts())
		}
	}
	var copied string
	was := copyText
	copyText = func(text string) { copied = text }
	t.Cleanup(func() { copyText = was })
	click(t, s, tt, "copy")
	if !strings.HasPrefix(copied, "http://127.0.0.1:") {
		t.Errorf("copied %q, want an address on 127.0.0.1", copied)
	}
	show(s, tt, settingsPath("", "obs"))
	if !tt.HasText(title) {
		t.Errorf("searching obs doesn't find the row: %q", tt.Texts())
	}
	show(s, tt, "/settings/desktop")
	click(t, s, tt, label)
	s.drain()
	tt.Frame()
	if s.svc.System.OverlayStatus().On || tt.HasText("Browser source address") {
		t.Error("turned off, the server listens or the addresses show")
	}
}

// TestOverlayPortInUse says so, in the page, when another program holds
// the port.
func TestOverlayPortInUse(t *testing.T) {
	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	s := newTestShellOf(t, listenOn(held.Addr().(*net.TCPAddr).Port))
	tt := testerOf(t, s, 1280, 2800)
	show(s, tt, "/settings/desktop")
	click(t, s, tt, "Serve what plays to overlays")
	s.drain()
	tt.Frame()
	if !tt.HasText("can't be used") || tt.HasText("Browser source address") {
		t.Errorf("a port in use isn't told: %q", tt.Texts())
	}
}
