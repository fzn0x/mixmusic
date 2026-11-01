package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/nowplaying"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestSpeedRanges has the engine, the settings and the system's controls
// offer the same speeds.
func TestSpeedRanges(t *testing.T) {
	if audio.MinSpeed != settings.MinSpeed || audio.MaxSpeed != settings.MaxSpeed ||
		nowplaying.MinRate != settings.MinSpeed || nowplaying.MaxRate != settings.MaxSpeed {
		t.Errorf("the engine plays from %v to %v, the settings from %v to %v, the system's controls from %v to %v",
			audio.MinSpeed, audio.MaxSpeed, settings.MinSpeed, settings.MaxSpeed, nowplaying.MinRate, nowplaying.MaxRate)
	}
}

// TestControlsRate tells the system's controls the speed, and keeps the
// one they ask as the songs' speed.
func TestControlsRate(t *testing.T) {
	np := NowPlaying{Track: &media.Track{ID: "yt:1", Title: "Song"}, Playing: true, Speed: 1.5}
	if info := controlsInfo(np, time.Now()); info.Rate != 1.5 {
		t.Errorf("rate %v", info.Rate)
	}
	np.Speed = 0
	if info := controlsInfo(np, time.Now()); info.Rate != 1 {
		t.Errorf("rate %v without a speed", info.Rate)
	}

	dir := t.TempDir()
	a, err := New(Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"), Builtin: builtin.FS, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ rate, want float64 }{{1.33, 1.35}, {0.5, 0.5}, {7, 3}} {
		a.controlsAsked(nowplaying.Request{Command: nowplaying.SetRate, Rate: c.rate})
		if got := a.settings.Get().SongSpeed; got != c.want {
			t.Errorf("a rate of %v keeps the speed %v, want %v", c.rate, got, c.want)
		}
	}
}
