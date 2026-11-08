package app

import (
	"context"
	"errors"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio/output"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestOutputDevicesListed lists the output devices the system tells of.
func TestOutputDevicesListed(t *testing.T) {
	want := []OutputDevice{{ID: "hp", Name: "Headphones"}, {ID: "sp", Name: "Speakers", Default: true}}
	a := newQuietApp(t, func(o *Options) {
		o.OutputDevices = func() ([]OutputDevice, error) { return want, nil }
	})
	got, err := (&Player{a}).OutputDevices(context.Background())
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("listed %+v, %v", got, err)
	}
}

// TestOutputNotes tells the windows once when the device chosen isn't
// there and the system's default plays, and once when it plays there
// again; not for an output of a device chosen before, and not again after
// the user chooses another device.
func TestOutputNotes(t *testing.T) {
	a := newQuietApp(t, nil)
	var mu sync.Mutex
	var notes []string
	stop := a.OnOutputDevice(func(note string) {
		mu.Lock()
		notes = append(notes, note)
		mu.Unlock()
	})
	defer stop()
	choose := func(d settings.OutputDevice) {
		t.Helper()
		st := a.settings.Get()
		st.OutputDevice = d
		if _, err := (&Preferences{a}).Set(st); err != nil {
			t.Fatal(err)
		}
	}
	headphones := settings.OutputDevice{ID: "hp", Name: "Headphones"}
	choose(headphones)
	if got := a.chosenOutput(); got != headphones {
		t.Fatalf("the engine opens %+v", got)
	}
	p := &Player{a}
	a.outputMoved(headphones, false)
	a.outputMoved(headphones, true)
	a.outputMoved(headphones, true)
	if !p.OutputFallback() {
		t.Error("no fallback while the headphones aren't there")
	}
	a.outputMoved(settings.OutputDevice{ID: "old"}, false)
	a.outputMoved(headphones, false)
	want := []string{
		"Headphones isn't available: playing on the system's default device until it's back",
		"Playing on Headphones again",
	}
	mu.Lock()
	if !slices.Equal(notes, want) {
		t.Errorf("notes %q, want %q", notes, want)
	}
	notes = nil
	mu.Unlock()

	a.outputMoved(headphones, true)
	choose(settings.OutputDevice{ID: "sp", Name: "Speakers"})
	if p.OutputFallback() {
		t.Error("still a fallback after choosing another device")
	}
	a.outputMoved(headphones, false) // the output of the headphones, stopping
	mu.Lock()
	if len(notes) != 1 {
		t.Errorf("notes %q", notes)
	}
	mu.Unlock()
	if got := outputNote("", true); got != "The output device chosen isn't available: playing on the system's default device until it's back" {
		t.Errorf("a device of no name: %q", got)
	}
}

// TestStartNativeOnAGoneDevice starts the engine's output on a device that
// isn't there, which plays silence on the default device and says so. It
// opens the audio device, so it runs only with MIXMUSIC_AUDIO=1.
func TestStartNativeOnAGoneDevice(t *testing.T) {
	if os.Getenv("MIXMUSIC_AUDIO") != "1" {
		t.Skip("MIXMUSIC_AUDIO=1 runs the tests that open the audio device")
	}
	a := newQuietApp(t, nil)
	gone := settings.OutputDevice{ID: "mixmusic-test-no-such-device", Name: "Gone"}
	a.output.chosen = gone
	notes := make(chan string, 4)
	defer a.OnOutputDevice(func(note string) { notes <- note })()
	out, err := a.startNative(2, func(out []float32, _ int) { clear(out) }, nil)
	if errors.Is(err, output.ErrNoDevice) || errors.Is(err, output.ErrUnsupported) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer out.Stop()
	select {
	case note := <-notes:
		if note != outputNote("Gone", true) {
			t.Errorf("note %q", note)
		}
	case <-time.After(5 * time.Second):
		t.Error("no note that the device is gone")
	}
}
