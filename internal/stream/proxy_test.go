package stream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testID = "w3XzgjlKFjc"

// upstream serves size bytes of audio with range requests, refusing URLs
// whose generation is below minGen as YouTube refuses expired ones.
func upstream(t *testing.T, size int, minGen *atomic.Int32) (*httptest.Server, []byte) {
	t.Helper()
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i * 7)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var gen int32
		fmt.Sscanf(r.URL.Query().Get("gen"), "%d", &gen)
		if gen < minGen.Load() {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.Header.Get("Range") == "" {
			t.Error("upstream request without a range")
		}
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	}))
	t.Cleanup(srv.Close)
	return srv, data
}

// testProxy returns a proxy whose yt-dlp resolves to srv, the generation
// of the URL increasing at each run.
func testProxy(srv *httptest.Server, size int) *Proxy {
	var gen atomic.Int32
	r := NewResolver(nil, "")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		g := gen.Add(1)
		return fmt.Appendf(nil, `{"url": "%s/a?gen=%d&expire=%d", "ext": "webm", "acodec": "opus", "filesize": %d}`,
			srv.URL, g, time.Now().Add(6*time.Hour).Unix(), size), nil
	}
	return NewProxy(r)
}

func get(t *testing.T, h http.Handler, header string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/yt/"+testID, nil)
	if header != "" {
		req.Header.Set("Range", header)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestProxyServesWholeStreamInChunks(t *testing.T) {
	var minGen atomic.Int32
	size := chunkSize*2 + 12345
	srv, data := upstream(t, size, &minGen)
	rec := get(t, testProxy(srv, size), "")
	if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), data) {
		t.Fatalf("status %d, %d bytes of %d", rec.Code, rec.Body.Len(), size)
	}
	if rec.Header().Get("Accept-Ranges") != "bytes" || !strings.HasPrefix(rec.Header().Get("Content-Type"), "audio/webm") {
		t.Errorf("headers = %v", rec.Header())
	}
}

func TestProxyServesRanges(t *testing.T) {
	var minGen atomic.Int32
	size := chunkSize + 500
	srv, data := upstream(t, size, &minGen)
	p := testProxy(srv, size)

	rec := get(t, p, fmt.Sprintf("bytes=%d-%d", chunkSize-10, chunkSize+9))
	if rec.Code != 206 || !bytes.Equal(rec.Body.Bytes(), data[chunkSize-10:chunkSize+10]) {
		t.Errorf("middle range: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	if got, want := rec.Header().Get("Content-Range"), fmt.Sprintf("bytes %d-%d/%d", chunkSize-10, chunkSize+9, size); got != want {
		t.Errorf("Content-Range = %q, want %q", got, want)
	}
	if rec := get(t, p, "bytes=-100"); rec.Code != 206 || !bytes.Equal(rec.Body.Bytes(), data[size-100:]) {
		t.Errorf("suffix range: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	if rec := get(t, p, fmt.Sprintf("bytes=%d-", size+10)); rec.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("range past the end: %d", rec.Code)
	}
}

func TestProxyResolvesExpiredURLsAgain(t *testing.T) {
	var minGen atomic.Int32
	srv, data := upstream(t, 1000, &minGen)
	p := testProxy(srv, 1000)
	if rec := get(t, p, ""); rec.Code != 200 {
		t.Fatalf("first play: %d", rec.Code)
	}
	minGen.Store(2) // YouTube now refuses the first URL
	rec := get(t, p, "bytes=0-99")
	if rec.Code != 206 || !bytes.Equal(rec.Body.Bytes(), data[:100]) {
		t.Errorf("after expiry: %d %q", rec.Code, rec.Body.String())
	}
}

func TestProxyWaitsUntilYouTubeServesTheStream(t *testing.T) {
	available := time.Now().Add(300 * time.Millisecond)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if time.Now().Before(available) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		http.ServeContent(w, r, "", time.Time{}, strings.NewReader("audio"))
	}))
	t.Cleanup(srv.Close)
	r := NewResolver(nil, "")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		return fmt.Appendf(nil, `{"url": "%s/a", "ext": "webm", "filesize": 5, "available_at": %.3f}`,
			srv.URL, float64(available.UnixMilli()+1)/1000), nil
	}
	if rec := get(t, NewProxy(r), ""); rec.Code != 200 || rec.Body.String() != "audio" {
		t.Errorf("%d %q", rec.Code, rec.Body.String())
	}
}

func TestProxyCachesWholePlays(t *testing.T) {
	var minGen atomic.Int32
	srv, data := upstream(t, 2000, &minGen)
	p := testProxy(srv, 2000)
	p.Cache = NewCache(t.TempDir(), 1<<20)
	get(t, p, "bytes=0-")
	srv.Close() // from now on only the cache can answer
	rec := get(t, p, "")
	if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), data) {
		t.Errorf("from the cache: %d, %d bytes", rec.Code, rec.Body.Len())
	}
}

func TestProxyTellsWhyItFailed(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	var minGen atomic.Int32
	srv, _ := upstream(t, 1000, &minGen)
	p := testProxy(srv, 1000)
	minGen.Store(100) // YouTube refuses every URL, also resolved again
	if rec := get(t, p, ""); rec.Code != http.StatusBadGateway {
		t.Fatalf("refused: %d", rec.Code)
	}
	if got := p.Failure(testID); !strings.HasPrefix(got, "YouTube refused to send its audio") {
		t.Errorf("refused: Failure = %q", got)
	}
	minGen.Store(0)
	if rec := get(t, p, ""); rec.Code != 200 || p.Failure(testID) != "" {
		t.Errorf("served again: %d, Failure = %q", rec.Code, p.Failure(testID))
	}

	limited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(limited.Close)
	p = testProxy(limited, 1000)
	get(t, p, "")
	if got := p.Failure(testID); !strings.HasPrefix(got, "YouTube is turning down requests") {
		t.Errorf("rate limited: Failure = %q", got)
	}

	srv.Close()
	p = testProxy(srv, 1000)
	get(t, p, "")
	if got := p.Failure(testID); !strings.HasPrefix(got, "MixMusic could not reach YouTube's audio servers") {
		t.Errorf("unreachable: Failure = %q", got)
	}
	if strings.Contains(logs.String(), "gen=") || !strings.Contains(logs.String(), testID) {
		t.Errorf("the log names the stream URL, or not the video:\n%s", logs.String())
	}
}

// TestProxyTellsAudioCame calls Seen when YouTube answers with audio, and
// not when it refuses it or can't be reached.
func TestProxyTellsAudioCame(t *testing.T) {
	var minGen atomic.Int32
	srv, _ := upstream(t, 1000, &minGen)
	p := testProxy(srv, 1000)
	var seen atomic.Int32
	p.Seen = func() { seen.Add(1) }
	if rec := get(t, p, ""); rec.Code != 200 || seen.Load() != 1 {
		t.Errorf("served: %d, seen %d times", rec.Code, seen.Load())
	}
	minGen.Store(100)
	get(t, p, "")
	srv.Close()
	get(t, p, "")
	if seen.Load() != 1 {
		t.Errorf("seen %d times", seen.Load())
	}
}

func TestProxyRejectsBadPaths(t *testing.T) {
	p := NewProxy(NewResolver(nil, ""))
	for _, path := range []string{"/yt/bad", "/other/" + testID, "/yt/"} {
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 404 {
			t.Errorf("%s: %d", path, rec.Code)
		}
	}
}

func TestParseRange(t *testing.T) {
	for _, tc := range []struct {
		header             string
		start, end         int64
		partial, satisfied bool
	}{
		{"", 0, 99, false, true},
		{"bytes=0-", 0, 99, true, true},
		{"bytes=10-19", 10, 19, true, true},
		{"bytes=90-200", 90, 99, true, true},
		{"bytes=-10", 90, 99, true, true},
		{"bytes=100-", 0, 0, true, false},
		{"bytes=a-b", 0, 0, true, false},
	} {
		start, end, partial, ok := parseRange(tc.header, 100)
		if start != tc.start || end != tc.end || partial != tc.partial || ok != tc.satisfied {
			t.Errorf("parseRange(%q) = %d, %d, %v, %v", tc.header, start, end, partial, ok)
		}
	}
}

func TestProxyServesDirectStreamsAtOnce(t *testing.T) {
	f := newFakeTube(t)
	// The media server refuses the URLs that expired, by their expire.
	var expired atomic.Int64
	f.serves(func(w http.ResponseWriter, r *http.Request) {
		if exp, _ := strconv.ParseInt(r.URL.Query().Get("expire"), 10, 64); exp <= expired.Load() {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		serveClen(w, r)
	})
	r := NewResolver(nil, "")
	r.Direct = f.direct()
	r.Run = func(context.Context, ...string) ([]byte, error) {
		t.Error("yt-dlp ran for streams the direct route finds")
		return nil, errors.New("no yt-dlp here")
	}
	p := NewProxy(r)
	get := func(header string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", Path(songID), nil)
		if header != "" {
			req.Header.Set("Range", header)
		}
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, req)
		return rec
	}

	start := time.Now()
	if rec := get(""); rec.Code != 200 || rec.Body.Len() != song251 {
		t.Fatalf("first play: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("served in %s", took)
	}
	var first int64
	f.locked(func() { first = f.expires })
	expired.Store(first)
	if rec := get("bytes=0-99"); rec.Code != 206 || rec.Body.Len() != 100 {
		t.Errorf("after expiry: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	if n := len(f.players()); n != 2 {
		t.Errorf("%d player requests: an expired stream is found again through the direct route", n)
	}
}

// TestProxyServesAndCachesAAC serves the AAC-LC stream of a video YouTube
// has only in AAC, as its type, and keeps it in the cache, which serves it
// as that type without the network, and evicts it as any other.
func TestProxyServesAndCachesAAC(t *testing.T) {
	f := newFakeTube(t)
	f.answers(func(playerRequest) []byte { return withoutOpus(f.recorded("")) })
	r := NewResolver(nil, "")
	r.Direct = f.direct()
	r.Run = func(context.Context, ...string) ([]byte, error) { return nil, errors.New("no yt-dlp here") }
	p := NewProxy(r)
	dir := t.TempDir()
	p.Cache = NewCache(dir, 1<<30)
	get := func(header string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", Path(songID), nil)
		if header != "" {
			req.Header.Set("Range", header)
		}
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, req)
		return rec
	}
	if rec := get(""); rec.Code != 200 || rec.Body.Len() != 3244833 || rec.Header().Get("Content-Type") != `audio/mp4; codecs="mp4a.40.2"` {
		t.Fatalf("%d, %d bytes of %q", rec.Code, rec.Body.Len(), rec.Header().Get("Content-Type"))
	}
	if _, err := os.Stat(filepath.Join(dir, songID+".m4a")); err != nil || !p.Cache.Has(songID) {
		t.Fatalf("not cached as .m4a: %v", err)
	}
	f.srv.Close() // from now on only the cache can answer
	if rec := get("bytes=100-199"); rec.Code != 206 || rec.Body.Len() != 100 || rec.Header().Get("Content-Type") != "audio/mp4" {
		t.Errorf("from the cache: %d, %d bytes of %q", rec.Code, rec.Body.Len(), rec.Header().Get("Content-Type"))
	}
	p.Cache.SetLimit(1 << 20)
	if p.Cache.Has(songID) {
		t.Error("a cached AAC stream outlived a limit it no longer fits")
	}
}

// TestProxyServesKeptFirst serves downloaded audio before the cache and
// YouTube.
func TestProxyServesKeptFirst(t *testing.T) {
	p := NewProxy(NewResolver(nil, ""))
	p.Resolver.Run = func(context.Context, ...string) ([]byte, error) {
		t.Error("the proxy looked for the stream of a track it keeps")
		return nil, errors.New("no")
	}
	p.Cache = NewCache(t.TempDir(), 1<<20)
	p.Kept = keptStore{testID: "kept audio"}
	rec := get(t, p, "bytes=5-9")
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "audio" {
		t.Errorf("%d %q", rec.Code, rec.Body.String())
	}
	if _, n := p.Cache.Usage(); n != 0 {
		t.Errorf("the cache keeps %d tracks", n)
	}
}

type keptStore map[string]string

func (k keptStore) Serve(w http.ResponseWriter, r *http.Request, videoID string) bool {
	data, ok := k[videoID]
	if ok {
		http.ServeContent(w, r, "", time.Time{}, strings.NewReader(data))
	}
	return ok
}
