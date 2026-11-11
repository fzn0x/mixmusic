package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
)

// TestAudioAsAccountRow turns Play audio as your account on and off from
// Settings › Playback › Sound, beside Quality: off at first, saved as
// audioAsAccount, its words telling what it risks, and asking to sign in
// to YouTube Music first while signed out. A search for "premium" finds
// it.
func TestAudioAsAccountRow(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 3200)
	show(s, tt, "/settings/playback")
	signedOut := audioAsAccountDescription + " Sign in to YouTube Music in accounts first."
	if !tt.HasText(signedOut) {
		t.Errorf("signed out, the row doesn't say what it risks and asks to sign in: %q", tt.Texts())
	}
	var above float32
	for _, text := range []string{"Quality", audioAsAccountTitle, "Crossfade"} {
		r, ok := tt.Find(text)
		if !ok || r.Y <= above {
			t.Fatalf("Settings › Playback shows %q out of place, at %v: %q", text, r, tt.Texts())
		}
		above = r.Y
	}
	if s.svc.Preferences.Get().AudioAsAccount {
		t.Fatal("on at first")
	}
	focused := func() bool { return tt.Focused(audioAsAccountTitle) && !tt.Focused(signedOut) }
	for i := 0; i < 200 && !focused(); i++ {
		tt.Key(0, ui.KeyTab)
	}
	if !focused() {
		t.Fatalf("Tab doesn't reach the switch: %q", tt.Texts())
	}
	for _, want := range []bool{true, false} {
		tt.Key(0, ui.KeySpace)
		s.drain()
		tt.Frame()
		if got := s.svc.Preferences.Get().AudioAsAccount; got != want {
			t.Errorf("the switch saved audioAsAccount %v, not %v", got, want)
		}
	}

	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{YouTube: app.YouTubeAccount{SignedIn: true}}}
	tt.Frame()
	if !tt.HasText(audioAsAccountDescription) || tt.HasText(signedOut) {
		t.Errorf("signed in, the row asks to sign in: %q", tt.Texts())
	}
	for _, q := range []string{"premium", "bitrate", "as your account"} {
		show(s, tt, settingsPath("", q))
		if !tt.HasText(audioAsAccountTitle) {
			t.Errorf("searching %q doesn't find the row: %q", q, tt.Texts())
		}
	}
}

// premiumShell is a test shell whose accounts tell that the YouTube Music
// account has Premium, when premium is set.
func premiumShell(t *testing.T, premium bool) *shell {
	s := newTestShell(t)
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{YouTube: app.YouTubeAccount{SignedIn: true, Premium: premium}}}
	return s
}

// TestAudioAsAccountOffer offers once, as the accounts tell the account has
// Premium, to play audio as the account: the offer says what it risks, and
// turns the switch on only when asked. Not now, Escape, or a Space pressed
// for the music answer it too, leaving the switch off; it never shows
// again.
func TestAudioAsAccountOffer(t *testing.T) {
	for _, answer := range []string{"turn on", "not now", "escape", "space"} {
		s := premiumShell(t, true)
		tt := tester(t, s)
		tt.Frame()
		if !tt.HasText(audioAsAccountOfferText) || !tt.HasText("play audio as your account") {
			t.Fatalf("%s: no offer: %q", answer, tt.Texts())
		}
		if !tt.Focused("not now") {
			t.Errorf("%s: not now hasn't the focus", answer)
		}
		switch answer {
		case "escape":
			tt.Key(0, ui.KeyEscape)
		case "space":
			tt.Key(0, ui.KeySpace)
		default:
			click(t, s, tt, answer)
		}
		s.drain()
		tt.Frame()
		set := s.svc.Preferences.Get()
		if set.AudioAsAccount != (answer == "turn on") || !set.AudioAsAccountOffered {
			t.Errorf("%s: audioAsAccount %v, offered %v", answer, set.AudioAsAccount, set.AudioAsAccountOffered)
		}
		tt.Frame()
		if tt.HasText(audioAsAccountOfferText) {
			t.Errorf("%s: the offer shows again", answer)
		}
		// Turned off afterwards, it's not offered again.
		s.set.AudioAsAccount = false
		s.save()
		tt.Frame()
		if tt.HasText(audioAsAccountOfferText) {
			t.Errorf("%s: the offer shows again once the switch is off", answer)
		}
	}
}

// TestAudioAsAccountNotOffered offers nothing of an account without
// Premium, nor while the switch is on already, and waits while another
// dialog asks something.
func TestAudioAsAccountNotOffered(t *testing.T) {
	s := premiumShell(t, false)
	tt := tester(t, s)
	tt.Frame()
	if tt.HasText(audioAsAccountOfferText) {
		t.Error("offered without Premium")
	}

	s = premiumShell(t, true)
	s.set.AudioAsAccount = true
	s.save()
	tt = tester(t, s)
	tt.Frame()
	if tt.HasText(audioAsAccountOfferText) {
		t.Error("offered while on")
	}

	s = premiumShell(t, true)
	asked := false
	s.ask("another question", "Something else first.", "ok", func() { asked = true })
	tt = tester(t, s)
	tt.Frame()
	if tt.HasText(audioAsAccountOfferText) || !tt.HasText("Something else first.") {
		t.Fatalf("the offer didn't wait for the other question: %q", tt.Texts())
	}
	click(t, s, tt, "ok")
	tt.Frame()
	if !asked || !tt.HasText(audioAsAccountOfferText) {
		t.Errorf("the offer didn't come after the other question, answered %v: %q", asked, tt.Texts())
	}
}
