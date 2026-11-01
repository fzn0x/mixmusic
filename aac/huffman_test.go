package aac

import (
	"math/big"
	"slices"
	"testing"
)

// codebooks returns the entries of the spectral codebooks, by codebook,
// and of the scalefactor codebook, as rows of the length, the codeword
// and the values.
func codebooks() map[int][][]int32 {
	books := map[int][][]int32{}
	for cb, book := range [...][][6]int32{1: hcb1[:], hcb2[:], hcb3[:], hcb4[:]} {
		for _, e := range book {
			books[cb] = append(books[cb], e[:])
		}
	}
	for cb, book := range [...][][4]int32{5: hcb5[:], hcb6[:], hcb7[:], hcb8[:], hcb9[:], hcb10[:], hcb11[:]} {
		for _, e := range book {
			books[cb] = append(books[cb], e[:])
		}
	}
	for _, e := range hcbSF {
		books[0] = append(books[0], e[:])
	}
	return books
}

// TestCodebooks checks that each codebook is a complete prefix code, which
// codes each tuple of its values once, and that the decoder reads each
// codeword back, whatever bits follow it.
func TestCodebooks(t *testing.T) {
	initTables()
	// The values of the tuples of each codebook: their range, and whether
	// they are signed.
	ranges := map[int][2]int{
		1: {-1, 1}, 2: {-1, 1}, 3: {0, 2}, 4: {0, 2},
		5: {-4, 4}, 6: {-4, 4}, 7: {0, 7}, 8: {0, 7},
		9: {0, 12}, 10: {0, 12}, 11: {0, 16}, 0: {0, 120},
	}
	for cb, book := range codebooks() {
		dim := len(book[0]) - 2
		lo, hi := ranges[cb][0], ranges[cb][1]
		size := 1
		for range dim {
			size *= hi - lo + 1
		}
		if len(book) != size {
			t.Errorf("codebook %d: %d codewords, want %d", cb, len(book), size)
		}
		// Kraft's sum of a complete prefix code is 1.
		kraft := new(big.Rat)
		seen := map[[4]int32]bool{}
		for _, e := range book {
			kraft.Add(kraft, big.NewRat(1, 1<<e[0]))
			var tuple [4]int32
			copy(tuple[:], e[2:])
			for _, v := range e[2:] {
				if v < int32(lo) || v > int32(hi) {
					t.Errorf("codebook %d: value %d out of [%d, %d]", cb, v, lo, hi)
				}
			}
			if seen[tuple] {
				t.Errorf("codebook %d: %v coded twice", cb, tuple)
			}
			seen[tuple] = true
		}
		if kraft.Cmp(big.NewRat(1, 1)) != 0 {
			t.Errorf("codebook %d: Kraft sum %v", cb, kraft)
		}

		// Each codeword decodes, followed by ones or by zeros.
		for i, e := range book {
			for _, tail := range []uint32{0, 0xFFFFFF} {
				var w bitWriter
				w.write(uint32(e[1]), int(e[0]))
				w.write(tail, 24)
				var r bitReader
				r.init(w.data)
				var got int
				if cb == 0 {
					got = r.decode(&sfBook)
					if got != int(e[2]) {
						t.Fatalf("scalefactor codeword %d decodes as %d, want %d", i, got, e[2])
					}
				} else {
					b := &spectralBooks[cb]
					got = r.decode(&b.huffman)
					var want []int8
					for _, v := range e[2:] {
						want = append(want, int8(v))
					}
					if b.dim != dim || !slices.Equal(b.values[got][:dim], want) {
						t.Fatalf("codebook %d: codeword %d decodes as %v, want %v", cb, i, b.values[got][:b.dim], want)
					}
				}
				if r.pos != int(e[0]) {
					t.Fatalf("codebook %d: codeword %d of %d bits read as %d", cb, i, e[0], r.pos)
				}
			}
		}
	}
}
