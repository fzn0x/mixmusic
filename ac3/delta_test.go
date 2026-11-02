package ac3

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// segment is a segment of a delta bit allocation, as the bit stream codes
// it.
type segment struct{ offset, length, ba int }

// deltaPlan is the delta bit allocation TestFFmpegDelta gives the first
// full-bandwidth channel, and the coupling channel, whose bands are
// counted from its first.
var deltaPlan = [2][]segment{
	{{12, 3, 6}, {4, 2, 3}, {6, 6, 7}},
	{{0, 1, 6}, {1, 1, 3}, {1, 2, 7}},
}

// cplPlan returns the coupling channel's delta, of deltaPlan, for a
// coupling channel whose bands start at band: what of it ends by band 50.
func cplPlan(band int) []segment {
	plan := deltaPlan[1]
	for i, end := 0, band; i < len(plan); i++ {
		end += plan[i].offset + plan[i].length
		if end > 50 {
			return plan[:i]
		}
	}
	return plan
}

// blockCopy is what TestFFmpegDelta keeps of a block it rewrites: where
// its fields are, and its mantissas in the order of the bit stream, with
// the bit allocations its delta makes.
type blockCopy struct {
	start, delta, mant int // the block, its deltbaie and its mantissas, in bits
	cplinu             bool
	nf                 int
	order              []int // the states of the exponent sets, in the order of their mantissas
	sets               [maxStates]struct {
		start, end int
		value      [maxBins]float32 // the mantissa of each bin of bits
		bap        [maxBins]uint8   // the bit allocation with the delta
	}
}

// reencode rewrites the blocks of a syncframe of no dither nor dynamic
// range with the delta bit allocation of deltaPlan, which block 0 gives
// and the others reuse, and their mantissas quantized again as the bit
// allocation then asks. It returns nil when the frame lacks the room.
func reencode(t testing.TB, f []byte) bitstring {
	t.Helper()
	var blocks [6]blockCopy
	var cpl []segment // the delta of the coupling channel, of block 0
	cplBand := -1     // the first band of the coupling channel, of block 0
	d := NewDecoder()
	d.onBlock = func(blk int) {
		b := &blocks[blk]
		nf := d.h.nfchans
		b.start, b.delta, b.mant = d.ditherAt-nf, d.deltaAt, d.mantAt
		b.cplinu, b.nf = d.cplinu, nf
		if d.dynrng != 1 {
			t.Fatal("a block has a dynrng")
		}
		got := false
		for c := range nf {
			b.order = append(b.order, c)
			if d.cplinu && d.chincpl[c] && !got {
				b.order = append(b.order, cplIndex)
				got = true
			}
		}
		if d.h.lfeon {
			b.order = append(b.order, lfeIndex)
		}
		// The coefficients of the rematrixed bands, back to what the bit
		// stream holds.
		if d.h.acmod == ModeStereo {
			l, r := &d.ch[0], &d.ch[1]
			nb := d.rematBands()
			for i := range nb {
				if !d.rematflg[i] {
					continue
				}
				hi := rematBounds[i+1]
				if d.cplinu && i == nb-1 {
					hi = 37 + 12*d.cplbegf
				}
				for bin := rematBounds[i]; bin < min(hi, l.end, r.end); bin++ {
					a, c := l.coef[bin], r.coef[bin]
					l.coef[bin], r.coef[bin] = (a+c)/2, (a-c)/2
				}
			}
		}
		for _, s := range b.order {
			ch := d.ch[s]
			set := &b.sets[s]
			set.start, set.end = ch.start, ch.end
			for bin := ch.start; bin < ch.end; bin++ {
				if ch.bap[bin] != 0 {
					set.value[bin] = ch.coef[bin] / expScale[ch.exp[bin]]
				}
			}
			var plan []segment
			switch s {
			case 0:
				plan = deltaPlan[0]
			case cplIndex:
				band := int(maskTab[ch.start])
				if cplBand < 0 {
					cplBand, cpl = band, cplPlan(band)
				}
				if band != cplBand {
					cpl = nil // the coupling moved, as its delta can't
				}
				plan = cpl
			}
			ch.delta = plan != nil
			ch.deltnseg = len(plan) - 1
			for i, g := range plan {
				ch.deltoffst[i], ch.deltlen[i], ch.deltba[i] = g.offset, g.length, g.ba
			}
			if err := d.allocate(&ch, false); err != nil {
				t.Fatal(err)
			}
			set.bap = ch.bap
		}
	}
	if _, err := d.Decode(f, make([]float32, MaxChannels*FrameLength)); err != nil {
		t.Fatal(err)
	}

	in := toBits(f)
	out := append(bitstring{}, in[:blocks[0].start]...)
	put := func(v, n int) {
		for b := n - 1; b >= 0; b-- {
			out = append(out, byte(v>>b&1))
		}
	}
	segments := func(plan []segment) {
		put(len(plan)-1, 3)
		for _, g := range plan {
			put(g.offset, 5)
			put(g.length, 4)
			put(g.ba, 3)
		}
	}
	for blk := range blocks {
		b := &blocks[blk]
		if in[b.delta] != 0 {
			t.Fatal("ffmpeg wrote a delta bit allocation")
		}
		if b.cplinu != blocks[0].cplinu || (b.cplinu && len(cpl) == 0) {
			return nil // a coupling channel's delta can't be reused where coupling starts or moves
		}
		out = append(out, in[b.start:b.delta]...)
		if blk > 0 {
			// deltbaie stays 0: the blocks after the first reuse its delta.
			out = append(out, in[b.delta:b.mant]...)
			out = append(out, writeMantissas(b)...)
			continue
		}
		put(1, 1) // deltbaie
		if b.cplinu {
			put(deltaNew, 2)
		}
		for c := range b.nf {
			if c == 0 {
				put(deltaNew, 2)
			} else {
				put(deltaNone, 2)
			}
		}
		if b.cplinu {
			segments(cpl)
		}
		segments(deltaPlan[0])
		out = append(out, in[b.delta+1:b.mant]...)
		out = append(out, writeMantissas(b)...)
	}
	room := len(in) - 18
	if len(out) > room {
		return nil
	}
	for len(out) < room {
		out = append(out, 0)
	}
	return append(out, in[room:]...)
}

// writeMantissas writes the mantissas of a block, each quantized as its
// bit allocation asks, with the grouped codes of the 3-, 5- and 11-level
// quantizers at the place of the first mantissa of their group.
func writeMantissas(b *blockCopy) bitstring {
	type field struct{ v, n int }
	var fields []field
	var open, count [16]int // the field of each bap's open group, and its members so far
	members := [16]int{1: 3, 2: 3, 4: 2}
	levels := [16]int{1: 3, 2: 5, 3: 7, 4: 11, 5: 15}
	widths := [16]int{1: 5, 2: 7, 4: 7}
	// nearest is the code of the level of an n-level quantizer nearest v:
	// the levels are (2k + 1 - n) / n.
	nearest := func(v float32, n int) int {
		code := int(math.Round((float64(v)*float64(n) + float64(n) - 1) / 2))
		return min(n-1, max(0, code))
	}
	for _, s := range b.order {
		set := &b.sets[s]
		for bin := set.start; bin < set.end; bin++ {
			v := set.value[bin]
			switch bap := int(set.bap[bin]); bap {
			case 0:
			case 1, 2, 4:
				code := nearest(v, levels[bap])
				if count[bap] == 0 {
					open[bap] = len(fields)
					fields = append(fields, field{0, widths[bap]})
				}
				fields[open[bap]].v = fields[open[bap]].v*levels[bap] + code
				count[bap] = (count[bap] + 1) % members[bap]
			case 3, 5:
				fields = append(fields, field{nearest(v, levels[bap]), bap + 0})
				if bap == 3 {
					fields[len(fields)-1].n = 3
				} else {
					fields[len(fields)-1].n = 4
				}
			default:
				bits := mantissaBits[bap]
				q := int(math.Round(float64(v) * float64(int(1)<<(bits-1))))
				q = min(1<<(bits-1)-1, max(-1<<(bits-1), q))
				fields = append(fields, field{q & (1<<bits - 1), bits})
			}
		}
	}
	// The last groups, filled with mantissas of 0.
	for _, bap := range []int{1, 2, 4} {
		for count[bap] != 0 {
			fields[open[bap]].v = fields[open[bap]].v*levels[bap] + levels[bap]/2
			count[bap] = (count[bap] + 1) % members[bap]
		}
	}
	var s bitstring
	for _, f := range fields {
		for b := f.n - 1; b >= 0; b-- {
			s = append(s, byte(f.v>>b&1))
		}
	}
	return s
}

// TestFFmpegDelta checks the delta bit allocation against ffmpeg's, whose
// encoder never writes one: it gives each block of streams it encodes the
// delta of deltaPlan, which raises and lowers the masking curve, and
// writes their mantissas again, quantized as the bit allocation then
// asks. Frames without the room for it stay as they were.
func TestFFmpegDelta(t *testing.T) {
	haveFFmpeg(t)
	dir := t.TempDir()
	for _, e := range []encoding{encodings[0], encodings[1], encodings[6], encodings[9]} {
		t.Run(e.name, func(t *testing.T) {
			data, err := os.ReadFile(e.encode(t, dir))
			if err != nil {
				t.Fatal(err)
			}
			n := 0
			edited := edit(t, undither(t, data), func(s bitstring, m blockMarks) bitstring {
				if s[len(s)-18] != 0 {
					return s
				}
				if r := reencode(t, s.bytes()); r != nil {
					n++
					return r
				}
				return s
			})
			t.Logf("%d frames of %d have a delta", n, len(frames(t, data)))
			if n == 0 {
				t.Fatal("no frame has room for the delta")
			}
			checkEdit(t, filepath.Join(dir, e.name+"-delta.ac3"), edited, "delta")
		})
	}
}
