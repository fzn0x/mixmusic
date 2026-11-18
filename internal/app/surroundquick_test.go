package app

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/online"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestSurroundAfterTheQuickRun plays, with 5.1 on, a track whose stream
// yt-dlp's quick run found, which lists no 5.1 audio: the track starts in
// stereo, and plays the 5.1 audio the full run then finds behind it, from
// where it is. With 5.1 off, the quick run alone runs (#540).
func TestSurroundAfterTheQuickRun(t *testing.T) {
	for _, on := range []bool{true, false} {
		a := newQuietApp(t, nil)
		// No request leaves the computer: yt-dlp alone finds streams, as the
		// test answers, and nothing asks how loud they are.
		a.resolver.Direct = nil
		a.resolver.Tools = nil
		a.loudness = nil
		a.online = online.New(func(context.Context) error { return nil })
		// The full run answers once the track started.
		full, release := make(chan struct{}, 4), make(chan struct{})
		var once sync.Once
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		a.resolver.Run = func(ctx context.Context, args ...string) ([]byte, error) {
			expire, formats := time.Now().Add(6*time.Hour).Unix(), ""
			if !slices.Contains(args, "--extractor-args") {
				full <- struct{}{}
				select {
				case <-release:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				formats = fmt.Sprintf(`{"format_id": "380", "url": "https://example.com/ac3?expire=%d", "ext": "m4a",
					"protocol": "https", "acodec": "ac-3", "vcodec": "none", "abr": 384, "audio_channels": 6, "filesize": 9000}`, expire)
			}
			return fmt.Appendf(nil, `{"url": "https://example.com/opus?expire=%d", "ext": "webm", "acodec": "opus", "filesize": 1000,
				"formats": [%s]}`, expire, formats), nil
		}
		if _, err := (&Preferences{a}).Update(func(s *settings.Settings) {
			s.Surround, s.SkipAI, s.Autoplay, s.Crossfade = on, false, false, 0
		}); err != nil {
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

		const id = "qqqqqqqqq51"
		p.PlayTracks([]media.Track{ytTrack(id)}, 0)
		waitPlayer(t, p, "the track playing", func(s player.State) bool { return s.Playing && s.Source != nil })
		p.Time(900*time.Millisecond, 200*time.Second)
		if got := stream.loaded(); len(got) != 1 || got[0] != "yt/"+id+" 0s" {
			t.Fatalf("as the track started, the engine loaded %q", got)
		}
		once.Do(func() { close(release) })
		if !on {
			time.Sleep(100 * time.Millisecond)
			if got := stream.loaded(); len(got) != 1 || got[0] != "yt/"+id+" 0s" || len(full) != 0 || p.State().Source.SurroundURL != "" {
				t.Errorf("with 5.1 off, the engine loaded %q, after %d full runs of yt-dlp", got, len(full))
			}
			continue
		}
		eventually(t, "the 5.1 audio", func() bool { return len(stream.loaded()) > 1 })
		if got := stream.loaded(); len(got) != 2 || got[0] != "yt/"+id+" 0s" || !strings.HasPrefix(got[1], "yt/"+id+"/5.1 900ms") || len(full) != 1 {
			t.Errorf("the engine loaded %q, after %d full runs of yt-dlp", got, len(full))
		}
	}
}
