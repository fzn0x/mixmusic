//go:build darwin && (amd64 || arm64)

package app

import (
	"errors"
	"os"
	"testing"

	"github.com/fzn0x/mixmusic/internal/audio/output"
)

// TestProcessesOfCoreAudio finds this process among Core Audio's once it
// plays, with the process macOS holds responsible for it as its parent. It
// opens the audio device, so it runs only with MIXMUSIC_AUDIO=1.
func TestProcessesOfCoreAudio(t *testing.T) {
	if os.Getenv("MIXMUSIC_AUDIO") != "1" {
		t.Skip("MIXMUSIC_AUDIO=1 runs the tests that open the audio device")
	}
	s, err := output.Start(2, func(out []float32, _ int) { clear(out) }, nil)
	if errors.Is(err, output.ErrNoDevice) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	self := uint32(os.Getpid())
	owner := responsible(self)
	if owner == 0 {
		t.Fatal("macOS holds no process responsible for this one")
	}
	for _, p := range processes() {
		if p.pid == self {
			if p.parent != owner {
				t.Errorf("this process's parent is %d, not %d, which macOS holds responsible for it", p.parent, owner)
			}
			return
		}
	}
	t.Errorf("this process, %d, isn't among Core Audio's", self)
}

// TestWebViewAudioOfOthers finds no GPU process of MixMusic's among those
// of other apps.
func TestWebViewAudioOfOthers(t *testing.T) {
	self := responsible(uint32(os.Getpid()))
	procs := []proc{
		{pid: 50, parent: self + 1, name: "com.apple.WebKit.GPU"}, // Safari's
		{pid: 51, parent: 0, name: "com.apple.WebKit.GPU"},        // whose owner macOS doesn't tell
	}
	if got := webViewAudio(procs); got != 0 {
		t.Errorf("took process %d for MixMusic's", got)
	}
	if self == 0 {
		t.Skip("macOS tells no process responsible for this one")
	}
	procs = append(procs, proc{pid: 52, parent: self, name: "com.apple.WebKit.GPU"})
	if got := webViewAudio(procs); got != 52 {
		t.Errorf("MixMusic's GPU process is %d, want 52", got)
	}
}
