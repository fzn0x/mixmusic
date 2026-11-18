package main

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/locales"
)

// TestPseudoLocaleFailureDetails shows a track that couldn't play in
// Settings › About, and its details, in en-XA: the ways tried are the
// language's, and why each failed too, when MixMusic knows the error, as
// the stream's of no audio; YouTube's own words are as it says them
// (#800).
func TestPseudoLocaleFailureDetails(t *testing.T) {
	usePseudoLocale(t)
	s, tt := failedShellOf(t, bothFailed{loginRequired, stream.ErrNoAudio})
	tt.SetLocale("en-XA")
	tt.SetSize(1280, 2400)
	show(s, tt, "/settings/about")
	if !tt.HasText(" · " + locales.ErrorStreamNoAudio()) {
		t.Errorf("the failure's row doesn't tell why in the language: %q", tt.Texts())
	}
	words := []string{"Broken Song", "The Band", "YouTube Music", loginRequired.Error()}
	checkPseudo(t, "Settings › About", tt.Texts(), words...)
	click(t, s, tt, locales.PlaybackErrorsDetailsOf(locales.FailureTrackBy("Broken Song", "The Band")))
	tt.Advance(time.Second)
	if !tt.HasText(locales.FailureTried()) {
		t.Fatalf("no details: %q", tt.Texts())
	}
	if step := locales.FailureStepNumbered(2, locales.FailureStep("yt-dlp", locales.ErrorStreamNoAudio())); !tt.HasText(step) {
		t.Errorf("the details don't show %q: %q", step, tt.Texts())
	}
	checkPseudo(t, "The failure's details", tt.Texts(), words...)
}

// TestFailureWays names each way the player tries in the language.
func TestFailureWays(t *testing.T) {
	usePseudoLocale(t)
	for _, way := range []string{"the file", "finding it on YouTube Music", "finding its stream", "Spotify in MixMusic",
		"the live stream", "the download, or the offline cache", "the stream", "the stream, tried again",
		"its 5.1 surround audio", "YouTube's player API", "as your account", "as your account, tried again"} {
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
