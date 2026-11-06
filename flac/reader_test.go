package flac

import (
	"bytes"
	"crypto/md5"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// fixtures are the files of testdata, a fraction of a second of sine
// waves each, as ffmpeg 7.0 encodes them, with what their STREAMINFO
// tells:
//
//   - stereo-16, at 44.1 kHz, with a little noise, at compression level 5
//   - mono-24, of 24 bits at 96 kHz, at level 8
//   - surround, six channels of 5.1 at 8 kHz, at level 2
//   - level0, at 11.025 kHz, silent at first, at level 0, with ffmpeg's
//     PADDING
//
// They were made with
//
//	ffmpeg -fflags +bitexact -f lavfi -i "aevalsrc=0.4*sin(2*PI*440*t)+0.05*sin(2*PI*5000*t)+0.0005*(random(0)-0.5)|0.4*sin(2*PI*660*t)+0.0005*(random(1)-0.5):s=44100:d=0.2" -c:a flac -sample_fmt s16 -compression_level 5 -flags:a +bitexact stereo-16.flac
//	ffmpeg -fflags +bitexact -f lavfi -i "aevalsrc=0.5*sin(2*PI*1000*t)+0.00002*(random(0)-0.5):s=96000:d=0.1" -c:a flac -sample_fmt s32 -bits_per_raw_sample 24 -compression_level 8 -flags:a +bitexact mono-24.flac
//	ffmpeg -fflags +bitexact -f lavfi -i "aevalsrc=0.25*sin(2*PI*440*t)|0.25*sin(2*PI*550*t)|0.25*sin(2*PI*660*t)|0.25*sin(2*PI*60*t)|0.25*sin(2*PI*770*t)|0.25*sin(2*PI*880*t):s=8000:d=0.2:c=5.1(side)" -c:a flac -sample_fmt s16 -compression_level 2 -flags:a +bitexact surround.flac
//	ffmpeg -fflags +bitexact -f lavfi -i "aevalsrc=if(lt(t\,0.15)\,0\,0.5*sin(2*PI*300*t))|if(lt(t\,0.1)\,0\,0.3*sin(2*PI*200*t)):s=11025:d=0.3" -c:a flac -sample_fmt s16 -compression_level 0 -flags:a +bitexact level0.flac
//
// and the PADDING of the first three taken out.
var fixtures = []struct {
	name                string
	rate, channels, bps int
	samples             int64
	minBlock, maxBlock  int
}{
	{"stereo-16.flac", 44100, 2, 16, 8820, 4608, 4608},
	{"mono-24.flac", 96000, 1, 24, 9600, 8192, 8192},
	{"surround.flac", 8000, 6, 16, 1600, 192, 192},
	{"level0.flac", 11025, 2, 16, 3308, 256, 256},
}

func open(t testing.TB, data []byte) *Reader {
	t.Helper()
	r, err := NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func readFixture(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// decodeAll reads the frames of r on to the end, and returns their
// samples, interleaved.
func decodeAll(t testing.TB, r *Reader) []int32 {
	t.Helper()
	var out []int32
	next := int64(-1)
	for {
		f, err := r.ReadFrame()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		if next >= 0 && f.Sample != next {
			t.Fatalf("a frame at sample %d after one that ended at %d", f.Sample, next)
		}
		n := len(f.Samples[0])
		next = f.Sample + int64(n)
		for i := range n {
			for _, ch := range f.Samples {
				out = append(out, ch[i])
			}
		}
	}
}

// sum is the MD5 of samples as STREAMINFO has it: little-endian, in as
// many bytes as bps bits take.
func sum(samples []int32, bps int) [16]byte {
	n := (bps + 7) / 8
	b := make([]byte, 0, len(samples)*n)
	for _, s := range samples {
		for k := range n {
			b = append(b, byte(s>>(8*k)))
		}
	}
	return md5.Sum(b)
}

// TestFixtures decodes the fixtures, which their STREAMINFO tells, to the
// samples whose MD5 it has.
func TestFixtures(t *testing.T) {
	for _, c := range fixtures {
		t.Run(c.name, func(t *testing.T) {
			r := open(t, readFixture(t, c.name))
			i := r.Info()
			if i.SampleRate != c.rate || i.Channels != c.channels || i.BitsPerSample != c.bps || i.Samples != c.samples ||
				i.MinBlock != c.minBlock || i.MaxBlock != c.maxBlock {
				t.Fatalf("info %+v", i)
			}
			samples := decodeAll(t, r)
			if int64(len(samples)) != c.samples*int64(c.channels) {
				t.Fatalf("%d samples, want %d", len(samples), c.samples*int64(c.channels))
			}
			if got := sum(samples, c.bps); got != i.MD5 {
				t.Errorf("MD5 %x, want %x", got, i.MD5)
			}
			if i.Duration() <= 0 {
				t.Errorf("duration %v", i.Duration())
			}
		})
	}
}

// TestSeek seeks to samples across each fixture, by the search for frames
// as well as from the start, and reads on as decoding from the start
// does.
func TestSeek(t *testing.T) {
	defer func(s int64) { seekSpan = s }(seekSpan)
	for _, span := range []int64{64 << 10, 256} {
		seekSpan = span
		for _, c := range fixtures {
			data := readFixture(t, c.name)
			all := decodeAll(t, open(t, data))
			r := open(t, data)
			ch := int64(c.channels)
			for _, s := range []int64{c.samples / 2, 0, c.samples - 1, c.samples / 3, c.samples + 1000, 1} {
				first, err := r.SeekSample(s)
				if err != nil {
					t.Fatalf("%s: SeekSample(%d): %v", c.name, s, err)
				}
				f, err := r.ReadFrame()
				if err != nil {
					t.Fatalf("%s: after SeekSample(%d): %v", c.name, s, err)
				}
				n := int64(len(f.Samples[0]))
				if f.Sample != first || first > s || first+n <= min(s, c.samples-1) {
					t.Fatalf("%s: SeekSample(%d) = %d, to a frame of %d samples from %d", c.name, s, first, n, f.Sample)
				}
				for i := range n {
					for k, samples := range f.Samples {
						if want := all[(first+i)*ch+int64(k)]; samples[i] != want {
							t.Fatalf("%s: after SeekSample(%d), sample %d of channel %d is %d, want %d", c.name, s, first+i, k, samples[i], want)
						}
					}
				}
			}
		}
	}
}

// bitWriter writes the bits of a hand-made stream.
type bitWriter struct {
	b    []byte
	acc  uint64
	bits uint
}

func (w *bitWriter) write(v uint64, n uint) {
	for i := int(n) - 1; i >= 0; i-- {
		w.acc = w.acc<<1 | v>>uint(i)&1
		w.bits++
		if w.bits == 8 {
			w.b = append(w.b, byte(w.acc))
			w.acc, w.bits = 0, 0
		}
	}
}

func (w *bitWriter) align() {
	for w.bits != 0 {
		w.write(0, 1)
	}
}

// codedNumber writes a frame's number as its header codes it.
func (w *bitWriter) codedNumber(v uint64) {
	if v < 0x80 {
		w.write(v, 8)
		return
	}
	n := 1 // the bytes that follow
	for v >= 1<<(5*n+6) && n < 6 {
		n++
	}
	if n == 6 {
		w.write(0xFE, 8)
	} else {
		w.write(uint64(0xFF00>>(n+1))&0xFF|v>>(6*n), 8)
	}
	for k := n - 1; k >= 0; k-- {
		w.write(0x80|v>>(6*k)&0x3F, 8)
	}
}

// spec is a hand-made frame: its header's fields as coded, and its
// subframes, which write writes.
type spec struct {
	variable             bool
	number               uint64
	blockCode, rateCode  int
	block, rate          int // as codes 6, 7 and 12 to 14 write them
	assign, sizeCode     int
	subframes            func(w *bitWriter)
	badHeader, badFooter bool
}

func (s spec) bytes() []byte {
	w := &bitWriter{}
	if s.variable {
		w.write(0xFFF9, 16)
	} else {
		w.write(0xFFF8, 16)
	}
	w.write(uint64(s.blockCode), 4)
	w.write(uint64(s.rateCode), 4)
	w.write(uint64(s.assign), 4)
	w.write(uint64(s.sizeCode), 3)
	w.write(0, 1)
	w.codedNumber(s.number)
	switch s.blockCode {
	case 6:
		w.write(uint64(s.block-1), 8)
	case 7:
		w.write(uint64(s.block-1), 16)
	}
	switch s.rateCode {
	case 12:
		w.write(uint64(s.rate/1000), 8)
	case 13:
		w.write(uint64(s.rate), 16)
	case 14:
		w.write(uint64(s.rate/10), 16)
	}
	c8 := crc8(w.b)
	if s.badHeader {
		c8++
	}
	w.write(uint64(c8), 8)
	s.subframes(w)
	w.align()
	c16 := crc16(w.b)
	if s.badFooter {
		c16++
	}
	w.write(uint64(c16), 16)
	return w.b
}

// stream is a FLAC stream of the frames, after a STREAMINFO of their
// audio.
func stream(rate, channels, bps int, samples int64, minBlock, maxBlock int, frames ...[]byte) []byte {
	w := &bitWriter{}
	w.write(0x664C6143, 32) // fLaC
	w.write(0x80, 8)        // the last block, STREAMINFO
	w.write(34, 24)
	w.write(uint64(minBlock), 16)
	w.write(uint64(maxBlock), 16)
	w.write(0, 48)
	w.write(uint64(rate), 20)
	w.write(uint64(channels-1), 3)
	w.write(uint64(bps-1), 5)
	w.write(uint64(samples), 36)
	w.write(0, 128)
	for _, f := range frames {
		w.b = append(w.b, f...)
	}
	return w.b
}

// verbatim writes a subframe of samples as they are, of bps bits, wasted
// the low bits of which are zero.
func verbatim(w *bitWriter, samples []int64, bps, wasted int) {
	w.write(0, 1)
	w.write(1, 6)
	if wasted > 0 {
		w.write(1, 1)
		w.write(1, uint(wasted)) // wasted-1 zeros, then a one
	} else {
		w.write(0, 1)
	}
	for _, s := range samples {
		w.write(uint64(s>>wasted)&(1<<(bps-wasted)-1), uint(bps-wasted))
	}
}

// TestHandMade decodes frames the fixtures' encoder doesn't write: of a
// variable stream, whose headers tell their samples, with the sizes and
// rates coded after the header, verbatim and constant subframes with
// wasted bits, an escaped partition of a residual, and a fixed
// predictor of order 4.
func TestHandMade(t *testing.T) {
	a := []int64{100, -200, 300, -400, 500, -600, 700, -800}
	b := []int64{4, 8, -12, 16, 20, 24, -28, 32}
	f1 := spec{variable: true, number: 0, blockCode: 6, block: 8, rateCode: 12, rate: 8000, assign: 1, sizeCode: 4,
		subframes: func(w *bitWriter) { verbatim(w, a, 16, 0); verbatim(w, b, 16, 2) }}.bytes()
	// A constant left channel and a side channel of escaped residuals.
	f2 := spec{variable: true, number: 8, blockCode: 7, block: 4, rateCode: 13, rate: 8000, assign: 8, sizeCode: 4,
		subframes: func(w *bitWriter) {
			w.write(0, 1)
			w.write(0, 6)
			w.write(0, 1)
			w.write(uint64(0x10000-5), 16)
			// The side channel, of 17 bits: a fixed predictor of order 0,
			// one partition, escaped to 6 bits.
			w.write(0, 1)
			w.write(8, 6)
			w.write(0, 1)
			w.write(0, 2)
			w.write(0, 4)
			w.write(15, 4)
			w.write(6, 5)
			for _, s := range []int64{1, -2, 3, -32} {
				w.write(uint64(s)&63, 6)
			}
		}}.bytes()
	// A fixed predictor of order 4 on a ramp, of Rice codes of parameter 1.
	ramp := []int64{0, 1, 4, 9, 16, 25, 36, 49}
	f3 := spec{variable: true, number: 1 << 33, blockCode: 6, block: 8, rateCode: 14, rate: 8000, assign: 1, sizeCode: 1,
		subframes: func(w *bitWriter) {
			w.write(0, 1)
			w.write(12, 6)
			w.write(0, 1)
			for _, s := range ramp[:4] {
				w.write(uint64(s), 8)
			}
			w.write(0, 2)
			w.write(0, 4)
			w.write(1, 4)
			for range 4 {
				w.write(1, 1) // 0 is 0 in zigzag: no zeros, then the low bit 0
				w.write(0, 1)
			}
			verbatim(w, ramp, 8, 0)
		}}.bytes()
	data := stream(8000, 2, 16, 0, 4, 8, f1, f2, f3)
	r := open(t, data)
	want := []struct {
		sample int64
		ch     [][]int64
	}{
		{0, [][]int64{a, b}},
		{8, [][]int64{{-5, -5, -5, -5}, {-6, -3, -8, 27}}},
		{1 << 33, [][]int64{ramp, ramp}},
	}
	for _, w := range want {
		f, err := r.ReadFrame()
		if err != nil {
			t.Fatal(err)
		}
		if f.Sample != w.sample {
			t.Fatalf("frame at %d, want %d", f.Sample, w.sample)
		}
		for c, samples := range w.ch {
			for i, s := range samples {
				if int64(f.Samples[c][i]) != s {
					t.Fatalf("frame %d, channel %d: %v, want %v", w.sample, c, f.Samples[c], samples)
				}
			}
		}
	}
	if _, err := r.ReadFrame(); err != io.EOF {
		t.Fatalf("after the last frame: %v", err)
	}
}

// TestPassesOver reads on past what isn't a frame, or a frame whose
// checksum fails, as a tag before the stream, junk between frames and an
// ID3v1 tag at the end.
func TestPassesOver(t *testing.T) {
	frame := func(n uint64, v int64, bad bool) []byte {
		return spec{number: n, blockCode: 6, block: 4, rateCode: 0, assign: 0, sizeCode: 0, badFooter: bad,
			subframes: func(w *bitWriter) { verbatim(w, []int64{v, v, v, v}, 16, 0) }}.bytes()
	}
	body := stream(8000, 1, 16, 16, 4, 4, frame(0, 1, false), []byte{0xFF, 0xF8, 0x00, 1, 2, 3}, frame(1, 2, true), frame(2, 3, false))
	tag := append([]byte("ID3\x04\x00\x00\x00\x00\x00\x05"), "hello"...)
	data := append(append(tag, body...), append([]byte("TAG"), make([]byte, 125)...)...)
	r := open(t, data)
	var got []int64
	for {
		f, err := r.ReadFrame()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, f.Sample, int64(f.Samples[0][0]))
	}
	if want := []int64{0, 1, 8, 3}; !equal(got, want) {
		t.Errorf("frames %v, want %v", got, want)
	}
}

func equal(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestErrors refuses what isn't FLAC, with ErrFormat.
func TestErrors(t *testing.T) {
	good := readFixture(t, "stereo-16.flac")
	for _, c := range []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"not FLAC", []byte("OggS\x00\x02 and so on")},
		{"cut in its STREAMINFO", good[:20]},
		{"no frames", good[:42]},
		{"a STREAMINFO of rate 0", stream(0, 2, 16, 0, 16, 16)},
		{"no STREAMINFO first", []byte("fLaC\x81\x00\x00\x00")},
	} {
		_, err := NewReader(bytes.NewReader(c.data))
		if !errors.Is(err, ErrFormat) {
			t.Errorf("%s: %v, want ErrFormat", c.name, err)
		}
	}
}
