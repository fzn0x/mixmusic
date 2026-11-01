package stream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recordRanges serves size bytes of audio as upstream does, keeping the
// ranges asked for.
func recordRanges(t *testing.T, size int, minGen *atomic.Int32) (*httptest.Server, []byte, func() []string) {
	t.Helper()
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i * 7)
	}
	var mu sync.Mutex
	var ranges []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var gen int32
		fmt.Sscanf(r.URL.Query().Get("gen"), "%d", &gen)
		if gen < minGen.Load() {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		mu.Lock()
		ranges = append(ranges, r.Header.Get("Range"))
		mu.Unlock()
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	}))
	t.Cleanup(srv.Close)
	return srv, data, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), ranges...)
	}
}

// fromStart is a resume of nothing kept.
func fromStart(Stream) (int64, error) { return 0, nil }

func TestDownloadWritesWholeStreamInChunks(t *testing.T) {
	var minGen atomic.Int32
	size := 2*chunkSize + 12345
	srv, data, ranges := recordRanges(t, size, &minGen)
	p := testProxy(srv, size)
	var buf bytes.Buffer
	var last, calls int64
	s, err := p.Download(context.Background(), testID, fromStart, &buf, func(done, total int64) {
		if done < last || total != int64(size) {
			t.Errorf("progress went from %d to %d of %d", last, done, total)
		}
		last = done
		calls++
	})
	if err != nil || !bytes.Equal(buf.Bytes(), data) {
		t.Fatalf("%d bytes of %d: %v", buf.Len(), size, err)
	}
	if last != int64(size) || calls < 3 {
		t.Errorf("progress told %d of %d, %d times", last, size, calls)
	}
	if s.Size != int64(size) || s.Container() != "webm" {
		t.Errorf("stream %+v", s)
	}
	if r := ranges(); len(r) != 3 || r[0] != fmt.Sprintf("bytes=0-%d", chunkSize-1) {
		t.Errorf("ranges %q", r)
	}
}

func TestDownloadResumesFrom(t *testing.T) {
	var minGen atomic.Int32
	size := 2*chunkSize + 12345
	srv, data, ranges := recordRanges(t, size, &minGen)
	p := testProxy(srv, size)
	from := int64(chunkSize + 10)
	var buf bytes.Buffer
	_, err := p.Download(context.Background(), testID, func(s Stream) (int64, error) {
		if s.Size != int64(size) {
			t.Errorf("resume was given a stream of %d bytes", s.Size)
		}
		return from, nil
	}, &buf, nil)
	if err != nil || !bytes.Equal(buf.Bytes(), data[from:]) {
		t.Fatalf("%d bytes of the %d after %d: %v", buf.Len(), int64(size)-from, from, err)
	}
	if r := ranges(); len(r) == 0 || r[0] != fmt.Sprintf("bytes=%d-%d", from, from+chunkSize-1) {
		t.Errorf("ranges %q", r)
	}
}

// TestDownloadResolvesAgainMidway finds the stream again when YouTube
// refuses its URL after the first chunk, and goes on.
func TestDownloadResolvesAgainMidway(t *testing.T) {
	var minGen atomic.Int32
	size := 2*chunkSize + 100
	srv, data := upstream(t, size, &minGen)
	p := testProxy(srv, size)
	var runs atomic.Int32
	run := p.Resolver.Run
	p.Resolver.Run = func(ctx context.Context, args ...string) ([]byte, error) {
		runs.Add(1)
		return run(ctx, args...)
	}
	var buf bytes.Buffer
	_, err := p.Download(context.Background(), testID, fromStart, &buf, func(done, _ int64) {
		if done >= chunkSize {
			minGen.Store(2) // the first URL expires
		}
	})
	if err != nil || !bytes.Equal(buf.Bytes(), data) {
		t.Fatalf("%d bytes of %d: %v", buf.Len(), size, err)
	}
	if runs.Load() != 2 {
		t.Errorf("yt-dlp ran %d times", runs.Load())
	}
}

// TestDownloadOfAnotherFile stops when the stream found again is another
// file: what came of the first would corrupt the second.
func TestDownloadOfAnotherFile(t *testing.T) {
	var minGen atomic.Int32
	size := 2*chunkSize + 100
	srv, _ := upstream(t, size, &minGen)
	var gen atomic.Int32
	r := NewResolver(nil, "")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		g := gen.Add(1)
		return fmt.Appendf(nil, `{"url": "%s/a?gen=%d&itag=251&lmt=%d&clen=%d", "ext": "webm", "acodec": "opus", "filesize": %d}`,
			srv.URL, g, 1000+g, size, size), nil
	}
	p := NewProxy(r)
	_, err := p.Download(context.Background(), testID, fromStart, &bytes.Buffer{}, func(done, _ int64) {
		if done >= chunkSize {
			minGen.Store(2)
		}
	})
	if !errors.Is(err, errChanged) {
		t.Errorf("a stream of another file: %v", err)
	}
}

// TestDownloadConnectsAgain goes on from the byte it reached after the
// connection drops.
func TestDownloadConnectsAgain(t *testing.T) {
	size := 100000
	srv, data, ranges := breakingUpstream(t, size, 2, false, func(int) int { return 30000 })
	p := testProxy(srv, size)
	p.backoff = 10 * time.Millisecond
	var buf bytes.Buffer
	if _, err := p.Download(context.Background(), testID, fromStart, &buf, nil); err != nil || !bytes.Equal(buf.Bytes(), data) {
		t.Fatalf("%d bytes of %d: %v", buf.Len(), size, err)
	}
	if r := ranges(); len(r) != 3 || r[1] != fmt.Sprintf("bytes=30000-%d", size-1) || r[2] != fmt.Sprintf("bytes=60000-%d", size-1) {
		t.Errorf("ranges %q", r)
	}
}

// TestDownloadRefusesOtherAudio refuses a stream of a format the engine
// doesn't play, before a byte comes.
func TestDownloadRefusesOtherAudio(t *testing.T) {
	r := NewResolver(nil, "")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		return []byte(`{"url": "https://example.com/a", "ext": "mp3", "acodec": "mp3", "filesize": 100}`), nil
	}
	_, err := NewProxy(r).Download(context.Background(), testID, func(Stream) (int64, error) {
		t.Error("resume was asked of a stream that won't play")
		return 0, nil
	}, &bytes.Buffer{}, nil)
	if !errors.Is(err, ErrNoAudio) {
		t.Errorf("an MP3 stream: %v", err)
	}
}

// TestDownloadThrottled tells YouTube turning requests down by its kind,
// in words that never quote the stream's URL.
func TestDownloadThrottled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)
	_, err := testProxy(srv, 1000).Download(context.Background(), testID, fromStart, &bytes.Buffer{}, nil)
	if !errors.Is(err, ErrThrottled) {
		t.Fatalf("HTTP 429: %v", err)
	}
	if why := PlainFailure(err); strings.Contains(why, srv.URL) || strings.Contains(err.Error(), "127.0.0.1") || why == "" {
		t.Errorf("%q, %q", why, err)
	}
}

// TestDownloadCanceled returns at once once canceled, while a chunk waits.
func TestDownloadCanceled(t *testing.T) {
	size := chunkSize + 500
	srv, _, holding := heldUpstream(t, size, 0)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := testProxy(srv, size).Download(ctx, testID, fromStart, &bytes.Buffer{}, nil)
		done <- err
	}()
	<-holding
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("canceled: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the download goes on once canceled")
	}
}

// failing is a writer that fails once it took n bytes, as a full disk.
type failing struct{ n int }

func (f *failing) Write(b []byte) (int, error) {
	if len(b) > f.n {
		n := f.n
		f.n = 0
		return n, errFull
	}
	f.n -= len(b)
	return len(b), nil
}

var errFull = errors.New("disk full")

func TestDownloadWriterFails(t *testing.T) {
	var minGen atomic.Int32
	srv, _ := upstream(t, 100000, &minGen)
	if _, err := testProxy(srv, 100000).Download(context.Background(), testID, fromStart, &failing{n: 5000}, nil); !errors.Is(err, errFull) {
		t.Errorf("a writer that fails: %v", err)
	}
}

func TestStreamVersion(t *testing.T) {
	a := Stream{URL: "https://r1.googlevideo.com/videoplayback?expire=1&itag=251&clen=3612980&lmt=1714898353396736&ip=1.2.3.4", MimeType: "audio/webm"}
	b := a
	b.URL = "https://r7.googlevideo.com/videoplayback?expire=2&itag=251&clen=3612980&lmt=1714898353396736&ip=1.2.3.4"
	if a.Version() != b.Version() || a.Version() != "251/3612980/1714898353396736" || a.Itag() != 251 {
		t.Errorf("versions %q and %q, itag %d", a.Version(), b.Version(), a.Itag())
	}
	if strings.Contains(a.Version(), "1.2.3.4") {
		t.Error("the version holds the address")
	}
	c := b
	c.URL = strings.Replace(c.URL, "itag=251", "itag=250", 1)
	if c.Version() == a.Version() {
		t.Error("another format has the same version")
	}
	plain := Stream{URL: "https://example.com/a", MimeType: "audio/mp4", Size: 10}
	if plain.Version() != "audio/mp4/10" || plain.Container() != "mp4" {
		t.Errorf("version %q, container %q", plain.Version(), plain.Container())
	}
}
