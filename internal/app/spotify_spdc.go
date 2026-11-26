package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/browser"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/tools"
	"github.com/fzn0x/mixmusic/locales"
)

const spotifyWebLoginURL = "https://accounts.spotify.com/login?continue=https%3A%2F%2Faccounts.spotify.com%2Fstatus"

func spotifyDomain(domain []byte, sub bool) bool {
	d := strings.TrimPrefix(strings.ToLower(string(domain)), ".")
	return d == "spotify.com" || strings.HasSuffix(d, ".spotify.com")
}

func spotifyNetscapeCookies(data []byte, now time.Time) []*http.Cookie {
	return netscapeCookies(data, now, spotifyDomain)
}

func spotifyCookiesFromList(cookies []*http.Cookie, now time.Time) (spDC, spKey, spT string) {
	for _, c := range cookies {
		if c == nil || c.Name == "" {
			continue
		}
		if c.Domain != "" && !spotifyDomain([]byte(c.Domain), true) {
			continue
		}
		if !c.Expires.IsZero() && c.Expires.Before(now) {
			continue
		}
		switch c.Name {
		case "sp_dc":
			if spDC == "" {
				spDC = strings.TrimSpace(c.Value)
			}
		case "sp_key":
			if spKey == "" {
				spKey = strings.TrimSpace(c.Value)
			}
		case "sp_t":
			if spT == "" {
				spT = strings.TrimSpace(c.Value)
			}
		}
	}
	return spDC, spKey, spT
}

// parseSPDC extracts sp_dc (plus optional sp_key and sp_t) from a raw sp_dc
// value, a `sp_dc=...` pair, a full `Cookie:` header, or a Netscape cookies.txt
// block.
func parseSPDC(text string) (spDC, spKey, spT string) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", "", ""
	}
	if strings.Contains(trimmed, "\t") {
		return spotifyCookiesFromList(spotifyNetscapeCookies([]byte(trimmed), time.Now()), time.Now())
	}
	header := ""
	for _, line := range strings.Split(trimmed, "\n") {
		if name, value, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(name), "cookie") {
			header = value
			break
		}
	}
	if header == "" {
		header = strings.NewReplacer("\r", "", "\n", "").Replace(trimmed)
	}
	if strings.Contains(header, "=") {
		for _, part := range strings.Split(header, ";") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			parsed, err := http.ParseCookie(part)
			if err != nil || len(parsed) != 1 {
				continue
			}
			switch parsed[0].Name {
			case "sp_dc":
				if spDC == "" {
					spDC = strings.TrimSpace(parsed[0].Value)
				}
			case "sp_key":
				if spKey == "" {
					spKey = strings.TrimSpace(parsed[0].Value)
				}
			case "sp_t":
				if spT == "" {
					spT = strings.TrimSpace(parsed[0].Value)
				}
			}
		}
		return spDC, spKey, spT
	}
	if !strings.ContainsAny(trimmed, " \t\r\n;") && len(trimmed) >= 16 {
		return trimmed, "", ""
	}
	return "", "", ""
}

// spotifySignInSPDC exchanges spDC for a web-player token, persists it, switches
// Spotify playback to YouTube Music matching when needed, and reads the profile.
func (a *App) spotifySignInSPDC(ctx context.Context, spDC, spKey, spT string) (AccountsStatus, error) {
	tok, err := a.auth.ExchangeSPDC(ctx, spDC, spKey, spT)
	if err != nil {
		if errors.Is(err, spotify.ErrInvalidSPDC) {
			return AccountsStatus{}, worded("Spotify rejected that sp_dc token: sign in to open.spotify.com again and copy a fresh sp_dc cookie",
				locales.AccountSettingsSpotifyInvalidSPDC(), err)
		}
		return AccountsStatus{}, wordedLater("Spotify could not verify the web session: "+err.Error(),
			func() string { return locales.AccountSettingsSpotifyWebNotAccepted(ErrorText(err)) }, err)
	}

	a.spotify.SetToken(tok)
	a.mu.Lock()
	a.spotifyMe, a.profileErrs.spotify = nil, nil
	a.mu.Unlock()
	a.here.close()
	if err := a.tokens.Save(tok); err != nil {
		return AccountsStatus{}, err
	}
	if a.settings != nil && a.settings.Get().SpotifyPlayback != settings.SpotifyPlaybackMatch {
		if err := a.settings.Update(func(set *settings.Settings) error {
			set.SpotifyPlayback = settings.SpotifyPlaybackMatch
			return nil
		}); err != nil {
			log.Printf("mixmusic: switching Spotify playback to YouTube Music: %v", err)
		} else {
			a.apply(a.settings.Get())
		}
	}
	return a.spotifySignedIn(ctx)
}

// SpotifyWebLogin opens an embedded window to Spotify's login page and captures
// the sp_dc cookie once the user finishes signing in.
func (s *Accounts) SpotifyWebLogin(ctx context.Context) (AccountsStatus, error) {
	if err := needWebView(webViewSpotifySignIn); err != nil {
		return AccountsStatus{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()

	type spSession struct {
		spDC, spKey, spT string
	}
	done := make(chan spSession, 1)
	closed := make(chan struct{})
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:  locales.AccountSettingsSpotifyWindowTitle(),
		Width:  480,
		Height: 720,
		URL:    spotifyWebLoginURL,
	})

	check := func() {
		dc, key, spt := spotifyWindowSession()
		if dc != "" {
			select {
			case done <- spSession{dc, key, spt}:
			default:
			}
		}
	}
	win.Page().OnDidNavigate(func(string) { go check() })
	win.OnClosed(func() { close(closed) })
	defer win.Close()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var sess spSession
	for {
		select {
		case sess = <-done:
			return s.a.spotifySignInSPDC(ctx, sess.spDC, sess.spKey, sess.spT)
		case <-closed:
			return AccountsStatus{}, worded("the Spotify sign-in window closed before signing in",
				locales.AccountSettingsSpotifyWindowClosed(), nil)
		case <-ctx.Done():
			return AccountsStatus{}, worded("signing in to Spotify took too long; try again",
				locales.AccountSettingsSpotifyTooLong(), nil)
		case <-ticker.C:
			go check()
		}
	}
}

func spotifyWindowSession() (spDC, spKey, spT string) {
	var all []*http.Cookie
	for _, filter := range []mygo.CookieFilter{
		{URL: "https://open.spotify.com/"},
		{Domain: "spotify.com"},
	} {
		list, err := mygo.Cookies.List(filter)
		if err != nil {
			continue
		}
		for _, c := range list {
			all = append(all, &http.Cookie{
				Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path,
				Expires: c.Expires, Secure: c.Secure, HttpOnly: c.HTTPOnly,
			})
		}
	}
	return spotifyCookiesFromList(all, time.Now())
}

func clearSpotifyWindowCookies() {
	for _, domain := range []string{"spotify.com", "accounts.spotify.com"} {
		list, err := mygo.Cookies.List(mygo.CookieFilter{Domain: domain})
		if err != nil {
			continue
		}
		for _, c := range list {
			_ = mygo.Cookies.Delete(c)
		}
	}
}

// SpotifyBrowserLogin opens an isolated browser profile at Spotify's login page
// and captures the sp_dc cookie once the user signs in.
func (s *Accounts) SpotifyBrowserLogin(ctx context.Context, id string) (AccountsStatus, error) {
	ways := s.a.browsers()
	found := ways.Find()
	i := slices.IndexFunc(found, func(b browser.Browser) bool { return b.ID == id })
	if i < 0 {
		return AccountsStatus{}, wordedLater(fmt.Sprintf("MixMusic no longer finds %q on this computer", id),
			func() string { return locales.YoutubeBrowserGone(id) }, nil)
	}
	b := found[i]
	wait, cancel := context.WithTimeout(ctx, browserSignInTimeout)
	defer cancel()
	defer s.a.OnFlush(browser.KillAll)()
	cookies, err := ways.SignIn(wait, b, browser.Options{
		URL: spotifyWebLoginURL,
		SignedIn: func(cookies []*http.Cookie) bool {
			dc, _, _ := spotifyCookiesFromList(cookies, time.Now())
			return dc != ""
		},
	})
	switch {
	case errors.Is(err, browser.ErrClosed):
		return AccountsStatus{}, worded(b.Name+" closed before you signed in to Spotify",
			locales.AccountSettingsSpotifyBrowserClosed(b.Name), err)
	case ctx.Err() != nil:
		return AccountsStatus{}, ctx.Err()
	case errors.Is(err, context.DeadlineExceeded):
		return AccountsStatus{}, worded("signing in to Spotify took too long; try again",
			locales.AccountSettingsSpotifyTooLong(), err)
	case err != nil:
		return AccountsStatus{}, wordedLater(fmt.Sprintf("MixMusic couldn't open %s: %v", b.Name, err),
			func() string { return locales.YoutubeBrowserCouldntOpen(b.Name, ErrorText(err)) }, err)
	}
	dc, key, spt := spotifyCookiesFromList(cookies, time.Now())
	return s.a.spotifySignInSPDC(ctx, dc, key, spt)
}

// SpotifyImportCookies imports a signed-in Spotify web session (sp_dc) from an
// installed browser via yt-dlp's cookie extractor.
func (s *Accounts) SpotifyImportCookies(ctx context.Context, browserID, profile string) (AccountsStatus, error) {
	browsers := browsersOn(runtime.GOOS)
	i := slices.IndexFunc(browsers, func(b cookieBrowser) bool { return b.id == browserID })
	if i < 0 {
		return AccountsStatus{}, fmt.Errorf("MixMusic can't import the cookies of %q", browserID)
	}
	b := browsers[i]
	home, _ := os.UserHomeDir()
	spec, err := b.spec(profile, runtime.GOOS, home, os.Getenv)
	if err != nil {
		return AccountsStatus{}, err
	}
	cookies, undecrypted, err := s.a.browserCookiesFor(ctx, spec, spotifyNetscapeCookies)
	dc, key, spt := spotifyCookiesFromList(cookies, time.Now())
	switch {
	case dc != "":
		return s.a.spotifySignInSPDC(ctx, dc, key, spt)
	case err != nil:
		return AccountsStatus{}, b.problem(err, strings.TrimSpace(profile) != "")
	case undecrypted:
		return AccountsStatus{}, b.problem(tools.ErrCookiesSealed, false)
	}
	name := b.name
	return AccountsStatus{}, wordedLater(fmt.Sprintf("%s has no signed-in Spotify session: sign in to open.spotify.com in it, close it, then import again", name),
		func() string { return locales.AccountSettingsSpotifyNoBrowserSession(name) }, nil)
}

// SpotifyPasteSPDC signs in to Spotify using a pasted sp_dc cookie value,
// Cookie header, or Netscape cookies.txt excerpt.
func (s *Accounts) SpotifyPasteSPDC(ctx context.Context, text string) (AccountsStatus, error) {
	dc, key, spt := parseSPDC(text)
	if dc == "" {
		return AccountsStatus{}, worded("that isn't a Spotify sp_dc cookie: paste the sp_dc value or a Cookie header from open.spotify.com",
			locales.AccountSettingsSpotifyBadSPDC(), nil)
	}
	return s.a.spotifySignInSPDC(ctx, dc, key, spt)
}
