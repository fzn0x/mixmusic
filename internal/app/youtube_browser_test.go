package app

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/browser"
)

// fakeBrowsers has a find the browsers found, and sign in with one as
// signIn says, in place of a browser.
func fakeBrowsers(a *App, found []browser.Browser, signIn func(ctx context.Context, b browser.Browser, opts browser.Options) ([]*http.Cookie, error)) {
	a.opts.Browsers = &BrowserSignIn{Find: func() []browser.Browser { return found }, SignIn: signIn}
}

var foundBrowsers = []browser.Browser{
	{ID: "edge", Name: "Edge", Engine: browser.Chromium, Path: "msedge"},
	{ID: "zen", Name: "Zen", Engine: browser.Firefox, Path: "zen"},
}

// TestYouTubeSignInBrowsers lists the browsers found, by their names.
func TestYouTubeSignInBrowsers(t *testing.T) {
	a, _ := cookieApp(t, signedIn(true))
	fakeBrowsers(a, foundBrowsers, nil)
	got := (&Accounts{a}).YouTubeSignInBrowsers()
	if want := []YouTubeBrowser{{ID: "edge", Name: "Edge"}, {ID: "zen", Name: "Zen"}}; !slices.Equal(got, want) {
		t.Errorf("listed %v, want %v", got, want)
	}
}

// TestBrowserLogin signs in with the session of the browser's profile, at
// YouTube Music's sign-in, once it holds YouTube's signing cookies, of
// which MixMusic keeps those YouTube Music sends: not Google's, those that
// expired, or of another path.
func TestBrowserLogin(t *testing.T) {
	a, _ := cookieApp(t, signedIn(true))
	past, future := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	profile := []*http.Cookie{
		{Name: "SAPISID", Value: "google", Domain: ".google.com", Path: "/"},
		{Name: "SAPISID", Value: "yt-sapisid", Domain: ".youtube.com", Path: "/", Secure: true, Expires: future},
		{Name: "__Secure-3PAPISID", Value: "yt-3p", Domain: ".youtube.com", Path: "/", Secure: true, HttpOnly: true},
		{Name: "HOSTONLY", Value: "music-only", Domain: "music.youtube.com", Path: "/"},
		{Name: "WWW", Value: "www-only", Domain: "www.youtube.com", Path: "/"},
		{Name: "OLD", Value: "expired", Domain: ".youtube.com", Path: "/", Expires: past},
		{Name: "DEEP", Value: "a path", Domain: ".youtube.com", Path: "/youtubei"},
	}
	var opened browser.Browser
	fakeBrowsers(a, foundBrowsers, func(ctx context.Context, b browser.Browser, opts browser.Options) ([]*http.Cookie, error) {
		opened = b
		if opts.URL != youtubeSignInURL {
			t.Errorf("the browser opened at %q", opts.URL)
		}
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > browserSignInTimeout {
			t.Errorf("the sign-in waits until %v", deadline)
		}
		// Google's session alone isn't YouTube Music's yet.
		if opts.SignedIn(profile[:1]) || !opts.SignedIn(profile) {
			t.Error("the sign-in isn't told by YouTube's signing cookies")
		}
		return profile, nil
	})
	status, err := (&Accounts{a}).YouTubeBrowserLogin(context.Background(), "zen")
	if err != nil {
		t.Fatal(err)
	}
	if opened.ID != "zen" || !status.YouTube.SignedIn || status.YouTube.Account == nil || status.YouTube.Account.Name != "Ada" {
		t.Errorf("signed in with %q as %+v", opened.ID, status.YouTube)
	}
	kept, err := a.youtubeStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"SAPISID=yt-sapisid", "__Secure-3PAPISID=yt-3p", "HOSTONLY=music-only", "SIDCC=changed"}
	if got := cookieNames(kept.cookies); !slices.Equal(got, want) {
		t.Errorf("kept %q, want %q", got, want)
	}
}

// TestBrowserLoginEnds says why a sign-in through a browser ended without
// signing in, in the words of the interface, and signs nothing in.
func TestBrowserLoginEnds(t *testing.T) {
	for _, c := range []struct {
		name, id string
		err      error
		cancel   bool
		want     string
	}{
		{name: "closed", id: "edge", err: browser.ErrClosed, want: "Edge closed before you signed in to YouTube Music"},
		{name: "time is up", id: "edge", err: context.DeadlineExceeded, want: "signing in to YouTube Music took too long; try again"},
		{name: "doesn't open", id: "zen", err: errors.New("exec: no such file"), want: "MixMusic couldn't open Zen: exec: no such file"},
		{name: "not found", id: "brave", want: `MixMusic no longer finds "brave" on this computer`},
		{name: "cancelled", id: "edge", cancel: true, want: context.Canceled.Error()},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, requests := cookieApp(t, signedIn(true))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			fakeBrowsers(a, foundBrowsers, func(ctx context.Context, b browser.Browser, opts browser.Options) ([]*http.Cookie, error) {
				if c.cancel {
					cancel()
					<-ctx.Done()
					return nil, ctx.Err()
				}
				return nil, c.err
			})
			_, err := (&Accounts{a}).YouTubeBrowserLogin(ctx, c.id)
			if err == nil || err.Error() != c.want {
				t.Errorf("ended with %v, want %q", err, c.want)
			}
			if a.youtube.SignedIn() || requests.Load() != 0 {
				t.Errorf("signed in: %v, after %d requests", a.youtube.SignedIn(), requests.Load())
			}
		})
	}
}

// TestYouTubeMusicCookies keeps each cookie once, the first of its name.
func TestYouTubeMusicCookies(t *testing.T) {
	got := youtubeMusicCookies([]*http.Cookie{
		{Name: "SID", Value: "a", Domain: ".YouTube.com", Path: "/"},
		{Name: "SID", Value: "b", Domain: "music.youtube.com", Path: "/"},
	}, time.Now())
	if len(got) != 1 || got[0].Value != "a" || !strings.EqualFold(got[0].Domain, ".youtube.com") {
		t.Errorf("kept %+v", got)
	}
}
