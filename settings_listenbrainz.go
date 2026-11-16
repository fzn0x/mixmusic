package main

import (
	"context"
	"net/url"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
)

// The ListenBrainz group comes below Last.fm's, which the accounts' own
// part holds (accounts.go, at 10).
func init() { addSettings("accounts", 20, (*shell).listenbrainzSettings) }

// The texts of the ListenBrainz group, and what the search of the settings
// finds its form by.
const (
	listenbrainzHint   = "MixMusic keeps your token in your system's credential store, with the connection."
	listenbrainzServer = "Leave it empty for ListenBrainz itself. A ListenBrainz of your own takes its address, and Maloja its address followed by /apis/listenbrainz."
	// listenbrainzPlain warns of a server's plain http address.
	listenbrainzPlain = "Plain http works only on your local network, and your token travels to the server unencrypted: anyone on the network can read it."
	listenbrainzForm  = "Copy your user token from your ListenBrainz settings, or your own server's. Paste it here, and connect. " +
		"User token. Server. " + listenbrainzServer + " Connect to ListenBrainz. Scrobble, scrobbling, listens. Self-hosted, Maloja. Sign in, log in. " + listenbrainzHint
	listenbrainzLoves = "A like loves the song on ListenBrainz, and an unlike takes the love back."
)

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
		s.found(c, "Connect to ListenBrainz", listenbrainzForm, func() {
			s.steps(c,
				func() {
					ui.Text(c, "Copy your user token from ")
					s.linkOf(c, "your ListenBrainz settings", "https://listenbrainz.org/settings/")
					ui.Text(c, ", or your own server's.")
				},
				func() { ui.Text(c, "Paste it here, and connect.") },
			)
			token, server := s.listenbrainzDraft(listenbrainzTokenKey), s.listenbrainzDraft(listenbrainzServerKey)
			s.row(c, "User token", "", func() {
				ui.TextInput(c.Key(listenbrainzTokenKey), &token.text).Password().Placeholder("36 letters, digits and dashes").Width(260).Label("User token")
			})
			s.row(c, "Server", listenbrainzServer, func() {
				ui.TextInput(c.Key(listenbrainzServerKey), &server.text).Placeholder("https://api.listenbrainz.org").Width(260).Label("Server")
			})
			if plainHTTP(server.text) {
				s.listenbrainzWarning(c)
			}
			busy := s.signingIn == "listenbrainz"
			label := "connect to listenbrainz"
			if busy {
				label = "checking the token…"
			}
			ui.Row(c).Padding(12, 0).Children(func() {
				tok, addr := strings.TrimSpace(token.text), strings.TrimSpace(server.text)
				if s.button(c, "listenbrainz-in", label, true).Disabled(busy || tok == "").Clicked() {
					token.text = "" // it stays no longer than this try
					s.account("listenbrainz", "Connected to ListenBrainz", func(ctx context.Context) error {
						_, err := s.svc.Accounts.ListenBrainzLogin(ctx, tok, addr)
						return err
					})
				}
			})
			s.hint(c, listenbrainzHint)
		})
	})
}

// listenbrainzConnected is the group once connected: what becomes of the
// listens waiting, when the server refuses the token or another user's
// came; the switches; and disconnect.
func (s *shell) listenbrainzConnected(c *ui.Context, lb app.ListenBrainzAccount) {
	if lb.Other != "" {
		title, text := "Send the waiting listens to "+lb.Other+"?", listenbrainzOtherText(lb)
		s.found(c, title, text, func() {
			s.alert(c, title, text, func() {
				if s.button(c, "listenbrainz-send", "send them", true).Clicked() {
					s.account("", "Connected to ListenBrainz", func(ctx context.Context) error { return s.svc.Accounts.ListenBrainzResolve(ctx, true) })
				}
				if s.button(c, "listenbrainz-drop", "drop them", false).Clicked() {
					s.account("", "Connected to ListenBrainz", func(ctx context.Context) error { return s.svc.Accounts.ListenBrainzResolve(ctx, false) })
				}
			}).Margin(12, 0)
		})
	} else if lb.Expired {
		title, text := listenbrainzName(lb)+" no longer accepts this token", listenbrainzExpiredText(lb)
		s.found(c, title, text, func() {
			s.alert(c, title, text, func() {
				again := s.listenbrainzDraft(listenbrainzAgainKey)
				ui.TextInput(c.Key(listenbrainzAgainKey), &again.text).Password().Placeholder("a new user token").Width(260).Label("New user token")
				busy := s.signingIn == "listenbrainz"
				label := "connect again"
				if busy {
					label = "checking the token…"
				}
				tok := strings.TrimSpace(again.text)
				if s.button(c, "listenbrainz-reconnect", label, true).Disabled(busy || tok == "").Clicked() {
					again.text = ""
					s.account("listenbrainz", "Connected to ListenBrainz", func(ctx context.Context) error {
						return s.svc.Accounts.ListenBrainzReconnect(ctx, tok)
					})
				}
			}).Margin(12, 0)
		})
	}
	s.row(c, "Scrobbling as "+lb.Name, listenbrainzAbout(lb), func() {
		if s.switchOf(c.Key("listenbrainz-scrobbling"), &s.set.ListenBrainzScrobbling).Label("Scrobble to ListenBrainz").Changed() {
			s.save()
		}
	})
	if lb.Waiting > 0 && !lb.Expired && lb.Other == "" {
		s.hint(c, waitingText(lb.Waiting, "listen", listenbrainzName(lb)))
	}
	if lb.Plain {
		s.listenbrainzWarning(c)
	}
	s.row(c, "Send likes as loves", listenbrainzLoves, func() {
		if s.switchOf(c.Key("listenbrainz-loves"), &s.set.ListenBrainzLoves).Label("Send likes to ListenBrainz as loves").Changed() {
			s.save()
		}
	})
	s.found(c, "disconnect", "sign out log out of ListenBrainz", func() {
		ui.Row(c).Padding(12, 0).Children(func() {
			if s.button(c, "listenbrainz-out", "disconnect", false).Clicked() {
				s.ask("disconnect ListenBrainz", listenbrainzDisconnectText(lb), "disconnect", func() {
					s.account("", "", func(ctx context.Context) error { return s.svc.Accounts.ListenBrainzLogout(ctx) })
				})
			}
		})
	})
}

// listenbrainzWarning warns that the token travels unencrypted.
func (s *shell) listenbrainzWarning(c *ui.Context) {
	s.found(c, "", listenbrainzPlain, func() {
		ui.Text(c, listenbrainzPlain).FontSize(s.lk.smallSize()).TextColor(s.lk.danger).Padding(8, 0).MaxWidth(720)
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
	return "Your server"
}

// listenbrainzAbout says what scrobbling to the server does.
func listenbrainzAbout(lb app.ListenBrainzAccount) string {
	where := "your ListenBrainz listens"
	if lb.Server != "" {
		where = "your listens on " + listenbrainzName(lb)
	}
	return "What you listen to joins " + where + ": a track counts once half of it played, or four minutes."
}

// listenbrainzExpiredText tells what to do when the server refuses the
// token.
func listenbrainzExpiredText(lb app.ListenBrainzAccount) string {
	text := "Paste a new user token to go on scrobbling."
	if lb.Server == "" {
		text = "Paste a new user token from your ListenBrainz settings to go on scrobbling."
	}
	if lb.Waiting > 0 {
		text += " " + plural(lb.Waiting, "listen waits", "listens wait") + " to be sent."
	}
	return text
}

// listenbrainzOtherText asks what becomes of the waiting listens when the
// new token is another user's.
func listenbrainzOtherText(lb app.ListenBrainzAccount) string {
	return plural(lb.Waiting, "listen was", "listens were") + " heard as " + lb.Name + " and not sent yet. Send them to " +
		lb.Other + "'s listens, or drop them."
}

// listenbrainzDisconnectText is the question disconnecting asks.
func listenbrainzDisconnectText(lb app.ListenBrainzAccount) string {
	text := "MixMusic forgets your token; your listens stay on " + listenbrainzName(lb) + "."
	if lb.Waiting > 0 {
		text += " " + plural(lb.Waiting, "listen", "listens") + " not sent yet will be dropped."
	}
	return text
}
