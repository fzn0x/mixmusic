package aac

import "encoding/binary"

// bitReader reads a frame, or a config, most significant bit first.
// Reading past the end reads zeros and makes over true, which the decoder
// checks in its loops and after what it reads, so that a truncated frame
// is an error, not a panic.
type bitReader struct {
	data []byte
	pos  int // in bits
}

func (r *bitReader) init(data []byte) {
	r.data = data
	r.pos = 0
}

// over reports whether the reader went past the end of its data.
func (r *bitReader) over() bool { return r.pos > 8*len(r.data) }

// left is the number of bits left to read.
func (r *bitReader) left() int { return 8*len(r.data) - r.pos }

// peek returns the next n bits, at most 25, without reading them.
func (r *bitReader) peek(n int) uint32 {
	i := r.pos >> 3
	var w uint32
	if i+4 <= len(r.data) {
		w = binary.BigEndian.Uint32(r.data[i:])
	} else {
		for j := i; j < i+4; j++ {
			w <<= 8
			if j < len(r.data) {
				w |= uint32(r.data[j])
			}
		}
	}
	return w << (r.pos & 7) >> (32 - n)
}

// read reads n bits, at most 25.
func (r *bitReader) read(n int) int {
	v := r.peek(n)
	r.pos += n
	return int(v)
}

// flag reads a bit.
func (r *bitReader) flag() bool { return r.read(1) != 0 }

// skip skips n bits.
func (r *bitReader) skip(n int) { r.pos += n }

// align skips to the next byte.
func (r *bitReader) align() { r.pos = (r.pos + 7) &^ 7 }
