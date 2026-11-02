package ac3

import (
	"bytes"
	"errors"
	"math"
	"path/filepath"
	"testing"
)

// TestFrameSizes checks the sizes of syncframes against Table 5.18, and
// the 5/8 of them crc1 covers against Table 7.34, at 44.1 kHz, whose sizes
// aren't multiples of the rate.
func TestFrameSizes(t *testing.T) {
	words := [38]int{69, 70, 87, 88, 104, 105, 121, 122, 139, 140, 174, 175, 208, 209, 243, 244, 278, 279,
		348, 349, 417, 418, 487, 488, 557, 558, 696, 697, 835, 836, 975, 976, 1114, 1115, 1253, 1254, 1393, 1394}
	five8 := [38]int{42, 43, 53, 55, 65, 65, 75, 76, 86, 87, 108, 108, 130, 130, 151, 152, 173, 173,
		217, 217, 260, 261, 303, 305, 347, 348, 435, 435, 521, 522, 608, 610, 696, 696, 782, 783, 870, 871}
	for code := range 38 {
		w := frameWords(1, code)
		if w != words[code] {
			t.Errorf("frmsizecod %d at 44.1 kHz: %d words, want %d", code, w, words[code])
		}
		if f := w>>1 + w>>3; f != five8[code] {
			t.Errorf("frmsizecod %d at 44.1 kHz: 5/8 of %d words, want %d", code, f, five8[code])
		}
		if w := frameWords(0, code); w != 2*bitrates[code>>1] {
			t.Errorf("frmsizecod %d at 48 kHz: %d words", code, w)
		}
		if w := frameWords(2, code); w != 3*bitrates[code>>1] {
			t.Errorf("frmsizecod %d at 32 kHz: %d words", code, w)
		}
	}
}

// TestDynamicRange checks the gains of dynrng against Table 7.29 and the
// text after it: the 3 bits X gain (X+1) * 6.02 dB, the 5 bits Y 0 to
// -6.02 dB.
func TestDynamicRange(t *testing.T) {
	for _, c := range []struct {
		code int
		db   float64
	}{
		{0x00, 0}, {0x60, 18.06}, {0x40, 12.04}, {0x20, 6.02}, {0xE0, -6.02}, {0x80, -24.08},
		{0x1F, 6.02 - 0.14}, {0xFF, -0.14}, {0x7F, 24.08 - 0.14},
	} {
		if db := 20 * math.Log10(float64(dynamicRange(c.code))); math.Abs(db-c.db) > 0.01 {
			t.Errorf("dynrng %#02x: %.2f dB, want %.2f dB", c.code, db, c.db)
		}
	}
}

// TestDownmix checks the downmixes of streams of each audio coding mode
// against the equations of section 7.8.2.
func TestDownmix(t *testing.T) {
	const c, s = 0.5946035575013605, 0.5 // the middle mix levels
	h3 := math.Sqrt2 / 2
	for _, x := range []struct {
		name string
		h    header
		// The gains of L, C and Ls in the left output, unscaled.
		l, c, ls float64
	}{
		{"1+1", header{acmod: ModeDualMono}, 1, 0, 0},
		{"1/0", header{acmod: ModeMono}, 0, h3, 0},
		{"2/0", header{acmod: ModeStereo}, 1, 0, 0},
		{"3/0", header{acmod: Mode30, cmixlev: 1}, 1, c, 0},
		{"2/1", header{acmod: Mode21, surmixlev: 1}, 1, 0, s},
		{"3/1", header{acmod: Mode31, cmixlev: 1, surmixlev: 1}, 1, c, s},
		{"2/2", header{acmod: Mode22, surmixlev: 0}, 1, 0, h3},
		{"3/2", header{acmod: Mode32, cmixlev: 0, surmixlev: 2}, 1, h3, 0},
		{"3/2 reserved", header{acmod: Mode32, cmixlev: 3, surmixlev: 3}, 1, c, s},
		{"3/2 LoRo", header{acmod: Mode32, loroCmixlev: 2, loroSurmixlev: 1}, 1, 1, 0.8408964152537145},
	} {
		t.Run(x.name, func(t *testing.T) {
			h := x.h
			if h.loroCmixlev == 0 && h.loroSurmixlev == 0 {
				h.loroCmixlev, h.loroSurmixlev = -1, -1
			}
			m := downmix(&h)
			// The scale is that of the sum of the gains of the stream's
			// channels, in which a single surround channel mixes at
			// 0.707 slev.
			sum := x.l + x.c + x.ls
			if h.acmod == Mode21 || h.acmod == Mode31 {
				sum = x.l + x.c + x.ls*h3
			}
			scale := 1 / max(1, sum)
			want := [2][MaxChannels]float64{
				{Left: x.l, Center: x.c, LeftSurround: x.ls},
				{Right: x.l, Center: x.c, RightSurround: x.ls},
			}
			for o := range 2 {
				for ch := range MaxChannels {
					if g, w := float64(m[o][ch]), want[o][ch]*scale; math.Abs(g-w) > 1e-6 {
						t.Errorf("output %d, channel %d: %.4f, want %.4f", o, ch, g, w)
					}
				}
			}
		})
	}
}

// TestCRC checks that a syncframe of any bit flipped fails to decode, but
// for the syncword's, which makes it no syncframe.
func TestCRC(t *testing.T) {
	f := frames(t, readFile(t, filepath.Join("testdata", "5.1-48k-cpl.ac3")))[1]
	pcm := make([]float32, MaxChannels*FrameLength)
	d := NewDecoder()
	if _, err := d.Decode(f, pcm); err != nil {
		t.Fatal(err)
	}
	for bit := 16; bit < 8*len(f); bit += 37 {
		g := bytes.Clone(f)
		g[bit>>3] ^= 0x80 >> (bit & 7)
		if _, err := d.Decode(g, pcm); !errors.Is(err, ErrFormat) && !errors.Is(err, errors.ErrUnsupported) {
			t.Errorf("bit %d flipped: %v", bit, err)
		}
	}
}

// TestErrors checks the errors of data that isn't a whole AC-3 syncframe,
// and of too little room.
func TestErrors(t *testing.T) {
	f := frames(t, readFile(t, filepath.Join("testdata", "5.1-48k-cpl.ac3")))[0]
	d := NewDecoder()
	pcm := make([]float32, MaxChannels*FrameLength)
	bsid := func(b []byte, id int) []byte {
		b = bytes.Clone(b)
		b[5] = b[5]&7 | byte(id)<<3
		return b
	}
	e := frames(t, readFile(t, filepath.Join("testdata", "eac3-5.1-cpl.eac3")))[0]
	reduced := bytes.Clone(e)
	reduced[4] |= 0xC0 // fscod 3: of the reduced sample rates
	for _, c := range []struct {
		name string
		data []byte
		pcm  []float32
		err  error
	}{
		{"short", f[:len(f)-1], pcm, ErrFormat},
		{"header", f[:5], pcm, ErrFormat},
		{"no syncword", append([]byte{0}, f...), pcm, ErrFormat},
		{"bsid 9", bsid(f, 9), pcm, errors.ErrUnsupported},
		{"bsid 17", bsid(f, 17), pcm, ErrFormat},
		{"E-AC-3 at 24 kHz", reduced, pcm, errors.ErrUnsupported},
		{"E-AC-3 short", e[:len(e)-2], pcm, ErrFormat},
		{"E-AC-3 no room", e, pcm[:MaxChannels*FrameLength-1], ErrBufferTooSmall},
		{"no room", f, pcm[:len(pcm)-1], ErrBufferTooSmall},
		{"frame and a half", append(bytes.Clone(f), f[:100]...), pcm, ErrFormat},
	} {
		t.Run(c.name, func(t *testing.T) {
			if _, err := d.Decode(c.data, c.pcm); !errors.Is(err, c.err) {
				t.Errorf("%v, want %v", err, c.err)
			}
		})
	}
	// The decoder then decodes as usual.
	if n, err := d.Decode(f, pcm); err != nil || n != FrameLength {
		t.Errorf("after the errors: %d, %v", n, err)
	}
}

// TestDecodeSamples checks that Decode decodes several syncframes at once,
// as one at a time, and that Info tells the stream.
func TestDecodeSamples(t *testing.T) {
	data := readFile(t, filepath.Join("testdata", "5.1-44k-loro.ac3"))
	want, _, _ := decodeAll(t, data)
	d := NewDecoder()
	pcm := make([]float32, len(want))
	n, err := d.Decode(data, pcm)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(want)/MaxChannels {
		t.Fatalf("%d samples, want %d", n, len(want)/MaxChannels)
	}
	for i := range want {
		if pcm[i] != want[i] {
			t.Fatalf("sample %d: %g, want %g", i, pcm[i], want[i])
		}
	}
	info := d.Info()
	if info.SampleRate != 44100 || info.Mode != Mode32 || !info.LFE || info.Bitrate != 448000 || info.Channels() != 6 {
		t.Errorf("%+v", info)
	}
	// The stream's LoRo levels: C at -1.5 dB, the surrounds at -6 dB.
	if l, c, s := info.Downmix[0][Left], info.Downmix[0][Center], info.Downmix[0][LeftSurround]; math.Abs(float64(c/l)-0.8409) > 1e-4 || math.Abs(float64(s/l)-0.5) > 1e-4 {
		t.Errorf("downmix %v", info.Downmix)
	}
}

// TestReset checks that after Reset, a decoder decodes the frames after
// the first as a decoder that decoded the stream from its start does.
func TestReset(t *testing.T) {
	data := readFile(t, filepath.Join("testdata", "5.1-48k-edited.ac3"))
	all := frames(t, data)
	want, _, _ := decodeAll(t, data)
	d := NewDecoder()
	pcm := make([]float32, MaxChannels*FrameLength)
	for _, f := range all[:3] {
		if _, err := d.Decode(f, pcm); err != nil {
			t.Fatal(err)
		}
	}
	d.Reset()
	for i, f := range all[4:] {
		if _, err := d.Decode(f, pcm); err != nil {
			t.Fatal(err)
		}
		w := want[MaxChannels*FrameLength*(4+i):][:MaxChannels*FrameLength]
		from := 0
		if i == 0 {
			from = MaxChannels * BlockLength // the block that overlaps silence
		}
		for k := from; k < len(w); k++ {
			if pcm[k] != w[k] {
				t.Fatalf("frame %d, sample %d: %g, want %g", 4+i, k, pcm[k], w[k])
			}
		}
	}
}

// TestAllocations checks that Decode allocates nothing.
func TestAllocations(t *testing.T) {
	all := frames(t, readFile(t, filepath.Join("testdata", "5.1-48k-edited.ac3")))
	d := NewDecoder()
	pcm := make([]float32, MaxChannels*FrameLength)
	i := 0
	if n := testing.AllocsPerRun(50, func() {
		if _, err := d.Decode(all[i%len(all)], pcm); err != nil {
			t.Fatal(err)
		}
		i++
	}); n != 0 {
		t.Errorf("%v allocations a frame", n)
	}
}

// BenchmarkDecode decodes 5.1 at 384 kbit/s.
func BenchmarkDecode(b *testing.B) {
	all := frames(b, readFile(b, filepath.Join("testdata", "5.1-48k-edited.ac3")))
	d := NewDecoder()
	pcm := make([]float32, MaxChannels*FrameLength)
	b.ReportAllocs()
	b.SetBytes(int64(len(all[0])))
	i := 0
	for b.Loop() {
		if _, err := d.Decode(all[i%len(all)], pcm); err != nil {
			b.Fatal(err)
		}
		i++
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/32e6*100, "%realtime")
}
