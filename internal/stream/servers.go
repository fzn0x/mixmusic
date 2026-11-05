package stream

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// connectTimeout is how long a request of a stream waits for its
	// connection to one of YouTube's servers, the TLS handshake included,
	// while the stream has another server to ask: YouTube's connect in a
	// fraction of a second, and one the network doesn't reach would hold
	// the request for the transport's 10 s. The last server asked has the
	// transport's own limits, which a slow network may need.
	connectTimeout = 3 * time.Second
	// downFor is how long a server that a request couldn't reach is asked
	// after the other servers of a stream.
	downFor = 10 * time.Minute
	// maxDown is how many such servers mediaServers remembers.
	maxDown = 64
)

// errNoConnection is why a request gave up on a server that didn't connect
// in its time.
var errNoConnection = errors.New("stream: no connection to YouTube's server")

// serverHost matches the host of one of YouTube's media servers, as
// servers names them.
var serverHost = regexp.MustCompile(`^rr[0-9]+---[a-z0-9-]+\.googlevideo\.com$`)

// servers returns the URL of a stream on each of YouTube's servers that
// hold it, u first. A googlevideo URL names them in its mn parameter: the
// first is at rr<mvi>---<name>.googlevideo.com, and the others, which
// YouTube's player asks when one doesn't answer, at
// rr<fvip>---<name>.googlevideo.com, with fallback_count telling which.
// Any other URL is the stream's only one.
func servers(u *url.URL) []*url.URL {
	urls := []*url.URL{u}
	if !strings.HasSuffix(u.Hostname(), ".googlevideo.com") {
		return urls
	}
	q := u.Query()
	seen := map[string]bool{u.Host: true}
	for i, name := range strings.Split(q.Get("mn"), ",") {
		n := q.Get("fvip")
		if i == 0 {
			n = q.Get("mvi")
		}
		host := "rr" + n + "---" + name + ".googlevideo.com"
		if !serverHost.MatchString(host) || seen[host] {
			continue
		}
		seen[host] = true
		alt, aq := *u, u.Query()
		alt.Host = host
		if i == 0 {
			aq.Del("fallback_count")
		} else {
			aq.Set("fallback_count", strconv.Itoa(i))
		}
		alt.RawQuery = aq.Encode()
		urls = append(urls, &alt)
	}
	return urls
}

// mediaServers sends the requests of streams to YouTube's media servers:
// when one can't be reached, to the stream's next server, as YouTube's
// player does, and for downFor after, to the others first. Its zero value
// is ready to use, and safe for concurrent use.
type mediaServers struct {
	// connect is connectTimeout, unless a test shortens it.
	connect time.Duration

	mu   sync.Mutex
	down map[string]time.Time // the hosts a request couldn't reach, and when
}

// do sends req, a GET of a stream, to the server of its URL, and, when the
// connection to that server fails, the TLS handshake included, or doesn't
// come within connectTimeout, to the stream's other servers in turn. Any
// answer, a refusal too, is the stream's: no other server is asked then.
// Under a deadline of req's context, each server has its share of the time
// left. It returns the answer, and the URL of the server that gave it when
// it is not req's own. The log tells which server didn't answer, never its
// name or the URL.
func (m *mediaServers) do(client *http.Client, req *http.Request) (*http.Response, string, error) {
	urls := servers(req.URL)
	if len(urls) == 1 {
		res, err := client.Do(req)
		return res, "", err
	}
	urls = m.order(urls)
	ctx := req.Context()
	for i, u := range urls {
		limit := cmp.Or(m.connect, connectTimeout)
		if deadline, ok := ctx.Deadline(); ok {
			limit = min(limit, time.Until(deadline)/time.Duration(len(urls)-i))
		}
		last := i == len(urls)-1
		if last {
			limit = 0
		}
		res, connected, err := send(client, req, u, limit)
		if connected || ctx.Err() == nil {
			m.mark(u.Host, connected)
		}
		switch {
		case err == nil && u == req.URL:
			return res, "", nil
		case err == nil:
			return res, u.String(), nil
		case connected, ctx.Err() != nil, last:
			return nil, "", err
		}
		if errors.Is(err, errNoConnection) {
			err = fmt.Errorf("no connection in %s", limit.Round(time.Millisecond))
		}
		log.Printf("mixmusic: YouTube's server %d of %d of a stream didn't answer (%v); asking the next", i+1, len(urls), loggable(err))
	}
	return nil, "", errNoConnection // not reached: the last server returns
}

// send sends req to the server at u, and gives it up when no connection to
// it came within limit, unless limit is 0. It tells whether a connection
// came, before which nothing went out.
func send(client *http.Client, req *http.Request, u *url.URL, limit time.Duration) (*http.Response, bool, error) {
	ctx, cancel := context.WithCancelCause(req.Context())
	var mu sync.Mutex
	connected := false
	if limit > 0 {
		timer := time.AfterFunc(limit, func() {
			mu.Lock()
			defer mu.Unlock()
			if !connected {
				cancel(errNoConnection)
			}
		})
		defer timer.Stop()
	}
	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		GotConn: func(httptrace.GotConnInfo) {
			mu.Lock()
			defer mu.Unlock()
			connected = true
		},
	})
	r := req.Clone(ctx)
	r.URL, r.Host = u, u.Host
	res, err := client.Do(r)
	mu.Lock()
	got := connected
	mu.Unlock()
	if err != nil {
		// The transport tells only that the request was canceled.
		if errors.Is(context.Cause(ctx), errNoConnection) {
			err = errNoConnection
		}
		cancel(nil)
		return nil, got, err
	}
	res.Body = cancelingBody{res.Body, cancel}
	return res, true, nil
}

// cancelingBody is the body of an answer, which ends the context of its
// request once closed.
type cancelingBody struct {
	io.ReadCloser
	cancel context.CancelCauseFunc
}

func (b cancelingBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel(nil)
	return err
}

// order returns urls with those of the servers a request couldn't reach
// lately last, each in its order.
func (m *mediaServers) order(urls []*url.URL) []*url.URL {
	m.mu.Lock()
	defer m.mu.Unlock()
	var up, down []*url.URL
	for _, u := range urls {
		if at, ok := m.down[u.Host]; ok && time.Since(at) < downFor {
			down = append(down, u)
		} else {
			up = append(up, u)
		}
	}
	return append(up, down...)
}

// mark takes in whether a request reached a server.
func (m *mediaServers) mark(host string, reached bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if reached {
		delete(m.down, host)
		return
	}
	if m.down == nil || len(m.down) >= maxDown {
		m.down = map[string]time.Time{}
	}
	m.down[host] = time.Now()
}
