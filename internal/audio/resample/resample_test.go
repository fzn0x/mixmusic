package resample

import (
	"errors"
	"math"
	"testing"
)

// noise returns frames of interleaved white noise of channels channels,
// from -1 to 1, the same for a seed.
func noise(seed uint32, channels, frames int) []float32 {
	out := make([]float32, channels*frames)
	for i := range out {
		seed = seed*1664525 + 1013904223
		out[i] = float32(int32(seed)) / (1 << 31)
	}
	return out
}

// sameFloats reports where a and b first differ, bit for bit, or -1.
func sameFloats(a, b []float32) int {
	for i := range min(len(a), len(b)) {
		if math.Float32bits(a[i]) != math.Float32bits(b[i]) {
			return i
		}
	}
	if len(a) != len(b) {
		return min(len(a), len(b))
	}
	return -1
}

func TestNew(t *testing.T) {
	for _, c := range []struct {
		from, to, channels int
		err                error
	}{
		{0, 48000, 2, ErrBadArg},
		{44100, -1, 2, ErrBadArg},
		{44100, 48000, 0, ErrBadArg},
		{44100, 48000, 9, ErrBadArg},
		// 48000 phases.
		{44101, 48000, 2, errors.ErrUnsupported},
		{1, 48000, 1, errors.ErrUnsupported},
		{44100, 48000, 2, nil},
		{48000, 48000, 1, nil},
		{7350, 48000, 8, nil},
		{96000, 48000, 2, nil},
	} {
		r, err := New(c.from, c.to, c.channels)
		if !errors.Is(err, c.err) || (err == nil) != (r != nil) {
			t.Errorf("New(%d, %d, %d) = %v, %v; want the error %v", c.from, c.to, c.channels, r, err, c.err)
		}
	}
}

// TestEqualRates passes audio between equal rates through as it is.
func TestEqualRates(t *testing.T) {
	c, err := New(48000, 48000, 2)
	if err != nil {
		t.Fatal(err)
	}
	if c.Latency() != 0 || c.MaxOutput(100) != 100 {
		t.Errorf("Latency %d, MaxOutput(100) %d; want 0 and 100", c.Latency(), c.MaxOutput(100))
	}
	in := noise(1, 2, 1000)
	out := make([]float32, 2*600+1)
	read, written := c.Process(in[:2*700+1], out)
	if read != 600 || written != 600 || sameFloats(out[:2*600], in[:2*600]) >= 0 {
		t.Errorf("Process read %d and wrote %d frames; want 600 of each, as they were", read, written)
	}
	if n := c.Flush(out); n != 0 {
		t.Errorf("Flush wrote %d frames; want none", n)
	}
}

// TestLength converts streams of many lengths: a stream of k frames gives
// ceil(k·to/from) frames, of which Process holds back at most Latency.
func TestLength(t *testing.T) {
	for _, r := range [][2]int{{44100, 48000}, {22050, 48000}, {24000, 48000}, {32000, 48000}, {96000, 48000}, {48000, 44100}} {
		c, err := New(r[0], r[1], 2)
		if err != nil {
			t.Fatal(err)
		}
		lat := c.Latency()
		least := lat
		for _, k := range []int{0, 1, 2, 3, c.half - 1, c.half, c.half + 1, c.taps, 100, 147, 1000, 1023, 1024, 1025, 4097, 44100} {
			out := make([]float32, 2*(c.MaxOutput(k)+lat))
			_, written := c.Process(noise(2, 2, k), out)
			flushed := c.Flush(out[2*written:])
			want := (int64(k)*int64(r[1]) + int64(r[0]) - 1) / int64(r[0])
			if got := int64(written + flushed); got != want {
				t.Errorf("%d to %d Hz: %d frames gave %d; want %d", r[0], r[1], k, got, want)
			}
			if flushed > lat {
				t.Errorf("%d to %d Hz: %d frames left %d to Flush, more than the latency, %d", r[0], r[1], k, flushed, lat)
			}
			if k > c.taps {
				least = min(least, flushed)
			}
		}
		if least < lat-1 {
			t.Errorf("%d to %d Hz: Flush gave as few as %d frames; the latency is %d", r[0], r[1], least, lat)
		}
	}
}

// TestChunks converts a stream in pieces of odd sizes, of input and of
// output: the frames are those of one call, bit for bit.
func TestChunks(t *testing.T) {
	for _, r := range [][2]int{{44100, 48000}, {22050, 48000}, {32000, 48000}, {88200, 48000}} {
		for _, channels := range []int{1, 2, 3} {
			in := noise(3, channels, 20000)
			whole, err := New(r[0], r[1], channels)
			if err != nil {
				t.Fatal(err)
			}
			want := convert(t, whole, in)

			c, _ := New(r[0], r[1], channels)
			var got []float32
			out := make([]float32, 64*channels)
			sizes := []int{1, 7, 3, 1021, 13, 0, 5, 2047, 331}
			outs := []int{1, 63, 17, 2, 64, 9}
			for i, rest := 0, in; ; i++ {
				n := min(sizes[i%len(sizes)], len(rest)/channels)
				o := outs[i%len(outs)] * channels
				read, written := c.Process(rest[:n*channels], out[:o])
				got = append(got, out[:written*channels]...)
				rest = rest[read*channels:]
				if len(rest) == 0 && read == n {
					break
				}
			}
			for i := 0; ; i++ {
				o := outs[i%len(outs)] * channels
				n := c.Flush(out[:o])
				got = append(got, out[:n*channels]...)
				if n*channels < o {
					break
				}
			}
			if i := sameFloats(got, want); i >= 0 {
				t.Errorf("%d to %d Hz, %d channels: %d samples in pieces, %d at once, differing from %d", r[0], r[1], channels, len(got), len(want), i)
			}
		}
	}
}

// TestChannels converts each channel as the converter of a single one
// does, bit for bit, whatever the channels beside it.
func TestChannels(t *testing.T) {
	const frames = 5000
	for _, channels := range []int{2, 3, 6, 8} {
		in := noise(4, channels, frames)
		c, err := New(44100, 48000, channels)
		if err != nil {
			t.Fatal(err)
		}
		out := convert(t, c, in)
		for ch := range channels {
			mono := make([]float32, frames)
			for i := range mono {
				mono[i] = in[i*channels+ch]
			}
			m, _ := New(44100, 48000, 1)
			want := convert(t, m, mono)
			got := make([]float32, len(out)/channels)
			for i := range got {
				got[i] = out[i*channels+ch]
			}
			if i := sameFloats(got, want); i >= 0 {
				t.Errorf("%d channels: channel %d differs from its own conversion from frame %d", channels, ch, i)
			}
		}
	}
}

// TestImpulse converts a click: it comes out where it was in time, as the
// symmetric response of the filter, once the input of Latency frames of
// output past it has come.
func TestImpulse(t *testing.T) {
	const from, to = 44100, 48000
	c, err := New(from, to, 1)
	if err != nil {
		t.Fatal(err)
	}
	// Frame 1029 of the input falls on frame 1120 of the output, a frame
	// of both; frame 1000 on frame 1088.435, between two.
	for _, click := range []int{1029, 1000} {
		at := float64(click) * to / from
		in := make([]float32, 3000)
		in[click] = 1
		var out []float32
		buf := make([]float32, 8)
		// The input's frame with which the output's frame nearest the
		// click came.
		came := -1
		for k := range in {
			_, written := c.Process(in[k:k+1], buf)
			out = append(out, buf[:written]...)
			if came < 0 && len(out) > int(math.Round(at)) {
				came = k
			}
		}
		for {
			n := c.Flush(buf)
			out = append(out, buf[:n]...)
			if n < len(buf) {
				break
			}
		}
		// The centre of the click's energy: its time, for a response
		// symmetric about it.
		var sum, moment float64
		peak := 0
		for n, v := range out {
			e := float64(v) * float64(v)
			sum += e
			moment += float64(n) * e
			if math.Abs(float64(v)) > math.Abs(float64(out[peak])) {
				peak = n
			}
		}
		centre := moment / sum
		if math.Abs(centre-at) > 1e-3 || peak != int(math.Round(at)) {
			t.Errorf("a click at frame %d came out at %.4f, peaking at %d; want %.4f", click, centre, peak, at)
		}
		if click == 1029 {
			for d := 1; d < 50; d++ {
				if math.Abs(float64(out[1120+d]-out[1120-d])) > 1e-6 {
					t.Errorf("the response is not symmetric: %g after, %g before, %d frames from the click", out[1120+d], out[1120-d], d)
					break
				}
			}
		}
		// The click's frame of output came with the input's frame came: so
		// many frames of output past it, which the latency bounds. The
		// input's frames fall between the output's, so that it is up to a
		// frame of the input less.
		held := float64(came)*to/from - math.Round(at)
		t.Logf("a click at frame %d: out at %.5f, with the input of %.2f frames of output past it; the latency is %d", click, centre, held, c.Latency())
		if held > float64(c.Latency()) || held < float64(c.Latency())-1-to/from {
			t.Errorf("a click at frame %d came out with the input of %.2f frames of output past it; the latency is %d", click, held, c.Latency())
		}
	}
}

// TestDC converts a constant: it stays the same, with a gain of 1.
func TestDC(t *testing.T) {
	for _, r := range [][2]int{{44100, 48000}, {22050, 48000}, {24000, 48000}, {32000, 48000}, {11025, 48000}, {96000, 48000}, {88200, 48000}} {
		c, err := New(r[0], r[1], 2)
		if err != nil {
			t.Fatal(err)
		}
		const frames = 20000
		in := make([]float32, 2*frames)
		for i := range frames {
			in[2*i], in[2*i+1] = 0.5, -0.25
		}
		out := convert(t, c, in)
		// Away from the ends, where the windows hold only the constant.
		var worst float64
		margin := 2 * c.Latency()
		for i := margin; i < len(out)/2-margin; i++ {
			worst = max(worst, math.Abs(float64(out[2*i])-0.5)/0.5, math.Abs(float64(out[2*i+1])+0.25)/0.25)
		}
		t.Logf("%d to %d Hz: DC gain 1 within %.2g", r[0], r[1], worst)
		if worst > 1e-6 {
			t.Errorf("%d to %d Hz: DC gain off by %.2g", r[0], r[1], worst)
		}
	}
}

// TestSilence converts silence to silence, exactly.
func TestSilence(t *testing.T) {
	c, err := New(44100, 48000, 2)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range convert(t, c, make([]float32, 2*10000)) {
		if v != 0 {
			t.Fatalf("sample %d of silence is %g", i, v)
		}
	}
}

// TestTiny converts samples 600 dB down, and subnormal ones, which would
// be slow to compute, to silence.
func TestTiny(t *testing.T) {
	c, err := New(44100, 48000, 2)
	if err != nil {
		t.Fatal(err)
	}
	in := make([]float32, 2*5000)
	for i := range in {
		in[i] = []float32{0x1p-101, -0x1p-120, math.SmallestNonzeroFloat32, -1e-40}[i%4]
	}
	for i, v := range convert(t, c, in) {
		if v != 0 {
			t.Fatalf("sample %d is %g", i, v)
		}
	}
}

// TestReset converts noise, then, after a Reset as for a seek, a tone: the
// tone comes out as a new converter makes it, with nothing of the noise;
// silence after the noise comes out as silence.
func TestReset(t *testing.T) {
	const from, to = 44100, 48000
	tone := tones(1000, from, 5000)
	fresh, _ := New(from, to, 2)
	want := convert(t, fresh, tone)

	c, err := New(from, to, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, partial := range []int{1, 37, 2000} {
		out := make([]float32, 2*c.MaxOutput(partial))
		c.Process(noise(5, 2, partial), out)
		c.Reset()
		if i := sameFloats(convert(t, c, tone), want); i >= 0 {
			t.Errorf("after %d frames of noise and a Reset, a tone differs from sample %d", partial, i)
		}
		c.Process(noise(6, 2, partial), out)
		c.Reset()
		for i, v := range convert(t, c, make([]float32, 2*3000)) {
			if v != 0 {
				t.Errorf("after %d frames of noise and a Reset, sample %d of silence is %g", partial, i, v)
				break
			}
		}
	}
}

// TestNoAllocs converts without allocating, once made.
func TestNoAllocs(t *testing.T) {
	c, err := New(44100, 48000, 2)
	if err != nil {
		t.Fatal(err)
	}
	in := noise(7, 2, 1024)
	out := make([]float32, 2*(c.MaxOutput(1024)+c.Latency()))
	allocs := testing.AllocsPerRun(100, func() {
		_, written := c.Process(in, out)
		c.Flush(out[2*written:])
		c.Reset()
	})
	if allocs != 0 {
		t.Errorf("%v allocations a run", allocs)
	}
}
