// Package rangedec is the range decoder of Opus (RFC 6716, section 4.1),
// which SILK and CELT read their symbols with: a port of entdec.c and
// entcode.c of libopus.
//
// Copyright (c) 2001-2011 Timothy B. Terriberry. The libopus code it is
// ported from is under the BSD license of opus/LICENSE.libopus.
package rangedec

import "math/bits"

const (
	symBits   = 8
	codeBits  = 32
	symMax    = 1<<symBits - 1
	codeTop   = uint32(1) << (codeBits - 1)
	codeBot   = codeTop >> symBits
	codeExtra = (codeBits-2)%symBits + 1
	// windowSize is the size of the window of raw bits read from the end.
	windowSize = 32
	// uintBits is the number of bits of a uniform integer coded at once.
	uintBits = 8
	// BitRes is the resolution of TellFrac: 1/8 bit.
	BitRes = 3
)

// Decoder reads the symbols of one Opus frame: range coded ones from its
// start and raw bits from its end.
type Decoder struct {
	buf        []byte
	storage    uint32
	endOffs    uint32
	endWindow  uint32
	nendBits   int
	nbitsTotal int
	offs       uint32
	rng        uint32
	val        uint32
	ext        uint32
	rem        int
	err        bool
}

// Ilog returns the number of bits needed to write x, 0 for 0: EC_ILOG.
func Ilog(x uint32) int { return bits.Len32(x) }

// Init starts decoding buf.
func (d *Decoder) Init(buf []byte) {
	d.buf = buf
	d.storage = uint32(len(buf))
	d.endOffs = 0
	d.endWindow = 0
	d.nendBits = 0
	// The offset ec_tell() subtracts partial bits from: after the
	// normalization below, the same as in the encoder.
	d.nbitsTotal = codeBits + 1 - ((codeBits-codeExtra)/symBits)*symBits
	d.offs = 0
	d.rng = 1 << codeExtra
	d.rem = d.readByte()
	d.val = d.rng - 1 - uint32(d.rem>>(symBits-codeExtra))
	d.err = false
	d.normalize()
}

func (d *Decoder) readByte() int {
	if d.offs < d.storage {
		b := d.buf[d.offs]
		d.offs++
		return int(b)
	}
	return 0
}

func (d *Decoder) readByteFromEnd() int {
	if d.endOffs < d.storage {
		d.endOffs++
		return int(d.buf[d.storage-d.endOffs])
	}
	return 0
}

// normalize rescales a range that became too small, reading more input.
func (d *Decoder) normalize() {
	for d.rng <= codeBot {
		d.nbitsTotal += symBits
		d.rng <<= symBits
		// The rest of the last symbol, then the next one.
		sym := d.rem
		d.rem = d.readByte()
		sym = (sym<<symBits | d.rem) >> (symBits - codeExtra)
		d.val = ((d.val << symBits) + (symMax &^ uint32(sym))) & (codeTop - 1)
	}
}

// Decode returns the cumulative frequency of the next symbol, out of a total
// of ft; Update must follow with the symbol's range.
func (d *Decoder) Decode(ft uint32) uint32 {
	d.ext = d.rng / ft
	s := d.val / d.ext
	return ft - min(s+1, ft)
}

// DecodeBin is Decode for a total of 1<<bits.
func (d *Decoder) DecodeBin(b uint) uint32 {
	d.ext = d.rng >> b
	s := d.val / d.ext
	return 1<<b - min(s+1, 1<<b)
}

// Update takes in the symbol of cumulative frequencies [fl, fh) out of ft
// that Decode found.
func (d *Decoder) Update(fl, fh, ft uint32) {
	s := d.ext * (ft - fh)
	d.val -= s
	if fl > 0 {
		d.rng = d.ext * (fh - fl)
	} else {
		d.rng -= s
	}
	d.normalize()
}

// BitLogp decodes a bit whose probability of being 1 is 1/(1<<logp).
func (d *Decoder) BitLogp(logp uint) bool {
	r := d.rng
	v := d.val
	s := r >> logp
	ret := v < s
	if !ret {
		d.val = v - s
	}
	if ret {
		d.rng = s
	} else {
		d.rng = r - s
	}
	d.normalize()
	return ret
}

// ICDF decodes a symbol of an inverse cumulative distribution of 8-bit
// values out of 1<<ftb.
func (d *Decoder) ICDF(icdf []uint8, ftb uint) int {
	s := d.rng
	v := d.val
	r := s >> ftb
	ret := -1
	var t uint32
	for {
		t = s
		ret++
		s = r * uint32(icdf[ret])
		if v >= s {
			break
		}
	}
	d.val = v - s
	d.rng = t - s
	d.normalize()
	return ret
}

// ICDF16 is ICDF for a distribution of 16-bit values.
func (d *Decoder) ICDF16(icdf []uint16, ftb uint) int {
	s := d.rng
	v := d.val
	r := s >> ftb
	ret := -1
	var t uint32
	for {
		t = s
		ret++
		s = r * uint32(icdf[ret])
		if v >= s {
			break
		}
	}
	d.val = v - s
	d.rng = t - s
	d.normalize()
	return ret
}

// Uint decodes an integer uniformly distributed in [0, ft), ft > 1.
func (d *Decoder) Uint(ft uint32) uint32 {
	ft--
	ftb := Ilog(ft)
	if ftb > uintBits {
		ftb -= uintBits
		f := ft>>ftb + 1
		s := d.Decode(f)
		d.Update(s, s+1, f)
		t := s<<ftb | d.Bits(uint(ftb))
		if t <= ft {
			return t
		}
		d.err = true
		return ft
	}
	ft++
	s := d.Decode(ft)
	d.Update(s, s+1, ft)
	return s
}

// Bits reads n raw bits, n <= 25, from the end of the frame.
func (d *Decoder) Bits(n uint) uint32 {
	window := d.endWindow
	available := d.nendBits
	if uint(available) < n {
		for {
			window |= uint32(d.readByteFromEnd()) << available
			available += symBits
			if available > windowSize-symBits {
				break
			}
		}
	}
	ret := window & (1<<n - 1)
	window >>= n
	available -= int(n)
	d.endWindow = window
	d.nendBits = available
	d.nbitsTotal += int(n)
	return ret
}

// Tell returns the number of whole bits read so far, rounded up.
func (d *Decoder) Tell() int { return d.nbitsTotal - Ilog(d.rng) }

// TellFrac returns the number of bits read so far, in 1/8 bits, rounded up.
func (d *Decoder) TellFrac() uint32 {
	correction := [8]uint32{35733, 38967, 42495, 46340, 50535, 55109, 60097, 65535}
	nbits := uint32(d.nbitsTotal) << BitRes
	l := Ilog(d.rng)
	r := d.rng >> (l - 16)
	b := (r >> 12) - 8
	if r > correction[b] {
		b++
	}
	return nbits - uint32(l<<3) - b
}

// Err reports whether the frame was found corrupt.
func (d *Decoder) Err() bool { return d.err }

// Range returns the current range, which after a frame is its final range
// of the encoder: a check that both read the same bits.
func (d *Decoder) Range() uint32 { return d.rng }

// RangeBytes returns how many bytes the range coder read.
func (d *Decoder) RangeBytes() uint32 { return d.offs }

// Storage returns the size of the frame.
func (d *Decoder) Storage() uint32 { return d.storage }

// Shrink takes n bytes off the end of the frame, which the decoder of a
// hybrid frame keeps for a redundant CELT frame.
func (d *Decoder) Shrink(n uint32) { d.storage -= n }

// SetTell makes Tell report n bits read, as CELT does for a silent frame,
// which pretends it read all of them.
func (d *Decoder) SetTell(n int) { d.nbitsTotal += n - d.Tell() }
