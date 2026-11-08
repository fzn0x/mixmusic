package stream

import (
	"context"
	"errors"

	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/tools"
)

// ytdlpCalls keeps the arguments of each run of a fake yt-dlp, and has it
// answer as answer does.
type ytdlpCalls struct {
	mu    sync.Mutex
	calls [][]string
}

func (c *ytdlpCalls) run(answer func(ctx context.Context, quick bool) ([]byte, error)) func(context.Context, ...string) ([]byte, error) {
	return func(ctx context.Context, args ...string) ([]byte, error) {
		c.mu.Lock()
		c.calls = append(c.calls, args)
		c.mu.Unlock()
		return answer(ctx, slices.Contains(args, quickArgs))
	}
}

func (c *ytdlpCalls) all() [][]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.calls)
}

// toolsWithRuntime returns tools whose yt-dlp runs with a JavaScript
// runtime, Deno, in their folder.
func toolsWithRuntime(t *testing.T) *tools.Manager {
	t.Helper()
	dir := t.TempDir()
	deno := "deno"
	if runtime.GOOS == "windows" {
		deno += ".exe"
	}
	if err := os.WriteFile(filepath.Join(dir, deno), []byte("deno"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := tools.New(dir)
	m.Configure("", "deno")
	return m
}

// refusedDirect returns a direct route whose media server refuses the
// streams of the song, which yt-dlp finds then.
func refusedDirect(t *testing.T) *Direct {
	f := newFakeTube(t)
	f.refuses(http.StatusForbidden)
	return f.direct()
}

// TestYTDLPRunsQuicklyFirst has yt-dlp find a stream the direct route
// didn't as the visionOS client alone, without the JavaScript runtime and
// without the manifests, once: that run found it, the log says.
func TestYTDLPRunsQuicklyFirst(t *testing.T) {
	logs := captureLog(t)
	var calls ytdlpCalls
	var runs atomic.Int32
	found := fakeYTDLP("https://example.com/audio", &runs)
	r := NewResolver(toolsWithRuntime(t), filepath.Join(t.TempDir(), "yt-dlp"))
	r.Direct = refusedDirect(t)
	r.Run = calls.run(func(ctx context.Context, quick bool) ([]byte, error) {
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > quickTimeout {
			t.Errorf("the quick run may take until %v", deadline)
		}
		return found(ctx, "--")
	})
	s, err := r.Resolve(context.Background(), songID)
	if err != nil || s.Header.Get("User-Agent") != "test" || !s.Available.Equal(time.Unix(1792626003, 0)) {
		t.Fatalf("stream %+v, %v", s, err)
	}
	all := calls.all()
	if len(all) != 1 {
		t.Fatalf("yt-dlp ran %d times: %q", len(all), all)
	}
	args := all[0]
	if i := slices.Index(args, "--extractor-args"); i < 0 || args[i+1] != "youtube:player_client=visionos;skip=hls,dash" {
		t.Errorf("the quick run's extractor arguments: %q", args)
	}
	if i := slices.Index(args, "-f"); i < 0 || args[i+1] != formats[QualityBest] || !slices.Contains(args, "-J") ||
		!slices.Contains(args, "--cache-dir") || !slices.Contains(args, "--no-warnings") {
		t.Errorf("the quick run's arguments: %q", args)
	}
	if slices.Contains(args, "--js-runtimes") {
		t.Errorf("the quick run names the JavaScript runtime: %q", args)
	}
	if n := len(args); n < 2 || args[n-2] != "--" || args[n-1] != "https://music.youtube.com/watch?v="+songID {
		t.Errorf("the quick run's URL: %q", args)
	}
	if !strings.Contains(logs.String(), "found the stream of "+songID+" in ") || !strings.Contains(logs.String(), "with yt-dlp's quick run in ") ||
		!strings.Contains(logs.String(), "(YouTube's media server refused the stream)") {
		t.Errorf("the log doesn't tell the quick run found it:\n%s", logs.String())
	}
}

// TestYTDLPRunsFullyWhenTheQuickRunFindsNone has yt-dlp find the stream the
// full way, with its default clients and the JavaScript runtime, once its
// quick run found no audio, failed, or gave what the engine doesn't play.
func TestYTDLPRunsFullyWhenTheQuickRunFindsNone(t *testing.T) {
	for name, quick := range map[string]func() ([]byte, error){
		"no format": func() ([]byte, error) {
			return nil, failure{"yt-dlp: [youtube] " + testID + ": Requested format is not available", nil}
		},
		"no audio": func() ([]byte, error) { return []byte(`{"formats": []}`), nil },
		"HLS, not live": func() ([]byte, error) {
			return []byte(`{"url": "https://example.com/a.m3u8", "protocol": "m3u8_native"}`), nil
		},
		"not JSON": func() ([]byte, error) { return []byte("<html>"), nil },
		"a bot check": func() ([]byte, error) {
			return nil, failure{"YouTube asks to confirm this is not a bot; try again in a while", ErrThrottled}
		},
		"a yt-dlp crash": func() ([]byte, error) { return nil, errors.New("yt-dlp: exit status 1") },
	} {
		logs := captureLog(t)
		var calls ytdlpCalls
		var runs atomic.Int32
		found := fakeYTDLP("https://example.com/audio", &runs)
		r := NewResolver(toolsWithRuntime(t), "")
		r.Direct = refusedDirect(t)
		r.Run = calls.run(func(ctx context.Context, isQuick bool) ([]byte, error) {
			if isQuick {
				return quick()
			}
			return found(ctx, "--")
		})
		s, err := r.Resolve(context.Background(), songID)
		if err != nil || s.Header.Get("User-Agent") != "test" {
			t.Errorf("%s: stream %+v, %v", name, s, err)
			continue
		}
		all := calls.all()
		if len(all) != 2 || !slices.Contains(all[0], quickArgs) {
			t.Errorf("%s: yt-dlp ran %q", name, all)
			continue
		}
		full := all[1]
		if slices.Contains(full, "--extractor-args") || !slices.Contains(full, "--js-runtimes") || !slices.Contains(full, "-J") {
			t.Errorf("%s: the full run's arguments: %q", name, full)
		}
		if !strings.Contains(logs.String(), "yt-dlp's quick run found no stream of "+songID+" in ") ||
			!strings.Contains(logs.String(), "with yt-dlp's full run in ") {
			t.Errorf("%s: the log doesn't tell which run found the stream:\n%s", name, logs.String())
		}
	}
}

// TestYTDLPQuickRunWithoutYTDLP tries no full run when yt-dlp isn't there,
// nor when the stream is no longer asked for.
func TestYTDLPQuickRunWithoutYTDLP(t *testing.T) {
	var calls ytdlpCalls
	r := NewResolver(nil, "")
	r.Run = calls.run(func(context.Context, bool) ([]byte, error) { return nil, tools.ErrNoYTDLP })
	if _, err := r.Resolve(context.Background(), testID); !errors.Is(err, tools.ErrNoYTDLP) || len(calls.all()) != 1 {
		t.Errorf("without yt-dlp: %v, after %d runs", err, len(calls.all()))
	}

	var stopped ytdlpCalls
	ctx, cancel := context.WithCancel(context.Background())
	r = NewResolver(nil, "")
	r.Run = stopped.run(func(context.Context, bool) ([]byte, error) {
		cancel()
		return nil, context.Canceled
	})
	if _, _, err := r.findWithYTDLP(ctx, testID, QualityBest, nil); err == nil || len(stopped.all()) != 1 {
		t.Errorf("no longer asked for: %v, after %d runs", err, len(stopped.all()))
	}
}

// TestYTDLPSkipsTheQuickRunTheDirectRouteRules out runs yt-dlp the full way
// at once when YouTube's player API told the visionOS client, which the
// quick run asks as too, that a video has no stream for it, and quickly
// first when the route failed otherwise.
func TestYTDLPSkipsTheQuickRunTheDirectRouteRulesOut(t *testing.T) {
	for name, c := range map[string]struct {
		fail  func(f *fakeTube)
		quick bool
	}{
		"made for kids":  {func(f *fakeTube) { f.answersWith("-unplayable") }, false},
		"age":            {func(f *fakeTube) { f.answersWith("-age") }, false},
		"no Opus or AAC": {func(f *fakeTube) { f.answers(func(playerRequest) []byte { return heAACOnly(f.recorded("")) }) }, false},
		"signatures": {func(f *fakeTube) {
			f.answers(func(playerRequest) []byte {
				return []byte(strings.ReplaceAll(string(f.recorded("")), "&expire=", "&n=AbCdEfGhIjKl&expire="))
			})
		}, false},
		"bot check":     {func(f *fakeTube) { f.answersWith("-bot-check") }, true},
		"HTTP 500":      {func(f *fakeTube) { f.answers(func(playerRequest) []byte { return nil }) }, true},
		"refused":       {func(f *fakeTube) { f.refuses(http.StatusForbidden) }, true},
		"another one's": {func(f *fakeTube) {}, true}, // the song's answer, for testID
	} {
		f := newFakeTube(t)
		c.fail(f)
		var calls ytdlpCalls
		var runs atomic.Int32
		found := fakeYTDLP("https://example.com/audio", &runs)
		r := NewResolver(nil, "")
		r.Direct = f.direct()
		r.Run = calls.run(func(ctx context.Context, _ bool) ([]byte, error) { return found(ctx, "--") })
		id := songID
		if name == "another one's" {
			id = testID
		}
		if _, err := r.Resolve(context.Background(), id); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		all := calls.all()
		if len(all) != 1 || slices.Contains(all[0], quickArgs) != c.quick {
			t.Errorf("%s: yt-dlp ran %q; want the quick run %v", name, all, c.quick)
		}
	}
	// Without the direct route, as with MIXMUSIC_YTDLP_ONLY, quickly first.
	if !quickHelps(nil) || !quickHelps(errDirectPaused) || !quickHelps(context.DeadlineExceeded) {
		t.Error("no quick run without the direct route's answer")
	}
}

// heAACOnly returns a recorded answer whose streams are of no codec the
// engine plays, but HE-AAC.
func heAACOnly(answer []byte) []byte {
	s := strings.ReplaceAll(string(withoutOpus(answer)), `codecs=\"mp4a.40.2\"`, `codecs=\"mp4a.40.5\"`)
	return []byte(s)
}

func TestWithoutJSRuntime(t *testing.T) {
	args := []string{"--no-warnings", "--cache-dir", "c", "--js-runtimes", "deno:/bin/deno", "--js-runtimes=node", "--proxy", "p"}
	if got := withoutJSRuntime(args); !slices.Equal(got, []string{"--no-warnings", "--cache-dir", "c", "--proxy", "p"}) {
		t.Errorf("withoutJSRuntime = %q", got)
	}
}
