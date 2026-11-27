package app

import (
	"bufio"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/player/playertest"
)

// controlEvents opens /control/events and sends the data of each event.
func controlEvents(t *testing.T, addr, token string) <-chan string {
	t.Helper()
	req, _ := http.NewRequest("GET", "http://"+addr+"/control/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("events: %d", resp.StatusCode)
	}
	t.Cleanup(func() { resp.Body.Close() })
	out := make(chan string, 64)
	go func() {
		defer close(out)
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			if d, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
				out <- d
			}
		}
	}()
	return out
}

// eventWithin reads the next event, or says none came.
func eventWithin(ch <-chan string, d time.Duration) (string, bool) {
	select {
	case s, ok := <-ch:
		return s, ok
	case <-time.After(d):
		return "", false
	}
}

// TestControlEventsAfterRestart shows the stream following the player when
// control was left on and the server started before the player exists,
// and when the player changed what control tells from the app itself:
// shuffle, the volume, and the mute (#591's review).
func TestControlEventsAfterRestart(t *testing.T) {
	for _, onBeforePlayer := range []bool{false, true} {
		name := "switched on after the player"
		if onBeforePlayer {
			name = "on at start, before the player"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			data := filepath.Join(dir, "data")
			if err := os.MkdirAll(data, 0o700); err != nil {
				t.Fatal(err)
			}
			if onBeforePlayer {
				writeSettings(t, filepath.Join(data, "settings.json"), `{"controlServer": true}`)
			}
			var p *player.Player
			a, err := New(Options{DataDir: data, CacheDir: filepath.Join(dir, "cache"),
				Version: "test", ProblemWait: -1,
				Remote: &RemoteOptions{Player: func() *player.Player { return p }},
				OverlayListen: func(network, addr string) (net.Listener, error) {
					return net.Listen(network, "127.0.0.1:0")
				}})
			if err != nil {
				t.Fatal(err)
			}
			p = player.New(songsServices{}, playertest.Stream{}, playertest.Device{}, player.Saved{})
			t.Cleanup(func() { a.stopOverlay(); p.Close(); a.Close() })
			if !onBeforePlayer {
				s := a.settings.Get().Clean()
				s.Overlay.ControlServer = true
				if err := a.settings.Set(s); err != nil {
					t.Fatal(err)
				}
				a.applyOverlay(a.settings.Get().Clean())
			}
			st := a.Services().System.ControlStatus()
			if !st.On || !st.Control {
				t.Fatalf("server %+v", st)
			}
			f, err := readControlFile(data)
			if err != nil {
				t.Fatal(err)
			}
			ch := controlEvents(t, st.Addr, f.Token)
			if _, ok := eventWithin(ch, 5*time.Second); !ok {
				t.Fatal("no first state")
			}
			p.ToggleShuffle() // a change made in the app, not over /control
			got, ok := eventWithin(ch, 3*time.Second)
			if !ok {
				t.Fatal("no event after the player's shuffle changed")
			}
			if !strings.Contains(got, `"shuffle":true`) {
				t.Errorf("event %s", got)
			}
			p.SetVolume(0.3)
			if got, ok := eventWithin(ch, 3*time.Second); !ok {
				t.Error("no event after the player's volume changed")
			} else if !strings.Contains(got, `"volume":30`) {
				t.Errorf("volume event %s", got)
			}
			p.ToggleMute()
			if got, ok := eventWithin(ch, 3*time.Second); !ok {
				t.Error("no event after the player's mute changed")
			} else if !strings.Contains(got, `"muted":true`) {
				t.Errorf("mute event %s", got)
			}
		})
	}
}
