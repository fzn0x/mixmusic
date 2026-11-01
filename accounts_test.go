package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// TestRefreshKeepsAccounts goes on showing the accounts while the shell
// asks for them again, as it does each time the app tells of a change:
// forgotten, they read as signed out until the answer came, and the
// library's Spotify tab blinked away (#50).
func TestRefreshKeepsAccounts(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{Spotify: app.SpotifyAccount{SignedIn: true}}}
	tt := tester(t, s)
	show(s, tt, "/library")
	if !tt.HasText("spotify") {
		t.Fatalf("the library has no Spotify tab: %q", tt.Texts())
	}
	s.refresh()
	tt.Frame()
	if !tt.HasText("spotify") {
		t.Errorf("asking for the accounts again hides the Spotify tab: %q", tt.Texts())
	}
	// The answer replaces them: the test's app has no Spotify sign-in.
	deadline := time.Now().Add(5 * time.Second)
	for tt.HasText("spotify") {
		if time.Now().After(deadline) {
			t.Fatal("the accounts' answer never showed")
		}
		time.Sleep(10 * time.Millisecond)
		s.drain()
		tt.Frame()
	}
}

// TestRefusedSpotifySignOut offers to sign out of a sign-in Spotify
// refused, which would fail every request of Spotify meanwhile, and asks
// first.
func TestRefusedSpotifySignOut(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{Spotify: app.SpotifyAccount{
		SignedIn: true, Error: "spotify: this Spotify account is not one of the app's users: add it under Users Management",
	}}}
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, "/settings/accounts")
	if !tt.HasText("Spotify refused the sign-in") {
		t.Fatalf("the refusal isn't shown: %q", tt.Texts())
	}
	click(t, s, tt, "sign out")
	if s.asking == nil || s.asking.title != "sign out of Spotify" {
		t.Errorf("signing out asks %+v", s.asking)
	}
}

// TestSpotifySteps asks, before signing in, for what Spotify asks of an
// app in development mode: an owner with Premium, and the account that
// signs in under Users Management.
func TestSpotifySteps(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true}
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, "/settings/accounts")
	texts := strings.Join(tt.Texts(), " ")
	for _, want := range []string{"a Spotify account that has Premium", "under Users Management, add your name", "Client ID here"} {
		if !strings.Contains(texts, want) {
			t.Errorf("the steps don't say %q: %q", want, tt.Texts())
		}
	}
}

// waitText waits for the window to show text, from what the app answers.
func waitText(t *testing.T, s *shell, tt *ui.Tester, text string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !tt.HasText(text) {
		if time.Now().After(deadline) {
			t.Fatalf("the window never showed %q: %q", text, tt.Texts())
		}
		time.Sleep(10 * time.Millisecond)
		s.drain()
		tt.Frame()
	}
}

// TestYouTubeCookieSignIn offers, signed out, to import a browser's
// YouTube Music session or to paste a cookie header, and says above them
// why a try failed. No browser is read: the yt-dlp the app has is not
// there.
func TestYouTubeCookieSignIn(t *testing.T) {
	s := newTestShell(t)
	s.set.YTDLPPath = filepath.Join(t.TempDir(), "no-yt-dlp")
	s.save()
	tt := testerOf(t, s, 1280, 2600)
	show(s, tt, "/settings/accounts")
	for _, want := range []string{"sign in to youtube music", "Import from a browser", "Firefox", "Paste a cookie header"} {
		if !tt.HasText(want) {
			t.Errorf("the accounts don't show %q: %q", want, tt.Texts())
		}
	}

	s.youtubeCookies.pasted = "PREF=f6=40000000; VISITOR_INFO1_LIVE=v"
	tt.Frame()
	click(t, s, tt, "sign in with it")
	if s.youtubeCookies.pasted != "" {
		t.Error("the pasted header stayed in the field")
	}
	waitText(t, s, tt, "That isn't a signed-in YouTube Music cookie header: copy the Cookie header of a request to music.youtube.com, from a tab signed in to it")
	if !tt.HasText("MixMusic didn't sign in to YouTube Music") || s.signingIn != "" {
		t.Errorf("the failure isn't told: %q, waiting for %q", tt.Texts(), s.signingIn)
	}

	click(t, s, tt, "import")
	waitText(t, s, tt, "yt-dlp not found at "+s.set.YTDLPPath)
	if tt.HasText("That isn't a signed-in YouTube Music cookie header") {
		t.Error("the failure of the try before stayed")
	}
}

// TestYouTubeAccountPicker asks for the account and channel of a session
// signed in to several Google accounts, naming each by its address, and
// again when the one used is no longer of the session.
func TestYouTubeAccountPicker(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{YouTube: app.YouTubeAccount{
		SignedIn: true, Account: &youtube.Account{Name: "Ada"},
	}}}
	channels := []youtube.AccountChannel{
		{Name: "Ada", Email: "ada@example.com", Current: true},
		{Name: "Bob's Band", Handle: "@band", PageID: "222", AuthUser: 1, Email: "bob@example.com"},
	}
	s.loads["channels"] = &loaded[[]youtube.AccountChannel]{done: true, value: channels}
	s.youtubePick = true
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, "/settings/accounts")
	for _, want := range []string{"Account and channel", "Ada · ada@example.com", "Your session reaches more than one account or channel. MixMusic uses the first account: choose another here."} {
		if !tt.HasText(want) {
			t.Errorf("the picker doesn't show %q: %q", want, tt.Texts())
		}
	}

	// The channel used is gone: what remains is offered, even alone.
	s.loads["channels"] = &loaded[[]youtube.AccountChannel]{done: true, value: []youtube.AccountChannel{{Name: "Ada", Email: "ada@example.com"}}}
	tt.Frame()
	if !tt.HasText("The account or channel MixMusic used is no longer one of this session: choose one.") || !tt.HasText("Channel") {
		t.Errorf("a channel gone isn't told: %q", tt.Texts())
	}
	s.loads["channels"] = &loaded[[]youtube.AccountChannel]{done: true, value: []youtube.AccountChannel{{Name: "Ada", Current: true}}}
	tt.Frame()
	if tt.HasText("Channel") || tt.HasText("Account and channel") {
		t.Errorf("a session of one channel offers a choice: %q", tt.Texts())
	}
}
