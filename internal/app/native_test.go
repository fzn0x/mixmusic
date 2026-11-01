package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/fzn0x/mixmusic/internal/audio"
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

// TestOpenNativeMeasures gives the engine the loudness of the tracks
// MixMusic knows, from the cache too.
func TestOpenNativeMeasures(t *testing.T) {
	webm, err := os.ReadFile("../webm/testdata/stereo-cues-front.webm")
	if err != nil {
		t.Fatal(err)
	}
	cache := stream.NewCache(t.TempDir(), 1<<20)
	for _, id := range []string{"w3XzgjlKFjc", "3IUqoyTxEU8"} {
		if err := os.WriteFile(filepath.Join(cache.Dir, id+".webm"), webm, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a := &App{proxy: stream.NewProxy(stream.NewResolver(nil, ""))}
	a.proxy.Cache = cache
	a.loudness = openBook(t, filepath.Join(t.TempDir(), "loudness.json"), nil)
	a.loudness.keep("w3XzgjlKFjc", -9.5)
	never := func() bool { return false }

	src, err := a.openNative(context.Background(), "http://stream.localhost/yt/w3XzgjlKFjc", never)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	if m, ok := src.(audio.Measured); !ok {
		t.Error("a track of known loudness opened without it")
	} else if v, ok := m.Loudness(); !ok || v != -9.5 {
		t.Errorf("loudness %v, %v", v, ok)
	}
	other, err := a.openNative(context.Background(), "http://stream.localhost/yt/3IUqoyTxEU8", never)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, ok := other.(audio.Measured); ok {
		t.Error("a track of unknown loudness opened with one")
	}
}
