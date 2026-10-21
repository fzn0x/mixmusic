package audio

import (
	"math"
	"testing"
)

// TestTap analyses what it hears as the engine's analyser does: a tone
// shows in its bin, and silence then brings the spectrum down to nothing.
func TestTap(t *testing.T) {
	tap := NewTap()
	var a Analysis
	tap.Analyse(&a)
	if a.Bytes != [analyserBins]byte{} {
		t.Fatalf("a tap that heard nothing shows %v", a.Bytes)
	}
	// A 1500 Hz tone, the eighth bin's frequency at Rate, at -40 dB: at
	// -54 dB in its bin, under the range's top, and its neighbours under it.
	const hz = 8 * Rate / FFTSize
	frames := make([]float32, 2*FFTSize)
	for i := range FFTSize {
		v := float32(0.01 * math.Sin(2*math.Pi*hz*float64(i)/Rate))
		frames[2*i], frames[2*i+1] = v, v
	}
	for range 20 {
		tap.Write(frames)
		tap.Analyse(&a)
	}
	loudest := 0
	for k, b := range a.Bytes {
		if b > a.Bytes[loudest] {
			loudest = k
		}
	}
	if loudest != 8 || a.Bytes[8] < 150 {
		t.Errorf("the tone shows loudest in bin %d, at %d; bin 8 is at %d", loudest, a.Bytes[loudest], a.Bytes[8])
	}
	for range 100 {
		tap.Silence()
		tap.Analyse(&a)
	}
	if a.Bytes != [analyserBins]byte{} {
		t.Errorf("after silence, the spectrum is still %v", a.Bytes)
	}
}
