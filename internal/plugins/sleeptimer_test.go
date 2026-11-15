package plugins

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// sleepTimerHost runs the community's Sleep Timer, as the JavaScript its
// TypeScript compiles to, with a queue of two tracks of a minute playing
// the first.
func sleepTimerHost(t *testing.T) *testHost {
	t.Helper()
	src := filepath.Join("..", "..", "community", "plugins", "sleep-timer")
	js, err := os.ReadFile(filepath.Join(src, "index.js"))
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("the sleep-timer plugin isn't compiled: CI runs bun run build:plugins before the tests")
		}
		t.Skip("the sleep-timer plugin isn't compiled: run bun run build:plugins")
	}
	manifest, err := os.ReadFile(filepath.Join(src, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	h := newTestHost(t)
	h.install("sleep-timer", string(manifest), map[string]string{"index.js": string(js)})
	h.enable("sleep-timer")
	h.wait("the commands", func() bool { return h.command("sleep-timer.track") != nil })
	tracks := []media.Track{
		{ID: "yt:1", Source: "youtube", Title: "One", DurationMs: 60_000},
		{ID: "yt:2", Source: "youtube", Title: "Two", DurationMs: 60_000},
	}
	h.p.PlayTracks(tracks, 0)
	h.wait("the first track to play", func() bool { return h.p.State().Playing })
	return h
}

func (h *testHost) command(id string) *Command {
	for _, c := range h.Commands() {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// The music pauses as the track nears its end, not as the next one starts:
// a pause does nothing while a track loads.
func TestSleepTimerStopsAtTheEnd(t *testing.T) {
	h := sleepTimerHost(t)
	h.command("sleep-timer.track").Run()
	h.wait("the timer's toast", func() bool { return h.ui.said("the music stops after this track") })

	h.p.Time(30*time.Second, 60*time.Second)
	time.Sleep(50 * time.Millisecond)
	if !h.p.State().Playing {
		t.Fatal("the music stopped in the middle of the track")
	}
	h.p.Time(59500*time.Millisecond, 60*time.Second)
	h.wait("the music to pause", func() bool { return !h.p.State().Playing })
	h.wait("the good night", func() bool { return h.ui.said("good night") })
	if cur, _ := h.p.State().Current(); cur.Track.ID != "yt:1" {
		t.Errorf("the player moved on to %q", cur.Track.ID)
	}
}

// A press of Next is not the end of the track: the timer waits for the end
// of the track that plays now.
func TestSleepTimerIgnoresNext(t *testing.T) {
	h := sleepTimerHost(t)
	h.command("sleep-timer.track").Run()
	h.wait("the timer's toast", func() bool { return h.ui.said("the music stops after this track") })

	h.p.Time(10*time.Second, 60*time.Second)
	h.p.Next()
	h.wait("the second track", func() bool {
		cur, _ := h.p.State().Current()
		return cur.Track.ID == "yt:2" && h.p.State().Playing
	})
	time.Sleep(50 * time.Millisecond)
	if h.ui.said("good night") || !h.p.State().Playing {
		t.Fatal("Next stopped the music")
	}
	h.p.Time(59500*time.Millisecond, 60*time.Second)
	h.wait("the music to pause", func() bool { return !h.p.State().Playing })
}
