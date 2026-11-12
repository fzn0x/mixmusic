// Package browser finds the browsers installed on this computer, and signs
// in to a site in one of them, with a profile of its own made for that
// sign-in alone: it opens the browser with the new profile at the site's
// sign-in page, waits for the cookies that tell the user signed in, takes
// them, closes the browser and deletes the profile. It never reads the
// user's own profiles.
//
// It reads the cookies of a browser made from Firefox from the profile's
// own cookie store, an SQLite file, which a reader of its own reads
// (sqlite.go). It opens no browser made from Chromium: one MixMusic drives
// tells the pages it opens so (navigator.webdriver), and Google won't sign
// in to such a browser, "This browser or app may not be secure".
package browser

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"
)

// Engine is what a browser is made from.
type Engine int

const (
	// Firefox is the engine of Firefox and Zen.
	Firefox Engine = iota + 1
)

// Browser is a browser installed on this computer.
type Browser struct {
	// ID names the browser to MixMusic, as "firefox", and Name to the
	// user, as "Firefox".
	ID, Name string
	Engine   Engine
	// Path is its program.
	Path string
	// Snap is the name of the snap a browser of Linux comes as, whose
	// sandbox reaches only some folders: its profile goes in its own
	// folder of ~/snap.
	Snap string
}

// Options are what a sign-in opens, and when it's done.
type Options struct {
	// URL is the page the browser opens at.
	URL string
	// SignedIn tells, of the cookies of the profile, whether the user
	// signed in.
	SignedIn func(cookies []*http.Cookie) bool
	// Every is how often the cookies are read: a second when zero.
	Every time.Duration
}

// ErrClosed is the error of a sign-in the user ended by closing the
// browser.
var ErrClosed = errors.New("the browser closed before signing in")

// errWindowsClosed tells that the user closed the browser's last window,
// which on macOS leaves the browser running.
var errWindowsClosed = errors.New("the browser's windows closed")

// settleReads is how many reads a sign-in waits at most, once the cookies
// say the user signed in, for them to stop changing: the site may set
// more as it comes back from the sign-in.
const settleReads = 5

// session is a browser that runs with a profile of its own.
type session interface {
	// cookies returns the cookies of the profile, or errWindowsClosed.
	cookies(ctx context.Context) ([]*http.Cookie, error)
	// exited is closed once the browser exited.
	exited() <-chan struct{}
	// quit closes the browser, and waits for it to exit.
	quit()
}

// starter starts a browser with the profile in dir, at url.
type starter func(ctx context.Context, b Browser, dir, url string) (session, error)

// extraArgs are more arguments of the browsers: the tests that run one
// hide it (--headless).
var extraArgs []string

// starters start the browsers of each engine. Tests replace them.
var starters = map[Engine]starter{Firefox: startFirefox}

// SignIn opens b with a new profile at opts.URL, and returns the cookies
// of the profile once opts.SignedIn tells that the user signed in, and
// they stopped changing. It returns ErrClosed when the user closes the
// browser first, and the context's error when it ends first: the caller's
// cancel, or its time limit. However it ends, the browser closes and the
// profile is deleted.
func SignIn(ctx context.Context, b Browser, opts Options) ([]*http.Cookie, error) {
	start := starters[b.Engine]
	if start == nil {
		return nil, fmt.Errorf("MixMusic can't open %s", b.Name)
	}
	dir, err := newProfile(b)
	if err != nil {
		return nil, err
	}
	defer removeProfile(dir)
	s, err := start(ctx, b, dir, opts.URL)
	if err != nil {
		return nil, err
	}
	defer s.quit()
	return wait(ctx, s, opts)
}

// wait reads the cookies of s until the user signed in, closed the
// browser, or ctx ended.
func wait(ctx context.Context, s session, opts Options) ([]*http.Cookie, error) {
	every := opts.Every
	if every <= 0 {
		every = time.Second
	}
	tick := time.NewTicker(every)
	defer tick.Stop()
	var signed []*http.Cookie
	settle := 0
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-s.exited():
			return nil, ErrClosed
		case <-tick.C:
		}
		cookies, err := s.cookies(ctx)
		switch {
		case errors.Is(err, errWindowsClosed):
			return nil, ErrClosed
		case err != nil:
			// The browser is starting, or exiting, which exited tells;
			// or its cookie store is being written: the next read tells.
			continue
		case !opts.SignedIn(cookies):
			signed, settle = nil, 0
			continue
		}
		if signed != nil && (sameCookies(signed, cookies) || settle >= settleReads) {
			return cookies, nil
		}
		signed = cookies
		settle++
	}
}

// sameCookies reports whether two reads of the cookies hold the same ones,
// with the same values.
func sameCookies(a, b []*http.Cookie) bool {
	key := func(c *http.Cookie) string { return c.Domain + "\x00" + c.Path + "\x00" + c.Name + "\x00" + c.Value }
	ka, kb := make([]string, len(a)), make([]string, len(b))
	for i, c := range a {
		ka[i] = key(c)
	}
	for i, c := range b {
		kb[i] = key(c)
	}
	slices.Sort(ka)
	slices.Sort(kb)
	return slices.Equal(ka, kb)
}
