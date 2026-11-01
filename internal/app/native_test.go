package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/store"
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
		"http://stream.localhost/live/not a video",
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
	webm, err := os.ReadFile("../../webm/testdata/stereo-cues-front.webm")
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

// TestPrepareStreamOfALiveVideo plays a live video at the address of its
// live stream, which the engine opens as one, with no loudness, from
// yt-dlp's stream and from the one the resolver holds.
func TestPrepareStreamOfALiveVideo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".m3u8") {
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:5\n#EXT-X-MEDIA-SEQUENCE:7\n#EXTINF:5.0,\nseg/7.ts\n")
			return
		}
		fmt.Fprint(w, "ID3 the audio of a segment")
	}))
	defer server.Close()
	var runs atomic.Int32
	r := stream.NewResolver(nil, "")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		runs.Add(1)
		return fmt.Appendf(nil, `{"url": "%s/live/index.m3u8", "protocol": "m3u8_native", "live_status": "is_live"}`, server.URL), nil
	}
	matches, err := store.Open(filepath.Join(t.TempDir(), "matches.json"), map[string]media.Track{})
	if err != nil {
		t.Fatal(err)
	}
	a := &App{resolver: r, cache: stream.NewCache(t.TempDir(), 1<<20), matcher: match.New(nil, matches), proxy: stream.NewProxy(r)}
	a.loudness = openBook(t, filepath.Join(t.TempDir(), "loudness.json"), func(context.Context, string) (float64, error) {
		t.Error("asked how loud a live stream is")
		return 0, stream.ErrNoLoudness
	})
	p := &Player{a: a}
	for range 2 {
		src, err := p.PrepareStream(context.Background(), ytTrack("liveRadio01"))
		if err != nil || !src.Live || src.Cached || !strings.HasSuffix(src.URL, "/live/liveRadio01") {
			t.Fatalf("source %+v, %v", src, err)
		}
		live, err := a.openNative(context.Background(), src.URL, func() bool { return false })
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := live.(audio.Measured); ok {
			t.Error("a live stream opened with a loudness")
		}
		b := make([]byte, 26)
		if _, err := io.ReadFull(live, b); err != nil || string(b) != "ID3 the audio of a segment" {
			t.Errorf("read %q, %v", b, err)
		}
		live.Close()
	}
	if runs.Load() != 1 {
		t.Errorf("%d runs of yt-dlp", runs.Load())
	}
}
