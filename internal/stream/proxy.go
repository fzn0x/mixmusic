package stream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
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

// Proxy serves the audio of YouTube videos at /yt/<video id>, fetching it
// from YouTube in chunks. It answers range requests, so the audio element
// that plays it can seek.
type Proxy struct {
	Resolver *Resolver
	HTTP     *http.Client
	// Cache keeps the audio of tracks played to their end, if set.
	Cache *Cache

	mu       sync.Mutex
	failures map[string]string // why the last request of a video failed
}

// NewProxy returns a proxy of the streams r resolves.
func NewProxy(r *Resolver) *Proxy {
	return &Proxy{Resolver: r, HTTP: &http.Client{Timeout: 2 * time.Minute}}
}

// Path returns the path the proxy serves a video's audio at.
func Path(videoID string) string { return "/yt/" + videoID }

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
		if off > start {
			if body, err = p.fetch(r.Context(), s, off, min(off+chunkSize-1, end)); err != nil {
				p.fail(nil, videoID, err) // the response has started: all we can do is stop
				return
			}
		}
		_, err := io.Copy(out, body)
		body.Close()
		if err != nil {
			return // the player went away, or YouTube did
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
	if capture != nil {
		capture.Commit()
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
	// A request's error quotes the stream URL, which carries the user's IP
	// address: the log keeps what went wrong, without it.
	logged := err
	var ue *url.Error
	if errors.As(err, &ue) {
		logged = ue.Err
	}
	log.Printf("mixmusic: the audio of %s: %v", videoID, logged)
	p.setFailure(videoID, plainFailure(err))
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

// fetch requests bytes start to end of a stream.
func (p *Proxy) fetch(ctx context.Context, s Stream, start, end int64) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range s.Header {
		req.Header[k] = v
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	res, err := p.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	switch res.StatusCode {
	case http.StatusPartialContent, http.StatusOK:
		if res.StatusCode == http.StatusOK && start > 0 {
			res.Body.Close()
			return nil, errors.New("stream: YouTube ignored the range")
		}
		return res.Body, nil
	case http.StatusForbidden, http.StatusGone:
		res.Body.Close()
		return nil, errExpired
	}
	res.Body.Close()
	return nil, statusError(res.StatusCode)
}

// probeSize asks YouTube for the length of a stream yt-dlp did not know.
func (p *Proxy) probeSize(ctx context.Context, s Stream) (int64, error) {
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
		return 0, err
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
