package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/fzn0x/mixmusic/internal/browser"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// browserSignInTimeout is how long signing in through a browser waits for
// the user.
const browserSignInTimeout = 10 * time.Minute

// BrowserSignIn finds the browsers that sign in to YouTube Music, and runs
// a sign-in in one, in place of the browsers installed: tests set it, and
// either function left nil is the system's.
type BrowserSignIn struct {
	Find   func() []browser.Browser
	SignIn func(ctx context.Context, b browser.Browser, opts browser.Options) ([]*http.Cookie, error)
}

// browsers returns the way MixMusic finds the browsers and signs in with
// them: the options', or the system's.
func (a *App) browsers() BrowserSignIn {
	var b BrowserSignIn
	if a.opts.Browsers != nil {
		b = *a.opts.Browsers
	}
	if b.Find == nil {
		b.Find = browser.Find
	}
	if b.SignIn == nil {
		b.SignIn = browser.SignIn
	}
	return b
}

// YouTubeSignInBrowsers returns the browsers installed on this computer
// that MixMusic can open to sign in to YouTube Music, each with a profile
// of its own: Chrome, Edge, Brave, Chromium, Vivaldi, Helium, Firefox and
// Zen, as found.
func (s *Accounts) YouTubeSignInBrowsers() []YouTubeBrowser {
	var list []YouTubeBrowser
	for _, b := range s.a.browsers().Find() {
		list = append(list, YouTubeBrowser{ID: b.ID, Name: b.Name})
	}
	return list
}

// YouTubeBrowserLogin opens a browser of YouTubeSignInBrowsers with a new,
// empty profile of its own, at YouTube Music's sign-in, and signs MixMusic
// in with the session the user signs in to there. Once they did, the
// browser closes and its profile is deleted, as when they close it first,
// when ctx is cancelled, or after browserSignInTimeout. The user's own
// profiles are never read. Of the session's cookies, MixMusic keeps those
// of YouTube Music, as the sign-in window does.
func (s *Accounts) YouTubeBrowserLogin(ctx context.Context, id string) (AccountsStatus, error) {
	ways := s.a.browsers()
	found := ways.Find()
	i := slices.IndexFunc(found, func(b browser.Browser) bool { return b.ID == id })
	if i < 0 {
		return AccountsStatus{}, fmt.Errorf("MixMusic no longer finds %q on this computer", id)
	}
	b := found[i]
	wait, cancel := context.WithTimeout(ctx, browserSignInTimeout)
	defer cancel()
	// MixMusic quitting meanwhile ends the browser at once.
	defer s.a.OnFlush(browser.KillAll)()
	cookies, err := ways.SignIn(wait, b, browser.Options{
		URL: youtubeSignInURL,
		SignedIn: func(cookies []*http.Cookie) bool {
			return youtube.HasSigningCookie(youtubeMusicCookies(cookies, time.Now()))
		},
	})
	switch {
	case errors.Is(err, browser.ErrClosed):
		return AccountsStatus{}, worded(b.Name+" closed before you signed in to YouTube Music", err)
	case ctx.Err() != nil:
		return AccountsStatus{}, ctx.Err()
	case errors.Is(err, context.DeadlineExceeded):
		return AccountsStatus{}, worded("signing in to YouTube Music took too long; try again", err)
	case err != nil:
		return AccountsStatus{}, fmt.Errorf("MixMusic couldn't open %s: %w", b.Name, err)
	}
	return s.a.youtubeSignIn(ctx, youtubeMusicCookies(cookies, time.Now()))
}

// youtubeMusicCookies returns the cookies of a browser that a request to
// music.youtube.com sends, as the sign-in window keeps them: those of
// youtube.com for every path, that have not expired, once each.
func youtubeMusicCookies(cookies []*http.Cookie, now time.Time) []*http.Cookie {
	var kept []*http.Cookie
	seen := map[string]bool{}
	for _, c := range cookies {
		if c.Name == "" || seen[c.Name] || c.Path != "/" || !youtubeDomain([]byte(c.Domain), false) {
			continue
		}
		if !c.Expires.IsZero() && c.Expires.Before(now) {
			continue
		}
		seen[c.Name] = true
		kept = append(kept, c)
	}
	return kept
}
