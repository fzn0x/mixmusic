package app

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/store"
)

// newQuietApp is an app in a folder of its own, with options set changes,
// whose player plays at once and silently. Neither Discord nor
// notifications: a notification would wait for MyGo's loop, which doesn't
// run here.
func newQuietApp(t *testing.T, set func(o *Options)) *App {
	t.Helper()
	dir := t.TempDir()
	opts := Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"), Builtin: builtin.FS, Version: "test"}
	if set != nil {
		set(&opts)
	}
	a, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	st := a.settings.Get()
	st.DiscordPresence, st.Notifications = false, false
	if err := a.settings.Set(st); err != nil {
		t.Fatal(err)
	}
	if a.queue, err = store.Open(filepath.Join(opts.DataDir, "queue.json"), player.Saved{}); err != nil {
		t.Fatal(err)
	}
	stream := &quietStream{}
	p := player.New(offlineServices{playerServices{a}}, stream, connectDevice{a}, player.Saved{})
	stream.p.Store(p)
	a.goPlayer.Store(p)
	// The player writes the queue beside its goroutine, which Close doesn't
	// wait for: Flush does, before the folder goes.
	t.Cleanup(func() {
		p.Close()
		p.Flush()
	})
	return a
}

// waitPlayer waits for the player's state to come to what ok wants.
func waitPlayer(t *testing.T, p *player.Player, what string, ok func(player.State) bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !ok(p.State()); {
		if time.Now().After(deadline) {
			t.Fatalf("%s didn't happen: %+v", what, p.State())
		}
		time.Sleep(time.Millisecond)
	}
}

// TestCommandVolumeAndMute steps the volume and mutes from outside the
// window, as global shortcuts do.
func TestCommandVolumeAndMute(t *testing.T) {
	a := newQuietApp(t, nil)
	p := a.GoPlayer()
	p.SetVolume(0.5)
	a.command(PlayerCommand{Command: CommandVolumeBy, Value: 0.05})
	a.command(PlayerCommand{Command: CommandVolumeBy, Value: 0.05})
	waitPlayer(t, p, "two steps up", func(s player.State) bool { return math.Abs(s.Volume-0.6) < 1e-9 })
	a.command(PlayerCommand{Command: CommandMute})
	waitPlayer(t, p, "muted", func(s player.State) bool { return s.Muted })
	a.command(PlayerCommand{Command: CommandVolumeBy, Value: -0.05})
	waitPlayer(t, p, "a step down, unmuted", func(s player.State) bool { return !s.Muted && math.Abs(s.Volume-0.55) < 1e-9 })
	a.command(PlayerCommand{Command: CommandVolumeBy, Value: -5})
	waitPlayer(t, p, "all the way down", func(s player.State) bool { return s.Volume == 0 })
}

// TestLikedNote tells what a like from outside the window did.
func TestLikedNote(t *testing.T) {
	track := media.Track{Title: "One More Time", Artists: []media.Ref{{Name: "Daft Punk"}}}
	if title, body := likedNote(track, true); title != "Added to liked songs" || body != "One More Time — Daft Punk" {
		t.Errorf("liked: %q, %q", title, body)
	}
	if title, body := likedNote(media.Track{Title: "Intro"}, false); title != "Removed from liked songs" || body != "Intro" {
		t.Errorf("no longer liked: %q, %q", title, body)
	}
}
