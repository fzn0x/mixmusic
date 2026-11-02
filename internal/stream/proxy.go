package stream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// chunkSize is how much of a stream one upstream request fetches. YouTube
// serves ranges of a few megabytes at full speed, and throttles requests
// for whole files.
const chunkSize = 4 << 20

// maxFailures is how many videos the proxy remembers the failure of.
const maxFailures = 64

// maxAvailableWait bounds how long the proxy waits for YouTube to serve a
// stream yt-dlp found, which takes a few seconds.
const maxAvailableWait = 30 * time.Second

const (
	// stallTimeout is how long a request of a stream waits for YouTube
	// without a byte once it went out: YouTube answers in a fraction of a
	// second, and a connection the network dropped sends nothing until the
	// system gives up on it, minutes later.
	stallTimeout = 15 * time.Second
	// reconnectTimeout is how long a download that stalled, or whose
	// connection dropped, keeps connecting again before the proxy gives it
	// up, the first wait being reconnectWait, doubled each time up to
	// stallTimeout.
	reconnectTimeout = 2 * time.Minute
	reconnectWait    = time.Second
)

// Proxy serves the audio of YouTube videos at /yt/<video id>, fetching it
// from YouTube in chunks. It answers range requests, so the audio element
// that plays it can seek.
type Proxy struct {
	Resolver *Resolver
	HTTP     *http.Client
	// Cache keeps the audio of tracks played to their end, if set.
	Cache *Cache
	// Seen, if set, is called when YouTube answers with audio, which tells
	// that the network answers.
	Seen func()

	mu       sync.Mutex
	failures map[string]string // why the last request of a video failed
	// stall, reconnect and backoff are stallTimeout, reconnectTimeout and
	// reconnectWait, unless a test shortens them.
	stall, reconnect, backoff time.Duration
	// liveReload, when a test sets it, is what a live stream's playlist
	// reloads after, in place of its target duration.
	liveReload time.Duration
}

// NewProxy returns a proxy of the streams r resolves. Its requests have no
// overall deadline, which would cut off a slow download that still comes:
// one that gets no bytes for stallTimeout connects again instead. Its
// connections are its own, so that it can drop them after a stall.
func NewProxy(r *Resolver) *Proxy {
	client := &http.Client{}
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		client.Transport = t.Clone()
	}
	return &Proxy{Resolver: r, HTTP: client}
}

// timing returns how long a request waits without a byte, how long a
// download that broke keeps connecting again, and the first wait before it
// does.
func (p *Proxy) timing() (stall, reconnect, backoff time.Duration) {
	stall, reconnect, backoff = stallTimeout, reconnectTimeout, reconnectWait
	if p.stall > 0 {
		stall = p.stall
	}
	if p.reconnect > 0 {
		reconnect = p.reconnect
	}
	if p.backoff > 0 {
		backoff = p.backoff
	}
	return stall, reconnect, backoff
}

// Path returns the path the proxy serves a video's audio at.
func Path(videoID string) string { return "/yt/" + videoID }

// LivePath returns the path of the live stream of a video, which OpenLive
// opens, as the engine's opener reads it.
func LivePath(videoID string) string { return "/live/" + videoID }

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	videoID, ok := strings.CutPrefix(r.URL.Path, "/yt/")
	if !ok || !ValidVideoID(videoID) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	p.setFailure(videoID, "")
	if p.Cache != nil && p.Cache.Serve(w, r, videoID) {
		return
	}
	s, err := p.resolve(r.Context(), videoID)
	if err != nil {
		p.fail(w, videoID, err)
		return
	}
	if s.Size == 0 {
		if s.Size, err = p.probeSize(r.Context(), s); err != nil {
			p.fail(w, videoID, err)
			return
		}
	}
	start, end, partial, ok := parseRange(r.Header.Get("Range"), s.Size)
	if !ok {
		w.Header().Set("Content-Range", "bytes */"+strconv.FormatInt(s.Size, 10))
		http.Error(w, "range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	// Fetch the first chunk before answering, so that an expired URL can
	// still be resolved again and a failure reported as an error.
	first, err := p.fetch(r.Context(), s, start, min(start+chunkSize-1, end))
	if errors.Is(err, errExpired) {
		p.Resolver.Forget(videoID)
		if s, err = p.resolve(r.Context(), videoID); err == nil {
			first, err = p.fetch(r.Context(), s, start, min(start+chunkSize-1, end))
		}
	}
	if err != nil {
		p.fail(w, videoID, err)
		return
	}
	defer first.Close()

	h := w.Header()
	h.Set("Content-Type", s.MimeType)
	h.Set("Accept-Ranges", "bytes")
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	if partial {
		h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, s.Size))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	if r.Method == http.MethodHead {
		return
	}

	var out io.Writer = w
	var capture *capture
	if p.Cache != nil && start == 0 && end == s.Size-1 {
		if capture = p.Cache.Begin(videoID, s.MimeType, s.Size); capture != nil {
			out = io.MultiWriter(w, capture)
			defer capture.Abort() // a no-op once committed
		}
	}
	flusher, _ := w.(http.Flusher)
	body := first
	for off := start; off <= end; off += chunkSize {
		// Once the response started, all a failure can do is stop it.
		if !p.relay(r.Context(), out, videoID, s, off, min(off+chunkSize-1, end), body) {
			return
		}
		body = nil
		if flusher != nil {
			flusher.Flush()
		}
	}
	if capture != nil {
		capture.Commit()
	}
}

// relay copies bytes from to to of a stream to out, reading body first when
// it is not nil. A download that stalls, or whose connection drops,
// connects again from the byte it reached, after a wait that doubles each
// time: the track waits meanwhile, and plays on as soon as the network
// answers. It gives up once the download stayed broken for reconnect. It
// returns whether all came, and tells why not, unless the player went
// away.
func (p *Proxy) relay(ctx context.Context, out io.Writer, videoID string, s Stream, from, to int64, body *stallingBody) bool {
	stall, reconnect, backoff := p.timing()
	wait := backoff
	var broke time.Time // when the download broke, while it stays broken
	for {
		var err error
		if body == nil {
			body, err = p.fetch(ctx, s, from, to)
		}
		// lost is set when the network lost the download, rather than YouTube
		// turning it down: the proxy connects again then.
		lost := false
		if err == nil {
			n, cerr := io.Copy(out, body)
			body.Close()
			rerr := body.err
			body = nil
			from += n
			if n > 0 && !broke.IsZero() {
				log.Printf("mixmusic: the audio of %s comes again", videoID)
				broke, wait = time.Time{}, backoff
			}
			switch {
			case from > to:
				return true
			case rerr != nil:
				err, lost = rerr, true
			case cerr != nil:
				return false // the player went away
			default:
				err, lost = io.ErrUnexpectedEOF, true // the answer ended early
			}
		} else {
			var ue *url.Error
			lost = errors.Is(err, errStalled) || errors.As(err, &ue)
		}
		switch {
		case ctx.Err() != nil:
			return false
		case !lost:
			p.fail(nil, videoID, err)
			return false
		case broke.IsZero():
			broke = time.Now()
			log.Printf("mixmusic: the audio of %s broke off at byte %d of %d: %v; connecting again", videoID, from, s.Size, loggable(err))
		case time.Since(broke)+wait > reconnect:
			p.fail(nil, videoID, errStalled)
			return false
		}
		// The connections kept for reuse may have died with this one: the next
		// request dials anew.
		p.HTTP.CloseIdleConnections()
		t := time.NewTimer(wait)
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			return false
		}
		wait = min(2*wait, stall)
	}
}

// resolve returns the stream of a video once YouTube serves it: it refuses
// a URL yt-dlp found for a few seconds, while it would show an ad. Streams
// of the direct route play at once.
func (p *Proxy) resolve(ctx context.Context, videoID string) (Stream, error) {
	s, err := p.Resolver.Resolve(ctx, videoID)
	if err != nil {
		return s, err
	}
	if wait := time.Until(s.Available); wait > 0 {
		wait = min(wait, maxAvailableWait)
		log.Printf("mixmusic: waiting %.1fs for YouTube to serve the stream of %s", wait.Seconds(), videoID)
		t := time.NewTimer(wait)
		defer t.Stop()
		select {
		case <-t.C:
		case <-ctx.Done():
			return Stream{}, ctx.Err()
		}
	}
	return s, nil
}

// Failure returns why the last request of a video's audio failed, in plain
// words, or "" when it did not fail. The audio element that made it only
// tells that its source cannot play.
func (p *Proxy) Failure(videoID string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.failures[videoID]
}

// setFailure remembers why the last request of a video failed, or that it
// did not when why is "".
func (p *Proxy) setFailure(videoID, why string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if why == "" {
		delete(p.failures, videoID)
		return
	}
	if p.failures == nil || len(p.failures) >= maxFailures {
		p.failures = map[string]string{}
	}
	p.failures[videoID] = why
}

// fail answers a request whose audio could not be served with err, when w
// is not nil, remembers why and logs it. A request the player gave up on,
// as when it skipped the track, is no failure.
func (p *Proxy) fail(w http.ResponseWriter, videoID string, err error) {
	if w != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
	if errors.Is(err, context.Canceled) {
		return
	}
	log.Printf("mixmusic: the audio of %s: %v", videoID, loggable(err))
	p.setFailure(videoID, plainFailure(err))
}

// loggable returns err without the stream URL a request's error quotes,
// which carries the user's IP address, and without the addresses a
// connection's error names, the computer's among them, which over IPv6 is
// its address on the internet: what went wrong, for the log.
func loggable(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	var oe *net.OpError
	if errors.As(err, &oe) {
		return fmt.Errorf("%s: %w", oe.Op, oe.Err)
	}
	return err
}

// plainFailure says in plain words why the audio of a video could not be
// served.
func plainFailure(err error) string {
	var status statusError
	var ue *url.Error
	switch {
	case errors.Is(err, errExpired):
		return "YouTube refused to send its audio; updating yt-dlp in Settings › Playback often fixes it"
	case errors.As(err, &status) && status == http.StatusTooManyRequests:
		return "YouTube is turning down requests from this network for now; try again in a while"
	case errors.As(err, &status):
		return fmt.Sprintf("YouTube answered HTTP %d for its audio", int(status))
	case errors.Is(err, errStalled):
		return "YouTube stopped sending the audio, as happens when the connection drops; check the connection and play the track again"
	case errors.As(err, &ue):
		return "MixMusic could not reach YouTube's audio servers, which a firewall, a VPN or a DNS filter may block"
	}
	return strings.TrimPrefix(err.Error(), "stream: ")
}

// errExpired is returned when YouTube refuses a stream URL, which happens
// once it expired.
var errExpired = errors.New("stream: URL refused")

// statusError is returned when YouTube answers a stream request with an
// unexpected HTTP status.
type statusError int

func (e statusError) Error() string { return fmt.Sprintf("stream: YouTube answered HTTP %d", int(e)) }

// errStalled is returned when a request of a stream waited stallTimeout
// for YouTube without a byte.
var errStalled = errors.New("stream: YouTube stopped sending the audio")

// stalled returns errStalled in place of err when ctx, a request's, was
// canceled for a stall. Go's HTTP/1 client tells the cause itself; its
// HTTP/2 client only that the request was canceled.
func stalled(ctx context.Context, err error) error {
	if errors.Is(context.Cause(ctx), errStalled) {
		return errStalled
	}
	return err
}

// fetch requests bytes start to end of a stream. The request is given up
// for a stall when an answer, a redirect's too, doesn't come within
// stallTimeout of its request going out, and its body gives it up when a
// read waits as long for bytes; connecting has the transport's own limits.
func (p *Proxy) fetch(ctx context.Context, s Stream, start, end int64) (*stallingBody, error) {
	return p.request(ctx, s.URL, s.Header, start, end)
}

// request requests bytes start to end of what is at rawURL, with header,
// as fetch does, or all of it when end is below 0: the playlists and
// segments of a live stream, which the transport may then have come
// compressed.
func (p *Proxy) request(ctx context.Context, rawURL string, header http.Header, start, end int64) (*stallingBody, error) {
	stall, _, _ := p.timing()
	ctx, cancel := context.WithCancelCause(ctx)
	timer := time.AfterFunc(stall, func() { cancel(errStalled) })
	timer.Stop()
	// The transport tells when each request went out, and when its answer
	// came, until Do returns. It may tell that the last request went out
	// after Do returned its answer, as it writes requests on a goroutine of
	// its own: the timer times the body's reads by then.
	var mu sync.Mutex
	answered := false
	waiting := func(on bool) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case answered:
		case on:
			timer.Reset(stall)
		default:
			timer.Stop()
		}
	}
	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		WroteRequest:         func(httptrace.WroteRequestInfo) { waiting(true) },
		GotFirstResponseByte: func() { waiting(false) },
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		cancel(nil)
		return nil, err
	}
	for k, v := range header {
		req.Header[k] = v
	}
	if end >= 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	}
	res, err := p.HTTP.Do(req)
	mu.Lock()
	answered = true
	timer.Stop()
	mu.Unlock()
	if err != nil {
		err = stalled(ctx, err)
		cancel(nil)
		return nil, err
	}
	switch res.StatusCode {
	case http.StatusPartialContent, http.StatusOK:
		if res.StatusCode == http.StatusOK && start > 0 {
			res.Body.Close()
			cancel(nil)
			return nil, errors.New("stream: YouTube ignored the range")
		}
		if p.Seen != nil {
			p.Seen()
		}
		return &stallingBody{body: res.Body, ctx: ctx, cancel: cancel, timer: timer, stall: stall}, nil
	case http.StatusForbidden, http.StatusGone:
		res.Body.Close()
		cancel(nil)
		return nil, errExpired
	}
	res.Body.Close()
	cancel(nil)
	return nil, statusError(res.StatusCode)
}

// stallingBody is the body of a stream request, which gives the request up
// when a read waits stall for bytes. Only the wait for YouTube counts, not
// the time its reader takes with what came.
type stallingBody struct {
	body   io.ReadCloser
	ctx    context.Context
	cancel context.CancelCauseFunc
	timer  *time.Timer
	stall  time.Duration
	// err is why a read failed, other than at the end: a stall, or a
	// connection that dropped.
	err error
}

func (b *stallingBody) Read(p []byte) (int, error) {
	b.timer.Reset(b.stall)
	n, err := b.body.Read(p)
	b.timer.Stop()
	if err != nil && err != io.EOF {
		err = stalled(b.ctx, err)
		if b.err == nil {
			b.err = err
		}
	}
	return n, err
}

func (b *stallingBody) Close() error {
	b.timer.Stop()
	err := b.body.Close()
	b.cancel(nil)
	return err
}

// probeSize asks YouTube for the length of a stream yt-dlp did not know.
func (p *Proxy) probeSize(ctx context.Context, s Stream) (int64, error) {
	stall, _, _ := p.timing()
	ctx, cancel := context.WithTimeoutCause(ctx, stall, errStalled)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return 0, err
	}
	for k, v := range s.Header {
		req.Header[k] = v
	}
	req.Header.Set("Range", "bytes=0-0")
	res, err := p.HTTP.Do(req)
	if err != nil {
		return 0, stalled(ctx, err)
	}
	res.Body.Close()
	_, total, ok := strings.Cut(res.Header.Get("Content-Range"), "/")
	if size, err := strconv.ParseInt(total, 10, 64); ok && err == nil && size > 0 {
		return size, nil
	}
	return 0, errors.New("stream: YouTube did not tell the stream's length")
}

// parseRange reads a Range header of one range against a resource of size
// bytes. It returns the first and last byte, whether the request asked for
// a range at all, and false when the range cannot be satisfied.
func parseRange(header string, size int64) (start, end int64, partial, ok bool) {
	if header == "" {
		return 0, size - 1, false, size > 0
	}
	spec, found := strings.CutPrefix(header, "bytes=")
	if !found || strings.Contains(spec, ",") {
		return 0, size - 1, false, size > 0 // unsupported: send it all
	}
	from, to, _ := strings.Cut(strings.TrimSpace(spec), "-")
	switch {
	case from == "": // the last n bytes
		n, err := strconv.ParseInt(to, 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, true, false
		}
		start, end = max(size-n, 0), size-1
	default:
		var err error
		if start, err = strconv.ParseInt(from, 10, 64); err != nil {
			return 0, 0, true, false
		}
		end = size - 1
		if to != "" {
			if end, err = strconv.ParseInt(to, 10, 64); err != nil {
				return 0, 0, true, false
			}
			end = min(end, size-1)
		}
	}
	if start < 0 || start > end || start >= size {
		return 0, 0, true, false
	}
	return start, end, true, true
}
