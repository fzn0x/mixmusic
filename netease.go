package main

import (
	"context"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// neteaseAccount signs in to NetEase Cloud Music with the QR code of its
// sign-in page, in a window, and out.
func (s *shell) neteaseAccount(c *ui.Context, ne app.NetEaseAccount) {
	s.group(c, "网易云音乐", func() {
		if !ne.SignedIn {
			s.found(c, locales.AccountSettingsNetEaseSignInTitle(), locales.AccountSettingsNetEaseHint(), func() {
				s.hint(c, locales.AccountSettingsNetEaseHint())
				label := locales.AccountSettingsNetEaseSignInButton()
				if s.signingIn == "netease" {
					label = locales.AccountSettingsNetEaseWaitingWindow()
				}
				if s.button(c, "netease-in", label, true).Disabled(s.signingIn != "").Clicked() {
					s.account("netease", locales.AccountSettingsNetEaseSignedIn(), func(ctx context.Context) error {
						_, err := s.svc.Accounts.NetEaseLogin(ctx)
						return err
					})
				}
			})
			return
		}
		if ne.Error != "" {
			s.found(c, locales.AccountSettingsNetEaseRefused(), ne.Error, func() {
				s.alert(c, locales.AccountSettingsNetEaseRefused(), ne.Error, nil).Margin(12, 0)
			})
		}
		who := locales.AccountSettingsSignedIn()
		if ne.Profile != nil {
			who = locales.AccountSettingsSignedInAs(ne.Profile.Name)
		}
		s.row(c, who, locales.AccountSettingsNetEaseSignedInDesc(), func() {
			if s.button(c, "netease-out", locales.AccountSettingsSignOut(), false).Clicked() {
				s.ask(locales.AccountSettingsNetEaseSignOutTitle(), locales.AccountSettingsNetEaseSignOutBody(), locales.AccountSettingsSignOut(), func() {
					s.account("", "", func(ctx context.Context) error { return s.svc.Accounts.NetEaseLogout(ctx) })
				})
			}
		}, "sign out, log out")
	})
}

// neteaseLibrary shows the user's playlists on NetEase Cloud Music, then
// the songs they liked there. NetEase keeps no albums or artists for it.
func (s *shell) neteaseLibrary(c *ui.Context) {
	col := s.svc.Collection
	none := func(context.Context) ([]media.Album, error) { return nil, nil }
	noArtists := func(context.Context) ([]media.Artist, error) { return nil, nil }
	s.serviceLibrary(c, "netease",
		fetch(s, "netease:playlists", col.NetEasePlaylists),
		fetch(s, "netease:albums", none),
		fetch(s, "netease:artists", noArtists),
		func(c *ui.Context) {
			s.sectionTitle(c, "Liked on NetEase Cloud Music")
			liked := fetch(s, "netease:liked", col.NetEaseLiked)
			switch {
			case !liked.done:
				s.loading(c)
			case liked.err != nil:
				s.problem(c, liked.err, func() { s.forget("netease:liked") })
			default:
				s.trackList(c, "netease-liked", liked.value.Tracks, trackListOptions{art: true, album: true, empty: "No liked songs on NetEase Cloud Music."})
			}
		})
}
