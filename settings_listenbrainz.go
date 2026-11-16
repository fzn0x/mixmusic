package main

import (
	"context"
	"net/url"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/locales"
)

// The ListenBrainz group comes below Last.fm's, which the accounts' own
// part holds (accounts.go, at 10).
func init() { addSettings("accounts", 20, (*shell).listenbrainzSettings) }

// The keys of the drafts of the group's inputs.
const (
	listenbrainzTokenKey  = "listenbrainz-token"
	listenbrainzServerKey = "listenbrainz-server"
	listenbrainzAgainKey  = "listenbrainz-again"
)

// listenbrainzDraft is the draft of an input of the group, by its key.
func (s *shell) listenbrainzDraft(key string) *draft {
	d := s.drafts[key]
	if d == nil {
		d = &draft{}
		s.drafts[key] = d
	}
	return d
}

// listenbrainzSettings connect MixMusic to ListenBrainz, or to a server of
// the user's that speaks its API, and disconnect it.
func (s *shell) listenbrainzSettings(c *ui.Context) {
	lb := s.accounts().ListenBrainz
	s.group(c, "ListenBrainz", func() {
		if lb.Connected {
			s.listenbrainzConnected(c, lb)
			return
		}
		// The form shows whole, as its button needs its fields.
		s.found(c, locales.ListenBrainzSettingsConnectTitle(), locales.ListenBrainzSettingsForm(), func() {
			s.steps(c,
				func() {
					ui.Text(c, locales.ListenBrainzSettingsStep1())
					s.linkOf(c, locales.ListenBrainzSettingsTokenLink(), "https://listenbrainz.org/settings/")
					ui.Text(c, locales.ListenBrainzSettingsStep1Tail())
				},
				func() { ui.Text(c, locales.ListenBrainzSettingsStep2()) },
			)
			token, server := s.listenbrainzDraft(listenbrainzTokenKey), s.listenbrainzDraft(listenbrainzServerKey)
			s.row(c, locales.ListenBrainzSettingsUserToken(), "", func() {
				ui.TextInput(c.Key(listenbrainzTokenKey), &token.text).Password().Placeholder(locales.ListenBrainzSettingsTokenPlaceholder()).Width(260).Label(locales.ListenBrainzSettingsUserToken())
			})
			s.row(c, locales.ListenBrainzSettingsServerLabel(), locales.ListenBrainzSettingsServer(), func() {
				ui.TextInput(c.Key(listenbrainzServerKey), &server.text).Placeholder("https://api.listenbrainz.org").Width(260).Label(locales.ListenBrainzSettingsServerLabel())
			})
			if plainHTTP(server.text) {
				s.listenbrainzWarning(c)
			}
			busy := s.signingIn == "listenbrainz"
			label := locales.ListenBrainzSettingsConnect()
			if busy {
				label = locales.ListenBrainzSettingsChecking()
			}
			ui.Row(c).Padding(12, 0).Children(func() {
				tok, addr := strings.TrimSpace(token.text), strings.TrimSpace(server.text)
				if s.button(c, "listenbrainz-in", label, true).Disabled(busy || tok == "").Clicked() {
					token.text = "" // it stays no longer than this try
					s.account("listenbrainz", locales.ListenBrainzSettingsConnected(), func(ctx context.Context) error {
						_, err := s.svc.Accounts.ListenBrainzLogin(ctx, tok, addr)
						return err
					})
				}
			})
			s.hint(c, locales.ListenBrainzSettingsHint())
		})
	})
}

// listenbrainzConnected is the group once connected: what becomes of the
// listens waiting, when the server refuses the token or another user's
// came; the switches; and disconnect.
func (s *shell) listenbrainzConnected(c *ui.Context, lb app.ListenBrainzAccount) {
	if lb.Other != "" {
		title, text := locales.ListenBrainzSettingsOtherTitle(lb.Other), listenbrainzOtherText(lb)
		s.found(c, title, text, func() {
			s.alert(c, title, text, func() {
				if s.button(c, "listenbrainz-send", locales.ListenBrainzSettingsSendThem(), true).Clicked() {
					s.account("", locales.ListenBrainzSettingsConnected(), func(ctx context.Context) error { return s.svc.Accounts.ListenBrainzResolve(ctx, true) })
				}
				if s.button(c, "listenbrainz-drop", locales.ListenBrainzSettingsDropThem(), false).Clicked() {
					s.account("", locales.ListenBrainzSettingsConnected(), func(ctx context.Context) error { return s.svc.Accounts.ListenBrainzResolve(ctx, false) })
				}
			}).Margin(12, 0)
		})
	} else if lb.Expired {
		title, text := locales.ListenBrainzSettingsNoLongerAccepts(listenbrainzName(lb)), listenbrainzExpiredText(lb)
		s.found(c, title, text, func() {
			s.alert(c, title, text, func() {
				again := s.listenbrainzDraft(listenbrainzAgainKey)
				ui.TextInput(c.Key(listenbrainzAgainKey), &again.text).Password().Placeholder(locales.ListenBrainzSettingsNewTokenPlaceholder()).Width(260).Label(locales.ListenBrainzSettingsNewTokenLabel())
				busy := s.signingIn == "listenbrainz"
				label := locales.ListenBrainzSettingsConnectAgain()
				if busy {
					label = locales.ListenBrainzSettingsChecking()
				}
				tok := strings.TrimSpace(again.text)
				if s.button(c, "listenbrainz-reconnect", label, true).Disabled(busy || tok == "").Clicked() {
					again.text = ""
					s.account("listenbrainz", locales.ListenBrainzSettingsConnected(), func(ctx context.Context) error {
						return s.svc.Accounts.ListenBrainzReconnect(ctx, tok)
					})
				}
			}).Margin(12, 0)
		})
	}
	s.row(c, locales.AccountSettingsScrobblingAs(lb.Name), listenbrainzAbout(lb), func() {
		if s.switchOf(c.Key("listenbrainz-scrobbling"), &s.set.ListenBrainzScrobbling).Label(locales.ListenBrainzSettingsScrobbleSwitch()).Changed() {
			s.save()
		}
	})
	if lb.Waiting > 0 && !lb.Expired && lb.Other == "" {
		s.hint(c, waitingText(lb.Waiting, "listen", listenbrainzName(lb)))
	}
	if lb.Plain {
		s.listenbrainzWarning(c)
	}
	s.row(c, locales.ListenBrainzSettingsLovesTitle(), locales.ListenBrainzSettingsLoves(), func() {
		if s.switchOf(c.Key("listenbrainz-loves"), &s.set.ListenBrainzLoves).Label(locales.ListenBrainzSettingsLovesSwitch()).Changed() {
			s.save()
		}
	})
	s.found(c, locales.AccountSettingsDisconnect(), "sign out log out of ListenBrainz", func() {
		ui.Row(c).Padding(12, 0).Children(func() {
			if s.button(c, "listenbrainz-out", locales.AccountSettingsDisconnect(), false).Clicked() {
				s.ask(locales.ListenBrainzSettingsDisconnectAskTitle(), listenbrainzDisconnectText(lb), locales.AccountSettingsDisconnect(), func() {
					s.account("", "", func(ctx context.Context) error { return s.svc.Accounts.ListenBrainzLogout(ctx) })
				})
			}
		})
	})
}

// listenbrainzWarning warns that the token travels unencrypted.
func (s *shell) listenbrainzWarning(c *ui.Context) {
	s.found(c, "", locales.ListenBrainzSettingsPlain(), func() {
		ui.Text(c, locales.ListenBrainzSettingsPlain()).FontSize(s.lk.smallSize()).TextColor(s.lk.danger).Padding(8, 0).MaxWidth(720)
	})
}

// plainHTTP reports whether a server's address, as the user writes it, is
// one of plain http.
func plainHTTP(addr string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(addr)), "http://")
}

// listenbrainzName names the server the connection is to: ListenBrainz,
// or the host of the user's own.
func listenbrainzName(lb app.ListenBrainzAccount) string {
	if lb.Server == "" {
		return "ListenBrainz"
	}
	if u, err := url.Parse(lb.Server); err == nil && u.Host != "" {
		return u.Host
	}
	return locales.ListenBrainzSettingsYourServer()
}

// listenbrainzAbout says what scrobbling to the server does.
func listenbrainzAbout(lb app.ListenBrainzAccount) string {
	if lb.Server != "" {
		return locales.ListenBrainzSettingsAboutServer(listenbrainzName(lb))
	}
	return locales.ListenBrainzSettingsAboutListenBrainz()
}

// listenbrainzExpiredText tells what to do when the server refuses the
// token.
func listenbrainzExpiredText(lb app.ListenBrainzAccount) string {
	if lb.Server == "" {
		return locales.ListenBrainzSettingsExpiredListenBrainz(lb.Waiting)
	}
	return locales.ListenBrainzSettingsExpiredServer(lb.Waiting)
}

// listenbrainzOtherText asks what becomes of the waiting listens when the
// new token is another user's.
func listenbrainzOtherText(lb app.ListenBrainzAccount) string {
	return locales.ListenBrainzSettingsOtherText(lb.Waiting, lb.Name, lb.Other)
}

// listenbrainzDisconnectText is the question disconnecting asks.
func listenbrainzDisconnectText(lb app.ListenBrainzAccount) string {
	return locales.ListenBrainzSettingsDisconnectAskBody(listenbrainzName(lb), lb.Waiting)
}
