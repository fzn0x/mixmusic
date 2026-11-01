package audio

import (
	"bytes"
	"errors"
	"math"
	"os"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/ac3"
	"github.com/fzn0x/mixmusic/mp4"
)

// Test files of 5.1 surround, from mp4's testdata, where the commands that
// made them are: a second of AC-3 at 48 kHz, of E-AC-3 in fragments, and
// half a second of AC-3 at 44.1 kHz. Each channel is a sine of its own, at
// a quarter of full scale: surroundTones.
var (
	surroundAC3    = "../../mp4/testdata/surround-ac3.mp4"
	surroundEAC3   = "../../mp4/testdata/surround-eac3.mp4"
	surroundAC3At4 = "../../mp4/testdata/surround-ac3-44k.mp4"
	surroundFiles  = []string{surroundAC3, surroundEAC3, surroundAC3At4}
	// surroundTones are the frequencies of the channels, in the order of
	// ac3's Left to RightSurround.
	surroundTones = [6]float64{440, 550, 660, 60, 770, 880}
)

// needDecoder skips t while ac3 decodes nothing, as before its decoders
// came: err is what decoding gave.
func needDecoder(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, errors.ErrUnsupported) || errors.Is(err, errUnsupported) {
		t.Skipf("ac3 doesn't decode this stream yet: %v", err)
	}
}

// toneOf returns the amplitude and the phase, in frames, of the tone of hz
// in channel c of pcm, of ch channels interleaved at Rate, as tone does.
func toneOf(pcm []float32, ch, c int, hz float64) (amplitude, phase float64) {
	var ss, cc, xs, xc float64
	for i := Rate / 10; i < len(pcm)/ch-Rate/10; i++ {
		w := 2 * math.Pi * hz * float64(i) / Rate
		s, co, x := math.Sin(w), math.Cos(w), float64(pcm[ch*i+c])
		xs, xc, ss, cc = xs+x*s, xc+x*co, ss+s*s, cc+co*co
	}
	a, b := xs/ss, xc/cc
	return math.Hypot(a, b), math.Atan2(b, a) / (2 * math.Pi * hz) * Rate
}

// checkSurroundTones checks that each channel of pcm, of ch channels, holds
// the tone of its channel of 5.1 at a quarter of full scale, and none of
// another, the tones of a stereo downmix by m when ch is 2.
func checkSurroundTones(t *testing.T, pcm []float32, ch int, m *[2][6]float32) {
	t.Helper()
	for c := range ch {
		for src, hz := range surroundTones {
			want := 0.0
			switch {
			case ch == 6 && src == c:
				want = 0.25
			case ch == 2:
				want = 0.25 * float64(m[c][src])
			}
			a, _ := toneOf(pcm, ch, c, hz)
			// The LFE's 60 Hz passes the encoder's low-pass at 120 Hz: within
			// half a dB, like the others.
			if want == 0 && a > 0.003 || want > 0 && math.Abs(20*math.Log10(a/want)) > 0.5 {
				t.Errorf("%d channels: channel %d holds %v Hz at %.4f, want %.4f", ch, c, hz, a, want)
			}
		}
	}
}

func TestSurroundTrack(t *testing.T) {
	for _, path := range surroundFiles {
		t.Run(path, func(t *testing.T) {
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			tr, tm, err := openTrack(f)
			if err != nil {
				t.Fatal(err)
			}
			if tr.channels() != 6 {
				t.Fatalf("%d channels", tr.channels())
			}
			// ffmpeg's encoders prime with a block, which the edit list skips.
			rate := 48000
			if path == surroundAC3At4 {
				rate = 44100
			}
			if want := 256 * time.Second / time.Duration(rate); tm.delay != want || tm.preRoll != 1536*time.Second/time.Duration(rate) {
				t.Errorf("delay %v, pre-roll %v", tm.delay, tm.preRoll)
			}
			if err := tr.read(); err != nil {
				t.Fatal(err)
			}
			p, err := tr.decode()
			needDecoder(t, err)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.pcm)%6 != 0 || p.to != len(p.pcm)/6 || p.downmix == nil {
				t.Errorf("a packet of %d samples, to %d, downmix %v", len(p.pcm), p.to, p.downmix)
			}
		})
	}
}

// TestDeckPlaysSurround plays the files of 5.1 on a deck: six channels at
// Rate, each with its tone, for as long as the file says.
func TestDeckPlaysSurround(t *testing.T) {
	for _, path := range surroundFiles {
		t.Run(path, func(t *testing.T) {
			d := newDeck(path, 0, fileOpener, func() {})
			defer d.close()
			got := drain(t, d)
			err := d.failure()
			needDecoder(t, err)
			if err != nil {
				t.Fatal(err)
			}
			total, _ := d.info()
			if r := d.ring.Load(); r.channels != 6 || len(got)%6 != 0 {
				t.Fatalf("a ring of %d channels, %d samples", r.channels, len(got))
			}
			if n := int64(len(got) / 6); total == 0 || (duration(n)-total).Abs() > time.Millisecond {
				t.Errorf("%d frames, %v; the file tells %v", n, duration(n), total)
			}
			checkSurroundTones(t, got, 6, nil)
			if m := d.downmix.Load(); m == nil || m[0][0] == 0 || m[0][LFEChannel] != 0 {
				t.Errorf("downmix %v", m)
			}
		})
	}
}

// LFEChannel is the LFE's place among the channels of 5.1.
const LFEChannel = ac3.LFE

// TestDeckSeeksSurround starts a deck of 5.1 within its track: the frames
// from there are those of the whole decode, from the same frame.
func TestDeckSeeksSurround(t *testing.T) {
	for _, path := range []string{surroundAC3, surroundEAC3} {
		whole := newDeck(path, 0, fileOpener, func() {})
		want := drain(t, whole)
		err := whole.failure()
		whole.close()
		needDecoder(t, err)
		for _, at := range []time.Duration{100 * time.Millisecond, 333 * time.Millisecond, 700 * time.Millisecond} {
			d := newDeck(path, at, fileOpener, func() {})
			got := drain(t, d)
			d.close()
			start := frames(at)
			if n := int64(len(got) / 6); n != int64(len(want)/6)-start {
				t.Errorf("%s at %v: %d frames, want %d", path, at, n, int64(len(want)/6)-start)
				continue
			}
			if s := snr(got, want[6*start:]); s < 60 {
				t.Errorf("%s at %v: SNR %.1f dB against the whole decode", path, at, s)
			}
		}
	}
}

// TestEnginePlaysSurround plays a track of 5.1 on a device of 5.1 with 5.1
// on, and mixed down on a device of stereo, or with 5.1 off.
func TestEnginePlaysSurround(t *testing.T) {
	for _, c := range []struct {
		name           string
		device, wanted bool
		channels       int
	}{
		{"on 5.1", true, true, 6},
		{"on stereo", false, true, 2},
		{"with 5.1 off", true, false, 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, dev, rec := newTestEngine(t)
			dev.surround = c.device
			e.SetSurround(c.wanted)
			if err := e.Load(surroundAC3, 0, false); err != nil {
				t.Fatal(err)
			}
			buffered(t, e, 0, Rate/2)
			e.mu.Lock()
			d := e.decks[0]
			e.mu.Unlock()
			needDecoder(t, d.failure())
			if err := e.Play(); err != nil {
				t.Fatal(err)
			}
			if dev.channels != c.channels {
				t.Fatalf("the output plays %d channels", dev.channels)
			}
			got := dev.play(t, Rate+4800)
			checkSurroundTones(t, got[:c.channels*Rate], c.channels, d.downmix.Load())
			rec.waitFor(t, "ended")
		})
	}
}

// TestSurroundDecodeAllocates decodes the packets of a file of 5.1 again
// and again: after the first, nothing may be allocated.
func TestSurroundDecodeAllocates(t *testing.T) {
	f, err := os.Open(surroundAC3)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := mp4.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var packets []mp4.Packet
	for {
		p, err := r.ReadPacket()
		if err != nil {
			break
		}
		packets = append(packets, p)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	tr, _, err := openTrack(f)
	if err != nil {
		t.Fatal(err)
	}
	mt := tr.(*mp4Track)
	i := 0
	decode := func() {
		mt.p = packets[i%len(packets)]
		i++
		if _, err = mt.decode(); err != nil {
			return
		}
	}
	decode()
	needDecoder(t, err)
	if allocs := testing.AllocsPerRun(100, decode); allocs > 0 || err != nil {
		t.Errorf("%v allocations a packet, %v", allocs, err)
	}
}

// BenchmarkSurroundDecode decodes the packets of a file of 5.1, as a deck
// does.
func BenchmarkSurroundDecode(b *testing.B) {
	data, err := os.ReadFile(surroundAC3)
	if err != nil {
		b.Fatal(err)
	}
	tr, _, err := openTrack(bytes.NewReader(data))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := tr.read(); err != nil {
			if err := tr.seek(0); err != nil {
				b.Fatal(err)
			}
			continue
		}
		if _, err := tr.decode(); err != nil {
			b.Skip(err)
		}
	}
}
