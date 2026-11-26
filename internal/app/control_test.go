package app

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/overlay"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/player/playertest"
)

// controlApp is an app whose programs' control plays on p, which is made
// with it.
func controlApp(t *testing.T) (*App, *player.Player) {
	t.Helper()
	var p *player.Player
	dir := t.TempDir()
	a, err := New(Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"),
		Version: "test", ProblemWait: -1,
		Remote: &RemoteOptions{Player: func() *player.Player { return p }},
		OverlayListen: func(network, addr string) (net.Listener, error) {
			return net.Listen(network, "127.0.0.1:0")
		}})
	if err != nil {
		t.Fatal(err)
	}
	p = player.New(songsServices{}, playertest.Stream{}, playertest.Device{}, player.Saved{})
	t.Cleanup(func() {
		a.stopOverlay()
		p.Close()
		a.Close()
	})
	return a, p
}

// waitControl waits for the player to hold what cond tells.
func waitControl(t *testing.T, p *player.Player, what string, cond func(player.State) bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if cond(p.State()) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s: %+v", what, p.State())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestControlFileKept shows control.json keeps the token and the port,
// which only the user may read.
func TestControlFileKept(t *testing.T) {
	dir := t.TempDir()
	token, err := newControlToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 64 || strings.Trim(token, "0123456789abcdef") != "" {
		t.Fatalf("the token %q", token)
	}
	if err := writeControlFile(dir, controlFile{Port: 8284, Token: token}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ControlFileName)
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows keeps the permissions of a file as access control lists,
	// which a mode doesn't tell.
	if runtime.GOOS != "windows" {
		if got := st.Mode().Perm(); got != 0o600 {
			t.Errorf("control.json is readable as %o, want 600", got)
		}
	}
	got, err := readControlFile(dir)
	if err != nil || got.Port != 8284 || got.Token != token {
		t.Fatalf("read %+v, %v", got, err)
	}
	// A file that names no token, or no port, is no file.
	for _, bad := range []controlFile{{Port: 8284, Token: "short"}, {Port: 80, Token: token}, {Port: 8284}} {
		data, _ := json.Marshal(bad)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readControlFile(dir); err == nil {
			t.Errorf("a file of %+v was read", bad)
		}
	}
}

// TestControlCommandsThePlayer shows the routes' commands reaching the
// player, and the state that follows.
func TestControlCommandsThePlayer(t *testing.T) {
	a, p := controlApp(t)
	h := controlHost{a}
	one := controlSong("yt:1", "One")
	two := controlSong("yt:2", "Two")
	p.PlayTracks([]media.Track{one, two}, 0)
	waitControl(t, p, "the first track", func(st player.State) bool {
		cur, ok := st.Current()
		return ok && cur.Track.ID == one.ID
	})

	ex := h.Extra()
	if ex.Volume < 0 || ex.Volume > 100 {
		t.Errorf("the volume %d", ex.Volume)
	}
	if len(ex.Queue) != 1 || ex.Queue[0].ID != two.ID || !ex.CanNext {
		t.Errorf("the queue %+v, can next %v", ex.Queue, ex.CanNext)
	}
	if ex.Repeat != "off" {
		t.Errorf("the repeat %q", ex.Repeat)
	}

	if err := h.Do(overlay.Command{Op: "volume", Percent: new(40.0)}); err != nil {
		t.Fatal(err)
	}
	if got := h.Extra().Volume; got != 40 {
		t.Errorf("the volume after 40%%: %d", got)
	}
	if err := h.Do(overlay.Command{Op: "mute"}); err != nil {
		t.Fatal(err)
	}
	if !h.Extra().Muted {
		t.Error("mute didn't mute")
	}
	if err := h.Do(overlay.Command{Op: "repeat", Repeat: "one"}); err != nil {
		t.Fatal(err)
	}
	if got := h.Extra().Repeat; got != "one" {
		t.Errorf("the repeat after one: %q", got)
	}
	if err := h.Do(overlay.Command{Op: "repeat"}); err != nil {
		t.Fatal(err)
	}
	if got := h.Extra().Repeat; got == "one" {
		t.Errorf("a cycle left the repeat at %q", got)
	}
	// A seek is taken as the seconds it names, which a track still loading
	// holds back as it holds every move.
	if err := h.Do(overlay.Command{Op: "seek", Position: new(30.0)}); err != nil {
		t.Fatal(err)
	}
	if err := h.Do(overlay.Command{Op: "seek", By: new(-25.0)}); err != nil {
		t.Fatal(err)
	}
	// The track the queue holds plays, and one it doesn't says why it can't.
	if err := h.Do(overlay.Command{Op: "track", ID: two.ID}); err != nil {
		t.Fatal(err)
	}
	waitControl(t, p, "the second track", func(st player.State) bool {
		cur, ok := st.Current()
		return ok && cur.Track.ID == two.ID
	})
	if err := h.Do(overlay.Command{Op: "track", ID: "yt:zzz"}); !errors.Is(err, overlay.ErrNoTrack) {
		t.Errorf("a track in no queue: %v", err)
	}
	if err := h.Do(overlay.Command{Op: "like"}); err != nil {
		t.Fatal(err)
	}
	if err := h.Do(overlay.Command{Op: "dislike"}); err != nil {
		t.Fatal(err)
	}
}

// TestControlWithoutAPlayer shows a command that can't run saying why.
func TestControlWithoutAPlayer(t *testing.T) {
	dir := t.TempDir()
	a, err := New(Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"),
		Version: "test", ProblemWait: -1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	h := controlHost{a}
	if err := h.Do(overlay.Command{Op: "play"}); !errors.Is(err, overlay.ErrNoPlayer) {
		t.Errorf("no player: %v", err)
	}
	if ex := h.Extra(); ex.Volume != 0 || len(ex.Queue) != 0 {
		t.Errorf("no player tells %+v", ex)
	}
}

// TestControlSwitchServes shows the switch turning the server on, with a
// token kept where a script finds it.
func TestControlSwitchServes(t *testing.T) {
	a, p := controlApp(t)
	s := a.settings.Get().Clean()
	s.Overlay.ControlServer = true
	if err := a.settings.Set(s); err != nil {
		t.Fatal(err)
	}
	a.applyOverlay(a.settings.Get().Clean())
	st := a.Services().System.ControlStatus()
	if !st.On || !st.Control {
		t.Fatalf("the server %+v", st)
	}
	t.Cleanup(func() { a.stopOverlay() })
	f, err := readControlFile(a.opts.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	if f.Port != s.OverlayPort {
		t.Errorf("control.json names port %d, want %d", f.Port, s.OverlayPort)
	}
	// The state answers with the token, and refuses without.
	resp, err := http.Get("http://" + st.Addr + "/control/state")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no token: %d", resp.StatusCode)
	}
	req, _ := http.NewRequest("GET", "http://"+st.Addr+"/control/state", nil)
	req.Header.Set("Authorization", "Bearer "+f.Token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var state map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("with the token: %d %v", resp.StatusCode, state)
	}
	if _, ok := state["volume"]; !ok {
		t.Errorf("the state has no volume: %v", state)
	}
	// The player's own changes reach the state: the queue it holds, and
	// what /now-playing tells of it, which the app reports.
	p.PlayTracks([]media.Track{controlSong("yt:1", "One"), controlSong("yt:2", "Two")}, 0)
	waitControl(t, p, "the track", func(st player.State) bool {
		cur, ok := st.Current()
		return ok && cur.Track.ID == "yt:1"
	})
	a.reportOverlay(NowPlaying{Track: &media.Track{ID: "yt:1", Source: media.SourceYouTube, Title: "One",
		DurationMs: 200000}, Playing: true})
	req, _ = http.NewRequest("GET", "http://"+st.Addr+"/control/state", nil)
	req.Header.Set("Authorization", "Bearer "+f.Token)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if err := json.NewDecoder(resp2.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	queue, _ := state["queue"].([]any)
	if state["title"] != "One" || len(queue) != 1 {
		t.Errorf("the state after the player: %v", state)
	}
}

// controlSong is a track of YouTube Music.
func controlSong(id, title string) media.Track {
	return media.Track{ID: id, Source: media.SourceYouTube, Title: title, DurationMs: 200000,
		Artists: []media.Ref{{Name: "Someone"}}}
}
