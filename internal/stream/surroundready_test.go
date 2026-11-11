package stream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// waitLine is what the proxy logs as it waits for YouTube to serve a
// stream.
const waitLine = "for YouTube to serve the stream of "

// TestProxyPreparesSurround makes the 5.1 audio of a video the direct
// route told of ready, while its stereo audio could play: yt-dlp finds it,
// and the preparation waits for YouTube to serve it. The proxy then serves
// it at once, without yt-dlp or a wait. A preparation given up while
// YouTube holds the audio back keeps what yt-dlp found.
func TestProxyPreparesSurround(t *testing.T) {
	logs := captureLog(t)
	f := newFakeTube(t)
	f.answersWith("-surround")
	f.answersTVWith("-surround")
	var minGen atomic.Int32
	srv5, data5 := upstream(t, 5000, &minGen)
	var runs atomic.Int32
	r := NewResolver(nil, "")
	r.Direct = f.direct()
	const wait = 400 * time.Millisecond
	r.Run = func(context.Context, ...string) ([]byte, error) {
		runs.Add(1)
		return surroundInfo(fmt.Sprintf(`{"format_id": "380", "url": "%s/a?expire=%d", "ext": "m4a", "protocol": "https", "acodec": "ac-3",
			"vcodec": "none", "abr": 384, "audio_channels": 6, "filesize": %d, "available_at": %.3f}`,
			srv5.URL, time.Now().Add(6*time.Hour).Unix(), len(data5), float64(time.Now().Add(wait).UnixMilli())/1000)), nil
	}
	p := NewProxy(r)
	ctx := context.Background()
	if p.SurroundReady(surroundID) {
		t.Fatal("5.1 audio ready of a video not resolved")
	}
	// The TV client told of 5.1 audio, which yt-dlp has yet to find.
	knownSurround(t, r, surroundID)
	if p.SurroundReady(surroundID) || runs.Load() != 0 {
		t.Fatalf("ready before yt-dlp found it, after %d runs of yt-dlp", runs.Load())
	}
	start := time.Now()
	if err := p.PrepareSurround(ctx, surroundID); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took < wait-50*time.Millisecond || runs.Load() != 1 {
		t.Errorf("prepared in %v, after %d runs of yt-dlp", took, runs.Load())
	}
	if !p.SurroundReady(surroundID) {
		t.Fatal("not ready once prepared")
	}
	waits := strings.Count(logs.String(), waitLine)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, httptest.NewRequest("GET", SurroundPath(surroundID), nil))
	if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), data5) {
		t.Fatalf("5.1: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	if n := strings.Count(logs.String(), waitLine); n != waits || runs.Load() != 1 {
		t.Errorf("served once prepared after %d more waits and %d runs of yt-dlp:\n%s", n-waits, runs.Load(), logs.String())
	}
	if err := p.PrepareSurround(ctx, surroundID); err != nil || runs.Load() != 1 {
		t.Errorf("prepared again: %v, after %d runs of yt-dlp", err, runs.Load())
	}

	// Given up while YouTube holds it back: what yt-dlp found is kept, and
	// ready once YouTube serves it.
	r.Forget(surroundID)
	knownSurround(t, r, surroundID)
	short, cancel := context.WithTimeout(ctx, wait/4)
	err := p.PrepareSurround(short, surroundID)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) || p.SurroundReady(surroundID) {
		t.Fatalf("given up: %v, ready %v", err, p.SurroundReady(surroundID))
	}
	for deadline := time.Now().Add(5 * time.Second); !p.SurroundReady(surroundID); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("never ready once YouTube served it")
		}
	}
	if runs.Load() != 2 {
		t.Errorf("yt-dlp ran %d times", runs.Load())
	}
}

// TestProxyPreparesSurroundBesideStereo makes ready the 5.1 audio yt-dlp
// found beside a video's stereo audio, which waits only for YouTube to
// serve it; the 5.1 audio of the cache is ready at once, and a video of
// none can't be.
func TestProxyPreparesSurroundBesideStereo(t *testing.T) {
	var minGen atomic.Int32
	srv, data := upstream(t, 3000, &minGen)
	srv5, data5 := upstream(t, 5000, &minGen)
	var runs atomic.Int32
	var surround atomic.Bool
	surround.Store(true)
	const wait = 300 * time.Millisecond
	r := NewResolver(nil, "")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		runs.Add(1)
		expire := time.Now().Add(6 * time.Hour).Unix()
		formats := ""
		if surround.Load() {
			formats = fmt.Sprintf(`{"format_id": "380", "url": "%s/a?expire=%d", "ext": "m4a", "protocol": "https", "acodec": "ac-3",
				"vcodec": "none", "abr": 384, "audio_channels": 6, "filesize": %d, "available_at": %.3f}`,
				srv5.URL, expire, len(data5), float64(time.Now().Add(wait).UnixMilli())/1000)
		}
		return fmt.Appendf(nil, `{"url": "%s/a?expire=%d", "ext": "webm", "acodec": "opus", "filesize": %d, "formats": [%s]}`,
			srv.URL, expire, len(data), formats), nil
	}
	p := NewProxy(r)
	dir := t.TempDir()
	p.Cache = NewCache(dir, 1<<20)
	ctx := context.Background()
	if s, err := r.Resolve(ctx, testID); err != nil || s.Surround == nil || s.Surround.URL == "" {
		t.Fatalf("stream %+v, %v", s, err)
	}
	if p.SurroundReady(testID) {
		t.Fatal("ready before YouTube serves it")
	}
	start := time.Now()
	if err := p.PrepareSurround(ctx, testID); err != nil || time.Since(start) < wait-100*time.Millisecond || runs.Load() != 1 {
		t.Fatalf("prepared in %v: %v, after %d runs of yt-dlp", time.Since(start), err, runs.Load())
	}
	if !p.SurroundReady(testID) {
		t.Error("not ready once prepared")
	}

	// From the cache, at once, without yt-dlp.
	const cachedID = "cccccccccc5"
	if err := os.WriteFile(filepath.Join(dir, SurroundKey(cachedID)+".m4a"), []byte("5.1 audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !p.SurroundReady(cachedID) {
		t.Error("the 5.1 audio of the cache isn't ready")
	}
	if err := p.PrepareSurround(ctx, cachedID); err != nil || runs.Load() != 1 {
		t.Errorf("the 5.1 audio of the cache: %v, after %d runs of yt-dlp", err, runs.Load())
	}

	// Of a video of none: never.
	surround.Store(false)
	const noneID = "nnnnnnnnnn0"
	if err := p.PrepareSurround(ctx, noneID); !errors.Is(err, errNoSurround) || p.SurroundReady(noneID) {
		t.Errorf("of a video of none: %v, ready %v", err, p.SurroundReady(noneID))
	}
}
