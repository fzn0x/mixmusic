package stream

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestLiveCaptions reads the captions of a music video YouTube has
// uploaded captions of, signed out as the visionOS client, without a
// proof-of-origin token: YouTube changes what it asks of clients from time
// to time. It runs with MIXMUSIC_LIVE=1:
//
//	MIXMUSIC_LIVE=1 go test ./internal/stream -run LiveCaptions -v
func TestLiveCaptions(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to test against YouTube")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	d := NewDirect()
	// The music video of Never Gonna Give You Up, captioned in several
	// languages.
	caps, err := d.Captions(ctx, "dQw4w9WgXcQ")
	if err != nil {
		t.Fatal(err)
	}
	var made *CaptionTrack
	for i, tr := range caps.Tracks {
		if !tr.Speech && made == nil {
			made = &caps.Tracks[i]
		}
	}
	if made == nil || caps.Length < time.Minute {
		t.Fatalf("captions %+v", caps)
	}
	lines, err := d.CaptionText(ctx, *made)
	if err != nil || len(lines) < 10 {
		t.Fatalf("%d lines of %s, %v", len(lines), made.Language, err)
	}
	t.Logf("%d tracks, %d lines of %s, the first at %d ms", len(caps.Tracks), len(lines), made.Language, lines[0].StartMs)
}
