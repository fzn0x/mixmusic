package audio

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/fzn0x/mixmusic/aac"
	"github.com/fzn0x/mixmusic/ac3"
	"github.com/fzn0x/mixmusic/mp4"
	"github.com/fzn0x/mixmusic/resample"
)

// frameDecoder is the decoder of an MP4 track's samples: AAC-LC's, or
// AC-3's and E-AC-3's.
type frameDecoder interface {
	Decode(frame []byte, pcm []float32) (int, error)
	Reset()
}

// mp4Track is the audio track of an MP4 file: AAC-LC, as YouTube serves it
// at 44.1 kHz, in stereo, or the AC-3 and E-AC-3 of its 5.1 surround, in
// six channels; converted to Rate.
type mp4Track struct {
	r   *mp4.Reader
	dec frameDecoder
	// surround is the AC-3 decoder of a surround track, whose stream tells
	// its downmix; nil for AAC.
	surround *ac3.Decoder
	// downmix is the downmix the surround stream told last.
	downmix *[2][6]float32
	rs      *resample.Converter
	p       mp4.Packet // the packet read last
	// ch is the channels of the decoded frames, and frameLength the frames
	// a sample decodes to at most.
	ch, frameLength int
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

// openMP4 returns the audio track of an MP4 file: AAC-LC, or the AC-3 or
// E-AC-3 of 5.1 surround. Its errors are worded for the player.
func openMP4(src io.ReadSeeker) (track, timing, error) {
	r, err := mp4.NewReader(src)
	if err != nil {
		return nil, timing{}, readError(err)
	}
	t := r.Track()
	switch t.Codec {
	case "mp4a.40.2":
		return openAAC(r, t)
	case "ac-3", "ec-3":
		return openAC3(r, t)
	}
	return nil, timing{}, fmt.Errorf("%w (%s in MP4)", errUnsupported, t.Codec)
}

// openAAC returns the AAC-LC track of r.
func openAAC(r *mp4.Reader, t mp4.Track) (track, timing, error) {
	cfg, err := aac.ParseConfig(t.Config)
	if err != nil {
		return nil, timing{}, fmt.Errorf("%w (%v)", errUnsupported, err)
	}
	dec, err := aac.NewDecoder(cfg)
	if err != nil {
		return nil, timing{}, fmt.Errorf("%w (%v)", errUnsupported, err)
	}
	// Decode writes stereo, a mono stream's in both channels. A frame of
	// AAC overlaps the one before it: decoding starts two frames before a
	// seek's target, as the first frame after a reset is only half there,
	// and the second gives the converter what comes before the target.
	return newMP4Track(r, t, dec, nil, cfg.SampleRate, 2, aac.FrameLength, 2*aac.FrameLength)
}

// openAC3 returns the AC-3 or E-AC-3 track of r, of 5.1 surround, which
// decodes into six channels.
func openAC3(r *mp4.Reader, t mp4.Track) (track, timing, error) {
	rate := t.SampleRate
	if t.Codec == "ac-3" {
		cfg, err := ac3.ParseDAC3(t.Config)
		if err != nil {
			return nil, timing{}, fmt.Errorf("%w (%v)", errUnsupported, err)
		}
		rate = cfg.SampleRate
	}
	if rate <= 0 {
		return nil, timing{}, fmt.Errorf("%w (%s at %d Hz)", errUnsupported, t.Codec, rate)
	}
	dec := ac3.NewDecoder()
	// A sample's blocks overlap the block before them: decoding starts a
	// sample before a seek's target, whose last block settles the first
	// of the next, and gives the converter what comes before the target.
	return newMP4Track(r, t, dec, dec, rate, ac3.MaxChannels, ac3.FrameLength, ac3.FrameLength)
}

// newMP4Track returns the track r reads, which dec decodes into frames of
// ch channels at rate, of up to frameLength frames a sample; the decoding
// after a seek starts preRoll frames before its target.
func newMP4Track(r *mp4.Reader, t mp4.Track, dec frameDecoder, surround *ac3.Decoder, rate, ch, frameLength, preRoll int) (track, timing, error) {
	rs, err := resample.New(rate, Rate, ch)
	if err != nil {
		return nil, timing{}, fmt.Errorf("%w (%v)", errUnsupported, err)
	}
	r64 := int64(rate)
	g := gcd(r64, Rate)
	tm := timing{
		delay:   t.CodecDelay,
		preRoll: time.Duration(preRoll) * time.Second / time.Duration(rate),
		total:   r.Duration(),
	}
	tr := &mp4Track{
		r: r, dec: dec, surround: surround, rs: rs,
		ch: ch, frameLength: frameLength,
		rate: r64, up: Rate / g, down: r64 / g,
		origin:   framesAt(tm.delay, r64),
		originAt: frames(tm.delay),
		frame:    make([]float32, ch*frameLength),
		// The converter gives the frames of the input it holds back with
		// those of a frame, or at the end.
		pcm: make([]float32, ch*(rs.MaxOutput(frameLength)+rs.Latency())),
	}
	if scale := int64(t.Timescale); scale != r64 {
		tr.slack = (r64 + scale - 1) / scale
	}
	return tr, tm, nil
}

func (t *mp4Track) channels() int { return t.ch }

func (t *mp4Track) seek(at time.Duration) error {
	if err := t.r.Seek(at); err != nil {
		return err
	}
	t.dec.Reset()
	t.rs.Reset()
	t.started = false
	return nil
}

func (t *mp4Track) read() error {
	var err error
	t.p, err = t.r.ReadPacket()
	return err
}

func (t *mp4Track) decode() (decoded, error) {
	n, err := t.dec.Decode(t.p.Data, t.frame)
	if err != nil {
		if errors.Is(err, errors.ErrUnsupported) {
			return decoded{}, fmt.Errorf("%w (%v)", errUnsupported, err)
		}
		return decoded{}, fmt.Errorf("%w (%v)", errDecode, err)
	}
	if t.surround != nil {
		// The downmix rarely changes: a new one is kept only when it does,
		// for the deck to hand the audio thread.
		if m := t.surround.Info().Downmix; t.downmix == nil || m != *t.downmix {
			t.downmix = &m
		}
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
	ch := int64(t.ch)
	_, w := t.rs.Process(t.frame[ch*skip:ch*keep], t.pcm)
	return decoded{pcm: t.pcm[:t.ch*w], at: t.at, to: w, downmix: t.downmix}, nil
}

func (t *mp4Track) end() []float32 {
	n := t.rs.Flush(t.pcm)
	return t.pcm[:t.ch*n]
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
