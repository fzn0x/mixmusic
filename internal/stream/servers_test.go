package stream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// The servers of the streams of these tests: the one of their URLs, which
// the network doesn't reach, and the next one, which serves them.
const (
	deadServer = "rr5---sn-dead.googlevideo.com"
	liveServer = "rr4---sn-live.googlevideo.com"
	// serverParams name both, as YouTube's URLs do, and the computer's IP
	// address, which the log never tells.
	serverParams = "mn=sn-dead%2Csn-live&mvi=5&fvip=4&ip=203.0.113.7"
)

func TestServers(t *testing.T) {
	for _, c := range []struct {
		url  string
		want []string // the hosts, with fallback_count
	}{
		{"https://" + deadServer + "/videoplayback?expire=1&" + serverParams,
			[]string{deadServer, liveServer + " 1"}},
		// The URL of the next server comes first, then the first server's.
		{"https://" + liveServer + "/videoplayback?expire=1&fallback_count=1&" + serverParams,
			[]string{liveServer + " 1", deadServer}},
		{"https://rr1---sn-a.googlevideo.com/videoplayback?mn=sn-a,sn-b,sn-c&mvi=1&fvip=2",
			[]string{"rr1---sn-a.googlevideo.com", "rr2---sn-b.googlevideo.com 1", "rr2---sn-c.googlevideo.com 2"}},
		// Names that make no server's host are passed over.
		{"https://rr1---sn-a.googlevideo.com/videoplayback?mn=sn-a,evil.example%2F,&mvi=1&fvip=2", []string{"rr1---sn-a.googlevideo.com"}},
		{"https://rr1---sn-a.googlevideo.com/videoplayback?mn=sn-a,sn-b&mvi=1", []string{"rr1---sn-a.googlevideo.com"}},
		{"https://rr1---sn-a.googlevideo.com/videoplayback?expire=1", []string{"rr1---sn-a.googlevideo.com"}},
		// Only YouTube's media servers have others.
		{"https://example.com/videoplayback?" + serverParams, []string{"example.com"}},
	} {
		u, _ := url.Parse(c.url)
		var got []string
		for i, s := range servers(u) {
			q := s.Query()
			host := s.Host
			if n := q.Get("fallback_count"); n != "" {
				host += " " + n
			}
			got = append(got, host)
			if i == 0 {
				if s != u {
					t.Errorf("%s: the first is not the URL itself", c.url)
				}
				continue
			}
			// The rest of the URL stays as it was.
			q.Del("fallback_count")
			orig := u.Query()
			orig.Del("fallback_count")
			if s.Path != u.Path || q.Encode() != orig.Encode() {
				t.Errorf("%s: %s", c.url, s)
			}
		}
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%s: %v, want %v", c.url, got, c.want)
		}
	}
}

// silentServer accepts connections and never answers on them, as a server
// the network reaches no further than its TCP handshake: a TLS handshake
// with it waits on. It returns its address, and counts the connections.
func silentServer(t *testing.T) (addr string, conns func() int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var held []net.Conn
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			held = append(held, c)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, c := range held {
			c.Close()
		}
	})
	return ln.Addr().String(), func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(held)
	}
}

// refusingAddress is an address where nothing listens.
func refusingAddress(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

// serversClient returns a client of a network where each host of servers
// is at its address, other names don't resolve and IP addresses are as
// they are, over TLS that trusts the test servers' certificate.
func serversClient(t *testing.T, servers map[string]string) *http.Client {
	t.Helper()
	// Every test server has the same certificate, of example.com.
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	tlsConfig := srv.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	tlsConfig.ServerName = "example.com"
	var d net.Dialer
	tr := &http.Transport{
		TLSClientConfig:     tlsConfig,
		TLSHandshakeTimeout: 10 * time.Second, // as http.DefaultTransport's
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, _ := net.SplitHostPort(addr)
			if to, ok := servers[host]; ok {
				return d.DialContext(ctx, network, to)
			}
			if net.ParseIP(host) == nil {
				return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
			}
			return d.DialContext(ctx, network, addr)
		},
	}
	t.Cleanup(tr.CloseIdleConnections)
	return &http.Client{Transport: tr}
}

// audioServer serves data over TLS with range requests, and keeps the host
// and the fallback_count of each request.
func audioServer(t *testing.T, data []byte) (srv *httptest.Server, asked func() []string) {
	t.Helper()
	var mu sync.Mutex
	var seen []string
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Host+" "+r.URL.Query().Get("fallback_count"))
		mu.Unlock()
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), seen...)
	}
}

// serversProxy returns a proxy whose yt-dlp finds the stream of size bytes
// on deadServer, then on liveServer, over client.
func serversProxy(client *http.Client, size int) *Proxy {
	r := NewResolver(nil, "")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		return fmt.Appendf(nil, `{"url": "https://%s/videoplayback?expire=%d&%s", "ext": "webm", "acodec": "opus", "filesize": %d}`,
			deadServer, time.Now().Add(6*time.Hour).Unix(), serverParams, size), nil
	}
	p := NewProxy(r)
	p.HTTP = client
	return p
}

// TestProxyAsksTheNextServer serves a stream whose server can't be reached
// from the stream's next server, as YouTube's player does: whether that
// server never answers the TLS handshake, refuses the connection or has no
// address. The requests after go to the next server at once.
func TestProxyAsksTheNextServer(t *testing.T) {
	size := chunkSize + 1000
	data := audioBytes(size)
	live, asked := audioServer(t, data)
	silent, conns := silentServer(t)
	for _, dead := range []struct{ name, addr string }{
		{"no TLS answer", silent},
		{"refused", refusingAddress(t)},
		{"no address", ""},
	} {
		logged := captureLog(t)
		hosts := map[string]string{liveServer: live.Listener.Addr().String()}
		if dead.addr != "" {
			hosts[deadServer] = dead.addr
		}
		p := serversProxy(serversClient(t, hosts), size)
		p.media.connect = 300 * time.Millisecond
		before, started := conns(), time.Now()
		rec := get(t, p, "")
		if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), data) || p.Failure(testID) != "" {
			t.Fatalf("%s: %d, %d bytes of %d, Failure %q", dead.name, rec.Code, rec.Body.Len(), size, p.Failure(testID))
		}
		if took := time.Since(started); took > 5*time.Second {
			t.Errorf("%s: served after %s", dead.name, took)
		}
		// A seek asks the next server at once too.
		if rec := get(t, p, "bytes=1000-"); rec.Code != http.StatusPartialContent || !bytes.Equal(rec.Body.Bytes(), data[1000:]) {
			t.Errorf("%s: after a seek: %d, %d bytes", dead.name, rec.Code, rec.Body.Len())
		}
		if dead.addr == silent && conns()-before != 1 {
			t.Errorf("%s: %d connections to the server that doesn't answer", dead.name, conns()-before)
		}
		for _, a := range asked() {
			if a != liveServer+" 1" {
				t.Errorf("%s: asked %q", dead.name, a)
			}
		}
		l := logged.String()
		if !strings.Contains(l, "server 1 of 2 of a stream didn't answer") || strings.Count(l, "didn't answer") != 1 ||
			strings.Contains(l, "googlevideo") || strings.Contains(l, "sn-") || strings.Contains(l, "203.0.113.7") {
			t.Errorf("%s: the log doesn't tell the server didn't answer, or names it:\n%s", dead.name, l)
		}
	}
}

// TestProxyAsksNoOtherServerOfAnAnswer keeps to the stream's server when it
// answered: a refusal, a limit on requests or an error are YouTube's
// answer about the stream, which its other servers give too.
func TestProxyAsksNoOtherServerOfAnAnswer(t *testing.T) {
	for status, why := range map[int]string{
		http.StatusForbidden:          "YouTube refused to send its audio",
		http.StatusTooManyRequests:    "YouTube is turning down requests",
		http.StatusServiceUnavailable: "YouTube answered HTTP 503",
	} {
		first := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
		t.Cleanup(first.Close)
		next, asked := audioServer(t, audioBytes(1000))
		p := serversProxy(serversClient(t, map[string]string{deadServer: first.Listener.Addr().String(), liveServer: next.Listener.Addr().String()}), 1000)
		p.media.connect = 300 * time.Millisecond
		if rec := get(t, p, ""); rec.Code != http.StatusBadGateway || !strings.HasPrefix(p.Failure(testID), why) {
			t.Errorf("HTTP %d: %d, Failure %q", status, rec.Code, p.Failure(testID))
		}
		if a := asked(); len(a) != 0 {
			t.Errorf("HTTP %d: asked the next server: %q", status, a)
		}
	}
}

// TestProxyGivesUpWhenNoServerAnswers fails a stream none of whose servers
// can be reached, as before: the last one asked has the transport's own
// limits, which a slow network may need.
func TestProxyGivesUpWhenNoServerAnswers(t *testing.T) {
	logged := captureLog(t)
	first, firstConns := silentServer(t)
	next, nextConns := silentServer(t)
	client := serversClient(t, map[string]string{deadServer: first, liveServer: next})
	handshake := 700 * time.Millisecond
	client.Transport.(*http.Transport).TLSHandshakeTimeout = handshake
	p := serversProxy(client, 1000)
	p.media.connect = 100 * time.Millisecond
	started := time.Now()
	if rec := get(t, p, ""); rec.Code != http.StatusBadGateway {
		t.Errorf("status %d", rec.Code)
	}
	if took := time.Since(started); took < handshake || took > 5*time.Second {
		t.Errorf("gave up after %s", took)
	}
	if why := p.Failure(testID); !strings.HasPrefix(why, "MixMusic could not reach YouTube's audio servers") {
		t.Errorf("Failure = %q", why)
	}
	if firstConns() != 1 || nextConns() != 1 {
		t.Errorf("%d and %d connections", firstConns(), nextConns())
	}
	if l := logged.String(); !strings.Contains(l, "TLS handshake timeout") || strings.Contains(l, "googlevideo") {
		t.Errorf("the log doesn't tell why, or names the server:\n%s", l)
	}
}

// TestDirectAsksTheNextServer checks a stream of the direct route on its
// next server when its own can't be reached, within the direct route's
// time, and the stream goes on with that server: yt-dlp doesn't run.
func TestDirectAsksTheNextServer(t *testing.T) {
	logged := captureLog(t)
	f := newFakeTube(t)
	f.answers(func(playerRequest) []byte {
		return []byte(strings.ReplaceAll(string(f.recorded("")), f.srv.URL+"/media/videoplayback?",
			"https://"+deadServer+"/videoplayback?"+serverParams+"&"))
	})
	live := httptest.NewTLSServer(http.HandlerFunc(serveClen))
	t.Cleanup(live.Close)
	silent, conns := silentServer(t)
	client := serversClient(t, map[string]string{deadServer: silent, liveServer: live.Listener.Addr().String()})

	d := f.direct()
	d.HTTP = client
	d.media.connect = 300 * time.Millisecond
	r := NewResolver(nil, "")
	r.Direct = d
	r.Run = func(context.Context, ...string) ([]byte, error) {
		t.Error("yt-dlp ran")
		return nil, errors.New("no yt-dlp")
	}
	s, err := r.Resolve(context.Background(), songID)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(s.URL)
	if u.Host != liveServer || u.Query().Get("fallback_count") != "1" || s.Size != song251 {
		t.Errorf("stream on %s, fallback_count %q, of %d bytes", u.Host, u.Query().Get("fallback_count"), s.Size)
	}
	if l := logged.String(); !strings.Contains(l, "server 1 of 2 of a stream didn't answer (no connection in 300ms)") || strings.Contains(l, "sn-") {
		t.Errorf("the log doesn't tell the server didn't answer, or names it:\n%s", l)
	}

	// The next stream asks the server that answered first.
	if _, err := d.Stream(context.Background(), songID, QualityBest); err != nil || conns() != 1 {
		t.Errorf("the next stream: %v, after %d connections to the server that doesn't answer", err, conns())
	}

	// Each server has its share of the time the direct route has left.
	d = f.direct()
	d.HTTP = client
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := d.Stream(ctx, songID, QualityBest); err != nil {
		t.Errorf("in 2s: %v", err)
	}
}

// TestLiveNextServer finds the stream of a song, and then again with its
// server never answering, as a network may leave one of YouTube's servers:
// the direct route checks it on the stream's next server within its time,
// and the proxy reads all of it from there. It runs with MIXMUSIC_LIVE=1:
//
//	MIXMUSIC_LIVE=1 go test ./internal/stream -run LiveNextServer -v
func TestLiveNextServer(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to test against YouTube")
	}
	logged := captureLog(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s, err := NewDirect().Stream(ctx, musicVideoID, QualityBest)
	if err != nil {
		t.Fatal(err)
	}
	// Never the URL, nor the servers' names: they tell where this machine is.
	u, _ := url.Parse(s.URL)
	own := u.Host
	if n := len(servers(u)); n < 2 {
		t.Fatalf("the stream names %d servers", n)
	}

	silent, conns := silentServer(t)
	tr := http.DefaultTransport.(*http.Transport).Clone()
	var dialer net.Dialer
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if host, _, _ := net.SplitHostPort(addr); host == own {
			addr = silent
		}
		return dialer.DialContext(ctx, network, addr)
	}
	t.Cleanup(tr.CloseIdleConnections)
	client := &http.Client{Transport: tr}

	d := NewDirect()
	d.HTTP = client
	dctx, dcancel := context.WithTimeout(ctx, directTimeout)
	start := time.Now()
	moved, err := d.Stream(dctx, musicVideoID, QualityBest)
	dcancel()
	if err != nil {
		t.Fatalf("checked on the next server: %v", err)
	}
	if mu, _ := url.Parse(moved.URL); mu.Host == own {
		t.Log("YouTube gave a stream on another server this time")
	} else {
		t.Logf("checked on the next server in %s", time.Since(start).Round(time.Millisecond))
	}

	r := NewResolver(nil, "")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		return fmt.Appendf(nil, `{"url": %q, "ext": "webm", "acodec": "opus", "filesize": %d}`, s.URL, s.Size), nil
	}
	p := NewProxy(r)
	p.HTTP = client
	start = time.Now()
	rec := get(t, p, "")
	if rec.Code != http.StatusOK || int64(rec.Body.Len()) != s.Size {
		t.Fatalf("%d, %d bytes of %d: %s", rec.Code, rec.Body.Len(), s.Size, p.Failure(testID))
	}
	t.Logf("read %d bytes from the next server in %s, after %d connections to the server that doesn't answer",
		s.Size, time.Since(start).Round(time.Millisecond), conns())
	if l := logged.String(); strings.Contains(l, own) || strings.Contains(l, "googlevideo") {
		t.Errorf("the log names a server:\n%s", l)
	}
}
