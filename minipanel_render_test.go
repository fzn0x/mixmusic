package main

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/cast"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// TestRenderMiniPanel draws the mini player with each of its panels, below
// it in the window it opens in and above it in its narrowest at the
// screen's foot, in Mix and Daylight, to MIXMUSIC_RENDER, for a look.
func TestRenderMiniPanel(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	for _, theme := range []string{"mix", "daylight"} {
		for _, place := range []struct {
			name       string
			x, y, w, h int
		}{
			{"below", 100, 100, 420, 132},
			{"above", 100, workArea.Height - 112, 340, 112},
		} {
			for _, panel := range []string{"Queue", "Lyrics", "Output device", "Why it couldn't play"} {
				s := renderMiniShell(t, theme, panel == "Why it couldn't play")
				tt, f := miniAt(t, s, place.x, place.y, place.w, place.h)
				if panel == "Why it couldn't play" {
					// After the mini player opened, which tells of it.
					s.p.PlayTracks([]media.Track{brokenTrack}, 0)
					until(t, s, tt, "the error's button", func() bool { _, ok := tt.Find(panel); return ok })
				}
				click(t, s, tt, panel)
				f.settle(t, s)
				if panel == "Lyrics" {
					s.p.Time(2500*time.Millisecond, 211*time.Second)
					s.settle(t, func(st player.State) bool { return st.Position == 2500*time.Millisecond })
				}
				if panel == "Output device" {
					until(t, s, tt, "the devices", func() bool { f.run(); return tt.HasText("Headphones") })
				}
				// The progress under the pointer.
				if r, ok := tt.Find("Seek"); ok && panel == "Queue" {
					tt.Move(r.X+r.W*0.6, r.Y+r.H/2)
				}
				renderTo(t, s, tt, dir, "mini-"+theme+"-"+place.name+"-"+strings.ToLower(strings.Fields(panel)[0]))
			}
		}
	}
}

// renderMiniShell is a mini player's shell in theme, with covers, output
// and cast devices, playing the test's tracks a minute in; or, failing,
// a player whose tracks can't play.
func renderMiniShell(t *testing.T, theme string, failing bool) *shell {
	t.Helper()
	outputs := []app.OutputDevice{{ID: "hp", Name: "Headphones (Arctis Nova 7)"}, {ID: "sp", Name: "Speakers (Realtek Audio)", Default: true}}
	s := miniShell(t, outputs, cast.Device{ID: "tv", Name: "Living Room TV", Model: "Chromecast", Host: "192.0.2.1", Port: 8009})
	if failing {
		s = newTestShellWith(t, func(o *app.Options) {
			o.CastBrowse = func(context.Context) ([]cast.Device, error) { return nil, nil }
		}, func(a *app.App) player.Services { return failingPlayer{testPlayer{a: a}} }, player.Saved{})
	}
	base := coverServer(t, s)
	wear(s, theme, nil)
	tracks := make([]media.Track, len(testTracks))
	for i, tr := range testTracks {
		tr.Artwork = []media.Image{{URL: base + "/cover/" + strconv.Itoa(i) + ".png", Width: 300, Height: 300}}
		tracks[i] = tr
	}
	if failing {
		return s
	}
	s.p.PlayTracks(tracks, 0)
	s.settle(t, func(st player.State) bool {
		cur, ok := st.Current()
		return ok && len(cur.Track.Artwork) > 0 && st.Playing && !st.Loading && st.Position > 0
	})
	s.p.Time(time.Minute, 211*time.Second)
	s.settle(t, func(st player.State) bool { return st.Position == time.Minute })
	return s
}
