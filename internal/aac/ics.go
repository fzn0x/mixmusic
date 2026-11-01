package aac

// Port of Syntax/ICSInfo.cs and Syntax/ICStream.cs: the individual channel
// stream, which codes the spectrum of a channel.

import (
	"errors"
	"fmt"
)

// Window sequences.
const (
	onlyLongSequence   = 0
	longStartSequence  = 1
	eightShortSequence = 2
	longStopSequence   = 3
)

// maxBands bounds the bands of a channel in a frame: 8 groups of up to 15
// short bands, or up to 51 long ones.
const maxBands = 120

// icsInfo is ics_info: the windows of a channel in a frame.
type icsInfo struct {
	seq      int // window sequence
	shape    int // window shape: 0 for sine, 1 for KBD
	maxSFB   int // the bands coded of each window
	windows  int // 1, or 8 for short windows
	groups   int // the groups of short windows that share scalefactors
	groupLen [8]int
	// swb are the offsets of the bands of a window, which end at 1024 or
	// 128.
	swb []uint16
}

func (info *icsInfo) parse(r *bitReader, t *rateTables) error {
	r.skip(1) // ics_reserved_bit
	info.seq = r.read(2)
	info.shape = r.read(1)
	info.groups = 1
	info.groupLen[0] = 1
	if info.seq == eightShortSequence {
		info.maxSFB = r.read(4)
		for range 7 {
			if r.flag() { // scale_factor_grouping
				info.groupLen[info.groups-1]++
			} else {
				info.groupLen[info.groups] = 1
				info.groups++
			}
		}
		info.windows = 8
		info.swb = t.short
	} else {
		info.maxSFB = r.read(6)
		info.windows = 1
		info.swb = t.long
		if r.flag() {
			// predictor_data_present: the prediction of AAC Main, or LTP.
			return fmt.Errorf("%w: prediction in an AAC-LC stream", ErrFormat)
		}
	}
	if info.maxSFB > len(info.swb)-1 {
		return fmt.Errorf("%w: %d bands of %d", ErrFormat, info.maxSFB, len(info.swb)-1)
	}
	return nil
}

// pulse is pulse_data: amplitudes added to quantized values of a long
// window.
type pulse struct {
	present bool
	count   int
	offset  [4]int // the index of each pulse in the spectrum
	amp     [4]int32
}

func (p *pulse) parse(r *bitReader, info *icsInfo) error {
	p.count = r.read(2) + 1
	start := r.read(6)
	if start >= len(info.swb)-1 {
		return fmt.Errorf("%w: pulses from band %d of %d", ErrFormat, start, len(info.swb)-1)
	}
	k := int(info.swb[start])
	for i := range p.count {
		k += r.read(5)
		if k >= FrameLength {
			return fmt.Errorf("%w: pulse at %d", ErrFormat, k)
		}
		p.offset[i] = k
		p.amp[i] = int32(r.read(4))
	}
	return nil
}

// ics is individual_channel_stream, what it codes of a channel's spectrum.
type ics struct {
	info       icsInfo
	globalGain int
	// bandType is the codebook of each band of each group, at
	// g*maxSFB+sfb.
	bandType [maxBands]uint8
	// sf is the scalefactor of each band, or its noise energy, or its
	// intensity position, as its codebook says.
	sf    [maxBands]int
	pulse pulse
	tns   tns
	// quant is the quantized spectrum: of one window, or of 8 in a row,
	// 128 values each.
	quant [FrameLength]int32
}

// parse reads an individual_channel_stream, whose ics_info the caller
// read when the channel shares it.
func (s *ics) parse(r *bitReader, t *rateTables, commonWindow bool) error {
	s.globalGain = r.read(8)
	if !commonWindow {
		if err := s.info.parse(r, t); err != nil {
			return err
		}
	}
	if err := s.parseSections(r); err != nil {
		return err
	}
	if err := s.parseScalefactors(r); err != nil {
		return err
	}
	if s.pulse.present = r.flag(); s.pulse.present {
		if s.info.seq == eightShortSequence {
			return fmt.Errorf("%w: pulse data in short windows", ErrFormat)
		}
		if err := s.pulse.parse(r, &s.info); err != nil {
			return err
		}
	}
	if s.tns.present = r.flag(); s.tns.present {
		s.tns.parse(r, &s.info)
	}
	if r.flag() {
		return fmt.Errorf("aac: gain control, of AAC SSR: %w", errors.ErrUnsupported)
	}
	return s.parseSpectralData(r)
}

// parseSections reads section_data: the codebook of each band.
func (s *ics) parseSections(r *bitReader) error {
	bits := 5
	if s.info.seq == eightShortSequence {
		bits = 3
	}
	esc := 1<<bits - 1
	maxSFB := s.info.maxSFB
	for g := range s.info.groups {
		band := s.bandType[g*maxSFB : (g+1)*maxSFB]
		for k := 0; k < maxSFB; {
			cb := r.read(4)
			if cb == 12 {
				return fmt.Errorf("%w: reserved codebook 12", ErrFormat)
			}
			end := k
			for {
				incr := r.read(bits)
				end += incr
				if incr != esc || end > maxSFB {
					break
				}
			}
			if end > maxSFB {
				return fmt.Errorf("%w: section past band %d", ErrFormat, maxSFB)
			}
			// Sections of no band are read again and again until the bits
			// run out.
			if r.over() {
				return fmt.Errorf("%w: section data past the frame", ErrFormat)
			}
			for ; k < end; k++ {
				band[k] = uint8(cb)
			}
		}
	}
	return nil
}

// parseScalefactors reads scale_factor_data: each coded band's
// scalefactor, noise energy or intensity position, which code the
// difference to the previous of their kind.
func (s *ics) parseScalefactors(r *bitReader) error {
	sf, noise, position := s.globalGain, s.globalGain-90, 0
	noisePCM := true
	n := s.info.groups * s.info.maxSFB
	for i, cb := range s.bandType[:n] {
		switch cb {
		case zeroBook:
			s.sf[i] = 0
		case intensityBook, intensityBook2:
			position += r.decode(&sfBook) - 60
			s.sf[i] = min(max(position, -155), 100)
		case noiseBook:
			// The first noise energy is coded in 9 bits, the others as
			// scalefactors.
			if noisePCM {
				noisePCM = false
				noise += r.read(9) - 256
			} else {
				noise += r.decode(&sfBook) - 60
			}
			s.sf[i] = min(max(noise, -100), 155)
		default:
			sf += r.decode(&sfBook) - 60
			if sf < 0 || sf > 255 {
				return fmt.Errorf("%w: scalefactor %d", ErrFormat, sf)
			}
			s.sf[i] = sf
		}
	}
	return nil
}

// parseSpectralData reads spectral_data into quant: the quantized values
// of the bands the sections code, band by band, and in each band window
// by window of its group.
func (s *ics) parseSpectralData(r *bitReader) error {
	clear(s.quant[:])
	info := &s.info
	var q [4]int32
	groupOff := 0
	for g := range info.groups {
		for sfb := range info.maxSFB {
			cb := int(s.bandType[g*info.maxSFB+sfb])
			if cb == zeroBook || cb >= noiseBook {
				continue
			}
			lo, hi := int(info.swb[sfb]), int(info.swb[sfb+1])
			dim := spectralBooks[cb].dim
			for w := range info.groupLen[g] {
				off := groupOff + 128*w
				for k := lo; k < hi; k += dim {
					if !r.decodeSpectral(cb, q[:]) {
						return fmt.Errorf("%w: escape too long", ErrFormat)
					}
					copy(s.quant[off+k:off+k+dim], q[:dim])
				}
			}
			if r.over() {
				return fmt.Errorf("%w: spectral data past the frame", ErrFormat)
			}
		}
		groupOff += 128 * info.groupLen[g]
	}
	return nil
}
