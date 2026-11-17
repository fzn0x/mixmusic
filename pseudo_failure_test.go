package main

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/locales"
)

// TestPseudoLocaleFailureDetails shows a track that couldn't play in
// Settings › About, and its details, in en-XA: the ways tried are the
// language's, and only why each failed is the error's own words.
func TestPseudoLocaleFailureDetails(t *testing.T) {
	usePseudoLocale(t)
	s, tt := failedShell(t)
	tt.SetLocale("en-XA")
	tt.SetSize(1280, 2400)
	show(s, tt, "/settings/about")
	words := []string{"Broken Song", "The Band", "YouTube Music",
		"YouTube's player API answered LOGIN_REQUIRED: Sign in to confirm your age", "this track is private"}
	checkPseudo(t, "Settings › About", tt.Texts(), words...)
	click(t, s, tt, locales.PlaybackErrorsDetailsOf(locales.FailureTrackBy("Broken Song", "The Band")))
	tt.Advance(time.Second)
	if !tt.HasText(locales.FailureTried()) {
		t.Fatalf("no details: %q", tt.Texts())
	}
	checkPseudo(t, "The failure's details", tt.Texts(), words...)
}

// TestFailureWays names each way the player tries in the language.
func TestFailureWays(t *testing.T) {
	usePseudoLocale(t)
	for _, way := range []string{"the file", "finding it on YouTube Music", "finding its stream", "Spotify in MixMusic",
		"the live stream", "the download, or the offline cache", "the stream", "the stream, tried again",
		"its 5.1 surround audio", "YouTube's player API"} {
		if got := failureWay(way); got == "" || got[0] != '[' {
			t.Errorf("%q says %q, which no catalog translates", way, got)
		}
	}
	// Names are as they are.
	for _, way := range []string{"yt-dlp", "Spotify Connect"} {
		if got := failureWay(way); got != way {
			t.Errorf("%q says %q", way, got)
		}
	}
}
