package app

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/discord"
	"github.com/fzn0x/mixmusic/internal/media"
)

func TestActivityOf(t *testing.T) {
	start := time.Date(2026, 10, 19, 20, 0, 0, 0, time.UTC)
	track := media.Track{
		ID:         "yt:a",
		Title:      "Let It Happen",
		Artists:    []media.Ref{{Name: "Tame Impala"}},
		Album:      &media.Ref{Name: "Currents"},
		DurationMs: 467000,
		Artwork: []media.Image{
			{URL: "https://example.com/120.jpg", Width: 120},
			{URL: "https://example.com/544.jpg", Width: 544},
			{URL: "https://example.com/1200.jpg", Width: 1200},
		},
	}
	a := activityOf(track, start, 0, false)
	if a.Type != discord.Listening || a.Details != "Let It Happen" || a.State != "Tame Impala" || a.LargeText != "Currents" || a.StatusDisplay != discord.ShowName {
		t.Errorf("activity = %+v", a)
	}
	if !a.Start.Equal(start) || !a.End.Equal(start.Add(467*time.Second)) {
		t.Errorf("times = %v to %v", a.Start, a.End)
	}
	if a.LargeImage != "https://example.com/544.jpg" {
		t.Errorf("artwork = %q, want the one of about 512 pixels", a.LargeImage)
	}
	if b := activityOf(media.Track{Title: "Live"}, start, 0, false); !b.End.IsZero() {
		t.Errorf("a track of unknown length has an end: %v", b.End)
	}
	// A live stream, whatever its catalog says, tells how long it played.
	if b := activityOf(track, start, 0, true); !b.End.IsZero() || !b.Start.Equal(start) {
		t.Errorf("a live stream from %v to %v", b.Start, b.End)
	}
}

func TestPresenceArtwork(t *testing.T) {
	if got := largeArtwork([]media.Image{{URL: "http://plain.example/a.jpg", Width: 512}}); got != "" {
		t.Errorf("plain http artwork used: %q", got)
	}
	got := largeArtwork([]media.Image{{URL: "https://a/60", Width: 60}, {URL: "https://a/226", Width: 226}})
	if got != "https://a/226" {
		t.Errorf("small artwork: %q, want the largest", got)
	}
}
