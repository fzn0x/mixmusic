package audio

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/fzn0x/mixmusic/internal/aac"
	"github.com/fzn0x/mixmusic/internal/audio/resample"
	"github.com/fzn0x/mixmusic/internal/mp4"
)

// aacTrack is the AAC-LC track of an MP4 file, as YouTube serves it at
// 44.1 kHz, converted to Rate.
type aacTrack struct {
	r   *mp4.Reader
	dec *aac.Decoder
	rs  *resample.Converter
	p   mp4.Packet // the packet read last
	// rate is the track's sampling frequency. The converter makes up frames
	// at Rate of each down frames at rate, the two rates divided by their
	// greatest common divisor.
	rate     int64
	up, down int64
	// slack is how many frames short of its frame a packet's duration may
	// fall with the packet whole: none in a timescale of rate, else the
	// frames of a tick, which the durations are rounded to.
	slack int64
	// origin is the track's first frame that plays, after the codec delay,
	// at its rate, and originAt the frame of the stream that plays it, the
	// codec delay at Rate. The converter's output falls on the frames at
	// Rate from originAt on when its input starts a multiple of down frames
	// from origin.
	origin, originAt int64
	// After a seek, the first packet tells drop, the frames the converter
	// is not given, so that its input starts at such a frame, and at, the
	// frame of the stream its output starts at.
	started  bool
	drop, at int64
	frame    []float32 // a decoded frame, at the track's rate
	pcm      []float32 // the converter's output
}

// openAAC returns the AAC-LC track of an MP4 file. Its errors are worded
// for the player.
func openAAC(src io.ReadSeeker) (track, timing, error) {
	r, err := mp4.NewReader(src)
	if err != nil {
		return nil, timing{}, readError(err)
	}
	t := r.Track()
	if t.Codec != "mp4a.40.2" {
		return nil, timing{}, fmt.Errorf("%w (%s in MP4)", errUnsupported, t.Codec)
	}
	cfg, err := aac.ParseConfig(t.Config)
	if err != nil {
		return nil, timing{}, fmt.Errorf("%w (%v)", errUnsupported, err)
	}
	dec, err := aac.NewDecoder(cfg)
	if err != nil {
		return nil, timing{}, fmt.Errorf("%w (%v)", errUnsupported, err)
	}
	// Decode writes stereo, a mono stream's in both channels.
	rs, err := resample.New(cfg.SampleRate, Rate, 2)
	if err != nil {
		return nil, timing{}, fmt.Errorf("%w (%v)", errUnsupported, err)
	}
	rate := int64(cfg.SampleRate)
	g := gcd(rate, Rate)
	// A frame of AAC overlaps the one before it: decoding starts two frames
	// before a seek's target, as the first frame after a reset is only half
	// there, and the second gives the converter what comes before the
	// target.
	tm := timing{
		delay:   t.CodecDelay,
		preRoll: time.Duration(2*aac.FrameLength) * time.Second / time.Duration(rate),
		total:   r.Duration(),
	}
	tr := &aacTrack{
		r: r, dec: dec, rs: rs,
		rate: rate, up: Rate / g, down: rate / g,
		origin:   framesAt(tm.delay, rate),
		originAt: frames(tm.delay),
		frame:    make([]float32, 2*aac.FrameLength),
		// The converter gives the frames of the input it holds back with
		// those of a frame, or at the end.
		pcm: make([]float32, 2*(rs.MaxOutput(aac.FrameLength)+rs.Latency())),
	}
	if scale := int64(t.Timescale); scale != rate {
		tr.slack = (rate + scale - 1) / scale
	}
	return tr, tm, nil
}

func (t *aacTrack) seek(at time.Duration) error {
	if err := t.r.Seek(at); err != nil {
		return err
	}
	t.dec.Reset()
	t.rs.Reset()
	t.started = false
	return nil
}

func (t *aacTrack) read() error {
	var err error
	t.p, err = t.r.ReadPacket()
	return err
}

func (t *aacTrack) decode() (decoded, error) {
	n, err := t.dec.Decode(t.p.Data, t.frame)
	if err != nil {
		if errors.Is(err, errors.ErrUnsupported) {
			return decoded{}, fmt.Errorf("%w (%v)", errUnsupported, err)
		}
		return decoded{}, fmt.Errorf("%w (%v)", errDecode, err)
	}
	// A packet's duration tells how much of its frame is audio: the last
	// one's ends early, the rest being the encoder's padding. Another's
	// falls short only by the rounding of a timescale other than the
	// track's rate, which the slack allows for.
	keep := int64(n)
	if d := t.p.Duration; d > 0 {
		if k := framesAt(d, t.rate); k < keep-t.slack {
			keep = k
		}
	}
	if !t.started {
		first := framesAt(t.p.Time, t.rate)
		t.drop = (t.origin - first) % t.down
		if t.drop < 0 {
			t.drop += t.down
		}
		t.at = t.originAt + (first+t.drop-t.origin)/t.down*t.up
		t.started = true
	}
	skip := min(t.drop, keep)
	t.drop -= skip
	_, w := t.rs.Process(t.frame[2*skip:2*keep], t.pcm)
	return decoded{pcm: t.pcm[:2*w], at: t.at, to: w}, nil
}

func (t *aacTrack) end() []float32 {
	n := t.rs.Flush(t.pcm)
	return t.pcm[:2*n]
}

// framesAt converts a duration to frames at rate, rounded.
func framesAt(d time.Duration, rate int64) int64 {
	s, ns := int64(d/time.Second), int64(d%time.Second)
	return s*rate + (ns*rate+int64(time.Second)/2)/int64(time.Second)
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
