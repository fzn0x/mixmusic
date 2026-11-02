//go:build (windows || darwin || linux) && (amd64 || arm64)

package output

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// TestStream plays half a second of silence on the default device, in
// stereo and asking for 5.1: render must be called for about as many
// frames as play in that time, of the channels it tells, and never after
// Stop. It opens the device, so it runs only with MIXMUSIC_AUDIO=1.
func TestStream(t *testing.T) {
	if os.Getenv("MIXMUSIC_AUDIO") != "1" {
		t.Skip("MIXMUSIC_AUDIO=1 runs the tests that open the audio device")
	}
	for _, asked := range []int{2, Surround} {
		t.Run(fmt.Sprint(asked, " channels"), func(t *testing.T) { testStream(t, asked) })
	}
}

func testStream(t *testing.T, asked int) {
	var frames, calls, channels atomic.Int64
	var stopped atomic.Bool
	var late atomic.Int64
	render := func(out []float32, ch int) {
		if stopped.Load() {
			late.Add(1)
		}
		if ch != 2 && ch != asked || len(out)%ch != 0 {
			t.Errorf("render of %d samples of %d channels", len(out), ch)
		}
		channels.Store(int64(ch))
		clear(out)
		frames.Add(int64(len(out) / ch))
		calls.Add(1)
	}
	s, err := Start(asked, render, func(err error) { t.Errorf("the stream failed: %v", err) })
	if errors.Is(err, ErrNoDevice) || errors.Is(err, ErrUnsupported) {
		t.Skip("no audio output device")
	}
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	time.Sleep(500 * time.Millisecond)
	delay := s.Delay()
	s.Stop()
	elapsed := time.Since(start)
	stopped.Store(true)
	time.Sleep(50 * time.Millisecond)

	// Rendered: the buffer filled at the start, then what the device played.
	want := elapsed.Seconds() * Rate
	got := float64(frames.Load())
	t.Logf("%d calls of %d channels, %.0f frames in %v (%.0f expected, and a buffer ahead), delay %v", calls.Load(), channels.Load(), got, elapsed, want, delay)
	if got < want*0.9 || got > want*1.1+Rate*bufferDuration.Seconds()*2 {
		t.Errorf("rendered %.0f frames in %v, want about %.0f", got, elapsed, want)
	}
	if delay <= 0 || delay > 4*bufferDuration {
		t.Errorf("delay %v", delay)
	}
	if late.Load() > 0 {
		t.Errorf("render was called %d times after Stop", late.Load())
	}
}

// TestStartStop starts and stops streams in a row, as the player does when
// it pauses and plays: each must open the device again, and end cleanly.
func TestStartStop(t *testing.T) {
	if os.Getenv("MIXMUSIC_AUDIO") != "1" {
		t.Skip("MIXMUSIC_AUDIO=1 runs the tests that open the audio device")
	}
	for i := range 5 {
		var n atomic.Int64
		s, err := Start(2, func(out []float32, _ int) { clear(out); n.Add(1) }, nil)
		if errors.Is(err, ErrNoDevice) || errors.Is(err, ErrUnsupported) {
			t.Skip("no audio output device")
		}
		if err != nil {
			t.Fatalf("start %d: %v", i, err)
		}
		time.Sleep(60 * time.Millisecond)
		s.Stop()
		if n.Load() == 0 {
			t.Errorf("start %d: render was never called", i)
		}
	}
}

// TestRenderPanic checks that a render that panics fails the stream, with
// the panic, instead of crashing the program.
func TestRenderPanic(t *testing.T) {
	if os.Getenv("MIXMUSIC_AUDIO") != "1" {
		t.Skip("MIXMUSIC_AUDIO=1 runs the tests that open the audio device")
	}
	var calls atomic.Int64
	failed := make(chan error, 1)
	s, err := Start(2, func(out []float32, _ int) {
		clear(out)
		if calls.Add(1) == 5 {
			panic("boom")
		}
	}, func(err error) { failed <- err })
	if errors.Is(err, ErrNoDevice) || errors.Is(err, ErrUnsupported) {
		t.Skip("no audio output device")
	}
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-failed:
		t.Logf("failed: %v", err)
	case <-time.After(2 * time.Second):
		t.Error("the stream did not fail")
	}
	s.Stop() // after it failed: returns at once
}
