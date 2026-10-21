package audio

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio/output"
)

// TestEngineOnTheDevice plays a 3-second track through the device, at
// volume 0, so silently: the time must pass as the clock's, and the track
// end when it should, 3 seconds after it started to play. It opens the
// device, so it runs only with MIXMUSIC_AUDIO=1, as CI's audio jobs on
// Linux and macOS set it.
func TestEngineOnTheDevice(t *testing.T) {
	if os.Getenv("MIXMUSIC_AUDIO") != "1" {
		t.Skip("MIXMUSIC_AUDIO=1 runs the tests that open the audio device")
	}
	start := func(render func([]float32), failed func(error)) (Output, error) {
		return output.Start(render, failed)
	}
	rec := newRecorder()
	e := NewEngine(fileOpener, start, rec)
	defer e.Close()
	e.SetVolume(0)
	e.master = newSmoother(0, glide)
	err := e.Load(cuesFront, 0, true)
	if errors.Is(err, output.ErrNoDevice) || errors.Is(err, output.ErrUnsupported) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	// The track plays once Load returns, which starts the device first: on
	// a slow machine, as CI's macOS runners, that alone took 0.7 seconds,
	// which aren't the track's.
	began := time.Now()
	time.Sleep(time.Second)
	if p := e.Position(); p < 800*time.Millisecond || p > 1200*time.Millisecond {
		t.Errorf("position %v after a second", p)
	}
	select {
	case <-time.After(5 * time.Second):
	case <-func() chan struct{} {
		c := make(chan struct{})
		go func() { rec.waitFor(t, "ended"); close(c) }()
		return c
	}():
	}
	took := time.Since(began)
	if !rec.has("ended") || took < 2800*time.Millisecond || took > 3600*time.Millisecond {
		t.Errorf("ended %v after %v (events %q)", rec.has("ended"), took, rec.events)
	}
	if rec.has("buffering true") {
		t.Errorf("it buffered: %q", rec.events)
	}
	t.Logf("ended after %v; events %q", took.Round(time.Millisecond), rec.events)
}
