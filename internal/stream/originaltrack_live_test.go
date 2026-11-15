package stream

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestLiveYTDLPOriginalTrack runs yt-dlp, signed out, over a video dubbed
// in 23 languages with each quality's selector, and checks it prints the
// original track's format, of language preference 10. It runs with
// MIXMUSIC_LIVE=1 and a yt-dlp on the PATH:
//
//	MIXMUSIC_LIVE=1 go test ./internal/stream -run LiveYTDLPOriginalTrack -v
func TestLiveYTDLPOriginalTrack(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to test against YouTube")
	}
	path, err := exec.LookPath("yt-dlp")
	if err != nil {
		t.Skip("no yt-dlp on the PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	for _, q := range []Quality{QualityBest, QualitySaver} {
		out, err := exec.CommandContext(ctx, path, "--no-playlist", "-f", formats[q],
			"--print", "%(format_id)s %(language)s %(language_preference)s",
			"--", "https://music.youtube.com/watch?v=0e3GPea1Tyg").Output()
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		got := strings.TrimSpace(string(out))
		t.Logf("%s: %s", q, got)
		if !strings.HasSuffix(got, " 10") {
			t.Errorf("%s: picked %q, want a track of language preference 10", q, got)
		}
	}
}
