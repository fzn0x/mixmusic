package audio

import (
	"sync/atomic"
	"time"
)

// ringFrames is the capacity of a ring, a power of two: 0.68 s at Rate.
const ringFrames = 1 << 15

// ring passes decoded frames from a deck's decoder, its only writer, to the
// audio thread, its only reader, without locks: the counters say what each
// may touch. A deck makes a new ring when it seeks, so that the reader
// drops what the old one held.
type ring struct {
	// channels is the channels of its frames, interleaved: 2, or 6 for
	// 5.1 surround.
	channels int
	buf      []float32    // ringFrames frames
	read     atomic.Int64 // the frames read
	write    atomic.Int64 // the frames written
	// start is the frame of the track the ring starts at.
	start int64
	// done is set after the last frame: the track's end, or an error.
	done atomic.Bool
	// space wakes the writer when the reader frees some.
	space chan struct{}
}

// newRing returns a ring of frames of channels, of the track from frame
// start.
func newRing(start int64, channels int) *ring {
	return &ring{channels: channels, buf: make([]float32, channels*ringFrames), start: start, space: make(chan struct{}, 1)}
}

// readable returns the frames the reader may read.
func (r *ring) readable() int64 { return r.write.Load() - r.read.Load() }

// readTo moves up to len(out)/channels frames into out, and returns how
// many.
func (r *ring) readTo(out []float32) int {
	ch := r.channels
	n := int(min(int64(len(out)/ch), r.readable()))
	at := int(r.read.Load() % ringFrames)
	first := min(n, ringFrames-at)
	copy(out[:ch*first], r.buf[ch*at:])
	copy(out[ch*first:ch*n], r.buf)
	r.read.Add(int64(n))
	select {
	case r.space <- struct{}{}:
	default:
	}
	return n
}

// writeFrom moves the frames of in into the ring, waiting for space as
// long as wait allows: it returns the frames written, all of them unless
// wait returned false.
func (r *ring) writeFrom(in []float32, wait func(<-chan struct{}) bool) int {
	ch := r.channels
	written := 0
	for written < len(in)/ch {
		free := ringFrames - int(r.readable())
		if free == 0 {
			if !wait(r.space) {
				return written
			}
			continue
		}
		n := min(free, len(in)/ch-written)
		at := int(r.write.Load() % ringFrames)
		first := min(n, ringFrames-at)
		copy(r.buf[ch*at:], in[ch*written:ch*(written+first)])
		copy(r.buf, in[ch*(written+first):ch*(written+n)])
		r.write.Add(int64(n))
		written += n
	}
	return written
}

// position returns the frame of the track the reader is at.
func (r *ring) position() int64 { return r.start + r.read.Load() }

// frames converts a duration to frames at Rate, rounded.
func frames(d time.Duration) int64 {
	return (int64(d)*Rate + int64(time.Second)/2) / int64(time.Second)
}

// duration converts frames at Rate to a duration.
func duration(n int64) time.Duration {
	return time.Duration(n * int64(time.Second) / Rate)
}
