package audio

import (
	"fmt"
	"math"
	"time"

	"github.com/fzn0x/mixmusic/resample"
)

// pcmCodec is the decoder of a file kept on a computer, whose audio comes
// at a rate of its own, in channels of its own: FLAC, WAV, MP3 or Vorbis.
type pcmCodec interface {
	// seek moves to the packet that holds frame f, at the codec's rate, or
	// the last before it, and returns the frame that packet starts at.
	seek(f int64) (int64, error)
	// read decodes the next packet into frames of the codec's channels,
	// interleaved, which are the codec's until the next read: io.EOF after
	// the last.
	read() ([]float32, error)
}

// pcmTrack is the track of a pcmCodec, mixed down to stereo, a mono
// file's in both channels, and converted to Rate.
type pcmTrack struct {
	codec pcmCodec
	// in is the codec's channels, and layout their mix into stereo, for
	// more than two.
	in     int
	layout *[2][8]float32
	conv   converter
	// next is the frame, at the codec's rate, of the first frame of the
	// packet read next; pcm the packet read last, in stereo.
	next   int64
	pcm    []float32
	stereo []float32
}

// newPCMTrack returns the track of codec, of in channels at rate, which
// plays as tm tells: its duration, and its codec's delay.
func newPCMTrack(codec pcmCodec, rate, in int, tm timing) (track, timing, error) {
	if in < 1 || in > 8 {
		return nil, timing{}, fmt.Errorf("%w (%d channels)", errUnsupported, in)
	}
	conv, err := newConverter(rate, tm.delay)
	if err != nil {
		return nil, timing{}, err
	}
	t := &pcmTrack{codec: codec, in: in, conv: conv}
	if in > 2 {
		t.layout = downmixOf(in)
	}
	// The converter's filter, and a decoder's overlap of packets, settle
	// within 50 ms before a seek's target.
	tm.preRoll = 50 * time.Millisecond
	return t, tm, nil
}

func (t *pcmTrack) channels() int { return 2 }

func (t *pcmTrack) seek(at time.Duration) error {
	first, err := t.codec.seek(framesAt(at, t.conv.rate))
	if err != nil {
		return err
	}
	t.next = first
	t.conv.reset()
	return nil
}

func (t *pcmTrack) read() error {
	pcm, err := t.codec.read()
	if err != nil {
		return err
	}
	n := len(pcm) / t.in
	switch t.in {
	case 2:
		t.pcm = pcm[:2*n]
		return nil
	case 1:
		t.stereo = grow(t.stereo, 2*n)
		for i, v := range pcm[:n] {
			t.stereo[2*i], t.stereo[2*i+1] = v, v
		}
	default:
		t.stereo = grow(t.stereo, 2*n)
		m := t.layout
		for i := range n {
			var l, r float32
			for c, v := range pcm[i*t.in : (i+1)*t.in] {
				l += m[0][c] * v
				r += m[1][c] * v
			}
			t.stereo[2*i], t.stereo[2*i+1] = l, r
		}
	}
	t.pcm = t.stereo[:2*n]
	return nil
}

func (t *pcmTrack) decode() (decoded, error) {
	first := t.next
	t.next += int64(len(t.pcm) / 2)
	return t.conv.convert(first, t.pcm), nil
}

func (t *pcmTrack) end() []float32 { return t.conv.flush() }

// grow returns b with room for n values.
func grow(b []float32, n int) []float32 {
	if cap(b) < n {
		return make([]float32, n)
	}
	return b[:n]
}

// converter converts the stereo frames of a track at its rate to Rate,
// as mp4Track does: after a seek, its input starts at a frame of the
// track a multiple of down frames from the first that plays, after the
// codec delay, so that its output falls on the frames at Rate from the
// one that plays it.
type converter struct {
	rs *resample.Converter
	// rate is the track's, and up and down the frames at Rate made of
	// frames at rate, the two divided by their greatest common divisor.
	rate, up, down int64
	// origin is the track's first frame that plays, at its rate, and
	// originAt the frame of the stream that plays it, at Rate.
	origin, originAt int64
	// After a seek, the first packet tells drop, the frames the converter
	// is not given, and at, the frame at Rate its output starts at.
	started  bool
	drop, at int64
	pcm      []float32
}

// newConverter returns the converter of a track at rate, whose codec
// delays its audio by delay.
func newConverter(rate int, delay time.Duration) (converter, error) {
	rs, err := resample.New(rate, Rate, 2)
	if err != nil {
		return converter{}, fmt.Errorf("%w (%v)", errUnsupported, err)
	}
	r := int64(rate)
	g := gcd(r, Rate)
	return converter{rs: rs, rate: r, up: Rate / g, down: r / g, origin: framesAt(delay, r), originAt: frames(delay)}, nil
}

func (c *converter) reset() {
	c.rs.Reset()
	c.started = false
}

// convert converts the frames of in, the first of which is frame first of
// the track at its rate.
func (c *converter) convert(first int64, in []float32) decoded {
	n := int64(len(in) / 2)
	if !c.started {
		c.drop = (c.origin - first) % c.down
		if c.drop < 0 {
			c.drop += c.down
		}
		c.at = c.originAt + (first+c.drop-c.origin)/c.down*c.up
		c.started = true
	}
	skip := min(c.drop, n)
	c.drop -= skip
	c.pcm = grow(c.pcm, 2*(c.rs.MaxOutput(int(n))+c.rs.Latency()))
	_, w := c.rs.Process(in[2*skip:], c.pcm)
	return decoded{pcm: c.pcm[:2*w], at: c.at, to: w}
}

// flush returns the frames the converter holds back, at the end.
func (c *converter) flush() []float32 {
	c.pcm = grow(c.pcm, 2*(c.rs.Latency()+1))
	n := c.rs.Flush(c.pcm)
	return c.pcm[:2*n]
}

// downmixOf returns the mix into stereo of n channels, from 3 to 8, in
// the order of FLAC and of WAV's usual masks: left, right, center, LFE,
// the back and the side pairs, as ITU-R BS.775 mixes them down, without
// the LFE, and scaled so that no channel's sum clips.
func downmixOf(n int) *[2][8]float32 {
	const h = math.Sqrt2 / 2
	var m [2][8]float32
	set := func(c int, l, r float32) { m[0][c], m[1][c] = l, r }
	set(0, 1, 0)
	set(1, 0, 1)
	switch n {
	case 3:
		set(2, h, h)
	case 4:
		set(2, h, 0)
		set(3, 0, h)
	case 5:
		set(2, h, h)
		set(3, h, 0)
		set(4, 0, h)
	default: // 6 to 8: 5.1, 6.1 and 7.1
		set(2, h, h)
		switch n {
		case 6:
			set(4, h, 0)
			set(5, 0, h)
		case 7:
			set(4, h/2, h/2) // the back center
			set(5, h, 0)
			set(6, 0, h)
		case 8:
			set(4, h, 0)
			set(5, 0, h)
			set(6, h, 0)
			set(7, 0, h)
		}
	}
	var sum float32
	for _, v := range m[0] {
		sum += v
	}
	for i := range m {
		for c := range m[i] {
			m[i][c] /= sum
		}
	}
	return &m
}
