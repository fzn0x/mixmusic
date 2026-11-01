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

	"github.com/fzn0x/mixmusic/internal/opus"
	"github.com/fzn0x/mixmusic/internal/webm"
)

// Source is the WebM file of a track, as a deck reads it: its bytes as they
// arrive. A Read that waits for bytes returns an error once the
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
	// which drops at once what the old one held.
	ring atomic.Pointer[ring]
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
		url:     url,
		gain:    ramp{value: 1, target: 1},
		trim:    newSmoother(1, glide),
		cancel:  cancel,
		done:    make(chan struct{}),
		changed: make(chan struct{}, 1),
		seekTo:  -1,
	}
	d.lkfs.Store(math.Float64bits(math.NaN()))
	d.ring.Store(newRing(frames(start)))
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
	d.ring.Store(newRing(frames(t)))
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

	var r *webm.Reader
	for {
		r, err = webm.NewReader(src)
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
		return readError(err)
	}
	track := r.Track()
	if track.Codec != "A_OPUS" {
		return errUnsupported
	}
	head, err := parseOpusHead(track.CodecPrivate)
	if err != nil {
		return err
	}
	dec, err := opus.NewDecoder(Rate, 2)
	if err != nil {
		return errDecode
	}
	dec.SetGain(head.gain)
	// The audio of a packet plays at its time less the codec delay, which
	// is the pre-skip of the OpusHead when the file doesn't give it.
	delay := track.CodecDelay
	if delay == 0 {
		delay = time.Duration(head.preSkip) * time.Second / Rate
	}
	preRoll := track.SeekPreRoll
	if preRoll == 0 {
		preRoll = 80 * time.Millisecond
	}
	d.mu.Lock()
	if total := r.Duration(); total > delay {
		d.duration = total - delay
	}
	d.ready = true
	d.mu.Unlock()
	notify()

	pcm := make([]float32, 2*5760)
	// next is the frame of the stream, codec delay included, of the next
	// sample decoded, once known; skip is the frame to play from.
	var next, skip int64
	seekTo := func(t time.Duration) error {
		target := t + delay
		if err := r.Seek(max(0, target-preRoll)); err != nil {
			return err
		}
		dec.Reset()
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
		p, err := r.ReadPacket()
		if err == io.EOF {
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
		n, err := dec.Decode(p.Data, pcm, false)
		if err != nil {
			return errDecode
		}
		if next < 0 {
			next = frames(p.Time)
		}
		from, to := int64(0), int64(n)
		if next < skip {
			from = min(skip-next, to)
		}
		if p.DiscardPadding > 0 {
			to = max(from, to-frames(p.DiscardPadding))
		} else if p.DiscardPadding < 0 {
			from = max(from, min(frames(-p.DiscardPadding), to))
		}
		next += int64(n)
		if from < to {
			(*cur).writeFrom(pcm[2*from:2*to], wait)
		}
	}
}

// readError words a failure of the reader for the player.
func readError(err error) error {
	if errors.Is(err, webm.ErrFormat) || errors.Is(err, errors.ErrUnsupported) || errors.Is(err, webm.ErrNoAudio) {
		return fmt.Errorf("%w (%v)", errUnsupported, err)
	}
	return errNetwork
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
