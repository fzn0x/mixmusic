package ac3

// bitReader reads the bits of a syncframe, most significant first. A read
// past the end reads zeros, and over tells it happened.
type bitReader struct {
	b   []byte
	pos int // in bits
}

func (r *bitReader) init(b []byte) {
	r.b = b
	r.pos = 0
}

// read reads n bits, at most 24, as an unsigned number.
func (r *bitReader) read(n int) int {
	if n == 0 {
		return 0
	}
	i := r.pos >> 3
	var w uint32
	if i+4 <= len(r.b) {
		w = uint32(r.b[i])<<24 | uint32(r.b[i+1])<<16 | uint32(r.b[i+2])<<8 | uint32(r.b[i+3])
	} else {
		for k := range 4 {
			w <<= 8
			if i+k < len(r.b) {
				w |= uint32(r.b[i+k])
			}
		}
	}
	w <<= uint(r.pos & 7)
	r.pos += n
	return int(w >> uint(32-n))
}

// signed reads n bits as a two's complement number.
func (r *bitReader) signed(n int) int {
	v := r.read(n)
	return v << (64 - n) >> (64 - n)
}

func (r *bitReader) flag() bool { return r.read(1) != 0 }

// skip skips n bits.
func (r *bitReader) skip(n int) { r.pos += n }

// over reports whether the reads went past the end.
func (r *bitReader) over() bool { return r.pos > 8*len(r.b) }

// left returns the bits left to read.
func (r *bitReader) left() int { return 8*len(r.b) - r.pos }
