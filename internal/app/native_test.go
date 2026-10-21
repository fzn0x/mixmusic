package app

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/fzn0x/mixmusic/internal/stream"
)

// TestNativeAnalysisOfSilence checks that the analysis of silence, whose
// decibels are -Inf, which JSON can't carry, reaches the page.
func TestNativeAnalysisOfSilence(t *testing.T) {
	a := &App{}
	p := &Player{a: a}
	got := p.NativeAnalyser(true, true)
	if _, err := json.Marshal(got); err != nil {
		t.Fatalf("the analysis doesn't encode: %v", err)
	}
	if a.nativeEngine() != nil {
		if len(got.Bytes) != 128 || len(got.Decibels) != 128 || len(got.Waveform) != 256 {
			t.Errorf("%d bytes, %d decibels, %d samples", len(got.Bytes), len(got.Decibels), len(got.Waveform))
		}
		for _, db := range got.Decibels {
			if db != -1000 {
				t.Fatalf("silence at %v dB", db)
			}
		}
		a.native.Close()
	}
}

func TestOpenNativeTakesStreamURLs(t *testing.T) {
	a := &App{proxy: stream.NewProxy(stream.NewResolver(nil, ""))}
	never := func() bool { return false }
	for _, url := range []string{
		"http://stream.localhost/somewhere/w3XzgjlKFjc",
		"http://stream.localhost/yt/not a video",
		"http://stream.localhost/yt/",
		"::::",
	} {
		if _, err := a.openNative(context.Background(), url, never); err == nil {
			t.Errorf("%q opened", url)
		}
	}
}
