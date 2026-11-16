package main

// The sign-in to YouTube Music through a browser of the user's, opened
// with a profile of its own, made for the sign-in alone (#472): where
// Google refuses the sign-in window, a browser of the user's is one it
// knows.

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/locales"
)

// youtubeBrowserSignIn is the state of the sign-in through a browser: the
// browsers found, once looked for, and their names; the one chosen; while
// a sign-in waits, the browser's name and what cancels it; and why the
// last try failed.
type youtubeBrowserSignIn struct {
	looked   bool
	browsers []app.YouTubeBrowser
	names    []string
	chosen   string
	waiting  string
	cancel   context.CancelFunc
	problem  string
}

// youtubeBrowserSignIn offers to sign in to YouTube Music in a browser of
// the user's, of those found; busy is set while a sign-in of YouTube Music
// waits. While this one waits, it can be cancelled.
func (s *shell) youtubeBrowserSignIn(c *ui.Context, busy bool) {
	yb := &s.youtubeBrowser
	if !yb.looked {
		yb.looked = true
		yb.browsers = s.svc.Accounts.YouTubeSignInBrowsers()
		for _, b := range yb.browsers {
			yb.names = append(yb.names, b.Name)
		}
		if len(yb.names) > 0 {
			yb.chosen = yb.names[0]
		}
	}
	if !s.findable(locales.YoutubeBrowserTitle(), locales.YoutubeBrowserForm(), yb.problem) {
		return
	}
	s.show(c, func() {
		if yb.problem != "" {
			s.alert(c, locales.YoutubeBrowserDidntSignIn(), yb.problem, nil).Margin(12, 0)
		}
		if len(yb.names) == 0 {
			s.row(c, locales.YoutubeBrowserTitle(), locales.YoutubeBrowserNone(), func() {})
			return
		}
		s.row(c, locales.YoutubeBrowserTitle(), locales.YoutubeBrowserAbout(), func() {
			s.selectOf(c.Key("youtube-sign-in-browser"), &yb.chosen, yb.names).Width(200).Label(locales.YoutubeBrowserChoose())
		})
		ui.Row(c).Gap(8).Padding(12, 0).Children(func() {
			waiting := s.signingIn == "youtube-browser"
			label := locales.YoutubeBrowserSignInWith(yb.chosen)
			if waiting {
				label = locales.YoutubeBrowserWaiting(yb.waiting)
			}
			if s.button(c, "youtube-browser-in", label, false).Disabled(busy).Clicked() {
				s.signInWithBrowser()
			}
			if waiting && yb.cancel != nil && s.button(c, "youtube-browser-cancel", locales.LibraryCancel(), false).Clicked() {
				yb.cancel()
			}
		})
		s.hint(c, locales.YoutubeBrowserHint())
	})
}

// signInWithBrowser signs in to YouTube Music in the browser chosen,
// beside the main thread. A sign-in that fails says why above the way,
// until the next try, but one the user cancelled; one that works asks for
// the account and channel, when the session has more than one.
func (s *shell) signInWithBrowser() {
	yb := &s.youtubeBrowser
	i := slices.Index(yb.names, yb.chosen)
	if i < 0 {
		return
	}
	b := yb.browsers[i]
	ctx, cancel := context.WithCancel(context.Background())
	s.signingIn = "youtube-browser"
	yb.waiting, yb.cancel, yb.problem = b.Name, cancel, ""
	s.youtubeCookies.problem, s.youtubePick = "", false
	s.notes = append(s.notes, notice(locales.YoutubeBrowserSignInNotice(b.Name)))
	go func() {
		defer cancel()
		_, err := s.svc.Accounts.YouTubeBrowserLogin(ctx, b.ID)
		s.update(func() {
			if s.signingIn == "youtube-browser" {
				s.signingIn = ""
			}
			yb.cancel = nil
			switch {
			case errors.Is(err, context.Canceled):
				// The user cancelled, which tells itself.
			case err != nil:
				msg := err.Error()
				if msg != "" {
					msg = strings.ToUpper(msg[:1]) + msg[1:]
				}
				yb.problem = msg
				s.forget("accounts")
			default:
				s.youtubePick = true
				s.notes = append(s.notes, notice(locales.YoutubeBrowserSignedIn()))
				clear(s.loads)
			}
		})
	}()
}
