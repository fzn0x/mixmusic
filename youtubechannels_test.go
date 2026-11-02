package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// TestYouTubeChannelUnswitchable lists a brand channel YouTube tells no
// page ID of, saying why it can't be chosen; choosing it says so again,
// and the channel used stays.
func TestYouTubeChannelUnswitchable(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{YouTube: app.YouTubeAccount{
		SignedIn: true, Account: &youtube.Account{Name: "Ada", Handle: "@ada"},
	}}}
	s.loads["channels"] = &loaded[[]youtube.AccountChannel]{done: true, value: []youtube.AccountChannel{
		{Name: "Ada", Handle: "@ada", Current: true},
		{Name: "Ada Records", Handle: "@adarecords", PageID: "111"},
		{Name: "Ada Archive", Handle: "@adaarchive", Unswitchable: true},
	}}
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, "/settings/accounts")
	click(t, s, tt, "Ada (@ada)")
	const archive = "Ada Archive (@adaarchive) · YouTube doesn't let MixMusic switch to this channel"
	if !tt.HasText(archive) || !tt.HasText("Ada Records (@adarecords)") {
		t.Fatalf("the channels listed are %q", tt.Texts())
	}
	click(t, s, tt, archive)
	if !tt.HasText("YouTube doesn't let MixMusic switch to Ada Archive") {
		t.Errorf("choosing it isn't told: %q", tt.Texts())
	}
	if s.signingIn != "" || tt.HasText("Using YouTube Music as") {
		t.Error("choosing it switched")
	}
	if _, ok := tt.Find("Ada (@ada)"); !ok || tt.HasText(archive) {
		t.Errorf("the channel shown after choosing it: %q", tt.Texts())
	}
}
