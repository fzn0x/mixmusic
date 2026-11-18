package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/locales"
)

// TestFailureToastOffersSettings offers Settings › Playback in the toast of
// a track that couldn't play for a reason the playback settings change,
// as one YouTube plays only to signed-in viewers (#577), and details in
// the toast of another.
func TestFailureToastOffersSettings(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 820)
	show(s, tt, "/")
	needs := player.Notice{Message: "Couldn't play Midnight City: YouTube plays this track only to signed-in viewers",
		Error: true, Failure: 1, Settings: true}
	s.notes = append(s.notes, needs)
	tt.Frame()
	if !tt.HasText(needs.Message) || tt.HasText("details") {
		t.Errorf("the toast shows %q", tt.Texts())
	}
	toast := s.failureToastOf(needs)
	if toast.Action != locales.ShellToastSettings() || toast.Type != "error" {
		t.Fatalf("the toast %+v", toast)
	}
	toast.OnAction()
	s.drain()
	tt.Frame()
	if got := s.router.Path(); got != "/settings/playback" {
		t.Errorf("settings opened %q", got)
	}
	if toast := s.failureToastOf(player.Notice{Message: "Couldn't play Broken Song: this track is private", Error: true, Failure: 2}); toast.Action != "details" {
		t.Errorf("the toast of another failure: %+v", toast)
	}
}
