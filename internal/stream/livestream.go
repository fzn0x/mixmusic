package stream

// Live streams: the audio of a live video as it goes on, from the HLS
// media playlist the resolver found of it (RFC 8216). The live route loads
// the playlist about once per target duration, fetches each new segment
// once, in order, and hands their packed audio to the engine, which plays
// it as it comes.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	// liveEdge is how far from the end of its playlist a live stream
	// starts, in segments: RFC 8216, section 6.3.3, has a client start no
	// later than three target durations from the end, which leaves the
	// stream as much to ride out a slow network.
	liveEdge = 3
	// liveAhead is how many segments the live route holds that the engine
	// hasn't read, at most: with them, as while the track is paused, it
	// waits, and goes on from there once the engine reads on, while the
	// playlist still has the next segment, else from its live edge.
	liveAhead = 4
	// liveLag is how far behind the segments that came a playlist may be,
	// as one a cache kept a moment, before the live route takes it that
	// the stream's numbers started over.
	liveLag = liveEdge + liveAhead
	// maxSegment bounds a segment: YouTube's are of some 80 kB.
	maxSegment = 16 << 20
	// maxMisses is how many times in a row a segment the playlist lists
	// may be gone before the stream goes on without it.
	maxMisses = 3
	// renewWait is how long after it failed to find a stream about to
	// expire again the live route tries again.
	renewWait = time.Minute
)

var (
	// errLiveEnded is returned when the stream of a video that was live is
	// no longer: its broadcast ended.
	errLiveEnded = errors.New("stream: the live stream ended")
	// errNotLive is returned by OpenLive for a video that isn't live.
	errNotLive = errors.New("this track is no longer live")
)

// Live is the audio of a live video as MixMusic's own player reads it: the
// packed audio of the segments of its HLS media playlist, one after the
// other, from near the live edge on, as they come. Reads wait for the
// next segment; they return io.EOF once the broadcast ended, as the
// playlist tells. It keeps a few segments ahead of the reader, and none it
// read, so it can't seek but within the segment it reads. Its methods are
// safe for concurrent use.
type Live struct {
	p           *Proxy
	videoID     string
	ctx         context.Context
	cancel      context.CancelFunc
	interrupted func() bool

	mu   sync.Mutex
	cond *sync.Cond
	// segs are the segments that came that the reader hasn't read past, in
	// order, one after the other from the stream's first byte on.
	segs   []span
	end    int64 // where the next segment's bytes go
	pos    int64
	err    error // why the stream failed, in plain words
	ended  bool  // the broadcast ended, and its last segment came
	closed bool
	// started is closed once the first segment came, or the stream ended
	// or failed before.
	started chan struct{}
	once    sync.Once

	// What follows belongs to the goroutine that follows the stream: the
	// stream found again beside it, once it is about to expire.
	renewing  bool
	renewal   chan Stream
	nextRenew time.Time
}

// OpenLive starts loading the live stream of a video, from near its live
// edge, and returns once its first segment came: with why it couldn't, in
// plain words, as Failure tells. A Read waiting for a segment returns
// ErrInterrupted when interrupted, if not nil, returns true; Wake makes
// such a Read check it again.
func (p *Proxy) OpenLive(ctx context.Context, videoID string, interrupted func() bool) (*Live, error) {
	if !ValidVideoID(videoID) {
		return nil, ErrBadVideoID
	}
	p.setFailure(videoID, "")
	ctx, cancel := context.WithCancel(ctx)
	l := &Live{p: p, videoID: videoID, ctx: ctx, cancel: cancel, interrupted: interrupted,
		started: make(chan struct{}), renewal: make(chan Stream, 1)}
	l.cond = sync.NewCond(&l.mu)
	context.AfterFunc(ctx, l.Wake)
	go l.run()
	select {
	case <-l.started:
	case <-ctx.Done():
		l.Close()
		return nil, ctx.Err()
	}
	l.mu.Lock()
	err, empty := l.err, len(l.segs) == 0 && !l.ended
	l.mu.Unlock()
	if err != nil && empty {
		l.Close()
		return nil, err
	}
	return l, nil
}

// run follows the stream until it ends, fails, or is closed.
func (l *Live) run() {
	err := l.follow()
	ended := err == nil || errors.Is(err, errLiveEnded)
	if !ended && l.ctx.Err() == nil {
		err = l.failed(err)
	}
	l.mu.Lock()
	switch {
	case ended:
		l.ended = true
	case l.ctx.Err() == nil:
		l.err = err
	}
	l.cond.Broadcast()
	l.mu.Unlock()
	l.once.Do(func() { close(l.started) })
}

// failed logs why the stream failed, and returns it in plain words, as
// Failure tells it.
func (l *Live) failed(err error) error {
	l.p.fail(nil, l.videoID, err)
	if why := l.p.Failure(l.videoID); why != "" {
		return errors.New(why)
	}
	return errLoad
}

// follow fetches the segments of the stream as they come, and returns once
// the broadcast ended, or why it can't go on.
func (l *Live) follow() error {
	s, err := l.p.resolve(l.ctx, l.videoID)
	if err != nil {
		return err
	}
	if !s.Live {
		return errNotLive
	}
	next := int64(-1) // the media sequence number of the next segment
	misses := 0
	refresh := false
	for {
		if !l.waitRoom() {
			return l.ctx.Err()
		}
		began := time.Now()
		var m mediaPlaylist
		s, m, err = l.load(s, refresh)
		if err != nil {
			return err
		}
		refresh = false
		from := liveFrom(m, next)
		if next >= 0 && from != next {
			log.Printf("mixmusic: the live stream of %s left %d segments out of its playlist before they played; going on from near its live edge",
				l.videoID, max(from-next, 0))
		}
		next = from
		came := false
		for _, seg := range m.segments {
			if seg.seq < next {
				continue
			}
			if !l.hasRoom() {
				break // the playlist is loaded again once the reader made room
			}
			b, err := l.get(s, seg.uri, maxSegment)
			var status statusError
			switch {
			case errors.Is(err, errExpired):
				refresh = true // the stream is found again
			case errors.As(err, &status) && status == http.StatusNotFound:
				// Gone from the media server, as from a playlist loaded
				// late: once it stays gone, the stream goes on without it.
				if misses++; misses >= maxMisses {
					log.Printf("mixmusic: segment %d of the live stream of %s is gone; going on without it", seg.seq, l.videoID)
					next, misses = seg.seq+1, 0
				}
			case err != nil:
				return err
			}
			if err != nil {
				break
			}
			misses = 0
			l.put(b)
			next, came = seg.seq+1, true
		}
		if m.ended && next >= m.end() {
			return nil
		}
		// A client loads a playlist that changed again a target duration
		// after it began loading it, and one that didn't after half of it
		// (RFC 8216, section 6.3.4).
		wait := m.target
		if l.p.liveReload > 0 {
			wait = l.p.liveReload
		}
		if !came {
			wait /= 2
		}
		if refresh {
			wait = 0
		}
		if !l.sleep(time.Until(began.Add(wait))) {
			return l.ctx.Err()
		}
	}
}

// liveFrom returns the media sequence number of the segment a stream goes
// on from in playlist m: next, the segment after those that came, or, as
// it starts, when next is -1, liveEdge segments from the end. A stream
// whose next segment is gone from the playlist, as after a long pause, or
// a reload that came very late, goes on from there too, as does one whose
// numbers started over. A playlist a little behind what came has nothing
// new yet.
func liveFrom(m mediaPlaylist, next int64) int64 {
	if next >= m.sequence && next <= m.end()+liveLag {
		return next
	}
	return max(m.sequence, m.end()-liveEdge)
}

// load loads the playlist of stream s, which it finds again first when
// refresh is set, as after the media server refused a segment, and once
// more when the media server refuses the playlist: YouTube refuses URLs
// once they expired. It returns the stream the playlist is of: one found
// again beside the reloads, once s was about to expire.
func (l *Live) load(s Stream, refresh bool) (Stream, mediaPlaylist, error) {
	for {
		var err error
		if refresh {
			if s, err = l.refind(); err != nil {
				return s, mediaPlaylist{}, err
			}
		} else {
			s = l.renewed(s)
		}
		b, err := l.get(s, s.URL, maxPlaylist)
		if errors.Is(err, errExpired) && !refresh {
			refresh = true
			continue
		}
		if err != nil {
			return s, mediaPlaylist{}, err
		}
		base, err := url.Parse(s.URL)
		if err != nil {
			return s, mediaPlaylist{}, fmt.Errorf("%w: a URL that doesn't parse", errPlaylist)
		}
		m, err := parseMediaPlaylist(string(b), base)
		return s, m, err
	}
}

// refind finds the stream again, as after the media server refused its
// URL: errLiveEnded when the video is no longer live.
func (l *Live) refind() (Stream, error) {
	l.p.Resolver.Forget(l.videoID)
	s, err := l.p.resolve(l.ctx, l.videoID)
	if err == nil && !s.Live {
		return s, errLiveEnded
	}
	return s, err
}

// renewed returns the stream to go on with in place of s: s, or the stream
// found again once s was about to expire. Finding it may take as long as
// yt-dlp does, which the stream doesn't wait for: it goes on with s
// meanwhile, which plays a while yet.
func (l *Live) renewed(s Stream) Stream {
	select {
	case fresh := <-l.renewal:
		l.renewing = false
		if fresh.Live {
			return fresh
		}
		l.nextRenew = time.Now().Add(renewWait)
	default:
	}
	if !l.renewing && time.Until(s.Expires) < expiryMargin && time.Now().After(l.nextRenew) {
		l.renewing = true
		go func() {
			// The resolver finds a stream about to expire again.
			fresh, err := l.p.resolve(l.ctx, l.videoID)
			if err != nil {
				fresh = Stream{}
			}
			l.renewal <- fresh
		}()
	}
	return s
}

// get fetches what is at rawURL whole, a playlist or a segment of stream
// s, of limit bytes at most. A request that stalls, or whose connection
// drops, is made again after a wait that doubles each time, as the proxy
// does for a file, until it stayed broken for reconnect. The log tells
// when it broke and came again, never the URL.
func (l *Live) get(s Stream, rawURL string, limit int64) ([]byte, error) {
	stall, reconnect, backoff := l.p.timing()
	wait := backoff
	var broke time.Time // when it broke, while it stays broken
	for {
		body, err := l.p.request(l.ctx, rawURL, s.Header, 0, -1)
		lost := false
		if err == nil {
			var b []byte
			b, err = io.ReadAll(io.LimitReader(body, limit+1))
			body.Close()
			switch {
			case err == nil && int64(len(b)) > limit:
				return nil, fmt.Errorf("%w: a segment or playlist of more than %d bytes", errPlaylist, limit)
			case err == nil:
				if !broke.IsZero() {
					log.Printf("mixmusic: the live stream of %s comes again", l.videoID)
				}
				return b, nil
			case body.err != nil:
				err = body.err
			}
			lost = true
		} else {
			var ue *url.Error
			lost = errors.Is(err, errStalled) || errors.As(err, &ue)
		}
		switch {
		case l.ctx.Err() != nil:
			return nil, l.ctx.Err()
		case !lost:
			return nil, err
		case broke.IsZero():
			broke = time.Now()
			log.Printf("mixmusic: the live stream of %s broke off: %v; connecting again", l.videoID, loggable(err))
		case time.Since(broke)+wait > reconnect:
			return nil, errStalled
		}
		// The connections kept for reuse may have died with this one.
		l.p.HTTP.CloseIdleConnections()
		if !l.sleep(wait) {
			return nil, l.ctx.Err()
		}
		wait = min(2*wait, stall)
	}
}

// put hands the reader a segment.
func (l *Live) put(b []byte) {
	l.mu.Lock()
	l.segs = append(l.segs, span{off: l.end, data: b})
	l.end += int64(len(b))
	l.cond.Broadcast()
	l.mu.Unlock()
	l.once.Do(func() { close(l.started) })
}

// hasRoom tells whether the reader holds fewer than liveAhead segments it
// hasn't read.
func (l *Live) hasRoom() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.segs) < liveAhead
}

// waitRoom waits for the reader to have room for a segment, and tells
// whether the stream goes on.
func (l *Live) waitRoom() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for len(l.segs) >= liveAhead && !l.closed && l.ctx.Err() == nil {
		l.cond.Wait()
	}
	return !l.closed && l.ctx.Err() == nil
}

// sleep waits for d, and tells whether the stream goes on.
func (l *Live) sleep(d time.Duration) bool {
	if d <= 0 {
		return l.ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-l.ctx.Done():
		return false
	}
}

// Read reads the stream's packed audio, waiting for the segments that
// haven't come yet.
func (l *Live) Read(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for {
		if l.closed {
			return 0, errClosed
		}
		// The segments read past go: the fetcher has room for others.
		for len(l.segs) > 0 && l.segs[0].end() <= l.pos {
			l.segs[0] = span{}
			l.segs = l.segs[1:]
			l.cond.Broadcast()
		}
		if len(l.segs) > 0 && l.segs[0].off <= l.pos {
			sp := l.segs[0]
			n := copy(b, sp.data[l.pos-sp.off:])
			if l.pos += int64(n); l.pos == sp.end() {
				l.segs[0] = span{}
				l.segs = l.segs[1:]
				l.cond.Broadcast()
			}
			return n, nil
		}
		if l.err != nil {
			return 0, l.err
		}
		if l.ended {
			return 0, io.EOF
		}
		if l.interrupted != nil && l.interrupted() {
			return 0, ErrInterrupted
		}
		l.cond.Wait()
	}
}

// Seek moves where Read reads, within the segment it reads.
func (l *Live) Seek(offset int64, whence int) (int64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += l.pos
	default:
		return 0, errors.New("stream: a live stream has no end to seek from")
	}
	first := l.end
	if len(l.segs) > 0 {
		first = l.segs[0].off
	}
	if offset < first || offset > l.end {
		return 0, errors.New("stream: a live stream can't seek but within its segment")
	}
	l.pos = offset
	return offset, nil
}

// Wake makes a Read that waits check its interrupted function again.
func (l *Live) Wake() {
	l.mu.Lock()
	l.cond.Broadcast()
	l.mu.Unlock()
}

// Close stops the stream.
func (l *Live) Close() error {
	l.cancel()
	l.mu.Lock()
	l.closed = true
	l.segs = nil
	l.cond.Broadcast()
	l.mu.Unlock()
	return nil
}
