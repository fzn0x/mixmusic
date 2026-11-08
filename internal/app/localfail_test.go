package app

import (
	"context"
	"testing"

	"github.com/fzn0x/mixmusic/internal/audio"
)

// TestEngineSourcesOnThisComputer opens a track on this computer, and a
// download, for the engine as audio on this computer, whose failure to
// read is the file's rather than the network's.
func TestEngineSourcesOnThisComputer(t *testing.T) {
	a, _ := localApp(t)
	scanned(t, a)
	ctx := context.Background()
	local := func(t *testing.T, url string) bool {
		t.Helper()
		src, err := a.openNative(ctx, url, func() bool { return false })
		if err != nil {
			t.Fatal(err)
		}
		defer src.Close()
		l, ok := src.(audio.Local)
		return ok && l.Local()
	}
	tracks := (&LocalFiles{a}).Tracks()
	src, err := (&Player{a}).Prepare(ctx, tracks[0])
	if err != nil {
		t.Fatal(err)
	}
	if !local(t, src.URL) {
		t.Error("a track on this computer isn't local")
	}
	a.proxy.Kept = keptAudio{}
	if !local(t, streamURL("abcdefghijk")) {
		t.Error("a download isn't local")
	}
}
