//go:build linux && (amd64 || arm64)

package output

import (
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestStreamMovesBetweenSinks plays silence on a sink of its own, which
// pactl adds, then takes away, then adds again: the stream plays on the
// default sink while its sink is gone, and goes back to it once it's back,
// telling Moved each time. It runs with MIXMUSIC_AUDIO=1 where PulseAudio
// or PipeWire's PulseAudio server and pactl are, as on CI's runner.
func TestStreamMovesBetweenSinks(t *testing.T) {
	if os.Getenv("MIXMUSIC_AUDIO") != "1" {
		t.Skip("MIXMUSIC_AUDIO=1 runs the tests that open the audio device")
	}
	if _, err := exec.LookPath("pactl"); err != nil {
		t.Skip("no pactl")
	}
	if _, ok := pulseSinks(); !ok {
		t.Skip("no PulseAudio server")
	}
	const sink = "mixmusic_test_sink"
	load := func() string {
		out, err := exec.Command("pactl", "load-module", "module-null-sink", "sink_name="+sink,
			"sink_properties=device.description=MixMusic-test").Output()
		if err != nil {
			t.Fatalf("pactl load-module: %v", err)
		}
		return strings.TrimSpace(string(out))
	}
	unload := func(module string) {
		if err := exec.Command("pactl", "unload-module", module).Run(); err != nil {
			t.Errorf("pactl unload-module %s: %v", module, err)
		}
	}
	module := load()
	defer func() {
		if module != "" {
			unload(module)
		}
	}()

	devices, err := Devices()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(devices, func(d Device) bool { return d.ID == sink && d.Name == "MixMusic-test" }) {
		t.Fatalf("the sink isn't among the devices: %+v", devices)
	}

	var mu sync.Mutex
	var told []bool
	var frames atomic.Int64
	s, err := Open(Options{
		Channels: 2,
		Render:   func(out []float32, ch int) { clear(out); frames.Add(int64(len(out) / ch)) },
		Failed:   func(err error) { t.Errorf("the stream failed: %v", err) },
		Device:   sink,
		Moved: func(fallback bool) {
			mu.Lock()
			told = append(told, fallback)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	waitFor := func(want []bool) {
		t.Helper()
		for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
			mu.Lock()
			got := slices.Clone(told)
			mu.Unlock()
			if slices.Equal(got, want) {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("Moved told %v, want %v", got, want)
			}
		}
	}
	// plays waits for the stream to render more: a null sink that was idle
	// takes its stream at its next waking, up to 2 seconds later.
	plays := func(where string) {
		t.Helper()
		before := frames.Load()
		for deadline := time.Now().Add(5 * time.Second); frames.Load() < before+Rate/10; time.Sleep(50 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Errorf("nothing played on %s", where)
				return
			}
		}
	}
	waitFor([]bool{false})
	plays("the sink")

	unload(module)
	module = ""
	waitFor([]bool{false, true})
	plays("the default sink")

	module = load()
	waitFor([]bool{false, true, false})
	plays("the sink once it was back")
}
