package aac

// Port of Decoder.cs and Syntax/SyntacticElements.cs, with Syntax/CPE.cs,
// Syntax/DSE.cs and Syntax/FIL.cs: the decoder of a stream, which reads the
// elements of each raw data block and turns the spectra of its channels
// into samples.

import (
	"errors"
	"fmt"
	"math"
)

// Syntactic elements of a raw data block.
const (
	elemSCE = 0 // single channel element
	elemCPE = 1 // channel pair element
	elemCCE = 2 // coupling channel element
	elemLFE = 3 // LFE channel element
	elemDSE = 4 // data stream element
	elemPCE = 5 // program config element
	elemFIL = 6 // fill element
	elemEND = 7
)

// randomSeed is the first state of the generator of PNS's noise.
const randomSeed = 0x1F2E3D4C

// channel is the state of a channel: what the decoder read of it in the
// frame, and what it keeps of the last.
type channel struct {
	ics  ics
	spec [FrameLength]float32
	// overlap is the second half of the last frame's windows, which the
	// first half of this frame's overlaps.
	overlap [FrameLength]float32
	// prevShape is the window shape of the last frame, whose second half
	// the first half of this frame's window matches.
	prevShape int
}

// Decoder decodes an AAC-LC stream. It is not safe for concurrent use.
type Decoder struct {
	cfg    Config
	tables *rateTables
	r      bitReader
	ch     [2]channel

	// The M/S stereo of the channel pair.
	commonWindow bool
	msMask       int
	msUsed       [maxBands]bool

	// random is the state of the generator of PNS's noise.
	random uint32

	// Scratch of the filterbank.
	buf [2 * FrameLength]float32
	fft [FrameLength / 2][2]float32
	out [2][FrameLength]float32
}

// NewDecoder returns a decoder of streams of cfg, which must be of
// ObjectTypeLC, of 1 or 2 channels. It returns errors.ErrUnsupported for
// the others.
func NewDecoder(cfg Config) (*Decoder, error) {
	if cfg.ObjectType != ObjectTypeLC {
		return nil, fmt.Errorf("aac: audio object type %d: %w", cfg.ObjectType, errors.ErrUnsupported)
	}
	if cfg.Channels != 1 && cfg.Channels != 2 {
		return nil, fmt.Errorf("aac: %d channels: %w", cfg.Channels, errors.ErrUnsupported)
	}
	if cfg.SampleRate <= 0 {
		return nil, fmt.Errorf("%w: sampling frequency %d", ErrFormat, cfg.SampleRate)
	}
	initTables()
	d := &Decoder{cfg: cfg, tables: &rates[rateIndex(cfg.SampleRate)]}
	d.Reset()
	return d, nil
}

// Config returns the config of the decoder's stream.
func (d *Decoder) Config() Config { return d.cfg }

// SampleRate is the sampling frequency of what the decoder outputs, in Hz.
func (d *Decoder) SampleRate() int { return d.cfg.SampleRate }

// Reset makes the decoder forget the stream, as for a new one or after a
// seek: the frame decoded next overlaps silence. After a seek, decoding
// from the frame before the one sought, and dropping its samples, gives
// those of the frame sought whole.
func (d *Decoder) Reset() {
	for c := range d.ch {
		clear(d.ch[c].overlap[:])
		d.ch[c].prevShape = 0
	}
	d.random = randomSeed
}

// Decode decodes a frame, a raw data block, as an MP4 sample or an ADTS
// frame without its header holds it, into pcm, and returns the number of
// samples per channel: FrameLength. The samples are interleaved, of two
// channels, a mono stream's in both, as float32 of about [-1, 1], not
// clipped, but finite. pcm must have room for 2*FrameLength.
//
// A frame decodes into the second half of the last one's windows and the
// first half of its own: a stream's first frame, or the first after
// Reset, holds only half a window of audio. After an error the decoder
// decodes the next frame as if the bad one never came.
func (d *Decoder) Decode(frame []byte, pcm []float32) (int, error) {
	if len(pcm) < 2*FrameLength {
		return 0, ErrBufferTooSmall
	}
	d.r.init(frame)
	if err := d.parse(); err != nil {
		return 0, err
	}
	channels := d.cfg.Channels
	d.dequantize(&d.ch[0], nil)
	if channels == 2 {
		d.dequantize(&d.ch[1], &d.ch[0])
		d.applyMS()
		d.applyIS()
	}
	for c := range channels {
		ch := &d.ch[c]
		info := &ch.ics.info
		if ch.ics.tns.present {
			maxBands := d.tables.tnsLong
			if info.seq == eightShortSequence {
				maxBands = d.tables.tnsShort
			}
			ch.ics.tns.apply(ch.spec[:], info, maxBands)
		}
		d.synthesize(ch, info, d.out[c][:])
	}
	left, right := &d.out[0], &d.out[channels-1]
	for i := range FrameLength {
		pcm[2*i] = finite(left[i])
		pcm[2*i+1] = finite(right[i])
	}
	return FrameLength, nil
}

// finite returns v, or 0 for an infinity or a NaN, which only a corrupt
// frame makes, and which would spoil the filters the samples go through
// after.
func finite(v float32) float32 {
	if v-v != 0 {
		return 0
	}
	return v
}

// parse reads the elements of a raw data block, up to its END: one SCE in
// a mono stream, one CPE in a stereo one, and those that hold no audio.
func (d *Decoder) parse() error {
	r := &d.r
	audio := false
	for {
		id := r.read(3)
		if r.over() {
			return fmt.Errorf("%w: frame ends before its END element", ErrFormat)
		}
		switch id {
		case elemSCE, elemCPE:
			if audio {
				return fmt.Errorf("%w: a second channel element", ErrFormat)
			}
			if id == elemSCE && d.cfg.Channels == 2 {
				return fmt.Errorf("%w: an SCE in a stereo stream", ErrFormat)
			}
			if id == elemCPE && d.cfg.Channels == 1 {
				return fmt.Errorf("%w: a CPE in a mono stream", ErrFormat)
			}
			audio = true
			r.skip(4) // element_instance_tag
			var err error
			if id == elemSCE {
				err = d.ch[0].ics.parse(r, d.tables, false)
			} else {
				err = d.parseCPE()
			}
			if err != nil {
				return err
			}
		case elemCCE:
			return fmt.Errorf("aac: coupling channel element: %w", errors.ErrUnsupported)
		case elemLFE:
			return fmt.Errorf("aac: LFE channel element: %w", errors.ErrUnsupported)
		case elemDSE:
			r.skip(4) // element_instance_tag
			align := r.flag()
			n := r.read(8)
			if n == 255 {
				n += r.read(8)
			}
			if align {
				r.align()
			}
			r.skip(8 * n)
		case elemPCE:
			// A program config element in a frame changes nothing the
			// decoder decodes.
			var pce programConfig
			if err := pce.parse(r); err != nil {
				return err
			}
		case elemFIL:
			// Fill data, or an extension payload: SBR and PS data, or
			// dynamic range control, which the decoder skips alike.
			n := r.read(4)
			if n == 15 {
				n += r.read(8) - 1
			}
			r.skip(8 * n)
		case elemEND:
			if !audio {
				return fmt.Errorf("%w: frame without a channel element", ErrFormat)
			}
			return nil
		}
		if r.over() {
			return fmt.Errorf("%w: element past the frame", ErrFormat)
		}
	}
}

// parseCPE reads a channel_pair_element, after its element_instance_tag.
func (d *Decoder) parseCPE() error {
	r := &d.r
	left, right := &d.ch[0].ics, &d.ch[1].ics
	d.commonWindow = r.flag()
	d.msMask = 0
	clear(d.msUsed[:])
	if d.commonWindow {
		if err := left.info.parse(r, d.tables); err != nil {
			return err
		}
		right.info = left.info
		d.msMask = r.read(2)
		switch d.msMask {
		case 1:
			for i := range left.info.groups * left.info.maxSFB {
				d.msUsed[i] = r.flag()
			}
		case 2:
			for i := range d.msUsed {
				d.msUsed[i] = true
			}
		case 3:
			return fmt.Errorf("%w: reserved M/S mask", ErrFormat)
		}
	}
	if err := left.parse(r, d.tables, d.commonWindow); err != nil {
		return err
	}
	return right.parse(r, d.tables, d.commonWindow)
}

// dequantize turns the quantized spectrum of c into c.spec: the inverse
// quantization of each value, scaled by the gain of its band's
// scalefactor, and noise in the bands PNS substitutes. left is, when c is
// the right channel of a pair, its left channel, whose spectrum is done.
func (d *Decoder) dequantize(c, left *channel) {
	s := &c.ics
	info := &s.info
	clear(c.spec[:])
	groupOff := 0
	for g := range info.groups {
		for sfb := range info.maxSFB {
			i := g*info.maxSFB + sfb
			lo, hi := int(info.swb[sfb]), int(info.swb[sfb+1])
			switch s.bandType[i] {
			case zeroBook, intensityBook, intensityBook2:
				// Zeros, or the left channel's spectrum, which applyIS
				// copies later.
			case noiseBook:
				// Noise of the band's energy, in each window of the
				// group.
				gain := pow2q[s.sf[i]+100] * outScale
				for w := range info.groupLen[g] {
					x := c.spec[groupOff+128*w+lo : groupOff+128*w+hi]
					var energy float32
					for k := range x {
						d.random = d.random*1664525 + 1013904223
						x[k] = float32(int32(d.random))
						energy += x[k] * x[k]
					}
					scale := float32(float64(gain) / math.Sqrt(float64(energy)))
					for k := range x {
						x[k] *= scale
					}
				}
				if left != nil && d.msMask != 0 && d.msUsed[i] && left.ics.bandType[i] == noiseBook {
					// Both channels substitute noise, and ms_used is set:
					// the standard has them share the left channel's, each
					// at its own energy, rather than M/S code the band.
					// The generator moved on all the same, so that the
					// noise of the bands after doesn't depend on which
					// bands share theirs.
					ratio := pow2q[s.sf[i]+100] / pow2q[left.ics.sf[i]+100]
					for w := range info.groupLen[g] {
						off := groupOff + 128*w
						for k := lo; k < hi; k++ {
							c.spec[off+k] = left.spec[off+k] * ratio
						}
					}
				}
			default:
				gain := pow2q[s.sf[i]] * outScale
				for w := range info.groupLen[g] {
					off := groupOff + 128*w
					for k := lo; k < hi; k++ {
						c.spec[off+k] = dequantize(s.quant[off+k]) * gain
					}
				}
			}
		}
		groupOff += 128 * info.groupLen[g]
	}
}
