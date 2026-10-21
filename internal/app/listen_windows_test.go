//go:build windows && (amd64 || arm64)

package app

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/audio/output"
	"github.com/fzn0x/mixmusic/internal/player"
)

// TestProcesses finds this process among those that run, by its name.
func TestProcesses(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range processes() {
		if p.pid == uint32(os.Getpid()) {
			if !strings.EqualFold(p.name, filepath.Base(exe)) {
				t.Errorf("this process is named %q, not %q", p.name, filepath.Base(exe))
			}
			return
		}
	}
	t.Errorf("this process, %d, isn't among those that run", os.Getpid())
}

// TestListenerHears has the listener hear this process play a tone at
// -100 dB, which no one hears, into its tap, and stop when it hears
// nothing. It opens the audio device, so it runs only with MIXMUSIC_AUDIO=1.
func TestListenerHears(t *testing.T) {
	if os.Getenv("MIXMUSIC_AUDIO") != "1" {
		t.Skip("MIXMUSIC_AUDIO=1 runs the tests that open the audio device")
	}
	phase := 0
	s, err := output.Start(func(out []float32) {
		for i := 0; i < len(out); i += 2 {
			v := float32(1e-5 * math.Sin(2*math.Pi*1500*float64(phase)/output.Rate))
			out[i], out[i+1] = v, v
			phase++
		}
	}, nil)
	if errors.Is(err, output.ErrNoDevice) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	l := newListener()
	l.follow(player.State{Playing: true, Source: &player.Source{Mode: player.ModeSpotify}})
	heard := false
	var a audio.Analysis
	for deadline := time.Now().Add(3 * time.Second); !heard && time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		l.tap.Analyse(&a)
		for _, db := range a.Decibels {
			heard = heard || db > -200
		}
	}
	if !heard {
		t.Fatal("the listener heard nothing of the tone")
	}
	if l.failed {
		t.Error("a capture failed")
	}
	l.follow(player.State{Source: &player.Source{Mode: player.ModeSpotify}}) // paused
	for deadline := time.Now().Add(3 * time.Second); len(l.wake) > 0 && time.Now().Before(deadline); {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond) // the worker stops the capture
	n := a
	l.tap.Analyse(&n)
	if n.Waveform != [audio.FFTSize]float32{} {
		t.Error("once the listener hears nothing, the tap still holds the tone")
	}
}
