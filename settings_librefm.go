package main

import (
	"context"
	"net/url"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/librefm"
	"github.com/fzn0x/mixmusic/locales"
)

// The Libre.fm group comes below ListenBrainz's (settings_listenbrainz.go,
// at 20).
func init() { addSettings("accounts", 30, (*shell).librefmSettings) }

// The keys of the drafts of the group's inputs.
const (
	librefmAddressKey = "librefm-address"
	librefmKeyKey     = "librefm-key"
	librefmSecretKey  = "librefm-secret"
)

// librefmForm is what the Libre.fm form holds besides its texts: whether
// the server is another GNU FM server than Libre.fm, and whether the
// user's own API account signs the calls.
type librefmForm struct {
	other, own bool
}

// librefmDraft is the draft of an input of the group, by its key.
func (s *shell) librefmDraft(key string) *draft {
	d := s.drafts[key]
	if d == nil {
		d = &draft{}
		s.drafts[key] = d
	}
	return d
}

// librefmSettings connect MixMusic to Libre.fm, or to another GNU FM
// server, and disconnect it.
func (s *shell) librefmSettings(c *ui.Context) {
	fm := s.accounts().Librefm
	s.group(c, "Libre.fm", func() {
		if fm.Connected {
			s.librefmConnected(c, fm)
			return
		}
		// The form shows whole, as its button needs its fields.
		s.found(c, locales.LibrefmSettingsConnectTitle(), locales.LibrefmSettingsForm(), func() { s.librefmConnect(c) })
	})
}

// librefmConnect is the form that connects: the server, the user's own
// API account, if any, and the button.
func (s *shell) librefmConnect(c *ui.Context) {
	f := &s.librefm
	s.steps(c,
		func() { ui.Text(c, locales.LibrefmSettingsStep1()) },
		func() { ui.Text(c, locales.LibrefmSettingsStep2()) },
	)
	labels := []string{"Libre.fm", locales.LibrefmSettingsAnother()}
	s.row(c, locales.ListenBrainzSettingsServerLabel(), locales.LibrefmSettingsServerDesc(), func() {
		chosen := labels[0]
		if f.other {
			chosen = labels[1]
		}
		if s.selectOf(c.Key("librefm-server"), &chosen, labels).Width(260).Label(locales.ListenBrainzSettingsServerLabel()).Changed() {
			f.other = chosen == labels[1]
		}
	})
	addr := s.librefmDraft(librefmAddressKey)
	server := ""
	if f.other {
		s.row(c, locales.LibrefmSettingsAddress(), "", func() {
			ui.TextInput(c.Key(librefmAddressKey), &addr.text).Placeholder("https://gnufm.example.com").Width(260).Label(locales.LibrefmSettingsAddress())
		})
		if plainHTTP(addr.text) {
			s.librefmWarning(c)
		}
		server = strings.TrimSpace(addr.text)
	}
	s.row(c, locales.LibrefmSettingsOwnAccount(), locales.LibrefmSettingsOwnAccountDesc(), func() {
		s.switchOf(c.Key("librefm-own"), &f.own).Label(locales.LibrefmSettingsOwnAccount())
	})
	key, secret := s.librefmDraft(librefmKeyKey), s.librefmDraft(librefmSecretKey)
	if f.own {
		s.row(c, locales.AccountSettingsAPIKey(), "", func() {
			ui.TextInput(c.Key(librefmKeyKey), &key.text).Placeholder(locales.AccountSettingsCharsPlaceholder()).Width(260).Label(locales.AccountSettingsAPIKey())
		})
		s.row(c, locales.AccountSettingsSharedSecret(), "", func() {
			ui.TextInput(c.Key(librefmSecretKey), &secret.text).Password().Placeholder(locales.AccountSettingsCharsPlaceholder()).Width(260).Label(locales.AccountSettingsSharedSecret())
		})
	}
	name := librefmFormName(server)
	busy := s.signingIn == "librefm"
	label := locales.LibrefmSettingsConnect(strings.ToLower(name))
	if busy {
		label = locales.LibrefmSettingsWaiting(name)
	}
	own := f.own
	k, sec := strings.TrimSpace(key.text), strings.TrimSpace(secret.text)
	missing := f.other && server == "" || own && (k == "" || sec == "")
	ui.Row(c).Padding(12, 0).Children(func() {
		if s.button(c, "librefm-in", label, true).Disabled(busy || missing).Clicked() {
			s.notes = append(s.notes, notice(locales.LibrefmSettingsApprove(name)))
			s.account("librefm", locales.LibrefmSettingsConnected(name), func(ctx context.Context) error {
				_, err := s.svc.Accounts.LibrefmLogin(ctx, server, own, k, sec)
				if err == nil {
					s.update(func() { secret.text = "" }) // kept with the connection now
				}
				return err
			})
		}
	})
	s.hint(c, locales.LibrefmSettingsHint())
}

// librefmConnected is the group once connected: what becomes of the
// scrobbles waiting, when the server refuses the session or another
// user approved; the switches; and disconnect.
func (s *shell) librefmConnected(c *ui.Context, fm app.LibrefmAccount) {
	name := librefmName(fm.Server)
	if fm.Other != "" {
		title, text := locales.AccountSettingsLastfmSendTitle(fm.Other), locales.AccountSettingsLastfmOther(fm.Waiting, fm.Name, fm.Other)
		s.found(c, title, text, func() {
			s.alert(c, title, text, func() {
				if s.button(c, "librefm-send", locales.AccountSettingsLastfmSend(), true).Clicked() {
					s.account("", locales.LibrefmSettingsConnected(name), func(ctx context.Context) error { return s.svc.Accounts.LibrefmResolve(ctx, true) })
				}
				if s.button(c, "librefm-drop", locales.AccountSettingsLastfmDrop(), false).Clicked() {
					s.account("", locales.LibrefmSettingsConnected(name), func(ctx context.Context) error { return s.svc.Accounts.LibrefmResolve(ctx, false) })
				}
			}).Margin(12, 0)
		})
	} else if fm.Expired {
		title, text := locales.LibrefmSettingsGone(name), locales.LibrefmSettingsExpired(fm.Waiting)
		s.found(c, title, text, func() {
			s.alert(c, title, text, func() {
				busy := s.signingIn == "librefm"
				label := locales.AccountSettingsLastfmConnectAgain()
				if busy {
					label = locales.LibrefmSettingsWaiting(name)
				}
				if s.button(c, "librefm-again", label, true).Disabled(busy).Clicked() {
					s.notes = append(s.notes, notice(locales.LibrefmSettingsApprove(name)))
					s.account("librefm", locales.LibrefmSettingsConnected(name), func(ctx context.Context) error { return s.svc.Accounts.LibrefmReconnect(ctx) })
				}
			}).Margin(12, 0)
		})
	}
	s.row(c, locales.AccountSettingsScrobblingAs(fm.Name), locales.LibrefmSettingsAbout(name), func() {
		if s.switchOf(c.Key("librefm-scrobbling"), &s.set.LibrefmScrobbling).Label(locales.LibrefmSettingsScrobbleSwitch(name)).Changed() {
			s.save()
		}
	})
	if fm.Waiting > 0 && !fm.Expired && fm.Other == "" {
		s.hint(c, waitingText(fm.Waiting, "scrobble", name))
	}
	if fm.Plain {
		s.librefmWarning(c)
	}
	s.row(c, locales.AccountSettingsLastfmLoves(), locales.LibrefmSettingsLovesDesc(name), func() {
		if s.switchOf(c.Key("librefm-loves"), &s.set.LibrefmLoves).Label(locales.LibrefmSettingsLovesSwitch(name)).Changed() {
			s.save()
		}
	})
	s.found(c, locales.AccountSettingsDisconnect(), "sign out log out of Libre.fm GNU FM", func() {
		ui.Row(c).Padding(12, 0).Children(func() {
			if s.button(c, "librefm-out", locales.AccountSettingsDisconnect(), false).Clicked() {
				s.ask(locales.LibrefmSettingsDisconnectTitle(name), locales.LibrefmSettingsDisconnectBody(name, fm.Waiting), locales.AccountSettingsDisconnect(), func() {
					s.account("", "", func(ctx context.Context) error { return s.svc.Accounts.LibrefmLogout(ctx) })
				})
			}
		})
	})
}

// librefmWarning warns that the password and the session travel
// unencrypted.
func (s *shell) librefmWarning(c *ui.Context) {
	s.found(c, "", locales.LibrefmSettingsPlain(), func() {
		ui.Text(c, locales.LibrefmSettingsPlain()).FontSize(s.lk.smallSize()).TextColor(s.lk.danger).Padding(8, 0).MaxWidth(720)
	})
}

// librefmName names the server of a connection, its address "" for
// Libre.fm: Libre.fm, or the host of another GNU FM server.
func librefmName(server string) string {
	if server == "" || server == librefm.DefaultServer {
		return "Libre.fm"
	}
	if u, err := url.Parse(server); err == nil && u.Host != "" {
		return u.Host
	}
	return server
}

// librefmFormName names the server the form connects to, at the address
// addr as the user writes it, "" for Libre.fm.
func librefmFormName(addr string) string {
	server, _, err := librefm.ParseServer(addr)
	if err != nil {
		return strings.TrimSpace(addr)
	}
	return librefmName(server)
}
