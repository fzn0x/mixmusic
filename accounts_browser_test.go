package main

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/browser"
)

// browserShell is a test shell whose computer has the browsers found, and
// whose sign-in in one ends as signIn says: no browser opens.
func browserShell(t *testing.T, found []browser.Browser, signIn func(ctx context.Context) error) *shell {
	t.Helper()
	return newTestShellOf(t, func(o *app.Options) {
		o.Browsers = &app.BrowserSignIn{
			Find: func() []browser.Browser { return found },
			SignIn: func(ctx context.Context, b browser.Browser, opts browser.Options) ([]*http.Cookie, error) {
				return nil, signIn(ctx)
			},
		}
	})
}

var testBrowsers = []browser.Browser{
	{ID: "firefox", Name: "Firefox", Engine: browser.Firefox, Path: "firefox"},
	{ID: "zen", Name: "Zen", Engine: browser.Firefox, Path: "zen"},
}

// TestBrowserSignIn offers the browsers found, signed out, beside the
// sign-in window; waits for the one chosen with a way to cancel it, which
// says nothing more; and says why a sign-in the user ended by closing the
// browser didn't sign in.
func TestBrowserSignIn(t *testing.T) {
	started := make(chan struct{}, 4)
	closed := false
	s := browserShell(t, testBrowsers, func(ctx context.Context) error {
		started <- struct{}{}
		if closed {
			return browser.ErrClosed
		}
		<-ctx.Done()
		return ctx.Err()
	})
	tt := testerOf(t, s, 1280, 2600)
	show(s, tt, "/settings/accounts")
	for _, want := range []string{"sign in to youtube music", youtubeBrowserTitle, youtubeBrowserAbout, "Firefox", "sign in with Firefox", "Paste a cookie header"} {
		if !tt.HasText(want) {
			t.Errorf("the accounts don't show %q: %q", want, tt.Texts())
		}
	}

	click(t, s, tt, "sign in with Firefox")
	<-started
	if !tt.HasText("waiting for Firefox…") || !tt.HasText("cancel") || s.signingIn != "youtube-browser" {
		t.Fatalf("a sign-in in Firefox waits as %q: %q", s.signingIn, tt.Texts())
	}
	click(t, s, tt, "cancel")
	until(t, s, tt, "the sign-in to end", func() bool { return s.signingIn == "" })
	if tt.HasText("MixMusic didn't sign in to YouTube Music") || tt.HasText("waiting for Firefox…") {
		t.Errorf("a cancelled sign-in shows %q", tt.Texts())
	}

	closed = true
	click(t, s, tt, "sign in with Firefox")
	<-started
	waitText(t, s, tt, "Firefox closed before you signed in to YouTube Music")
	if !tt.HasText("MixMusic didn't sign in to YouTube Music") || s.signingIn != "" {
		t.Errorf("the closed browser isn't told: %q", tt.Texts())
	}
}

// TestBrowserSignInNone says which browsers MixMusic looked for, where it
// found none, and offers no button.
func TestBrowserSignInNone(t *testing.T) {
	s := browserShell(t, nil, func(context.Context) error { t.Error("a browser opened"); return nil })
	tt := testerOf(t, s, 1280, 2600)
	show(s, tt, "/settings/accounts")
	if !tt.HasText(youtubeBrowserNone) {
		t.Errorf("no browser shows %q", tt.Texts())
	}
	for _, text := range tt.Texts() {
		if strings.HasPrefix(text, "sign in with ") && text != "sign in with it" {
			t.Errorf("no browser offers %q", text)
		}
	}
}

// TestSettingsFindBrowserSignIn finds the sign-in through a browser by
// its words, whole, with the browsers to choose from.
func TestSettingsFindBrowserSignIn(t *testing.T) {
	s := browserShell(t, testBrowsers, func(context.Context) error { return nil })
	tt := testerOf(t, s, 1280, 6000)
	for _, q := range []string{"your browser", "temporary profile", "zen", "google refuses"} {
		show(s, tt, settingsPath("", q))
		showsOnly(t, tt, "searching "+q, []string{youtubeBrowserTitle, "sign in with Firefox"}, nil)
	}
	show(s, tt, settingsPath("", "crossfade"))
	showsOnly(t, tt, "searching crossfade", nil, []string{youtubeBrowserTitle})
}
