package stream

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/tools"
)

// fakeYTDLP returns a Run function printing what yt-dlp prints for a
// stream at streamURL, and counts its runs.
func fakeYTDLP(streamURL string, runs *atomic.Int32) func(context.Context, ...string) ([]byte, error) {
	return func(ctx context.Context, args ...string) ([]byte, error) {
		runs.Add(1)
		if !slices.Contains(args, "--") {
			return nil, errors.New("the URL must follow --")
		}
		expire := time.Now().Add(6 * time.Hour).Unix()
		return fmt.Appendf(nil, `{"url": "%s?expire=%d", "ext": "webm", "acodec": "opus", "abr": 139.3,
			"filesize": 1000, "duration": 207.5, "http_headers": {"User-Agent": "test"},
			"available_at": 1792626003}`, streamURL, expire), nil
	}
}

func TestResolveParsesAndCaches(t *testing.T) {
	var runs atomic.Int32
	r := NewResolver(nil, "")
	r.Run = fakeYTDLP("https://example.com/audio", &runs)

	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			s, err := r.Resolve(context.Background(), "w3XzgjlKFjc")
			if err != nil {
				t.Error(err)
				return
			}
			if s.MimeType != `audio/webm; codecs="opus"` || s.Size != 1000 || s.Header.Get("User-Agent") != "test" {
				t.Errorf("stream = %+v", s)
			}
			if s.Duration != 207500*time.Millisecond {
				t.Errorf("duration = %s", s.Duration)
			}
			if !s.Available.Equal(time.Unix(1792626003, 0)) {
				t.Errorf("available at %s", s.Available)
			}
		})
	}
	wg.Wait()
	if runs.Load() != 1 {
		t.Errorf("yt-dlp ran %d times for one video", runs.Load())
	}
	r.Forget("w3XzgjlKFjc")
	if _, err := r.Resolve(context.Background(), "w3XzgjlKFjc"); err != nil {
		t.Fatal(err)
	}
	if runs.Load() != 2 {
		t.Errorf("a forgotten video was not resolved again")
	}
}

func TestResolverFresh(t *testing.T) {
	var runs atomic.Int32
	r := NewResolver(nil, "")
	r.Run = fakeYTDLP("https://example.com/audio", &runs)
	if r.Fresh("w3XzgjlKFjc") {
		t.Error("fresh before it was resolved")
	}
	if _, err := r.Resolve(context.Background(), "w3XzgjlKFjc"); err != nil {
		t.Fatal(err)
	}
	if !r.Fresh("w3XzgjlKFjc") {
		t.Error("not fresh once resolved")
	}
	// A stream about to expire is resolved again.
	r.Run = func(context.Context, ...string) ([]byte, error) {
		return fmt.Appendf(nil, `{"url": "https://example.com/audio?expire=%d", "ext": "webm"}`, time.Now().Add(time.Minute).Unix()), nil
	}
	r.Forget("w3XzgjlKFjc")
	if _, err := r.Resolve(context.Background(), "w3XzgjlKFjc"); err != nil {
		t.Fatal(err)
	}
	if r.Fresh("w3XzgjlKFjc") {
		t.Error("fresh though about to expire")
	}
}

func TestResolveRejectsBadIDs(t *testing.T) {
	r := NewResolver(nil, "")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		t.Fatal("yt-dlp ran for a bad id")
		return nil, nil
	}
	for _, id := range []string{"", "--exec=calc", "short", "w3XzgjlKFjc/x"} {
		if _, err := r.Resolve(context.Background(), id); !errors.Is(err, ErrBadVideoID) {
			t.Errorf("Resolve(%q) = %v", id, err)
		}
	}
}

func TestYTDLPError(t *testing.T) {
	stderr := "WARNING: something\nERROR: [youtube] w3XzgjlKFjc: Video unavailable\n"
	if got := ytdlpError(stderr); got != "yt-dlp: w3XzgjlKFjc: Video unavailable" {
		t.Errorf("ytdlpError = %q", got)
	}
	if got := ytdlpError("just noise"); got != "" {
		t.Errorf("ytdlpError(noise) = %q", got)
	}
}

func TestPlainError(t *testing.T) {
	for msg, want := range map[string]string{
		"yt-dlp: pFptt7Cargc: Requested format is not available. Use --list-formats for a list of available formats": "YouTube gave no audio for this track; updating yt-dlp in Settings › Playback often fixes it",
		"yt-dlp: pFptt7Cargc: Sign in to confirm your age. This video may be inappropriate for some users.":          "YouTube plays this track only to signed-in adults, on its own website",
		"yt-dlp: pFptt7Cargc: Sign in to confirm you’re not a bot. Use --cookies-from-browser":                       "YouTube asks to confirm this is not a bot; try again in a while",
		"yt-dlp: pFptt7Cargc: Video unavailable. The uploader has not made this video available in your country":     "this track is not available in your country",
		"yt-dlp: pFptt7Cargc: Video unavailable. This content isn't available, try again later.":                     "YouTube is turning down requests from this network for now; try again in a while",
		"yt-dlp: w3XzgjlKFjc: Video unavailable":                                                                     "this track is no longer on YouTube Music",
		"yt-dlp: pFptt7Cargc: Private video. Sign in if you've been granted access to this video":                    "this track is private",
		"yt-dlp: pFptt7Cargc: Join this channel to get access to members-only content like this video":               "only the members of its channel can play this track",
		"yt-dlp: unable to download webpage: HTTP Error 500":                                                         "",
	} {
		if got := plainError(msg); got != want {
			t.Errorf("plainError(%q) = %q, want %q", msg, got, want)
		}
	}
}

func TestExpiry(t *testing.T) {
	if got := expiry("https://x.googlevideo.com/videoplayback?expire=1792361335&id=1"); got.Unix() != 1792361335 {
		t.Errorf("expiry = %v", got)
	}
	if got := expiry("https://example.com/a"); time.Until(got) < 5*time.Hour {
		t.Errorf("default expiry = %v", got)
	}
}

func TestMimeType(t *testing.T) {
	if got := mimeType("m4a", "mp4a.40.2"); !strings.HasPrefix(got, "audio/mp4") {
		t.Errorf("m4a = %q", got)
	}
}

func TestResolveTriesTheDirectRouteFirst(t *testing.T) {
	f := newFakeTube(t)
	r := NewResolver(nil, "")
	r.Direct = f.direct()
	r.Run = func(context.Context, ...string) ([]byte, error) {
		t.Error("yt-dlp ran for a stream the direct route found")
		return nil, errors.New("no yt-dlp here")
	}
	s, err := r.Resolve(context.Background(), songID)
	if err != nil || s.Size != song251 || !s.Available.IsZero() {
		t.Errorf("stream of %d bytes, available %v: %v", s.Size, s.Available, err)
	}
}

func TestResolveFallsBackToYTDLP(t *testing.T) {
	for name, fail := range map[string]func(f *fakeTube){
		"age":           func(f *fakeTube) { f.answersWith("-age") },
		"made for kids": func(f *fakeTube) { f.answersWith("-unplayable") },
		"bot check":     func(f *fakeTube) { f.answersWith("-bot-check") },
		"HTTP 500":      func(f *fakeTube) { f.answers(func(playerRequest) []byte { return nil }) },
		"refused":       func(f *fakeTube) { f.refuses(http.StatusForbidden) },
	} {
		f := newFakeTube(t)
		fail(f)
		var runs atomic.Int32
		r := NewResolver(nil, "")
		r.Direct = f.direct()
		r.Run = fakeYTDLP("https://example.com/audio", &runs)
		s, err := r.Resolve(context.Background(), songID)
		if err != nil || runs.Load() != 1 || s.Header.Get("User-Agent") != "test" {
			t.Errorf("%s: %d runs of yt-dlp, stream %+v, %v", name, runs.Load(), s, err)
		}
	}
}

func TestResolveSaysWhyWithoutYTDLP(t *testing.T) {
	f := newFakeTube(t)
	r := NewResolver(nil, "")
	r.Direct = f.direct()
	r.Run = func(context.Context, ...string) ([]byte, error) { return nil, tools.ErrNoYTDLP }
	f.answersWith("-age")
	if _, err := r.Resolve(context.Background(), songID); err == nil || err.Error() != "YouTube plays this track only to signed-in adults, on its own website" {
		t.Errorf("age-restricted, without yt-dlp: %v", err)
	}
	// YouTube gives the direct route nothing of a video made for children,
	// which yt-dlp would play.
	f.answersWith("-unplayable")
	if _, err := r.Resolve(context.Background(), "XqZsoesa55w"); !errors.Is(err, tools.ErrNoYTDLP) {
		t.Errorf("made for children, without yt-dlp: %v", err)
	}
}
