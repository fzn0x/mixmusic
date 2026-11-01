package aac

// Port of Huffman/HuffmanDec.cs and Huffman/HCB.cs. SharpJaad searches the
// codebooks of codebooks.go entry by entry; this decoder turns them into
// lookup tables first.

// Codebooks of the sections, which also tell the bands they don't code.
const (
	zeroBook       = 0  // a band of zeros
	escapeBook     = 11 // the codebook that escapes values of 16 and more
	noiseBook      = 13 // a band of noise, as PNS substitutes it
	intensityBook2 = 14 // a band of intensity stereo, out of phase
	intensityBook  = 15 // a band of intensity stereo, in phase
)

// A huffman is a codebook as lookup tables. The next bits bits of the
// stream index the first table, whose entries hold the length of the
// codeword they start and what it codes, or, for the codewords longer
// than bits, where the table of their next bits starts.
type huffman struct {
	bits    int
	entries []uint32
}

// An entry holds a length in its low bits, entryLink when it links to a
// table, and from entryShift up what its codeword codes, or where the
// table it links to starts. The length of a link is the number of bits
// that index its table.
const (
	entryLen   = 0x1F
	entryLink  = 0x20
	entryShift = 6
)

// codeword is an entry of a codebook: the length of a codeword, the
// codeword, and what it codes.
type codeword struct {
	len  int
	code uint32
	sym  int
}

// newHuffman builds the tables of a prefix code, with a first table of at
// most 9 bits.
func newHuffman(words []codeword) huffman {
	longest := 0
	for _, w := range words {
		longest = max(longest, w.len)
	}
	bits := min(longest, 9)
	h := huffman{bits: bits, entries: make([]uint32, 1<<bits)}
	// The length of the longest codeword after each prefix of bits bits,
	// less bits: the size of the table that prefix links to.
	sub := make([]int, 1<<bits)
	for _, w := range words {
		if w.len > bits {
			p := w.code >> (w.len - bits)
			sub[p] = max(sub[p], w.len-bits)
		}
	}
	for p, n := range sub {
		if n > 0 {
			h.entries[p] = uint32(len(h.entries))<<entryShift | entryLink | uint32(n)
			h.entries = append(h.entries, make([]uint32, 1<<n)...)
		}
	}
	for _, w := range words {
		// A codeword fills the entries of all the bits that may follow it
		// in its table.
		e := uint32(w.sym) << entryShift
		var first, count uint32
		if w.len <= bits {
			first, count = w.code<<(bits-w.len), 1<<(bits-w.len)
			e |= uint32(w.len)
		} else {
			link := h.entries[w.code>>(w.len-bits)]
			n := uint32(w.len - bits) // the bits of the codeword the table indexes
			m := link & entryLen      // the bits that index the table
			first, count = link>>entryShift+(w.code&(1<<n-1))<<(m-n), 1<<(m-n)
			e |= n
		}
		for i := first; i < first+count; i++ {
			h.entries[i] = e
		}
	}
	return h
}

// decode reads a codeword of h and returns what it codes.
func (r *bitReader) decode(h *huffman) int {
	e := h.entries[r.peek(h.bits)]
	if e&entryLink != 0 {
		r.pos += h.bits
		e = h.entries[e>>entryShift+r.peek(int(e&entryLen))]
	}
	r.pos += int(e & entryLen)
	return int(e >> entryShift)
}

// spectralBook is a spectral codebook: its codewords code the index of a
// tuple of values, four or two, which lack their signs when the codebook
// is unsigned. Codebook 11 escapes the values of 16 and more.
type spectralBook struct {
	huffman
	dim      int
	unsigned bool
	values   [][4]int8
}

// sfBook is the codebook of scalefactors, whose codewords code the
// difference to the previous scalefactor plus 60.
var sfBook huffman

// spectralBooks are the spectral codebooks, 1 to 11.
var spectralBooks [12]spectralBook

// initBooks builds sfBook and spectralBooks.
func initBooks() {
	words := make([]codeword, 0, len(hcbSF))
	for _, e := range hcbSF {
		words = append(words, codeword{int(e[0]), uint32(e[1]), int(e[2])})
	}
	sfBook = newHuffman(words)

	for cb, book := range [...][][6]int32{1: hcb1[:], hcb2[:], hcb3[:], hcb4[:]} {
		if book == nil {
			continue
		}
		b := &spectralBooks[cb]
		words = words[:0]
		for i, e := range book {
			words = append(words, codeword{int(e[0]), uint32(e[1]), i})
			b.values = append(b.values, [4]int8{int8(e[2]), int8(e[3]), int8(e[4]), int8(e[5])})
		}
		b.huffman = newHuffman(words)
		b.dim = 4
	}
	for cb, book := range [...][][4]int32{5: hcb5[:], hcb6[:], hcb7[:], hcb8[:], hcb9[:], hcb10[:], hcb11[:]} {
		if book == nil {
			continue
		}
		b := &spectralBooks[cb]
		words = words[:0]
		for i, e := range book {
			words = append(words, codeword{int(e[0]), uint32(e[1]), i})
			b.values = append(b.values, [4]int8{int8(e[2]), int8(e[3])})
		}
		b.huffman = newHuffman(words)
		b.dim = 2
	}
	for _, cb := range []int{3, 4, 7, 8, 9, 10, 11} {
		spectralBooks[cb].unsigned = true
	}
}

// decodeSpectral reads a codeword of codebook cb, 1 to 11, and the signs
// and escapes that follow it, into q, which has room for four values.
// It reports false for an escape longer than the standard allows.
func (r *bitReader) decodeSpectral(cb int, q []int32) bool {
	b := &spectralBooks[cb]
	v := &b.values[r.decode(&b.huffman)]
	q = q[:b.dim]
	for i := range q {
		q[i] = int32(v[i])
	}
	if !b.unsigned {
		return true
	}
	for i := range q {
		if q[i] != 0 && r.flag() {
			q[i] = -q[i]
		}
	}
	if cb == escapeBook {
		for i := range q {
			if q[i] == 16 || q[i] == -16 {
				e, ok := r.escape()
				if !ok {
					return false
				}
				if q[i] < 0 {
					e = -e
				}
				q[i] = e
			}
		}
	}
	return true
}

// escape reads the escape of a value of 16 or more: n ones and a zero,
// then n+4 bits, for 2^(n+4) plus those bits. The standard allows n up to
// 8, for values up to 8191, and escape reports false for more.
func (r *bitReader) escape() (int32, bool) {
	n := 4
	for r.flag() {
		if n++; n > 12 {
			return 0, false
		}
	}
	return int32(1<<n | r.read(n)), true
}
