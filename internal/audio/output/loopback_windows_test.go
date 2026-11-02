//go:build windows && (amd64 || arm64)

package output

import (
	"errors"
	"math"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"
)

// TestCaptureProcessChildren hears a tone that a child plays, as WebView2's
// audio service is a child of its browser process: the child runs this
// test again, which plays. Windows hears a process's children, but not
// theirs. It opens the audio device, so it runs only with MIXMUSIC_AUDIO=1.
func TestCaptureProcessChildren(t *testing.T) {
	if os.Getenv("MIXMUSIC_TONE_CHILD") != "" {
		// The child: it plays.
		phase := 0
		s, err := Start(2, func(out []float32, _ int) {
			for i := 0; i < len(out); i += 2 {
				v := float32(1e-5 * math.Sin(2*math.Pi*1000*float64(phase)/Rate))
				out[i], out[i+1] = v, v
				phase++
			}
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Second)
		s.Stop()
		return
	}
	if os.Getenv("MIXMUSIC_AUDIO") != "1" {
		t.Skip("MIXMUSIC_AUDIO=1 runs the tests that open the audio device")
	}
	var mu sync.Mutex
	peak := 0.0
	c, err := CaptureProcess(uint32(os.Getpid()), func(frames []float32) {
		mu.Lock()
		for _, v := range frames {
			peak = max(peak, math.Abs(float64(v)))
		}
		mu.Unlock()
	})
	if errors.Is(err, ErrNoLoopback) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()
	cmd := exec.Command(os.Args[0], "-test.run=^TestCaptureProcessChildren$")
	cmd.Env = append(os.Environ(), "MIXMUSIC_TONE_CHILD=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the child: %v\n%s", err, out)
	}
	mu.Lock()
	defer mu.Unlock()
	if peak < 0.5e-5 || peak > 1.5e-5 {
		t.Errorf("heard the child's tone peak at %g, where it peaks at 1e-5", peak)
	}
}

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
	s, err := Start(2, func(out []float32, _ int) {
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
