package main

import (
	"context"
	"os"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/locales"
)

// TestSpotifySPDCPasteUI checks the Spotify Sign-in method switcher, sp_dc paste
// validation error handling, and signed-in method badges in Settings › Accounts.
func TestSpotifySPDCPasteUI(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 2600)
	show(s, tt, "/settings/accounts")
	for _, want := range []string{"Sign-in method", "Sign in in a window", "open Spotify sign-in", "Paste your sp_dc cookie", "use sp_dc cookie"} {
		if !tt.HasText(want) {
			t.Errorf("accounts doesn't show %q: %q", want, tt.Texts())
		}
	}
	if tt.HasText("Client ID") {
		t.Errorf("accounts showed Client ID while in sp_dc mode: %q", tt.Texts())
	}

	s.spotifyCookies.pasted = "bad"
	tt.Frame()
	click(t, s, tt, "use sp_dc cookie")
	if s.spotifyCookies.pasted != "" {
		t.Error("pasted sp_dc stayed in field")
	}
	waitText(t, s, tt, "That isn't a Spotify sp_dc cookie: paste the sp_dc value or a Cookie header from open.spotify.com.")
	if !tt.HasText("MixMusic didn't sign in to Spotify") || s.signingIn != "" {
		t.Errorf("expected Spotify error alert, got %q (signingIn=%q)", tt.Texts(), s.signingIn)
	}

	// Signed in as a web session, shows the Web session (sp_dc) badge and description.
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{
		Spotify: app.SpotifyAccount{SignedIn: true, WebSession: true, Profile: &spotify.Profile{ID: "u1", Name: "Web User"}},
	}}
	tt.Frame()
	for _, want := range []string{locales.AccountSettingsSpotifyMethodWeb(), locales.AccountSettingsSpotifyWebSignedInDesc()} {
		if !tt.HasText(want) {
			t.Errorf("expected web-session signed-in text %q, got %q", want, tt.Texts())
		}
	}

	// Signed in with a Developer app, shows the Client ID (Developer API) badge and description.
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{
		Spotify: app.SpotifyAccount{SignedIn: true, WebSession: false, Profile: &spotify.Profile{ID: "u2", Name: "Dev User"}},
	}}
	tt.Frame()
	for _, want := range []string{locales.AccountSettingsSpotifyMethodClientID(), locales.AccountSettingsSpotifyDesc()} {
		if !tt.HasText(want) {
			t.Errorf("expected client-id signed-in text %q, got %q", want, tt.Texts())
		}
	}
}

// TestRenderSpotifySPDC captures screenshots of Settings › Accounts › Spotify
// in both signed-out modes and both signed-in states when MIXMUSIC_RENDER is set.
func TestRenderSpotifySPDC(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	s := browserShell(t, testBrowsers, func(context.Context) error { return nil })
	ttTall := testerOf(t, s, 1280, 1680)
	show(s, ttTall, "/settings/accounts")
	renderTo(t, s, ttTall, dir, "01-spotify-spdc-signin")

	click(t, s, ttTall, s.lk.sentence(s.lk.words(locales.AccountSettingsSpotifyMethodClientID())))
	ttClient := testerOf(t, s, 1280, 1120)
	show(s, ttClient, "/settings/accounts")
	renderTo(t, s, ttClient, dir, "02-spotify-clientid-signin")

	ttSignedIn := testerOf(t, s, 1280, 820)
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{
		Spotify: app.SpotifyAccount{SignedIn: true, WebSession: true, Profile: &spotify.Profile{ID: "u1", Name: "Ada"}},
	}}
	show(s, ttSignedIn, "/settings/accounts")
	renderTo(t, s, ttSignedIn, dir, "03-spotify-signedin-spdc")

	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{
		Spotify: app.SpotifyAccount{SignedIn: true, WebSession: false, Profile: &spotify.Profile{ID: "u2", Name: "Ada"}},
	}}
	show(s, ttSignedIn, "/settings/accounts")
	renderTo(t, s, ttSignedIn, dir, "04-spotify-signedin-clientid")
}
