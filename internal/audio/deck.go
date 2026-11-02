package audio

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fzn0x/mixmusic/mp4"
	"github.com/fzn0x/mixmusic/webm"
)

// Source is the file of a track, WebM or MP4, as a deck reads it: its bytes
// as they arrive. A Read that waits for bytes returns an error once the
// interrupted function its Opener got returns true; Wake makes such a Read
// check it again.
type Source interface {
	io.ReadSeeker
	Wake()
	Close() error
}

// Opener opens the Source of a track's URL, whose reads stop waiting when
// interrupted returns true. Its errors say why in the words of the
// interface, as the player shows them.
type Opener func(ctx context.Context, url string, interrupted func() bool) (Source, error)

// The failures of a deck, as the web player words those of its audio
// element.
var (
	errNetwork     = errors.New("the network failed while loading the audio")
	errDecode      = errors.New("the audio could not be decoded")
	errUnsupported = errors.New("the audio format is not supported")
)

// opusHead is what a deck needs of an OpusHead (RFC 7845, section 5.1).
type opusHead struct {
	channels int
	preSkip  int
	gain     int // in 1/256 dB
}

func parseOpusHead(b []byte) (opusHead, error) {
	if len(b) < 19 || string(b[:8]) != "OpusHead" || b[8]>>4 != 0 {
		return opusHead{}, errUnsupported
	}
	h := opusHead{
		channels: int(b[9]),
		preSkip:  int(binary.LittleEndian.Uint16(b[10:])),
		gain:     int(int16(binary.LittleEndian.Uint16(b[16:]))),
	}
	// Mapping family 0 is mono or stereo in one stream; the others, as
	// surround sound, need a multistream decoder.
	if b[18] != 0 || h.channels < 1 || h.channels > 2 {
		return opusHead{}, errUnsupported
	}
	return h, nil
}

// deck plays one track: its goroutine reads the track's file, decodes it,
// and fills the ring the audio thread plays from.
type deck struct {
	url string
	// ring is the ring of the current position: a new one at each seek,
	// which drops at once what the old one held. Its frames are of the
	// track's channels, once the deck read them, else stereo.
	ring atomic.Pointer[ring]
	// downmix is the stereo downmix of a surround track, as its stream
	// asks for it, once the deck decoded a frame of it: the audio thread
	// mixes its six channels down with it when the output plays two.
	downmix atomic.Pointer[[2][6]float32]
	// gain is the deck's gain, a ramp for crossfades: the audio thread's.
	gain ramp
	// playing is whether the deck plays, or holds at its position.
	playing atomic.Bool
	// lkfs is the loudness of the track, in LKFS, as the bits of a
	// float64: NaN until the deck opened a Measured source, which it does
	// before it decodes a frame.
	lkfs atomic.Uint64
	// trim turns a loud track down to an even loudness, and trimmed tells
	// that it took its level, with the deck's first frames: the audio
	// thread's.
	trim    smoother
	trimmed bool

	cancel context.CancelFunc
	done   chan struct{}
	// changed wakes the goroutine for a seek.
	changed chan struct{}

	mu       sync.Mutex
	channels int           // of the track, once its headers were read: 2 before
	seekTo   time.Duration // a seek asked for, or -1
	src      Source        // the open source, to wake for seeks
	err      error         // why the deck failed
	duration time.Duration
	ready    bool // whether the headers were read
}

// newDeck starts a deck of url at start. notify is called, from the deck's
// goroutine, when its state changes.
func newDeck(url string, start time.Duration, open Opener, notify func()) *deck {
	ctx, cancel := context.WithCancel(context.Background())
	d := &deck{
		url:      url,
		gain:     ramp{value: 1, target: 1},
		trim:     newSmoother(1, glide),
		cancel:   cancel,
		done:     make(chan struct{}),
		changed:  make(chan struct{}, 1),
		seekTo:   -1,
		channels: 2,
	}
	d.lkfs.Store(math.Float64bits(math.NaN()))
	d.ring.Store(newRing(frames(start), 2))
	go func() {
		defer close(d.done)
		cur := d.ring.Load()
		err := d.play(ctx, start, &cur, open, notify)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			d.mu.Lock()
			d.err = err
			d.mu.Unlock()
		}
		cur.done.Store(true)
		notify()
	}()
	return d
}

// close stops the deck's goroutine, and waits for it.
func (d *deck) close() {
	d.cancel()
	d.wake()
	<-d.done
}

// seek moves the deck to t: a new ring starts there, which the deck's
// goroutine fills.
func (d *deck) seek(t time.Duration) {
	t = max(0, t)
	d.mu.Lock()
	d.seekTo = t
	d.ring.Store(newRing(frames(t), d.channels))
	d.mu.Unlock()
	d.wake()
}

func (d *deck) wake() {
	d.mu.Lock()
	src := d.src
	d.mu.Unlock()
	if src != nil {
		src.Wake()
	}
	select {
	case d.changed <- struct{}{}:
	default:
	}
}

func (d *deck) seekPending() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.seekTo >= 0
}

// takeSeek returns the seek asked for, if any, with its ring: the goroutine
// fills that one, not a later seek's.
func (d *deck) takeSeek() (time.Duration, *ring, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	t := d.seekTo
	if t < 0 {
		return 0, nil, false
	}
	d.seekTo = -1
	return t, d.ring.Load(), true
}

// position returns the frame of the track the deck plays from.
func (d *deck) position() int64 { return d.ring.Load().position() }

// failure returns why the deck failed, or nil.
func (d *deck) failure() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.err
}

// info returns the track's duration, and whether the deck read it yet.
func (d *deck) info() (time.Duration, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.duration, d.ready
}

// play opens the track and decodes it from start into *cur, the ring of
// the position it plays, following seeks, until the deck closes: it
// returns nil then, or why it failed.
func (d *deck) play(ctx context.Context, start time.Duration, cur **ring, open Opener, notify func()) error {
	interrupted := func() bool { return ctx.Err() != nil || d.seekPending() }
	src, err := open(ctx, d.url, interrupted)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	// Before any frame: the audio thread sees the loudness with the first
	// frames the ring gives it.
	if m, ok := src.(Measured); ok {
		if lkfs, ok := m.Loudness(); ok {
			d.lkfs.Store(math.Float64bits(lkfs))
		}
	}
	d.mu.Lock()
	d.src = src
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		d.src = nil
		d.mu.Unlock()
		src.Close()
	}()

	var tr track
	var tm timing
	for {
		tr, tm, err = openTrack(src)
		if err == nil || !interrupted() {
			break
		}
		if ctx.Err() != nil {
			return nil
		}
		// A seek before the headers came: the decoding starts there, and
		// the headers are read again.
		start, *cur, _ = d.takeSeek()
		if _, err := src.Seek(0, io.SeekStart); err != nil {
			return errNetwork
		}
	}
	if err != nil {
		return err
	}
	delay := tm.delay
	ch := tr.channels()
	d.mu.Lock()
	if tm.total > delay {
		d.duration = tm.total - delay
	}
	d.ready = true
	// The rings from now on hold frames of the track's channels: the one
	// the audio thread reads takes them before it got a frame, as the one
	// a seek asked for meanwhile, which the loop below takes.
	d.channels = ch
	if r := d.ring.Load(); r.channels != ch {
		nr := newRing(r.start, ch)
		d.ring.Store(nr)
		if *cur == r {
			*cur = nr
		}
	}
	d.mu.Unlock()
	notify()

	// next is the frame of the stream, codec delay included, of the next
	// sample decoded, once known; skip is the frame to play from.
	var next, skip int64
	seekTo := func(t time.Duration) error {
		target := t + delay
		if err := tr.seek(max(0, target-tm.preRoll)); err != nil {
			return err
		}
		next, skip = -1, frames(target)
		return nil
	}
	// wait waits for space in the ring, unless a seek or the end comes.
	wait := func(space <-chan struct{}) bool {
		for !interrupted() {
			select {
			case <-space:
				return true
			case <-d.changed:
			case <-ctx.Done():
			}
		}
		return false
	}
	// atEnd waits at the end of the track for a seek, or for the end of
	// the deck.
	atEnd := func() bool {
		for !interrupted() {
			select {
			case <-d.changed:
			case <-ctx.Done():
			}
		}
		return ctx.Err() == nil
	}
	// write plays the frames of pcm, which follow those decoded before,
	// from from to to and from skip on.
	write := func(pcm []float32, from, to int64) {
		if next < skip {
			from = max(from, min(skip-next, to))
		}
		next += int64(len(pcm) / ch)
		if from < to {
			(*cur).writeFrom(pcm[int64(ch)*from:int64(ch)*to], wait)
		}
	}
	if err := seekTo(start); err != nil {
		if !interrupted() {
			return readError(err)
		}
	}
	for {
		if ctx.Err() != nil {
			return nil
		}
		if t, r, ok := d.takeSeek(); ok {
			*cur = r
			notify()
			if err := seekTo(t); err != nil {
				if interrupted() {
					continue
				}
				return readError(err)
			}
		}
		err := tr.read()
		if err != nil && err != io.EOF && !interrupted() && d.brokeNearEnd(err, next, skip, delay) {
			err = io.EOF // what's left is less than loading it again costs
		}
		if err == io.EOF {
			if pcm := tr.end(); len(pcm) > 0 {
				write(pcm, 0, int64(len(pcm)/ch))
			}
			(*cur).done.Store(true)
			notify()
			if !atEnd() {
				return nil
			}
			continue
		}
		if err != nil {
			if interrupted() {
				continue // a seek, or the end of the deck
			}
			return readError(err)
		}
		p, err := tr.decode()
		if err != nil {
			return err
		}
		if next < 0 {
			next = p.at
		}
		if p.downmix != nil && p.downmix != d.downmix.Load() {
			d.downmix.Store(p.downmix) // before the frames it mixes down
		}
		write(p.pcm, int64(p.from), int64(p.to))
	}
}

// readError words a failure of the reader for the player.
func readError(err error) error {
	if errors.Is(err, webm.ErrFormat) || errors.Is(err, errors.ErrUnsupported) || errors.Is(err, webm.ErrNoAudio) ||
		errors.Is(err, mp4.ErrFormat) || errors.Is(err, mp4.ErrNoAudio) {
		return fmt.Errorf("%w (%v)", errUnsupported, err)
	}
	return errNetwork
}

// endSlack is how close to its end a track whose stream breaks off ends
// there, rather than failing: loading it again for the moment left costs
// more silence than the moment holds.
const endSlack = time.Second

// brokeNearEnd reports whether err, a read that failed with the track
// decoded up to frame next of the stream (codec delay included), or from
// frame skip on before the first packet after a seek, is the stream
// breaking off within endSlack of the track's end. A track whose duration
// the file doesn't tell has no end to be near.
func (d *deck) brokeNearEnd(err error, next, skip int64, delay time.Duration) bool {
	if readError(err) != errNetwork {
		return false // a file it can't read, not a break
	}
	total, _ := d.info()
	at := next
	if at < 0 {
		at = skip
	}
	return total > 0 && duration(at)-delay >= total-endSlack
}

// ramp moves a gain linearly, as linearRampToValueAtTime does.
type ramp struct {
	value, target float64
	step          float64 // per frame
}

// to starts a ramp to target over d; at once when d is 0.
func (r *ramp) to(target float64, d time.Duration) {
	r.target = target
	if n := frames(d); n > 0 {
		r.step = (target - r.value) / float64(n)
		return
	}
	r.value, r.step = target, 0
}

// next returns the gain of the next frame.
func (r *ramp) next() float64 {
	if r.value != r.target {
		r.value += r.step
		if r.step == 0 || (r.step > 0) == (r.value >= r.target) {
			r.value, r.step = r.target, 0
		}
	}
	return r.value
}
