package overlay

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// start returns a server that listens on a port of 127.0.0.1 that the
// system picks, as tests never listen elsewhere.
func start(t *testing.T) (*Server, string) {
	t.Helper()
	s := New()
	s.Cover = func(ctx context.Context, url string) ([]byte, error) {
		if url == "https://covers.test/a.png" {
			return []byte("\x89PNG\r\n\x1a\n0000"), nil
		}
		return nil, errors.New("no such cover")
	}
	t.Cleanup(s.Close)
	if err := s.Apply(true, 0); err != nil {
		t.Fatal(err)
	}
	st := s.Status()
	if !st.On || !strings.HasPrefix(st.Addr, "127.0.0.1:") {
		t.Fatalf("status %+v, want listening on 127.0.0.1", st)
	}
	return s, "http://" + st.Addr
}

func until(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func get(t *testing.T, url string) (*http.Response, []byte) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, body
}

var song = Info{ID: "yt:1", Title: "Song", Artists: []string{"A", "B"}, Album: "Album", Source: "youtube",
	PositionMs: 10000, DurationMs: 200000, Playing: true, CoverURL: "https://covers.test/a.png"}

func TestNowPlaying(t *testing.T) {
	s, base := start(t)
	resp, body := get(t, base+"/now-playing")
	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatalf("%v: %s", err, body)
	}
	if p.Active || p.Artists == nil {
		t.Errorf("nothing plays, yet %s", body)
	}
	s.Update(song)
	resp, body = get(t, base+"/now-playing")
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content type %q", ct)
	}
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatal(err)
	}
	if !p.Active || !p.Playing || p.Title != "Song" || p.Artist != "A, B" || p.Album != "Album" ||
		p.Source != "youtube" || p.DurationMs != 200000 || p.PositionMs < 10000 || p.PositionMs > 15000 {
		t.Errorf("now playing %s", body)
	}
	if !strings.HasPrefix(p.Cover, base+"/cover?v=") {
		t.Errorf("cover %q, want an address of the server", p.Cover)
	}
	// The position moves on while it plays, and rests while paused.
	paused := song
	paused.Playing, paused.PositionMs = false, 30000
	s.Update(paused)
	time.Sleep(30 * time.Millisecond)
	_, body = get(t, base+"/now-playing")
	_ = json.Unmarshal(body, &p)
	if p.Playing || p.PositionMs != 30000 {
		t.Errorf("paused, and %s", body)
	}
}

func TestCover(t *testing.T) {
	s, base := start(t)
	if resp, _ := get(t, base+"/cover"); resp.StatusCode != 404 {
		t.Errorf("no track, cover status %d", resp.StatusCode)
	}
	s.Update(song)
	resp, body := get(t, base+"/cover")
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/png" || !strings.HasPrefix(string(body), "\x89PNG") {
		t.Errorf("cover %d %q %q", resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}
	other := song
	other.ID, other.CoverURL = "yt:2", "https://covers.test/missing.png"
	s.Update(other)
	if resp, _ := get(t, base+"/cover"); resp.StatusCode != 404 {
		t.Errorf("a cover that doesn't load has status %d", resp.StatusCode)
	}
}

func TestOverlayPage(t *testing.T) {
	_, base := start(t)
	resp, body := get(t, base+"/overlay")
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") ||
		!strings.Contains(string(body), "EventSource") {
		t.Errorf("overlay page %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if resp, _ := get(t, base+"/nothing"); resp.StatusCode != 404 {
		t.Errorf("another path has status %d", resp.StatusCode)
	}
}

func TestReadsOnly(t *testing.T) {
	_, base := start(t)
	for _, path := range []string{"/now-playing", "/cover", "/events", "/overlay", "/play", "/"} {
		for _, method := range []string{"POST", "PUT", "DELETE", "PATCH"} {
			req, _ := http.NewRequest(method, base+path, strings.NewReader("x"))
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("%s %s has status %d", method, path, resp.StatusCode)
			}
		}
	}
}

func TestOtherHostsRefused(t *testing.T) {
	_, base := start(t)
	req, _ := http.NewRequest("GET", base+"/now-playing", nil)
	req.Host = "example.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("a name of another host has status %d, want 404", resp.StatusCode)
	}
}

// event is one event of a stream.
type event struct{ name, data string }

// stream reads the events of /events into a channel.
func stream(t *testing.T, base string) (<-chan event, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", base+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		cancel()
		t.Fatalf("content type %q", ct)
	}
	out := make(chan event, 64)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		r := bufio.NewScanner(resp.Body)
		var ev event
		for r.Scan() {
			line := r.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				ev.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				ev.data = strings.TrimPrefix(line, "data: ")
			case line == "" && ev.name != "":
				out <- ev
				ev = event{}
			}
		}
	}()
	return out, cancel
}

func next(t *testing.T, ch <-chan event) payload {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("the stream ended")
		}
		var p payload
		if err := json.Unmarshal([]byte(ev.data), &p); err != nil {
			t.Fatal(err)
		}
		return p
	case <-time.After(5 * time.Second):
		t.Fatal("no event came")
	}
	return payload{}
}

func quiet(t *testing.T, ch <-chan event) {
	t.Helper()
	select {
	case ev := <-ch:
		t.Fatalf("an event came that shouldn't: %s", ev.data)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestEvents(t *testing.T) {
	s, base := start(t)
	ch, stop := stream(t, base)
	defer stop()
	if p := next(t, ch); p.Active {
		t.Errorf("first event of nothing playing: %+v", p)
	}
	s.Update(song)
	if p := next(t, ch); p.Title != "Song" || !p.Playing {
		t.Errorf("track change: %+v", p)
	}
	// The position moving on as it should is no event.
	moved := song
	moved.PositionMs = 10000 + 500
	s.Update(moved)
	quiet(t, ch)
	// A seek is.
	seek := song
	seek.PositionMs = 120000
	s.Update(seek)
	if p := next(t, ch); p.PositionMs < 120000 {
		t.Errorf("seek: %+v", p)
	}
	// So are pausing and playing again.
	pause := seek
	pause.Playing = false
	s.Update(pause)
	if p := next(t, ch); p.Playing {
		t.Errorf("pause: %+v", p)
	}
	s.Update(seek)
	if p := next(t, ch); !p.Playing {
		t.Errorf("play: %+v", p)
	}
	// The same again tells nothing.
	s.Update(seek)
	quiet(t, ch)
}

// TestSlowAndGoneClients has clients that never read and clients that
// leave, and shows the server goes on telling the others, and ends every
// stream's goroutine.
func TestSlowAndGoneClients(t *testing.T) {
	before := runtime.NumGoroutine()
	s, base := start(t)

	// Clients that connect and never read: raw connections that send the
	// request and sit.
	var idle []net.Conn
	for range 5 {
		c, err := net.Dial("tcp", strings.TrimPrefix(base, "http://"))
		if err != nil {
			t.Fatal(err)
		}
		idle = append(idle, c)
		if _, err := c.Write([]byte("GET /events HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n")); err != nil {
			t.Fatal(err)
		}
	}
	// Clients that leave at once.
	for range 5 {
		_, stop := stream(t, base)
		stop()
	}
	good, stopGood := stream(t, base)
	defer stopGood()
	next(t, good)
	// Many changes: the slow clients' buffers fill, and the server never
	// waits on them.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 2000 {
			info := song
			info.ID = "yt:" + strings.Repeat("x", 4096) + string(rune('a'+i%26))
			info.Title = strings.Repeat("t", 4096)
			s.Update(info)
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Update waits for a slow client")
	}
	last := song
	last.ID, last.Title = "yt:last", "Last"
	s.Update(last)
	for {
		if p := next(t, good); p.Title == "Last" {
			break
		}
	}
	for _, c := range idle {
		c.Close()
	}
	stopGood()
	until(t, "every stream to end", func() bool { return s.Streams() == 0 })
	s.Close()
	until(t, "the goroutines to end", func() bool { return runtime.NumGoroutine() <= before+2 })
}

func TestCloseEndsStreams(t *testing.T) {
	s, base := start(t)
	ch, stop := stream(t, base)
	defer stop()
	next(t, ch)
	until(t, "the stream to count", func() bool { return s.Streams() == 1 })
	s.Close()
	select {
	case _, ok := <-ch:
		if ok {
			// A last event may precede the end.
			<-ch
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the stream didn't end as the server closed")
	}
	until(t, "the stream to leave", func() bool { return s.Streams() == 0 })
}

func TestPortInUse(t *testing.T) {
	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	port := held.Addr().(*net.TCPAddr).Port
	s := New()
	defer s.Close()
	if err := s.Apply(true, port); err == nil {
		t.Fatal("a port in use was taken")
	}
	if st := s.Status(); st.On || !strings.Contains(st.Err, "can't be used") {
		t.Errorf("status %+v, want the port's problem", st)
	}
	// Once the port is free, Apply takes it.
	held.Close()
	if err := s.Apply(true, port); err != nil {
		t.Fatalf("the port went free, and %v", err)
	}
	if st := s.Status(); !st.On || st.Err != "" {
		t.Errorf("status %+v", st)
	}
}

func TestOffOnAndPortChange(t *testing.T) {
	s, base := start(t)
	s.Update(song)
	addr := strings.TrimPrefix(base, "http://")
	// Applying the same again changes nothing.
	if err := s.Apply(true, 0); err != nil {
		t.Fatal(err)
	}
	if s.Status().Addr != addr {
		t.Errorf("the same port moved from %s to %s", addr, s.Status().Addr)
	}
	ch, stop := stream(t, base)
	defer stop()
	next(t, ch)
	// Off: nothing listens, and the stream ends.
	if err := s.Apply(false, 0); err != nil {
		t.Fatal(err)
	}
	if s.Status().On {
		t.Error("still on")
	}
	if _, err := http.Get(base + "/now-playing"); err == nil {
		t.Error("the server answers while off")
	}
	until(t, "the stream to end", func() bool { return s.Streams() == 0 })
	// On again: it knows what plays.
	if err := s.Apply(true, 0); err != nil {
		t.Fatal(err)
	}
	base = "http://" + s.Status().Addr
	_, body := get(t, base+"/now-playing")
	if !strings.Contains(string(body), `"title":"Song"`) {
		t.Errorf("after on again: %s", body)
	}
	// Another port while running: the old address goes.
	old := base
	other, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := other.Addr().(*net.TCPAddr).Port
	other.Close()
	if err := s.Apply(true, p); err != nil {
		t.Fatal(err)
	}
	if got := s.Status().Addr; got != net.JoinHostPort("127.0.0.1", strconv.Itoa(p)) {
		t.Errorf("address %s, want port %d", got, p)
	}
	if _, err := http.Get(old + "/now-playing"); err == nil {
		t.Error("the old port still answers")
	}
	if _, body := get(t, "http://"+s.Status().Addr+"/now-playing"); !strings.Contains(string(body), "Song") {
		t.Errorf("on the new port: %s", body)
	}
}

func TestListensOnLoopbackOnly(t *testing.T) {
	var asked string
	s := New()
	s.Listen = func(network, addr string) (net.Listener, error) {
		asked = addr
		return net.Listen(network, "127.0.0.1:0")
	}
	defer s.Close()
	if err := s.Apply(true, 8284); err != nil {
		t.Fatal(err)
	}
	if asked != "127.0.0.1:8284" {
		t.Errorf("listened on %q, want 127.0.0.1:8284", asked)
	}
}
