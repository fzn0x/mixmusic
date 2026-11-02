package stream

import (
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// jumpAhead is how far beyond the download a read waits for it: farther
// on, the download starts again there with a range request, as an audio
// element's does when it seeks.
const jumpAhead = 512 << 10

var (
	// ErrInterrupted is returned by a Read that stopped waiting for bytes.
	ErrInterrupted = errors.New("stream: read interrupted")
	errClosed      = errors.New("stream: source closed")
	// errDropped stops a download the source no longer needs.
	errDropped = errors.New("stream: download dropped")
	// errLoad is why a download failed, when the proxy didn't say.
	errLoad = errors.New("the network failed while loading the audio")
)

// Source is the audio of a video as MixMusic's own player reads it: what
// the proxy serves at the video's path, from the cache or YouTube, which it
// downloads from the start in the background and keeps in memory, so that
// reading again and seeking back cost nothing. A read far beyond the
// download has it start again there. A track it downloads whole from its
// start goes to the cache, as one the page played to its end did.
type Source struct {
	p       *Proxy
	videoID string
	// path is what the source asks the proxy for: the video's Path, or its
	// SurroundPath.
	path        string
	ctx         context.Context
	cancel      context.CancelFunc
	interrupted func() bool

	mu   sync.Mutex
	cond *sync.Cond
	// spans are the bytes that came, in order, apart from one another.
	spans  []span
	size   int64     // from the first answer, or -1
	dl     *download // the download under way, or nil
	err    error     // why a download failed
	closed bool
	pos    int64
}

// span is bytes of the audio, from off.
type span struct {
	off  int64
	data []byte
}

func (s span) end() int64 { return s.off + int64(len(s.data)) }

// download is a request of the audio from start; at is where its next
// bytes go.
type download struct {
	start, at int64
	cancel    context.CancelFunc
}

// Open starts downloading the audio of a video, and returns once the proxy
// answered: with why it couldn't, in plain words, as Failure tells. A Read
// waiting for bytes returns ErrInterrupted when interrupted, if not nil,
// returns true; Wake makes such a Read check it again.
func (p *Proxy) Open(ctx context.Context, videoID string, interrupted func() bool) (*Source, error) {
	return p.open(ctx, videoID, Path(videoID), interrupted)
}

// OpenSurround is Open for a video's 5.1 surround audio, which it fails
// with why when YouTube has none of the video.
func (p *Proxy) OpenSurround(ctx context.Context, videoID string, interrupted func() bool) (*Source, error) {
	return p.open(ctx, videoID, SurroundPath(videoID), interrupted)
}

func (p *Proxy) open(ctx context.Context, videoID, path string, interrupted func() bool) (*Source, error) {
	ctx, cancel := context.WithCancel(ctx)
	s := &Source{p: p, videoID: videoID, path: path, ctx: ctx, cancel: cancel, interrupted: interrupted, size: -1}
	s.cond = sync.NewCond(&s.mu)
	s.mu.Lock()
	w, served := s.fetch(0)
	s.mu.Unlock()
	<-w.ready
	if w.status >= 400 {
		// The proxy says why once it returned.
		<-served
		s.Close()
		if why := p.Failure(videoID); why != "" {
			return nil, errors.New(why)
		}
		return nil, errLoad
	}
	return s, nil
}

// fetch starts downloading from off, in place of the download under way.
// Its caller holds mu.
func (s *Source) fetch(off int64) (*sourceWriter, chan struct{}) {
	if s.dl != nil {
		s.dl.cancel()
	}
	ctx, cancel := context.WithCancel(s.ctx)
	d := &download{start: off, at: off, cancel: cancel}
	s.dl = d
	w := &sourceWriter{s: s, d: d, header: http.Header{}, ready: make(chan struct{})}
	served := make(chan struct{})
	go func() {
		defer close(served)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.path, nil)
		if err == nil {
			if off > 0 {
				req.Header.Set("Range", "bytes="+strconv.FormatInt(off, 10)+"-")
			}
			s.p.ServeHTTP(w, req)
		}
		w.answer(http.StatusBadGateway) // when nothing was written
		s.finish(d)
	}()
	return w, served
}

// finish takes in the end of a download: one that stopped before the end
// of the audio, unless the rest came already, failed.
func (s *Source) finish(d *download) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dl != d {
		return // closed, or another took over
	}
	s.dl = nil
	if s.size < 0 || s.reach(d.at) != s.size {
		if why := s.p.Failure(s.videoID); why != "" {
			s.err = errors.New(why)
		} else {
			s.err = errLoad
		}
	}
	s.cond.Broadcast()
}

// put keeps b, the bytes from off, with those that came: what it holds
// already is dropped.
func (s *Source) put(off int64, b []byte) {
	if len(b) == 0 {
		return
	}
	// The first span that ends at off or after.
	i := sort.Search(len(s.spans), func(i int) bool { return s.spans[i].end() >= off })
	if i == len(s.spans) || s.spans[i].off > off {
		s.spans = slices.Insert(s.spans, i, span{off: off})
	}
	sp := &s.spans[i]
	if end := off + int64(len(b)); end > sp.end() {
		sp.data = append(sp.data, b[sp.end()-off:]...)
	}
	// It may reach the spans after it now.
	for i+1 < len(s.spans) && s.spans[i+1].off <= sp.end() {
		if next := s.spans[i+1]; next.end() > sp.end() {
			sp.data = append(sp.data, next.data[sp.end()-next.off:]...)
		}
		s.spans = slices.Delete(s.spans, i+1, i+2)
	}
}

// reach returns where the bytes that came from off end: off itself when
// the byte at off hasn't come.
func (s *Source) reach(off int64) int64 {
	i := sort.Search(len(s.spans), func(i int) bool { return s.spans[i].end() > off })
	if i < len(s.spans) && s.spans[i].off <= off {
		return s.spans[i].end()
	}
	return off
}

// Read reads the audio, waiting for bytes that haven't come yet.
func (s *Source) Read(b []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for {
		if s.closed {
			return 0, errClosed
		}
		if s.size >= 0 && s.pos >= s.size {
			return 0, io.EOF
		}
		i := sort.Search(len(s.spans), func(i int) bool { return s.spans[i].end() > s.pos })
		if i < len(s.spans) && s.spans[i].off <= s.pos {
			sp := s.spans[i]
			n := copy(b, sp.data[s.pos-sp.off:])
			s.pos += int64(n)
			return n, nil
		}
		if s.err != nil {
			return 0, s.err
		}
		if s.interrupted != nil && s.interrupted() {
			return 0, ErrInterrupted
		}
		// The download goes on to pos unless it ended, or is past it or far
		// from it; one asked for pos, which may send the audio from its
		// start, goes on.
		if d := s.dl; d == nil || (d.start != s.pos && (d.at > s.pos || s.pos-d.at > jumpAhead)) {
			s.fetch(s.pos)
		}
		s.cond.Wait()
	}
}

// Seek moves where Read reads.
func (s *Source) Seek(offset int64, whence int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += s.pos
	case io.SeekEnd:
		if s.size < 0 {
			return 0, errors.New("stream: the size is unknown")
		}
		offset += s.size
	default:
		return 0, errors.New("stream: invalid whence")
	}
	if offset < 0 {
		return 0, errors.New("stream: negative position")
	}
	s.pos = offset
	return offset, nil
}

// Wake makes a Read that waits check its interrupted function again.
func (s *Source) Wake() {
	s.mu.Lock()
	s.cond.Broadcast()
	s.mu.Unlock()
}

// Close stops the download.
func (s *Source) Close() error {
	s.cancel()
	s.mu.Lock()
	s.closed = true
	s.dl = nil
	s.cond.Broadcast()
	s.mu.Unlock()
	return nil
}

// sourceWriter is the response the proxy writes a download to.
type sourceWriter struct {
	s      *Source
	d      *download
	header http.Header
	status int
	once   sync.Once
	ready  chan struct{}
}

func (w *sourceWriter) Header() http.Header { return w.header }

func (w *sourceWriter) WriteHeader(status int) { w.answer(status) }

// answer takes the proxy's status, once, and where its bytes start: the
// whole audio may come for a range.
func (w *sourceWriter) answer(status int) {
	w.once.Do(func() {
		w.status = status
		s := w.s
		s.mu.Lock()
		switch status {
		case http.StatusOK:
			w.d.at = 0
			if n, err := strconv.ParseInt(w.header.Get("Content-Length"), 10, 64); err == nil && s.size < 0 {
				s.size = n
			}
		case http.StatusPartialContent:
			if from, size, ok := contentRange(w.header.Get("Content-Range")); ok {
				w.d.at = from
				if s.size < 0 {
					s.size = size
				}
			}
		}
		s.mu.Unlock()
		close(w.ready)
	})
}

func (w *sourceWriter) Write(b []byte) (int, error) {
	w.answer(http.StatusOK)
	s, d := w.s, w.d
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dl != d {
		return 0, errDropped // closed, or another took over: the proxy stops
	}
	if w.status != http.StatusOK && w.status != http.StatusPartialContent {
		return len(b), nil // the text of an error
	}
	s.put(d.at, b)
	d.at += int64(len(b))
	s.cond.Broadcast()
	if d.at < s.size && s.reach(d.at) == s.size {
		return len(b), errDropped // the rest came already
	}
	return len(b), nil
}

func (w *sourceWriter) Flush() {}

// contentRange reads the first byte and the size of a Content-Range, as
// "bytes 100-199/1000".
func contentRange(h string) (from, size int64, ok bool) {
	spec, found := strings.CutPrefix(h, "bytes ")
	first, rest, found2 := strings.Cut(spec, "-")
	_, total, found3 := strings.Cut(rest, "/")
	from, err1 := strconv.ParseInt(first, 10, 64)
	size, err2 := strconv.ParseInt(total, 10, 64)
	return from, size, found && found2 && found3 && err1 == nil && err2 == nil
}
