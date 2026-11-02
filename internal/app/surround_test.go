package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/stream"
)

// testSettings returns a settings file of a new install's settings, as fn
// changes them, in a temporary folder.
func testSettings(t *testing.T, fn ...func(*settings.Settings)) *store.File[settings.Settings] {
	t.Helper()
	s := settings.Default()
	for _, f := range fn {
		f(&s)
	}
	file, err := store.Open(filepath.Join(t.TempDir(), "settings.json"), s)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

// surroundYTDLP is what yt-dlp prints of a video of stereo Opus and, when
// surround, 5.1 AC-3 too.
func surroundYTDLP(surround bool) []byte {
	expire := time.Now().Add(6 * time.Hour).Unix()
	formats := ""
	if surround {
		formats = fmt.Sprintf(`{"format_id": "380", "url": "https://example.com/ac3?expire=%d", "ext": "m4a", "protocol": "https",
			"acodec": "ac-3", "vcodec": "none", "abr": 384, "audio_channels": 6, "filesize": 9000}`, expire)
	}
	return fmt.Appendf(nil, `{"url": "https://example.com/audio?expire=%d", "ext": "webm", "acodec": "opus", "filesize": 1000, "formats": [%s]}`,
		expire, formats)
}

// TestPrepareStreamTellsSurround gives the URL of a track's 5.1 audio
// beside its stereo audio's, when YouTube has some: from a stream found
// now or before, from the cache, or, with 5.1 on, asked for a track cached
// in stereo while the network answers.
func TestPrepareStreamTellsSurround(t *testing.T) {
	n := newNetTest(t)
	n.net.Store(true)
	n.ytdlp.Store(true)
	ctx := context.Background()
	videos := map[string]bool{"bbbbbbbbbbb": true, "ccccccccccc": false, "ddddddddddd": true, "eeeeeeeee5e": true}
	n.p.a.resolver.Run = func(_ context.Context, args ...string) ([]byte, error) {
		n.runs.Add(1)
		id := args[len(args)-1][len(args[len(args)-1])-11:]
		return surroundYTDLP(videos[id]), nil
	}
	surroundURL := func(id string) string {
		return schemeURL(streamScheme) + strings.TrimPrefix(stream.SurroundPath(id), "/")
	}
	for i := range 2 { // found now, then found before
		for _, id := range []string{"bbbbbbbbbbb", "ccccccccccc"} {
			src, err := n.p.PrepareStream(ctx, ytTrack(id))
			want := ""
			if videos[id] {
				want = surroundURL(id)
			}
			if err != nil || src.SurroundURL != want || src.Cached || !strings.HasSuffix(src.URL, "/yt/"+id) {
				t.Errorf("%d, %s: %+v, %v", i, id, src, err)
			}
		}
	}
	if n.runs.Load() != 2 {
		t.Errorf("yt-dlp ran %d times for two videos", n.runs.Load())
	}

	// Cached: stereo plays from the cache, and its 5.1 audio, cached
	// beside it, too.
	audio := filepath.Join(n.dir, "audio")
	if err := os.MkdirAll(audio, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string) {
		if err := os.WriteFile(filepath.Join(audio, name), []byte("audio"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("ddddddddddd.webm")
	write(stream.SurroundKey("ddddddddddd") + ".m4a")
	write("eeeeeeeee5e.webm")
	runs := n.runs.Load()
	for _, on := range []bool{false, true} {
		n.p.a.settings = testSettings(t, func(s *settings.Settings) { s.Surround = on })
		src, err := n.p.PrepareStream(ctx, ytTrack("ddddddddddd"))
		if err != nil || src.SurroundURL != surroundURL("ddddddddddd") || !src.Cached {
			t.Errorf("5.1 %v, cached in both: %+v, %v", on, src, err)
		}
	}
	if n.runs.Load() != runs {
		t.Errorf("yt-dlp ran for a track cached in both")
	}
	// Cached in stereo only: with 5.1 off it plays from the cache, telling
	// no 5.1 audio, which takes asking.
	n.p.a.settings = testSettings(t)
	if src, err := n.p.PrepareStream(ctx, ytTrack("eeeeeeeee5e")); err != nil || src.SurroundURL != "" || !src.Cached || n.runs.Load() != runs {
		t.Errorf("5.1 off, cached in stereo: %+v, %v, %d runs", src, err, n.runs.Load()-runs)
	}
	// With 5.1 on, it asks, and plays its 5.1 audio from the network.
	n.p.a.settings = testSettings(t, func(s *settings.Settings) { s.Surround = true })
	if src, err := n.p.PrepareStream(ctx, ytTrack("eeeeeeeee5e")); err != nil || src.SurroundURL != surroundURL("eeeeeeeee5e") || src.Cached {
		t.Errorf("5.1 on, cached in stereo: %+v, %v", src, err)
	}
	// While yt-dlp takes long, it plays from the cache, in stereo, after
	// surroundWait.
	wait := surroundWait
	surroundWait = 30 * time.Millisecond
	t.Cleanup(func() { surroundWait = wait })
	n.p.a.resolver.Forget("eeeeeeeee5e")
	n.hang.Store(true)
	released := make(chan struct{})
	t.Cleanup(func() { close(released) })
	run := n.p.a.resolver.Run
	n.p.a.resolver.Run = func(ctx context.Context, args ...string) ([]byte, error) {
		if n.hang.Load() {
			select {
			case <-ctx.Done():
			case <-released:
			}
			return nil, errors.New("yt-dlp gave up")
		}
		return run(ctx, args...)
	}
	started := time.Now()
	if src, err := n.p.PrepareStream(ctx, ytTrack("eeeeeeeee5e")); err != nil || src.SurroundURL != "" || !src.Cached || time.Since(started) > 2*time.Second {
		t.Errorf("5.1 on, cached in stereo, yt-dlp slow: %+v, %v, after %v", src, err, time.Since(started))
	}
	n.hang.Store(false)
	// Offline, it plays from the cache, in stereo.
	n.p.a.resolver.Forget("eeeeeeeee5e")
	n.net.Store(false)
	n.ytdlp.Store(false)
	n.p.a.online.Check(ctx)
	if src, err := n.p.PrepareStream(ctx, ytTrack("eeeeeeeee5e")); err != nil || src.SurroundURL != "" || !src.Cached {
		t.Errorf("5.1 on, cached in stereo, offline: %+v, %v", src, err)
	}
}
