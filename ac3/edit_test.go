package ac3

import (
	"os"
	"path/filepath"
	"testing"
)

// The tests here check the tools ffmpeg's encoder never uses against
// ffmpeg's decoder: they edit the bit streams it encodes so that they do,
// and compare the decodes of the edited streams.

// bitstring is the bits of a syncframe, one a byte, for tests that edit
// the bit stream.
type bitstring []byte

func toBits(b []byte) bitstring {
	s := make(bitstring, 8*len(b))
	for i := range s {
		s[i] = b[i>>3] >> (7 - i&7) & 1
	}
	return s
}

func (s bitstring) bytes() []byte {
	b := make([]byte, (len(s)+7)/8)
	for i, v := range s {
		b[i>>3] |= v << (7 - i&7)
	}
	return b
}

// replace returns s with bits in place of the bit at i.
func (s bitstring) replace(i int, bits ...byte) bitstring {
	return append(append(append(bitstring{}, s[:i]...), bits...), s[i+1:]...)
}

// blockMarks are where the fields a test edits are, in bits, in each block
// of a syncframe, and where its audio ends.
type blockMarks struct {
	nf     int
	dither [6]int // the dithflags of each block, after its blksw flags
	// phsflginu, and the phase flags, or -1, and the coupling bands.
	phsflginu, phsflg [6]int
	ncplbnd           [6]int
	end               int // the end of block 5
}

// mark decodes a syncframe, and returns where its blocks' fields are.
func mark(t testing.TB, f []byte) blockMarks {
	t.Helper()
	var m blockMarks
	d := NewDecoder()
	d.onBlock = func(blk int) {
		m.nf = d.h.nfchans
		m.dither[blk] = d.ditherAt
		m.phsflginu[blk], m.phsflg[blk], m.ncplbnd[blk] = d.phsflginuAt, d.phsflgAt, d.ncplbnd
		if blk == 5 {
			m.end = d.r.pos
		}
	}
	if _, err := d.Decode(f, make([]float32, MaxChannels*FrameLength)); err != nil {
		t.Fatal(err)
	}
	return m
}

// edit applies change to each syncframe of an AC-3 file, as a bitstring
// of it and where its fields are, and returns the file of the frames it
// returns, of their CRCs set again. A frame keeps its size: change must
// take bits it adds from the auxiliary bits after the audio.
func edit(t testing.TB, data []byte, change func(s bitstring, m blockMarks) bitstring) []byte {
	t.Helper()
	var out []byte
	for _, f := range frames(t, data) {
		s := change(toBits(f), mark(t, f))
		if len(s) != 8*len(f) {
			t.Fatalf("an edit made a frame of %d bits, of %d", len(s), 8*len(f))
		}
		g := s.bytes()
		setCRC(g)
		out = append(out, g...)
	}
	return out
}

// checkEdit decodes an edited file with ffmpeg and with the decoder, and
// checks that they agree, as the file's original decodes did, and that
// the edit made blocks use tool.
func checkEdit(t *testing.T, path string, data []byte, tool string) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	ours, d, use := decodeAll(t, data)
	if use[tool] == 0 {
		t.Fatalf("no block uses %s: %v", tool, use)
	}
	diffs, ok := compare(t, ffmpegDecode(t, path), ours, d.h.acmod, d.h.lfeon)
	t.Logf("%v; %v", diffs, use)
	if !ok {
		t.Errorf("the decodes differ: %v", diffs)
	}
}

// shortBlocks sets the blksw flags of some channels of some blocks of a
// frame, which then decode as two short transforms of their coefficients.
func shortBlocks(s bitstring, m blockMarks) bitstring {
	for blk, at := range m.dither {
		for c := range m.nf {
			if (blk+c)%3 != 0 {
				s[at-m.nf+c] = 1
			}
		}
	}
	return s
}

// TestFFmpegShortBlocks checks the short transforms against ffmpeg's: it
// sets the blksw flags of some channels of some blocks.
func TestFFmpegShortBlocks(t *testing.T) {
	haveFFmpeg(t)
	dir := t.TempDir()
	for _, e := range encodings[:3] {
		t.Run(e.name, func(t *testing.T) {
			data, err := os.ReadFile(e.encode(t, dir))
			if err != nil {
				t.Fatal(err)
			}
			short := edit(t, undither(t, data), shortBlocks)
			checkEdit(t, filepath.Join(dir, e.name+"-short.ac3"), short, "short")
		})
	}
}

// dynrngGains are the dynrngs dynamicRanges gives, in turn.
var dynrngGains = []int{0x40, 0xC5, 0x9F, 0x3F, 0xE0, 0x1A}

// dynamicRanges gives blocks 0 and 3 of a frame a dynrng, taking its bits
// from the auxiliary bits, when there are enough. The gain of block 0
// holds until block 3's. n counts the dynrngs it gave.
func dynamicRanges(n *int) func(s bitstring, m blockMarks) bitstring {
	return func(s bitstring, m blockMarks) bitstring {
		aux := len(s) - 18 - m.end // the auxiliary bits, before auxdatae
		if s[len(s)-18] != 0 {
			return s // auxdatae: the frame has data of its own there
		}
		blocks := []int{3, 0}
		if aux < 16 {
			blocks = blocks[1:]
		}
		if aux < 8 {
			return s
		}
		for _, blk := range blocks {
			// dynrnge is the bit after the dithflags: 0, as ffmpeg
			// writes no dynrng.
			at := m.dither[blk] + m.nf
			if s[at] != 0 {
				panic("a block has a dynrng")
			}
			g := dynrngGains[*n%len(dynrngGains)]
			*n++
			bits := []byte{1}
			for b := 7; b >= 0; b-- {
				bits = append(bits, byte(g>>b&1))
			}
			s = s.replace(at, bits...)
		}
		end := m.end + 8*len(blocks)
		return append(s[:end:end], s[end+8*len(blocks):]...)
	}
}

// TestFFmpegDynamicRange checks the dynamic range gains against ffmpeg's
// application of them: it gives blocks dynrngs.
func TestFFmpegDynamicRange(t *testing.T) {
	haveFFmpeg(t)
	dir := t.TempDir()
	for _, e := range encodings[:3] {
		t.Run(e.name, func(t *testing.T) {
			data, err := os.ReadFile(e.encode(t, dir))
			if err != nil {
				t.Fatal(err)
			}
			n := 0
			edited := edit(t, undither(t, data), dynamicRanges(&n))
			if n == 0 {
				t.Fatal("no frame has auxiliary bits to spare")
			}
			checkEdit(t, filepath.Join(dir, e.name+"-dynrng.ac3"), edited, "dynrng")
		})
	}
}

// phaseFlags sets phsflginu where coupling starts in a frame of a 2/0
// stream, and gives the coupling bands of each block of coordinates phase
// flags, of bits taken from the auxiliary bits, when there are enough. n
// counts the blocks it gave flags.
func phaseFlags(n *int) func(s bitstring, m blockMarks) bitstring {
	return func(s bitstring, m blockMarks) bitstring {
		need := 0
		for blk := range 6 {
			if m.phsflg[blk] >= 0 {
				need += m.ncplbnd[blk]
			}
		}
		if need == 0 || s[len(s)-18] != 0 || len(s)-18-m.end < need {
			return s
		}
		for blk := 5; blk >= 0; blk-- {
			if at := m.phsflg[blk]; at >= 0 {
				var flags []byte
				for b := range m.ncplbnd[blk] {
					flags = append(flags, byte((b+blk+*n)&1))
				}
				*n++
				// The bit at phsflg is the one after the coordinates,
				// which the flags go before.
				s = s.replace(at, append(flags, s[at])...)
			}
			if at := m.phsflginu[blk]; at >= 0 {
				s[at] = 1
			}
		}
		end := m.end + need
		return append(s[:end:end], s[end+need:]...)
	}
}

// TestFFmpegPhaseFlags checks the phase flags of a 2/0 stream's coupling
// against ffmpeg's: it gives blocks of coupling coordinates phase flags.
func TestFFmpegPhaseFlags(t *testing.T) {
	haveFFmpeg(t)
	dir := t.TempDir()
	e := encodings[7]
	if e.name != "stereo-44k-cpl" {
		t.Fatalf("encodings[7] is %s", e.name)
	}
	data, err := os.ReadFile(e.encode(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	edited := edit(t, undither(t, data), phaseFlags(&n))
	if n == 0 {
		t.Fatal("no frame has auxiliary bits to spare")
	}
	checkEdit(t, filepath.Join(dir, e.name+"-phase.ac3"), edited, "cpl-phase")
}
