package stream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// logs has the log written to a buffer for the test, which reads it with
// String.
type logs struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func captureLog(t *testing.T) *logs {
	l := &logs{}
	log.SetOutput(l)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return l
}

func (l *logs) Write(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(b)
}

func (l *logs) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// readAll reads all of s, failing the test unless it ends within a few
// seconds.
func readAll(t *testing.T, s *Source) ([]byte, error) {
	t.Helper()
	type result struct {
		b   []byte
		err error
	}
	done := make(chan result, 1)
	go func() {
		b, err := io.ReadAll(s)
		done <- result{b, err}
	}()
	select {
	case r := <-done:
		return r.b, r.err
	case <-time.After(10 * time.Second):
		t.Fatal("the read waits on")
		return nil, nil
	}
}

// breakingUpstream serves size bytes of audio with range requests. Its
// first answers send what break leaves of them, and then break off: hold
// stops sending without closing the connection, until the client gives
// up, and drop closes it. It counts the requests, and keeps their ranges.
func breakingUpstream(t *testing.T, size, answers int, hold bool, keep func(i int) int) (srv *httptest.Server, data []byte, ranges func() []string) {
	t.Helper()
	data = audioBytes(size)
	var mu sync.Mutex
	var seen []string
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		i := len(seen)
		seen = append(seen, r.Header.Get("Range"))
		mu.Unlock()
		if i >= answers {
			http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
			return
		}
		var from int
		fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-", &from)
		n := keep(i)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", from, size-1, size))
		w.Header().Set("Content-Length", fmt.Sprint(size-from))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(data[from : from+n])
		w.(http.Flusher).Flush()
		if !hold {
			panic(http.ErrAbortHandler)
		}
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	return srv, data, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), seen...)
	}
}

// TestProxyResumesAStalledDownload connects again to a download that stops
// sending, from the byte it reached, and plays on.
func TestProxyResumesAStalledDownload(t *testing.T) {
	logged := captureLog(t)
	size := 100000
	srv, data, ranges := breakingUpstream(t, size, 1, true, func(int) int { return 30000 })
	p := testProxy(srv, size)
	p.stall, p.backoff = 300*time.Millisecond, 10*time.Millisecond
	s, err := p.Open(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := readAll(t, s)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("%d bytes of %d, %v", len(got), size, err)
	}
	if why := p.Failure(testID); why != "" {
		t.Errorf("Failure = %q", why)
	}
	if r := ranges(); len(r) != 2 || r[1] != fmt.Sprintf("bytes=30000-%d", size-1) {
		t.Errorf("requests %q", r)
	}
	if l := logged.String(); !strings.Contains(l, testID+" broke off at byte 30000") || !strings.Contains(l, "comes again") {
		t.Errorf("the log doesn't tell the break:\n%s", l)
	}
}

// TestProxyResumesADroppedConnection does the same for a connection that
// drops, several times.
func TestProxyResumesADroppedConnection(t *testing.T) {
	captureLog(t)
	size := 100000
	srv, data, ranges := breakingUpstream(t, size, 3, false, func(i int) int { return 10000 })
	p := testProxy(srv, size)
	p.backoff = 10 * time.Millisecond
	s, err := p.Open(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := readAll(t, s)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("%d bytes of %d, %v", len(got), size, err)
	}
	if r := ranges(); len(r) != 4 || r[3] != fmt.Sprintf("bytes=30000-%d", size-1) {
		t.Errorf("requests %q", r)
	}
}

// TestProxyGivesUpAStalledDownload gives up a download that stays stalled
// once it kept connecting again for its time, with why.
func TestProxyGivesUpAStalledDownload(t *testing.T) {
	logged := captureLog(t)
	size := 100000
	// The first answer brings a few bytes, and the others none.
	srv, _, ranges := breakingUpstream(t, size, 1<<30, true, func(i int) int {
		if i == 0 {
			return 30000
		}
		return 0
	})
	p := testProxy(srv, size)
	p.stall, p.reconnect, p.backoff = 200*time.Millisecond, time.Second, 50*time.Millisecond
	s, err := p.Open(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	started := time.Now()
	got, err := readAll(t, s)
	if len(got) != 30000 || err == nil || !strings.HasPrefix(err.Error(), "YouTube stopped sending the audio") {
		t.Fatalf("%d bytes, %v", len(got), err)
	}
	if took := time.Since(started); took < p.reconnect/2 {
		t.Errorf("gave up after %s", took)
	}
	if n := len(ranges()); n < 3 {
		t.Errorf("%d requests", n)
	}
	l := logged.String()
	if !strings.Contains(l, testID+" broke off at byte 30000 of 100000") || !strings.Contains(l, testID+": stream: YouTube stopped sending the audio") ||
		strings.Contains(l, "gen=") {
		t.Errorf("the log doesn't tell the stall, or names the URL:\n%s", l)
	}
}

// audioBytes is size bytes of audio, as the test servers serve it.
func audioBytes(size int) []byte {
	b := make([]byte, size)
	for i := range b {
		b[i] = byte(i * 13)
	}
	return b
}

// TestProxyGivesUpAHeldChunk gives up a download whose next chunk YouTube
// never answers.
func TestProxyGivesUpAHeldChunk(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	size := chunkSize + 1000
	srv := stallingUpstream(t, size, release)
	p := testProxy(srv, size)
	// A whole chunk comes over the loopback: a stall of a second leaves a
	// busy runner room.
	p.stall, p.reconnect, p.backoff = time.Second, time.Second, 100*time.Millisecond
	s, err := p.Open(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := readAll(t, s)
	if len(got) != chunkSize || err == nil || !strings.HasPrefix(err.Error(), "YouTube stopped sending the audio") {
		t.Fatalf("%d bytes, %v", len(got), err)
	}
}

// TestProxyGivesUpWithoutAnAnswer fails a request YouTube never answers.
func TestProxyGivesUpWithoutAnAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	p := testProxy(srv, 1000)
	p.stall = 200 * time.Millisecond
	started := time.Now()
	if rec := get(t, p, ""); rec.Code != http.StatusBadGateway {
		t.Errorf("status %d", rec.Code)
	}
	if took := time.Since(started); took > 3*time.Second {
		t.Errorf("gave up after %s", took)
	}
	if why := p.Failure(testID); !strings.HasPrefix(why, "YouTube stopped sending the audio") {
		t.Errorf("Failure = %q", why)
	}
}

// TestProxyGivesUpTheSizeWithoutAnAnswer fails a stream whose length yt-dlp
// didn't know when YouTube never answers the request for it.
func TestProxyGivesUpTheSizeWithoutAnAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	r := NewResolver(nil, "")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		return fmt.Appendf(nil, `{"url": "%s/a", "ext": "webm", "acodec": "opus"}`, srv.URL), nil
	}
	p := NewProxy(r)
	p.stall = 200 * time.Millisecond
	if rec := get(t, p, ""); rec.Code != http.StatusBadGateway {
		t.Errorf("status %d", rec.Code)
	}
	if why := p.Failure(testID); !strings.HasPrefix(why, "YouTube stopped sending the audio") {
		t.Errorf("Failure = %q", why)
	}
}

// TestProxyWaitsForASlowStream lets a download take its time while bytes
// come: the limit is on the wait for each, not on the request.
func TestProxyWaitsForASlowStream(t *testing.T) {
	size := 40
	audio := audioBytes(size)
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Length", fmt.Sprint(size))
		w.WriteHeader(http.StatusOK)
		for i := range size {
			w.Write(audio[i : i+1])
			w.(http.Flusher).Flush()
			time.Sleep(25 * time.Millisecond)
		}
	}))
	t.Cleanup(srv.Close)
	p := testProxy(srv, size)
	p.stall = 300 * time.Millisecond
	rec := get(t, p, "")
	if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), audio) {
		t.Errorf("status %d, %d bytes of %d", rec.Code, rec.Body.Len(), size)
	}
	if why := p.Failure(testID); why != "" || requests.Load() != 1 {
		t.Errorf("Failure = %q, %d requests", why, requests.Load())
	}
}
