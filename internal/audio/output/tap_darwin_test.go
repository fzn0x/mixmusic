//go:build darwin && (amd64 || arm64)

package output

import (
	"errors"
	"math"
	"os"
	"slices"
	"sync"
	"testing"
	"time"
)

// TestCaptureProcess hears a tone this process plays, at -100 dB, which no
// one hears, through a muted tap, then silence once it stops, as a stream
// would. It opens the audio device, so it runs only with MIXMUSIC_AUDIO=1.
// The tap hears the tone only once the user allowed the test to record
// what apps play: until then, as where macOS can't ask, as over SSH, it
// hears silence at the same pace.
func TestCaptureProcess(t *testing.T) {
	if os.Getenv("MIXMUSIC_AUDIO") != "1" {
		t.Skip("MIXMUSIC_AUDIO=1 runs the tests that open the audio device")
	}
	const amplitude = 1e-5 // -100 dB
	phase := 0
	// The tone plays first: Core Audio knows a process once it plays.
	s, err := Start(2, func(out []float32, _ int) {
		for i := 0; i < len(out); i += 2 {
			v := float32(amplitude * math.Sin(2*math.Pi*1000*float64(phase)/Rate))
			out[i], out[i+1] = v, v
			phase++
		}
	}, nil)
	if errors.Is(err, ErrNoDevice) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(AudioProcesses(), func(p AudioProcess) bool { return p.PID == uint32(os.Getpid()) }) {
		t.Errorf("this process, %d, isn't among Core Audio's", os.Getpid())
	}
	var mu sync.Mutex
	var heard []float32
	c, err := startCapture(uint32(os.Getpid()), func(frames []float32) {
		mu.Lock()
		heard = append(heard, frames...)
		mu.Unlock()
	}, tapMuted)
	if errors.Is(err, ErrNoLoopback) {
		s.Stop()
		t.Skip(err)
	}
	if err != nil {
		s.Stop()
		t.Fatal(err)
	}
	allowed := permission() == tccAllowed
	time.Sleep(time.Second)
	s.Stop()
	mu.Lock()
	played := len(heard)
	peak := 0.0
	for _, v := range heard {
		peak = max(peak, math.Abs(float64(v)))
	}
	mu.Unlock()
	t.Logf("heard %d frames as the tone played, peaking at %g; allowed to record: %v", played/2, peak, allowed)
	if played < 2*Rate*8/10 {
		t.Errorf("heard %d frames in a second", played/2)
	}
	switch {
	case allowed && (peak < 0.5*amplitude || peak > 1.5*amplitude):
		t.Errorf("heard the tone peak at %g, where it peaks at %g", peak, amplitude)
	case !allowed && peak != 0:
		t.Errorf("heard %g, not allowed to record", peak)
	}
	time.Sleep(500 * time.Millisecond)
	c.Stop()
	mu.Lock()
	defer mu.Unlock()
	after := heard[played:]
	if len(after) < 2*Rate/10 {
		t.Errorf("heard %d frames of silence in half a second after the tone", len(after)/2)
	}
	for _, v := range after[min(len(after), 2*Rate/5):] {
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

// TestCaptureNoProcess fails for a process Core Audio doesn't know.
func TestCaptureNoProcess(t *testing.T) {
	if err := loadTaps(); err != nil {
		t.Skip(err)
	}
	if c, err := startCapture(1<<30, func([]float32) {}, tapMuted); err == nil {
		c.Stop()
		t.Error("captured a process that doesn't run")
	}
}

// TestAsk has TCC ask whether this process may record what other apps
// play, and waits for its answer, which comes at once where macOS can't
// ask, as over SSH, and is remembered once it allows it. It may show
// macOS's question, so it runs only with MIXMUSIC_TCC_ASK=1.
func TestAsk(t *testing.T) {
	if os.Getenv("MIXMUSIC_TCC_ASK") != "1" {
		t.Skip("MIXMUSIC_TCC_ASK=1 has macOS ask")
	}
	if err := loadTaps(); err != nil {
		t.Skip(err)
	}
	answer := make(chan bool, 1)
	go func() { answer <- ask() }()
	select {
	case ok := <-answer:
		t.Logf("allowed: %v; TCC tells %d", ok, permission())
		if !ok && allowed.Load() {
			t.Error("refused, ask remembers it allowed")
		}
	case <-time.After(2 * time.Minute):
		t.Fatal("TCC didn't answer")
	}
}
