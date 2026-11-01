package artwork

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// cover is a PNG of w×h pixels of one color.
func cover(w, h int, c color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

var blue = cover(8, 8, color.RGBA{0x20, 0x60, 0xe0, 0xff})

// clock is the store's time, which the test moves.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// lines keeps what the store logs.
type lines struct {
	mu sync.Mutex
	l  []string
}

func (l *lines) logf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.l = append(l.l, fmt.Sprintf(format, args...))
}

func (l *lines) get() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.l...)
}

// testStore is a store in dir, which downloads from srv, on the test's
// clock and log.
func testStore(t *testing.T, dir string, limit int64, srv *httptest.Server) (*Store, *clock, *lines) {
	t.Helper()
	s := NewStore(dir, limit)
	c := &clock{t: time.Now()}
	l := &lines{}
	s.now, s.logf = c.now, l.logf
	if srv != nil {
		s.HTTP = srv.Client()
	}
	t.Cleanup(s.Close)
	return s, c, l
}

// host is a host of images: it serves blue, as handle changes it, and
// counts what it is asked.
type host struct {
	srv    *httptest.Server
	hits   atomic.Int32
	mu     sync.Mutex
	handle func(w http.ResponseWriter, r *http.Request)
	asked  []http.Header
}

func newHost(t *testing.T) *host {
	h := &host{}
	h.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.hits.Add(1)
		h.mu.Lock()
		h.asked = append(h.asked, r.Header.Clone())
		handle := h.handle
		h.mu.Unlock()
		if handle != nil {
			handle(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(blue)
	}))
	t.Cleanup(h.srv.Close)
	return h
}

func (h *host) set(fn func(w http.ResponseWriter, r *http.Request)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handle = fn
}

func (h *host) last() http.Header {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.asked[len(h.asked)-1]
}

func mustBytes(t *testing.T, s *Store, rawURL string) []byte {
	t.Helper()
	data, err := s.Bytes(context.Background(), rawURL)
	if err != nil {
		t.Fatalf("Bytes(%s): %v", rawURL, err)
	}
	return data
}

// TestStoreKeepsCoversOnDisk shows a cover shown once again from disk,
// in the next run too, without its host.
func TestStoreKeepsCoversOnDisk(t *testing.T) {
	h := newHost(t)
	dir := t.TempDir()
	s, _, _ := testStore(t, dir, 1<<20, h.srv)
	for range 2 {
		if got := mustBytes(t, s, h.srv.URL+"/a.png"); !bytes.Equal(got, blue) {
			t.Fatalf("got %d bytes", len(got))
		}
	}
	if n := h.hits.Load(); n != 1 {
		t.Errorf("the host was asked %d times", n)
	}
	s.Close()
	if b, n := s.Usage(); n != 1 || b <= int64(len(blue)) {
		t.Errorf("Usage = %d bytes, %d images", b, n)
	}
	h.srv.Close()
	again, _, _ := testStore(t, dir, 1<<20, nil)
	if got := mustBytes(t, again, h.srv.URL+"/a.png"); !bytes.Equal(got, blue) {
		t.Errorf("the next run got %d bytes", len(got))
	}
	// Images of plain http addresses aren't kept.
	if _, err := again.Bytes(context.Background(), strings.Replace(h.srv.URL, "https:", "http:", 1)+"/a.png"); err == nil {
		t.Error("an http address was served from disk")
	}
}

// TestStoreDownloadsOnce downloads an image once for every caller who
// asks while it downloads.
func TestStoreDownloadsOnce(t *testing.T) {
	h := newHost(t)
	release := make(chan struct{})
	h.set(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.Write(blue)
	})
	s, _, _ := testStore(t, t.TempDir(), 1<<20, h.srv)
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if got, err := s.Bytes(context.Background(), h.srv.URL+"/a.png"); err != nil || !bytes.Equal(got, blue) {
				t.Errorf("got %d bytes, %v", len(got), err)
			}
		})
	}
	// A caller that stops waiting leaves the download to the others.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Bytes(ctx, h.srv.URL+"/a.png"); !errors.Is(err, context.Canceled) {
		t.Errorf("a caller that stopped got %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	if n := h.hits.Load(); n != 1 {
		t.Errorf("the host was asked %d times", n)
	}
}

// TestStoreHonoursNoStore keeps no image its host says not to.
func TestStoreHonoursNoStore(t *testing.T) {
	h := newHost(t)
	h.set(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Write(blue)
	})
	s, _, _ := testStore(t, t.TempDir(), 1<<20, h.srv)
	mustBytes(t, s, h.srv.URL+"/a.png")
	mustBytes(t, s, h.srv.URL+"/a.png")
	if n := h.hits.Load(); n != 2 {
		t.Errorf("the host was asked %d times", n)
	}
	if _, n := s.Usage(); n != 0 {
		t.Errorf("%d images kept", n)
	}
}

// TestStoreRevalidates asks the host of an image past its time whether
// it changed, with both of its validators, while the image shows.
func TestStoreRevalidates(t *testing.T) {
	h := newHost(t)
	red := cover(8, 8, color.RGBA{0xe0, 0x30, 0x30, 0xff})
	var changed atomic.Bool
	h.set(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=3600")
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Last-Modified", "Sun, 01 Nov 2026 01:00:00 GMT")
		// As i.scdn.co does, it answers If-Modified-Since alone.
		if !changed.Load() && r.Header.Get("If-Modified-Since") != "" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if changed.Load() {
			w.Write(red)
			return
		}
		w.Write(blue)
	})
	s, clk, _ := testStore(t, t.TempDir(), 1<<20, h.srv)
	a := h.srv.URL + "/a.png"
	mustBytes(t, s, a)
	clk.advance(30 * time.Minute)
	mustBytes(t, s, a)
	s.Close()
	if n := h.hits.Load(); n != 1 {
		t.Fatalf("a fresh image was checked: %d requests", n)
	}
	clk.advance(time.Hour)
	if got := mustBytes(t, s, a); !bytes.Equal(got, blue) {
		t.Error("a stale image didn't show from disk")
	}
	s.Close()
	if n := h.hits.Load(); n != 2 {
		t.Fatalf("a stale image wasn't checked: %d requests", n)
	}
	if asked := h.last(); asked.Get("If-None-Match") != `"v1"` || asked.Get("If-Modified-Since") == "" {
		t.Errorf("checked with %v", asked)
	}
	// The answer renewed its time.
	mustBytes(t, s, a)
	s.Close()
	if n := h.hits.Load(); n != 2 {
		t.Errorf("a renewed image was checked again: %d requests", n)
	}
	// A changed image replaces the one kept.
	changed.Store(true)
	clk.advance(2 * time.Hour)
	mustBytes(t, s, a)
	s.Close()
	if got := mustBytes(t, s, a); !bytes.Equal(got, red) {
		t.Error("the changed image didn't replace the one kept")
	}
}

// TestStoreServesStaleAtOnce shows a stale image at once, while its host
// doesn't answer, and offline.
func TestStoreServesStaleAtOnce(t *testing.T) {
	h := newHost(t)
	s, clk, l := testStore(t, t.TempDir(), 1<<20, h.srv)
	a := h.srv.URL + "/a.png"
	mustBytes(t, s, a)
	hang := make(chan struct{})
	h.set(func(w http.ResponseWriter, r *http.Request) { <-hang })
	t.Cleanup(func() { close(hang) }) // before the store's Close
	clk.advance(8 * 24 * time.Hour)
	start := time.Now()
	if got := mustBytes(t, s, a); !bytes.Equal(got, blue) || time.Since(start) > time.Second {
		t.Errorf("a stale image took %v", time.Since(start))
	}
	if len(l.get()) != 0 {
		t.Errorf("a check logged %q", l.get())
	}
}

// TestStorePrunesLeastRecentlyShown makes room past the limit, from the
// image shown least recently.
func TestStorePrunesLeastRecentlyShown(t *testing.T) {
	h := newHost(t)
	dir := t.TempDir()
	probe, _, _ := testStore(t, t.TempDir(), 1<<20, h.srv)
	mustBytes(t, probe, h.srv.URL+"/x.png")
	probe.Close()
	size, _ := probe.Usage()
	s, clk, _ := testStore(t, dir, size*7/2, h.srv)
	s.Usage()
	s.Close()
	for _, name := range []string{"a", "b", "c"} {
		mustBytes(t, s, h.srv.URL+"/"+name+".png")
		time.Sleep(20 * time.Millisecond) // the files' times differ
	}
	// a shows again, two hours later: the last to go.
	clk.advance(2 * time.Hour)
	mustBytes(t, s, h.srv.URL+"/a.png")
	mustBytes(t, s, h.srv.URL+"/d.png")
	s.Close()
	for name, want := range map[string]bool{"a": true, "b": false, "c": true, "d": true} {
		_, err := os.Stat(filepath.Join(dir, key(h.srv.URL+"/"+name+".png")))
		if got := err == nil; got != want {
			t.Errorf("%s kept %v", name, got)
		}
	}
	if b, n := s.Usage(); n != 3 || b > size*7/2 {
		t.Errorf("Usage = %d bytes, %d images, of %d", b, n, size*7/2)
	}
}

// TestStoreClear removes every image kept.
func TestStoreClear(t *testing.T) {
	h := newHost(t)
	dir := t.TempDir()
	s, _, _ := testStore(t, dir, 1<<20, h.srv)
	mustBytes(t, s, h.srv.URL+"/a.png")
	mustBytes(t, s, h.srv.URL+"/b.png")
	s.Close()
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	if b, n := s.Usage(); b != 0 || n != 0 {
		t.Errorf("Usage = %d bytes, %d images", b, n)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("%d files left", len(entries))
	}
}

// TestStoreErrors tells a failure that may pass from one that won't.
func TestStoreErrors(t *testing.T) {
	h := newHost(t)
	h.set(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/missing":
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusNotFound)
			w.Write(blue)
		case "/busy":
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusServiceUnavailable)
		case "/cut":
			w.Header().Set("Content-Length", "1000")
			w.Write(blue[:10])
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
		case "/page":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html>"))
		case "/huge":
			w.Write(make([]byte, 9<<20))
		}
	})
	s, _, _ := testStore(t, t.TempDir(), 1<<20, h.srv)
	for _, c := range []struct {
		path       string
		status     int
		temporary  bool
		retryAfter time.Duration
		says       string
	}{
		{"/missing", 404, false, 0, "HTTP 404"},
		{"/busy", 503, true, 7 * time.Second, "HTTP 503"},
		{"/cut", 0, true, 0, ""},
		{"/page", 0, false, 0, "not an image (text/html)"},
		{"/huge", 0, false, 0, "larger than 8 MB"},
	} {
		_, err := s.Bytes(context.Background(), h.srv.URL+c.path)
		var e *Error
		if !errors.As(err, &e) {
			t.Errorf("%s: %v", c.path, err)
			continue
		}
		if e.Status != c.status || e.Temporary != c.temporary || e.RetryAfter != c.retryAfter || !strings.Contains(e.Error(), c.says) {
			t.Errorf("%s: %+v", c.path, *e)
		}
	}
	var e *Error
	if _, err := s.Bytes(context.Background(), "ftp://example.com/a.png"); !errors.As(err, &e) || e.Temporary {
		t.Errorf("ftp: %v", err)
	}
}

// TestStoreLogsOncePerHostAMinute logs a host's failures at most once a
// minute, with how many there were, and when its covers load again,
// without their addresses.
func TestStoreLogsOncePerHostAMinute(t *testing.T) {
	h := newHost(t)
	h.set(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })
	s, clk, l := testStore(t, t.TempDir(), 1<<20, h.srv)
	for i := range 5 {
		s.Bytes(context.Background(), fmt.Sprintf("%s/vi/%d/hq.jpg?sqp=-oaymwE&rs=AOn4CLB", h.srv.URL, i))
		clk.advance(time.Second)
	}
	if got := l.get(); len(got) != 1 || !strings.Contains(got[0], "a cover from 127.0.0.1 didn't load (") || !strings.Contains(got[0], "HTTP 404") {
		t.Fatalf("logged %q", got)
	}
	clk.advance(time.Minute)
	s.Bytes(context.Background(), h.srv.URL+"/vi/5/hq.jpg?sqp=-oaymwE")
	got := l.get()
	if len(got) != 2 || !strings.Contains(got[1], "4 more didn't since") {
		t.Fatalf("logged %q", got)
	}
	h.set(nil)
	mustBytes(t, s, h.srv.URL+"/vi/6/hq.jpg")
	got = l.get()
	if len(got) != 3 || got[2] != "mixmusic: artwork: covers from 127.0.0.1 load again" {
		t.Fatalf("logged %q", got)
	}
	for _, line := range got {
		if strings.Contains(line, "?") || strings.Contains(line, "sqp=") || strings.Contains(line, "/vi/") {
			t.Errorf("a line quotes the address: %q", line)
		}
	}

	// A connection reset quotes the computer's own address and port.
	reset := newHost(t)
	reset.set(func(w http.ResponseWriter, r *http.Request) {
		conn, _, _ := w.(http.Hijacker).Hijack()
		if tc, ok := conn.(*tls.Conn); ok {
			if c, ok := tc.NetConn().(*net.TCPConn); ok {
				c.SetLinger(0)
			}
		}
		conn.Close()
	})
	s, _, l = testStore(t, t.TempDir(), 1<<20, reset.srv)
	s.Bytes(context.Background(), reset.srv.URL+"/a.png")
	for _, line := range l.get() {
		if strings.Contains(line, "->") && !strings.Contains(line, "…->") {
			t.Errorf("a line quotes the computer's address: %q", line)
		}
	}
}

// TestStoreImageDropsBadFile drops a file that doesn't decode, and
// downloads the image again.
func TestStoreImageDropsBadFile(t *testing.T) {
	h := newHost(t)
	dir := t.TempDir()
	s, _, _ := testStore(t, dir, 1<<20, h.srv)
	a := h.srv.URL + "/a.png"
	path := filepath.Join(dir, key(a))
	os.MkdirAll(dir, 0o755)
	os.WriteFile(path, []byte(`{"url":"`+a+`","fresh":"2099-01-01T00:00:00Z"}`+"\nnot a png"), 0o644)
	var e *Error
	if _, err := s.Image(context.Background(), a); !errors.As(err, &e) || e.Temporary {
		t.Fatalf("a bad file gave %v", err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("the bad file stays")
	}
	if _, err := s.Image(context.Background(), a); err != nil || h.hits.Load() != 1 {
		t.Errorf("downloading it again: %v, %d requests", err, h.hits.Load())
	}
}

// TestStoreCloseWhileOthersStart waits in Close while callers go on
// starting downloads, as when a test ends with its window still asking for
// covers. Run it with -race.
func TestStoreCloseWhileOthersStart(t *testing.T) {
	h := newHost(t)
	s, _, _ := testStore(t, t.TempDir(), 1<<20, h.srv)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for j := range 40 {
				if _, err := s.Bytes(context.Background(), fmt.Sprintf("%s/%d-%d.png", h.srv.URL, i, j)); err != nil {
					t.Error(err)
				}
			}
		})
	}
	for range 200 {
		s.Close()
	}
	wg.Wait()
}

// silencer is a dialer whose connections it can make go silent, as a
// network change can leave a connection: nothing arrives, and nothing it
// is sent goes anywhere, but it doesn't close.
type silencer struct {
	mu    sync.Mutex
	conns []*silentConn
}

type silentConn struct {
	net.Conn
	silent atomic.Bool
	closed chan struct{}
	once   sync.Once
}

func (d *silencer) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	c, err := (&net.Dialer{}).DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	sc := &silentConn{Conn: c, closed: make(chan struct{})}
	d.mu.Lock()
	d.conns = append(d.conns, sc)
	d.mu.Unlock()
	return sc, nil
}

// silence silences the connections made so far.
func (d *silencer) silence() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, c := range d.conns {
		c.silent.Store(true)
	}
}

func (c *silentConn) Read(p []byte) (int, error) {
	if !c.silent.Load() {
		n, err := c.Conn.Read(p)
		if !c.silent.Load() {
			return n, err
		}
	}
	<-c.closed
	return 0, net.ErrClosed
}

func (c *silentConn) Write(p []byte) (int, error) {
	if c.silent.Load() {
		return len(p), nil
	}
	return c.Conn.Write(p)
}

func (c *silentConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return c.Conn.Close()
}

// TestStoreDropsSilentConnections drops an HTTP/2 connection that went
// silent, so that the next covers download over a new one, rather than
// time out on it one after the other.
func TestStoreDropsSilentConnections(t *testing.T) {
	was := pingAfter
	pingAfter = 200 * time.Millisecond
	t.Cleanup(func() { pingAfter = was })
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(blue) }))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)
	s, _, _ := testStore(t, t.TempDir(), 1<<20, nil)
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	d := &silencer{}
	tr := s.HTTP.Transport.(*http.Transport)
	tr.TLSClientConfig, tr.DialContext, tr.Proxy = &tls.Config{RootCAs: pool}, d.dial, nil
	s.HTTP.Timeout = 3 * time.Second
	res, err := s.HTTP.Get(srv.URL + "/probe")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.ProtoMajor != 2 {
		t.Fatalf("the cover came over HTTP/%d", res.ProtoMajor)
	}
	d.silence()
	deadline := time.Now().Add(10 * time.Second)
	for i := 0; ; i++ {
		if _, err := s.Bytes(context.Background(), fmt.Sprintf("%s/%d.png", srv.URL, i)); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("covers still time out on the silent connection")
		}
	}
}
