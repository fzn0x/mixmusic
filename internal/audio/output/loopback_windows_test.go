//go:build windows && (amd64 || arm64)

package output

import (
	"errors"
	"math"
	"os"
	"sync"
	"testing"
	"time"
)

// TestCaptureProcess hears a tone this process plays, at -100 dB, which no
// one hears, at the level it plays, then silence once it stops. It opens
// the audio device, so it runs only with MIXMUSIC_AUDIO=1.
func TestCaptureProcess(t *testing.T) {
	if os.Getenv("MIXMUSIC_AUDIO") != "1" {
		t.Skip("MIXMUSIC_AUDIO=1 runs the tests that open the audio device")
	}
	var mu sync.Mutex
	var heard []float32
	c, err := CaptureProcess(uint32(os.Getpid()), func(frames []float32) {
		mu.Lock()
		heard = append(heard, frames...)
		mu.Unlock()
	})
	if errors.Is(err, ErrNoLoopback) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	const amplitude = 1e-5 // -100 dB
	phase := 0
	s, err := Start(func(out []float32) {
		for i := 0; i < len(out); i += 2 {
			v := float32(amplitude * math.Sin(2*math.Pi*1000*float64(phase)/Rate))
			out[i], out[i+1] = v, v
			phase++
		}
	}, nil)
	if errors.Is(err, ErrNoDevice) {
		c.Stop()
		t.Skip(err)
	}
	if err != nil {
		c.Stop()
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	s.Stop()
	mu.Lock()
	played := len(heard)
	peak := 0.0
	for _, v := range heard {
		peak = max(peak, math.Abs(float64(v)))
	}
	mu.Unlock()
	t.Logf("heard %d frames as the tone played, peaking at %g", played/2, peak)
	if peak < 0.5*amplitude || peak > 1.5*amplitude {
		t.Errorf("heard the tone peak at %g, where it peaks at %g", peak, amplitude)
	}
	time.Sleep(500 * time.Millisecond)
	c.Stop()
	mu.Lock()
	defer mu.Unlock()
	after := heard[played:]
	if len(after) < 2*Rate/10 {
		t.Errorf("heard %d frames of silence in half a second after the tone", len(after)/2)
	}
	tail := after[min(len(after), 2*Rate/5):]
	for _, v := range tail {
		if v != 0 {
			t.Fatalf("heard %g once the tone stopped", v)
		}
	}
	n := len(heard)
	time.Sleep(100 * time.Millisecond)
	if len(heard) != n {
		t.Error("write was called after Stop")
	}
}
