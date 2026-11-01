package app

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/volume"
)

// offlineServices are the app's, but for finding streams: a track plays
// from its ID.
type offlineServices struct{ playerServices }

func (offlineServices) Prepare(_ context.Context, t media.Track) (player.Source, error) {
	return player.Source{Mode: player.ModeStream, URL: "url:" + t.ID, Track: t}, nil
}

func (s offlineServices) PrepareStream(ctx context.Context, t media.Track) (player.Source, error) {
	return s.Prepare(ctx, t)
}

// quietStream is an engine that plays at once and tells no progress.
type quietStream struct{ p atomic.Pointer[player.Player] }

func (s *quietStream) Load(_ string, _ time.Duration, autoplay bool) error {
	if autoplay {
		s.p.Load().Playing(true)
	}
	return nil
}
func (s *quietStream) Preload(string)                        {}
func (s *quietStream) Crossfade(string, time.Duration) error { return nil }
func (s *quietStream) Play() error                           { s.p.Load().Playing(true); return nil }
func (s *quietStream) Pause()                                { s.p.Load().Playing(false) }
func (s *quietStream) Stop()                                 {}
func (s *quietStream) Unload()                               {}
func (s *quietStream) Seek(time.Duration)                    {}
func (s *quietStream) SetVolume(float64)                     {}

// TestFlush quits right after the queue and the volume changed while
// paused: the files keep them, and the windows' settings are kept first.
func TestFlush(t *testing.T) {
	dir := t.TempDir()
	data := filepath.Join(dir, "data")
	a, err := New(Options{DataDir: data, CacheDir: filepath.Join(dir, "cache"), Builtin: builtin.FS, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	// Neither Discord nor a notification: the notification would wait for
	// MyGo's loop, which doesn't run here.
	st := a.settings.Get()
	st.DiscordPresence, st.Notifications = false, false
	if err := a.settings.Set(st); err != nil {
		t.Fatal(err)
	}
	if a.queue, err = store.Open(filepath.Join(data, "queue.json"), player.Saved{}); err != nil {
		t.Fatal(err)
	}
	stream := &quietStream{}
	p := player.New(offlineServices{playerServices{a}}, stream, connectDevice{a}, player.Saved{})
	stream.p.Store(p)
	a.goPlayer.Store(p)
	t.Cleanup(p.Close)
	// The main window saves the whole of the settings it holds, whose
	// volume may be older than the player's.
	windows := 0
	defer a.OnFlush(func() {
		windows++
		stale := a.settings.Get()
		stale.Volume = 0.8
		if err := a.settings.Set(stale); err != nil {
			t.Error(err)
		}
	})()

	track := func(id string) media.Track {
		return media.Track{ID: "yt:" + id, Source: media.SourceYouTube, Title: id, DurationMs: 200000}
	}
	until := func(what string, ok func(player.State) bool) {
		t.Helper()
		for deadline := time.Now().Add(5 * time.Second); !ok(p.State()); time.Sleep(time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("%s didn't happen: %+v", what, p.State())
			}
		}
	}
	p.PlayTracks([]media.Track{track("a"), track("b")}, 0)
	until("a playing", func(s player.State) bool { return s.Playing })
	p.Pause()
	until("paused", func(s player.State) bool { return !s.Playing })
	p.Enqueue([]media.Track{track("c")})
	p.SetVolume(0.4)
	until("c and the volume", func(s player.State) bool { return len(s.Queue) == 3 && s.Volume == 0.4 })

	a.flush()
	q, err := store.Open(filepath.Join(data, "queue.json"), player.Saved{})
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Get(); len(got.Tracks) != 3 || got.Index != 0 {
		t.Errorf("queue.json keeps %d tracks at %d, want 3 at 0", len(got.Tracks), got.Index)
	}
	s, err := store.Open(filepath.Join(data, "settings.json"), st)
	if err != nil {
		t.Fatal(err)
	}
	if v := s.Get().Volume; v != volume.Saved(0.4) {
		t.Errorf("settings.json keeps the volume %v, want %v", v, volume.Saved(0.4))
	}
	if windows != 1 {
		t.Errorf("the windows' savers ran %d times, want once", windows)
	}
}
