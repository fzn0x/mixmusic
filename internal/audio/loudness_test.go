package audio

import (
	"context"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLoudnessTrim(t *testing.T) {
	for lkfs, want := range map[float64]float64{
		-7.93:  -6.07,
		-11.93: -2.07,
		-14:    0,
		-24.39: 0, // quiet recordings keep their level
		0:      -14,
		3:      0, // out of range: no measure a track can have
		-90:    0,
	} {
		if got := loudnessTrim(lkfs); math.Abs(got-want) > 1e-9 {
			t.Errorf("loudnessTrim(%v) = %v, want %v", lkfs, got, want)
		}
	}
	if got := loudnessTrim(math.NaN()); got != 0 {
		t.Errorf("loudnessTrim(NaN) = %v", got)
	}
}

// measuredSource is a memSource whose track's loudness is known.
type measuredSource struct {
	memSource
	lkfs float64
}

func (m measuredSource) Loudness() (float64, bool) { return m.lkfs, true }

// measuredOpener opens files as fileOpener does, with the loudness a URL
// tells after #, as "track.webm#-8".
func measuredOpener(ctx context.Context, url string, interrupted func() bool) (Source, error) {
	path, lkfs, measured := strings.Cut(url, "#")
	src, err := fileOpener(ctx, path, interrupted)
	if err != nil || !measured {
		return src, err
	}
	v, err := strconv.ParseFloat(lkfs, 64)
	if err != nil {
		return nil, err
	}
	return measuredSource{src.(memSource), v}, nil
}

// newMeasuredEngine returns an engine as newTestEngine does, which opens
// tracks with measuredOpener, with Even loudness on.
func newMeasuredEngine(t *testing.T) (*Engine, *fakeDevice) {
	dev := &fakeDevice{}
	e := NewEngine(measuredOpener, dev.start, newRecorder())
	dev.e = e
	e.master = newSmoother(1, glide)
	e.SetLoudness(true)
	t.Cleanup(e.Close)
	return e, dev
}

func TestEngineLoudness(t *testing.T) {
	want := reference(t, cuesFront)
	for url, db := range map[string]float64{
		cuesFront + "#-8":  -6, // a loud track, turned down from its first frame
		cuesFront + "#-20": 0,  // a quiet one, as it is
		cuesFront:          0,  // one of no known loudness, as it is
	} {
		e, dev := newMeasuredEngine(t)
		e.Load(url, 0, false)
		buffered(t, e, 0, ringFrames/2)
		e.Play()
		got := dev.play(t, len(want)/2)
		g := float32(1 * 1 * math.Pow(10, db/20))
		for i := 2 * fadeFrames; i < len(want); i++ {
			if w := want[i] * g; got[i] != w {
				t.Fatalf("%s: frame %d: %v, want %v", url, i/2, got[i], w)
			}
		}
	}
}

func TestEngineLoudnessSwitch(t *testing.T) {
	want := reference(t, cuesFront)
	e, dev := newMeasuredEngine(t)
	e.Load(cuesFront+"#-8", 0, false)
	buffered(t, e, 0, ringFrames/2)
	e.Play()
	at := 4800
	dev.play(t, at)
	e.SetLoudness(false)
	// It glides back, as the volume does: close after 0.1 s, and the track
	// as it is once settled.
	got := dev.play(t, int(frames(400*time.Millisecond)))
	for f := int(frames(100 * time.Millisecond)); f < len(got)/2; f++ {
		for c := range 2 {
			w := want[2*(at+f)+c]
			if math.Abs(float64(got[2*f+c]-w)) > 0.01*math.Abs(float64(w))+1e-7 {
				t.Fatalf("frame %d after turning it off: %v, want %v", f, got[2*f+c], w)
			}
		}
	}
	for f := int(frames(350 * time.Millisecond)); f < len(got)/2; f++ {
		if got[2*f] != want[2*(at+f)] || got[2*f+1] != want[2*(at+f)+1] {
			t.Fatalf("frame %d, settled: %v, want %v", f, got[2*f:2*f+2], want[2*(at+f):2*(at+f)+2])
		}
	}
}

func TestEngineLoudnessCrossfade(t *testing.T) {
	a, b := reference(t, cuesFront), reference(t, speech)
	e, dev := newMeasuredEngine(t)
	e.Load(cuesFront+"#-8", 0, false)
	buffered(t, e, 0, ringFrames/2)
	e.Play()
	at := 188 * quantum
	dev.play(t, at)
	e.Preload(speech + "#-11")
	buffered(t, e, 1, ringFrames/2)
	const fade = 300 * time.Millisecond
	if err := e.Crossfade(speech+"#-11", fade); err != nil {
		t.Fatal(err)
	}
	n := int(frames(fade)) + 4800
	got := dev.play(t, n)
	// Each track fades at its own level.
	ta, tb := math.Pow(10, -6.0/20), math.Pow(10, -3.0/20)
	ga, gb := ramp{value: 1, target: 1}, ramp{}
	ga.to(0, fade)
	gb.to(1, fade)
	for f := range n {
		wa, wb := float32(ga.next()*1*ta), float32(gb.next()*1*tb)
		for c := range 2 {
			want := float32(0) + a[2*(at+f)+c]*wa
			want += b[2*f+c] * wb
			if got[2*f+c] != want {
				t.Fatalf("frame %d of the fade: %v, want %v", f, got[2*f+c], want)
			}
		}
	}
}
