package app

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// newOverlayApp is an app without a player, whose reports are the test's
// alone, with the server of overlays listening on port of 127.0.0.1 where
// the system picks it, 0.
func newOverlayApp(t *testing.T, port int) *App {
	t.Helper()
	dir := t.TempDir()
	a, err := New(Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"),
		Builtin: builtin.FS, Version: "test",
		OverlayListen: func(network, addr string) (net.Listener, error) {
			return net.Listen(network, "127.0.0.1:"+strconv.Itoa(port))
		}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a
}

// TestOverlayFollowsSettingsAndPlayer serves nothing until the settings
// turn it on, tells what the player reports, and stops once they turn it
// off (#344).
func TestOverlayFollowsSettingsAndPlayer(t *testing.T) {
	a := newOverlayApp(t, 0)
	sys := &System{a}
	if st := sys.OverlayStatus(); st.On {
		t.Fatalf("a new install serves overlays: %+v", st)
	}
	prefs := &Preferences{a}
	if _, err := prefs.Update(func(s *settings.Settings) { s.OverlayServer = true }); err != nil {
		t.Fatal(err)
	}
	st := sys.OverlayStatus()
	if !st.On || !strings.HasPrefix(st.Addr, "127.0.0.1:") {
		t.Fatalf("status %+v, want on at 127.0.0.1", st)
	}
	track := media.Track{ID: "yt:abc", Title: "Song\nTitle", Artists: []media.Ref{{Name: "A"}, {Name: "B"}},
		Album: &media.Ref{Name: "Album"}, DurationMs: 180000}
	(&Player{a}).Report(NowPlaying{Track: &track, Playing: true, PositionMs: 5000})
	var got struct {
		Active, Playing bool
		Title, Artist   string
		Album, Source   string
		DurationMs      int
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := http.Get("http://" + st.Addr + "/now-playing")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		got.Title = ""
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if got.Title != "" || time.Now().After(deadline) {
			t.Logf("%s", body)
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !got.Active || !got.Playing || got.Title != "Song Title" || got.Artist != "A, B" ||
		got.Album != "Album" || got.Source != "youtube" || got.DurationMs != 180000 {
		t.Errorf("served %+v", got)
	}
	if _, err := prefs.Update(func(s *settings.Settings) { s.OverlayServer = false }); err != nil {
		t.Fatal(err)
	}
	if sys.OverlayStatus().On {
		t.Error("still on after turned off")
	}
	if _, err := http.Get("http://" + st.Addr + "/now-playing"); err == nil {
		t.Error("the server answers after turned off")
	}
}

// TestOverlayPortInUse tells that a port another program holds can't be
// used, in the status the settings page shows.
func TestOverlayPortInUse(t *testing.T) {
	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	port := held.Addr().(*net.TCPAddr).Port
	a := newOverlayApp(t, port)
	sys := &System{a}
	if _, err := (&Preferences{a}).Update(func(s *settings.Settings) { s.OverlayServer = true }); err != nil {
		t.Fatal(err)
	}
	if st := sys.OverlayStatus(); st.On || st.Err == "" {
		t.Errorf("status %+v, want the port's problem", st)
	}
}
