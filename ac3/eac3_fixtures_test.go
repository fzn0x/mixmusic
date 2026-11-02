package ac3

import (
	"bytes"
	"encoding/binary"
	"math"
	"path/filepath"
	"testing"
)

// TestSpxAttenuation checks the gains of the notch filter of spectral
// extension against rows of Table E.2.12.
func TestSpxAttenuation(t *testing.T) {
	for code, want := range map[int][3]float64{
		0:  {0.954841604, 0.911722489, 0.870550563},
		1:  {0.911722489, 0.831237896, 0.757858283},
		14: {0.5, 0.25, 0.125},
		31: {0.227930622, 0.051952369, 0.011841536},
	} {
		for i, w := range want {
			if g := float64(spxAttenuation[code][i]); math.Abs(g-w) > 1e-7 {
				t.Errorf("spxattentab[%d][%d] = %.9f, want %.9f", code, i, g, w)
			}
		}
	}
}

// TestSubstreams checks that the decoder decodes independent substream 0
// alone: the frames of a dependent substream, and of independent
// substream 1, between those of substream 0 change nothing.
func TestSubstreams(t *testing.T) {
	all := frames(t, readFile(t, filepath.Join("testdata", "eac3-5.1-cpl.eac3")))
	want, _, _ := decodeAll(t, bytes.Join(all, nil))
	other := frames(t, readFile(t, filepath.Join("testdata", "eac3-spx.eac3")))[0]
	retype := func(f []byte, strmtyp, substreamid int) []byte {
		f = bytes.Clone(f)
		v := binary.BigEndian.Uint16(f[2:])
		binary.BigEndian.PutUint16(f[2:], uint16(strmtyp)<<14|uint16(substreamid)<<11|v&0x7FF)
		setCRC(f)
		return f
	}
	d := NewDecoder()
	pcm := make([]float32, MaxChannels*FrameLength)
	var got []float32
	for _, f := range all {
		sample := bytes.Join([][]byte{f, retype(other, streamDependent, 0), retype(other, streamIndependent, 1), retype(other, streamDependent, 1)}, nil)
		n, err := d.Decode(sample, pcm)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, pcm[:MaxChannels*n]...)
	}
	if len(got) != len(want) {
		t.Fatalf("%d samples, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("sample %d: %g, want %g", i, got[i], want[i])
		}
	}
}

// TestEAC3Samples checks that Decode decodes an MP4 sample of E-AC-3 of
// frames of fewer blocks than 6 whole, and tells the stream.
func TestEAC3Samples(t *testing.T) {
	data := readFile(t, filepath.Join("testdata", "eac3-3-blocks.eac3"))
	want, _, _ := decodeAll(t, data)
	all := frames(t, data)
	d := NewDecoder()
	pcm := make([]float32, MaxChannels*FrameLength)
	var got []float32
	last := 0
	for i := 0; i+2 <= len(all); i += 2 {
		last = i + 1
		n, err := d.Decode(bytes.Join(all[i:i+2], nil), pcm)
		if err != nil {
			t.Fatal(err)
		}
		if n != FrameLength {
			t.Fatalf("%d samples of a sample", n)
		}
		got = append(got, pcm[:MaxChannels*n]...)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("sample %d: %g, want %g", i, got[i], want[i])
		}
	}
	info := d.Info()
	if info.SampleRate != 48000 || info.Mode != Mode32 || !info.LFE || info.Channels() != 6 {
		t.Errorf("%+v", info)
	}
	if rate := len(all[last]) * 8 * 48000 / 768; info.Bitrate != rate {
		t.Errorf("bitrate %d, want %d", info.Bitrate, rate)
	}
}

// TestEAC3Downmix checks the downmix of an E-AC-3 stream of mixing
// metadata: its Lo/Ro levels, and its LFE mix level, at which the LFE
// mixes into both outputs, less 4.5 dB.
func TestEAC3Downmix(t *testing.T) {
	_, d, _ := decodeAll(t, readFile(t, filepath.Join("testdata", "eac3-1-block-mix.eac3")))
	m := d.Info().Downmix
	// lorocmixlev 3, -1.5 dB; lorosurmixlev 6, -6 dB; lfemixlevcod 12, -2
	// dB and less 4.5 dB.
	l, c, s, lfe := 1.0, 0.8408964152537145, 0.5, math.Pow(10, -6.5/20)
	sum := l + c + s + lfe
	for o, want := range [2][MaxChannels]float64{
		{Left: l / sum, Center: c / sum, LFE: lfe / sum, LeftSurround: s / sum},
		{Right: l / sum, Center: c / sum, LFE: lfe / sum, RightSurround: s / sum},
	} {
		for ch, w := range want {
			if g := float64(m[o][ch]); math.Abs(g-w) > 1e-6 {
				t.Errorf("output %d, channel %d: %.4f, want %.4f", o, ch, g, w)
			}
		}
	}
	// Of no mixing metadata: the middle levels of AC-3.
	_, d, _ = decodeAll(t, readFile(t, filepath.Join("testdata", "eac3-5.1-cpl.eac3")))
	m = d.Info().Downmix
	if r := m[0][Center] / m[0][Left]; math.Abs(float64(r)-0.5946) > 1e-4 {
		t.Errorf("center at %.4f", r)
	}
}

// TestEAC3Allocations checks that Decode allocates nothing, of E-AC-3 of
// AHT and spectral extension.
func TestEAC3Allocations(t *testing.T) {
	for _, name := range []string{"eac3-aht-gaq", "eac3-spx", "eac3-3-blocks"} {
		all := frames(t, readFile(t, filepath.Join("testdata", name+".eac3")))
		d := NewDecoder()
		pcm := make([]float32, MaxChannels*FrameLength)
		i := 0
		if n := testing.AllocsPerRun(50, func() {
			if _, err := d.Decode(all[i%len(all)], pcm); err != nil {
				t.Fatal(err)
			}
			i++
		}); n != 0 {
			t.Errorf("%s: %v allocations a frame", name, n)
		}
	}
}

// BenchmarkDecodeEAC3 decodes E-AC-3: 5.1 of coupling, and mono of AHT.
func BenchmarkDecodeEAC3(b *testing.B) {
	for _, name := range []string{"eac3-5.1-cpl", "eac3-aht-gaq"} {
		b.Run(name, func(b *testing.B) {
			all := frames(b, readFile(b, filepath.Join("testdata", name+".eac3")))
			d := NewDecoder()
			pcm := make([]float32, MaxChannels*FrameLength)
			b.ReportAllocs()
			i := 0
			for b.Loop() {
				if _, err := d.Decode(all[i%len(all)], pcm); err != nil {
					b.Fatal(err)
				}
				i++
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/32e6*100, "%realtime")
		})
	}
}
