package stream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// quickThenFull returns a fake yt-dlp whose quick run finds the stereo
// audio of a video alone, as the visionOS client lists no 5.1 audio, and
// whose full run finds its 5.1 audio beside it, once full lets it answer.
func quickThenFull(t *testing.T, calls *ytdlpCalls, full func() ([]byte, error)) (run func(context.Context, ...string) ([]byte, error), release func()) {
	held := make(chan struct{})
	var once sync.Once
	release = func() { once.Do(func() { close(held) }) }
	t.Cleanup(release)
	return calls.run(func(ctx context.Context, quick bool) ([]byte, error) {
		if quick {
			return surroundInfo(), nil
		}
		select {
		case <-held:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return full()
	}), release
}

// withSurround is the full run's answer of a video of 5.1 audio.
func withSurround() ([]byte, error) { return surroundInfo(ytSurround("380", "ac-3", 384, "")), nil }

// waitTold returns the video Told was called with, failing the test when it
// isn't within a few seconds.
func waitTold(t *testing.T, told chan string) string {
	t.Helper()
	select {
	case id := <-told:
		return id
	case <-time.After(5 * time.Second):
		t.Fatal("not told of the 5.1 audio")
		return ""
	}
}

// TestSurroundBehindTheQuickRun finds, with 5.1 on, the stereo audio of a
// video with yt-dlp's quick run, which lists no 5.1 audio, and returns it
// at once: the full run looks for the 5.1 audio behind it, which is kept
// with the stream and told of once found, and plays without another run
// (#540).
func TestSurroundBehindTheQuickRun(t *testing.T) {
	logs := captureLog(t)
	var calls ytdlpCalls
	run, release := quickThenFull(t, &calls, withSurround)
	r := NewResolver(nil, "")
	r.Run = run
	told := make(chan string, 4)
	r.Told = func(id string) { told <- id }
	r.SetSurround(true)
	ctx := context.Background()

	// The stereo audio comes while the full run is held.
	s, err := r.Resolve(ctx, surroundID)
	if err != nil || s.Route != RouteQuick || s.Codec != "opus" || s.Surround != nil {
		t.Fatalf("stream %+v, surround %+v, %v", s, s.Surround, err)
	}
	for deadline := time.Now().Add(5 * time.Second); len(calls.all()) < 2; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the full run didn't look for the 5.1 audio: yt-dlp ran %q", calls.all())
		}
	}
	all := calls.all()
	if !slices.Contains(all[0], quickArgs) || slices.Contains(all[1], "--extractor-args") {
		t.Errorf("yt-dlp ran %q; want the quick run, then the full one", all)
	}
	if _, err := r.Surround(ctx, surroundID); !errors.Is(err, errNoSurround) {
		t.Errorf("5.1 before the full run found it: %v", err)
	}

	release()
	if id := waitTold(t, told); id != surroundID {
		t.Errorf("told of %s", id)
	}
	s, err = r.Resolve(ctx, surroundID)
	if err != nil || s.Route != RouteQuick || !strings.HasPrefix(s.URL, "https://example.com/opus") || s.Surround == nil ||
		s.Surround.URL != "https://example.com/380" || s.Surround.Codec != "ac-3" || s.Surround.Route != RouteFull {
		t.Fatalf("after the full run: stream %+v, surround %+v, %v", s, s.Surround, err)
	}
	if sur, err := r.Surround(ctx, surroundID); err != nil || sur.URL != "https://example.com/380" || sur.Header.Get("User-Agent") != "test" {
		t.Errorf("5.1: %+v, %v", sur, err)
	}
	if sur, ok := r.KnownSurround(surroundID); !ok || sur.URL != "https://example.com/380" {
		t.Errorf("known 5.1: %+v, %v", sur, ok)
	}
	// Once a stream: asking again runs yt-dlp no more, nor tells again.
	r.Resolve(ctx, surroundID)
	time.Sleep(50 * time.Millisecond)
	if n := len(calls.all()); n != 2 {
		t.Errorf("yt-dlp ran %d times", n)
	}
	select {
	case id := <-told:
		t.Errorf("told of %s again", id)
	default:
	}
	if !strings.Contains(logs.String(), "found the 5.1 surround audio of "+surroundID+" in ") ||
		!strings.Contains(logs.String(), "with yt-dlp's full run, behind its quick run") {
		t.Errorf("the log doesn't tell the full run found the 5.1 audio:\n%s", logs.String())
	}
}

// TestQuickRunUnchangedWithSurroundOff runs yt-dlp's quick run alone with
// 5.1 off, as before: no full run looks for 5.1 audio, and none is told
// of. Once 5.1 is on, the stream kept gets the look as it is asked for.
func TestQuickRunUnchangedWithSurroundOff(t *testing.T) {
	var calls ytdlpCalls
	run, release := quickThenFull(t, &calls, withSurround)
	release()
	r := NewResolver(nil, "")
	r.Run = run
	told := make(chan string, 4)
	r.Told = func(id string) { told <- id }
	ctx := context.Background()
	for range 3 {
		s, err := r.Resolve(ctx, surroundID)
		if err != nil || s.Route != RouteQuick || s.Surround != nil {
			t.Fatalf("stream %+v, surround %+v, %v", s, s.Surround, err)
		}
	}
	time.Sleep(50 * time.Millisecond)
	if all := calls.all(); len(all) != 1 || !slices.Contains(all[0], quickArgs) {
		t.Errorf("with 5.1 off, yt-dlp ran %q", all)
	}
	if _, err := r.Surround(ctx, surroundID); !errors.Is(err, errNoSurround) {
		t.Errorf("5.1 with 5.1 off: %v", err)
	}
	select {
	case id := <-told:
		t.Errorf("told of %s with 5.1 off", id)
	default:
	}

	r.SetSurround(true)
	r.Resolve(ctx, surroundID)
	waitTold(t, told)
	if s, err := r.Resolve(ctx, surroundID); err != nil || s.Surround == nil || s.Surround.URL != "https://example.com/380" || len(calls.all()) != 2 {
		t.Errorf("once 5.1 is on: surround %+v, %v, after %d runs of yt-dlp", s.Surround, err, len(calls.all()))
	}
}

// TestDownloadLooksForNoSurround has no full run look for 5.1 audio behind
// the stream the quick run found of a download, which keeps the stereo
// audio alone: a play of it does.
func TestDownloadLooksForNoSurround(t *testing.T) {
	var minGen atomic.Int32
	srv, data := upstream(t, 3000, &minGen)
	var calls ytdlpCalls
	r := NewResolver(nil, "")
	r.Run = calls.run(func(_ context.Context, quick bool) ([]byte, error) {
		formats := ""
		if !quick {
			formats = ytSurround("380", "ac-3", 384, "")
		}
		return fmt.Appendf(nil, `{"url": "%s/a?expire=%d", "ext": "webm", "acodec": "opus", "filesize": %d, "formats": [%s]}`,
			srv.URL, time.Now().Add(6*time.Hour).Unix(), len(data), formats), nil
	})
	told := make(chan string, 4)
	r.Told = func(id string) { told <- id }
	r.SetSurround(true)
	var buf bytes.Buffer
	if _, err := NewProxy(r).Download(context.Background(), testID, fromStart, &buf, nil); err != nil || !bytes.Equal(buf.Bytes(), data) {
		t.Fatalf("%d bytes of %d: %v", buf.Len(), len(data), err)
	}
	time.Sleep(50 * time.Millisecond)
	if all := calls.all(); len(all) != 1 || !slices.Contains(all[0], quickArgs) {
		t.Errorf("yt-dlp ran %q for a download", all)
	}
	if s, err := r.Resolve(context.Background(), testID); err != nil || s.Route != RouteQuick {
		t.Fatalf("stream %+v, %v", s, err)
	}
	waitTold(t, told)
	if n := len(calls.all()); n != 2 {
		t.Errorf("yt-dlp ran %d times, played once", n)
	}
}

// TestSurroundBehindFindsNone tells of no 5.1 audio when the full run
// behind the quick run finds none, fails, or ends after the video was
// forgotten, and looks once a stream; nor does it look behind a stream
// the full run found itself.
func TestSurroundBehindFindsNone(t *testing.T) {
	for name, full := range map[string]func() ([]byte, error){
		"none":   func() ([]byte, error) { return surroundInfo(), nil },
		"failed": func() ([]byte, error) { return nil, errors.New("yt-dlp: exit status 1") },
	} {
		var calls ytdlpCalls
		run, release := quickThenFull(t, &calls, full)
		release()
		r := NewResolver(nil, "")
		r.Run = run
		r.Told = func(id string) { t.Errorf("%s: told of %s", name, id) }
		r.SetSurround(true)
		for range 3 {
			if s, err := r.Resolve(context.Background(), surroundID); err != nil || s.Surround != nil {
				t.Errorf("%s: stream %+v, surround %+v, %v", name, s, s.Surround, err)
			}
		}
		time.Sleep(50 * time.Millisecond)
		if n := len(calls.all()); n != 2 {
			t.Errorf("%s: yt-dlp ran %d times", name, n)
		}
	}

	// Forgotten while the full run looked.
	var calls ytdlpCalls
	run, release := quickThenFull(t, &calls, withSurround)
	r := NewResolver(nil, "")
	r.Run = run
	r.Told = func(id string) { t.Errorf("forgotten: told of %s", id) }
	r.SetSurround(true)
	r.Resolve(context.Background(), surroundID)
	for deadline := time.Now().Add(5 * time.Second); len(calls.all()) < 2 && time.Now().Before(deadline); time.Sleep(time.Millisecond) {
	}
	r.Forget(surroundID)
	release()
	time.Sleep(50 * time.Millisecond)

	// The full run found the stream, and its 5.1 audio, or none, itself.
	var full ytdlpCalls
	r = NewResolver(nil, "")
	r.Run = full.run(func(_ context.Context, quick bool) ([]byte, error) {
		if quick {
			return nil, errors.New("yt-dlp: exit status 1")
		}
		return surroundInfo(), nil
	})
	r.SetSurround(true)
	if s, err := r.Resolve(context.Background(), surroundID); err != nil || s.Route != RouteFull || s.Surround != nil {
		t.Errorf("of the full run: stream %+v, %v", s, err)
	}
	time.Sleep(50 * time.Millisecond)
	if n := len(full.all()); n != 2 {
		t.Errorf("of the full run: yt-dlp ran %d times", n)
	}
}
