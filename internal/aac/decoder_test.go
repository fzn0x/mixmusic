package aac

import (
	"errors"
	"math"
	"math/rand/v2"
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
