package aac

import (
	"errors"
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

func TestNewDecoder(t *testing.T) {
	for _, c := range []struct {
		cfg Config
		err error
	}{
		{Config{ObjectType: ObjectTypeLC, SampleRate: 44100, Channels: 2}, nil},
		{Config{ObjectType: ObjectTypeLC, SampleRate: 8000, Channels: 1}, nil},
		{Config{ObjectType: ObjectTypeLC, SampleRate: 44100, Channels: 6}, errors.ErrUnsupported},
		{Config{ObjectType: ObjectTypeLC, SampleRate: 44100}, errors.ErrUnsupported},
		{Config{ObjectType: ObjectTypeSBR, SampleRate: 44100, Channels: 2}, errors.ErrUnsupported},
		{Config{ObjectType: ObjectTypeLC, Channels: 2}, ErrFormat},
	} {
		d, err := NewDecoder(c.cfg)
		if !errors.Is(err, c.err) || err == nil && c.err != nil || (err == nil) != (d != nil) {
			t.Errorf("NewDecoder(%+v) = %v; want %v", c.cfg, err, c.err)
			continue
		}
		if d != nil && (d.SampleRate() != c.cfg.SampleRate || d.Config() != c.cfg) {
			t.Errorf("NewDecoder(%+v): rate %d, config %+v", c.cfg, d.SampleRate(), d.Config())
		}
	}
}

// TestIMDCT checks the inverse MDCT of each size against its definition,
// computed in float64.
func TestIMDCT(t *testing.T) {
	initTables()
	rng := rand.New(rand.NewPCG(1, 2))
	for _, m := range []*mdct{&mdctLong, &mdctShort} {
		n := m.n
		in := make([]float32, n/2)
		for k := range in {
			in[k] = float32(rng.NormFloat64())
		}
		out := make([]float32, n)
		m.inverse(in, out, make([][2]float32, n/4))
		n0 := (float64(n)/2 + 1) / 2
		var sig, noise float64
		for i := range out {
			var want float64
			for k, x := range in {
				want += float64(x) * math.Cos(2*math.Pi/float64(n)*(float64(i)+n0)*(float64(k)+0.5))
			}
			want *= 2 / float64(n)
			sig += want * want
			noise += (want - float64(out[i])) * (want - float64(out[i]))
		}
		if snr := 10 * math.Log10(sig/noise); snr < 120 {
			t.Errorf("inverse MDCT of %d: SNR %.1f dB", n, snr)
		}
	}
}

// TestReset checks that a decoder decodes after Reset as a new one does.
func TestReset(t *testing.T) {
	cfg, frames := fixtureFrames(t, "stereo-44k-clicks")
	fresh, err := decodeFrames(t, cfg, frames[10:20]...)
	if err != nil {
		t.Fatal(err)
	}
	d, err := NewDecoder(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pcm := make([]float32, 2*FrameLength)
	for _, f := range frames[:10] {
		if _, err := d.Decode(f, pcm); err != nil {
			t.Fatal(err)
		}
	}
	d.Reset()
	var got []float32
	for _, f := range frames[10:20] {
		if _, err := d.Decode(f, pcm); err != nil {
			t.Fatal(err)
		}
		got = append(got, pcm...)
	}
	if !slices.Equal(got, fresh) {
		t.Error("the decoder decodes after Reset unlike a new one")
	}
}

// TestMono checks that a mono stream decodes into both channels.
func TestMono(t *testing.T) {
	cfg, frames := fixtureFrames(t, "mono-22k")
	pcm, err := decodeFrames(t, cfg, frames...)
	if err != nil {
		t.Fatal(err)
	}
	var energy float64
	for i := 0; i < len(pcm); i += 2 {
		if pcm[i] != pcm[i+1] {
			t.Fatalf("sample %d: %v and %v", i/2, pcm[i], pcm[i+1])
		}
		energy += float64(pcm[i]) * float64(pcm[i])
	}
	if energy == 0 {
		t.Error("silence")
	}
}

// TestAllocs checks that Decode allocates nothing.
func TestAllocs(t *testing.T) {
	for _, name := range []string{"stereo-44k-clicks", "mono-48k-bursts", "stereo-44k-tns"} {
		cfg, frames := fixtureFrames(t, name)
		d, err := NewDecoder(cfg)
		if err != nil {
			t.Fatal(err)
		}
		pcm := make([]float32, 2*FrameLength)
		i := 0
		allocs := testing.AllocsPerRun(len(frames), func() {
			if _, err := d.Decode(frames[i%len(frames)], pcm); err != nil {
				t.Fatal(err)
			}
			i++
		})
		if allocs != 0 {
			t.Errorf("%s: %v allocations a frame", name, allocs)
		}
	}
}

// BenchmarkDecode decodes fixtures, stereo at 44.1 kHz with short and long
// windows and the stereo tools, and mono at 48 kHz, and reports how many
// times faster than real time.
func BenchmarkDecode(b *testing.B) {
	for _, name := range []string{"stereo-44k-clicks", "stereo-44k-tns", "mono-48k-bursts"} {
		b.Run(name, func(b *testing.B) {
			cfg, frames := fixtureFrames(b, name)
			d, err := NewDecoder(cfg)
			if err != nil {
				b.Fatal(err)
			}
			pcm := make([]float32, 2*FrameLength)
			n := 0
			for b.Loop() {
				if _, err := d.Decode(frames[n%len(frames)], pcm); err != nil {
					b.Fatal(err)
				}
				n++
			}
			seconds := float64(n) * FrameLength / float64(cfg.SampleRate)
			b.ReportMetric(seconds/b.Elapsed().Seconds(), "x-realtime")
		})
	}
}
