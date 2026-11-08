//go:build (windows || darwin || linux) && (amd64 || arm64)

package output

import (
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestStreamDevices lists the system's output devices, then plays half a
// second of silence on the default one, by its ID, and on a device that
// isn't there, which plays on the default device and says so. It opens the
// device, so it runs only with MIXMUSIC_AUDIO=1, as CI's runners have one
// each: a PulseAudio null sink, and BlackHole.
func TestStreamDevices(t *testing.T) {
	if os.Getenv("MIXMUSIC_AUDIO") != "1" {
		t.Skip("MIXMUSIC_AUDIO=1 runs the tests that open the audio device")
	}
	devices, err := Devices()
	if errors.Is(err, ErrNoDevice) || errors.Is(err, ErrUnsupported) {
		t.Skip("no audio output device")
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) == 0 {
		t.Skip("no audio output device")
	}
	ids := map[string]bool{}
	defaults := 0
	for _, d := range devices {
		t.Logf("%q: %q, default %v", d.ID, d.Name, d.Default)
		if d.ID == "" || d.Name == "" || ids[d.ID] {
			t.Errorf("device %+v", d)
		}
		ids[d.ID] = true
		if d.Default {
			defaults++
		}
	}
	if defaults > 1 {
		t.Errorf("%d default devices", defaults)
	}
	// ALSA marks no device the default: its first plays.
	chosen := devices[0]
	for _, d := range devices {
		if d.Default {
			chosen = d
		}
	}
	t.Run("the default device", func(t *testing.T) { testStreamOn(t, chosen.ID, false) })
	t.Run("a device that isn't there", func(t *testing.T) { testStreamOn(t, "mixmusic-test-no-such-device", true) })
}

// testStreamOn plays half a second of silence on the device id, which
// tells Moved as it starts whether it plays on the default device in its
// place.
func testStreamOn(t *testing.T, id string, fallback bool) {
	var frames atomic.Int64
	var mu sync.Mutex
	var told []bool
	s, err := Open(Options{
		Channels: 2,
		Render: func(out []float32, ch int) {
			clear(out)
			frames.Add(int64(len(out) / ch))
		},
		Failed: func(err error) { t.Errorf("the stream failed: %v", err) },
		Device: id,
		Moved: func(fb bool) {
			mu.Lock()
			told = append(told, fb)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	s.Stop()
	mu.Lock()
	defer mu.Unlock()
	if len(told) == 0 || told[0] != fallback {
		t.Errorf("Moved told %v as the stream started, want %v", told, fallback)
	}
	if len(told) > 1 {
		t.Errorf("Moved told %v, as if the stream moved", told)
	}
	if n := frames.Load(); n < Rate/4 {
		t.Errorf("rendered %d frames in half a second", n)
	}
}
