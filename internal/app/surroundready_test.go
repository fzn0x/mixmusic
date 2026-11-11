package app

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/online"
	"github.com/fzn0x/mixmusic/internal/player"
)

// surroundServices are the app's services, finding streams as the app
// does, whose plays stop being counted as the test ends.
type surroundServices struct{ endingServices }

func (s surroundServices) Prepare(ctx context.Context, t media.Track) (player.Source, error) {
	return s.playerServices.PrepareStream(ctx, t)
}

func (s surroundServices) PrepareStream(ctx context.Context, t media.Track) (player.Source, error) {
	return s.playerServices.PrepareStream(ctx, t)
}

// readyStream is an engine that plays at once, as quietStream, and keeps
// what it was given to load: the path, from where, and of 5.1 audio,
// whether the proxy served it at once then.
type readyStream struct {
	quietStream
	a     *App
	mu    sync.Mutex
	loads []string
}

func (s *readyStream) Load(rawURL string, cue audio.Cue) error {
	load := strings.TrimPrefix(rawURL, schemeURL(streamScheme)) + " " + cue.Start.String()
	if videoID, surround, err := videoOf(rawURL); err == nil && surround {
		if s.a.proxy.SurroundReady(videoID) {
			load += " ready"
		} else {
			load += " not ready"
		}
	}
	s.mu.Lock()
	s.loads = append(s.loads, load)
	s.mu.Unlock()
	return s.quietStream.Load(rawURL, cue)
}

func (s *readyStream) loaded() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.loads)
}

// TestSurroundStartsInStereo plays, with 5.1 on, a track whose 5.1 audio
// yt-dlp found beside its stereo audio, which YouTube serves only a moment
// later, as it does a URL yt-dlp just found: the engine plays its stereo
// audio meanwhile, and its 5.1 audio from where the track is once the
// proxy serves that at once. So does turning 5.1 on mid-track (#457).
func TestSurroundStartsInStereo(t *testing.T) {
	a := newQuietApp(t, nil)
	// No request leaves the computer: yt-dlp alone finds streams, as the
	// test answers, and nothing asks how loud they are.
	a.resolver.Direct = nil
	a.loudness = nil
	a.online = online.New(func(context.Context) error { return nil })
	const wait = 1200 * time.Millisecond
	a.resolver.Run = func(context.Context, ...string) ([]byte, error) {
		expire := time.Now().Add(6 * time.Hour).Unix()
		return fmt.Appendf(nil, `{"url": "https://example.com/opus?expire=%d", "ext": "webm", "acodec": "opus", "filesize": 1000,
			"formats": [{"format_id": "380", "url": "https://example.com/ac3?expire=%d", "ext": "m4a", "protocol": "https",
			"acodec": "ac-3", "vcodec": "none", "abr": 384, "audio_channels": 6, "filesize": 9000, "available_at": %.3f}]}`,
			expire, expire, float64(time.Now().Add(wait).UnixMilli())/1000), nil
	}
	st := a.settings.Get()
	st.Surround, st.SkipAI, st.Autoplay, st.Crossfade = true, false, false, 0
	if err := a.settings.Set(st); err != nil {
		t.Fatal(err)
	}
	stream := &readyStream{a: a}
	svc := surroundServices{endingServices{offlineServices{playerServices{a}}, &plays{}}}
	p := player.New(svc, stream, connectDevice{a}, player.Saved{})
	stream.p.Store(p)
	a.goPlayer.Store(p)
	t.Cleanup(func() {
		p.Close()
		p.Flush()
		svc.plays.end()
	})

	// A cold play: stereo at once, then 5.1 from where it is, served at
	// once. The next track isn't found yet, before a second played.
	const cold, later = "sssssssss51", "ttttttttt51"
	p.PlayTracks([]media.Track{ytTrack(cold), ytTrack(later)}, 0)
	waitPlayer(t, p, "the track playing", func(s player.State) bool { return s.Playing && s.Source != nil })
	if got := stream.loaded(); len(got) != 1 || got[0] != "yt/"+cold+" 0s" {
		t.Fatalf("with its 5.1 audio not served yet, the engine loaded %q", got)
	}
	p.Time(900*time.Millisecond, 200*time.Second)
	eventually(t, "the 5.1 audio", func() bool { return len(stream.loaded()) > 1 })
	if got := stream.loaded(); len(got) != 2 || got[1] != "yt/"+cold+"/5.1 900ms ready" {
		t.Fatalf("the engine loaded %q", got)
	}

	// 5.1 turned on mid-track: stereo plays on until the 5.1 audio is
	// served at once.
	st.Surround = false
	if err := a.settings.Set(st); err != nil {
		t.Fatal(err)
	}
	p.SettingsChanged(a.settings.Get().Clean())
	p.Next()
	waitPlayer(t, p, "the next track playing", func(s player.State) bool {
		return s.Playing && s.Source != nil && strings.HasSuffix(s.Source.URL, later)
	})
	p.Time(12800*time.Millisecond, 200*time.Second)
	waitPlayer(t, p, "the time told", func(s player.State) bool { return s.Position == 12800*time.Millisecond })
	st.Surround = true
	if err := a.settings.Set(st); err != nil {
		t.Fatal(err)
	}
	p.SettingsChanged(a.settings.Get().Clean())
	eventually(t, "the next track's 5.1 audio", func() bool { return strings.HasPrefix(stream.loaded()[len(stream.loaded())-1], "yt/"+later+"/5.1") })
	got := stream.loaded()
	if want := []string{"yt/" + later + " 0s", "yt/" + later + "/5.1 12.8s ready"}; !slices.Equal(got[len(got)-2:], want) {
		t.Errorf("the engine loaded %q", got)
	}
}
