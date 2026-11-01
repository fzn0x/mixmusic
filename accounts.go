package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
)

// accountSettings sign in and out of Spotify, YouTube Music and Last.fm.
func (s *shell) accountSettings(c *ui.Context) {
	st := s.accounts()
	s.spotifyAccount(c, st.Spotify)
	s.youtubeAccount(c, st.YouTube)
	s.lastfmAccount(c, st.Lastfm)
}

// The hints of the sign-ins, and what the search of the settings finds
// their forms by: their steps, fields, buttons and hints. Each form shows
// whole, or not at all.
const (
	spotifyHint = "Spotify lets an app in development mode serve up to five accounts added under Users Management, while its owner has Premium. Search shows ten results of each kind, and the tracks of playlists you do not own stay hidden to apps."
	spotifyForm = "Create an app in the Spotify developer dashboard, with the Web API, signed in with a Spotify account that has Premium. " +
		"Add this redirect URI to it. In its settings, under Users Management, add your name and the email of the Spotify account you sign in with. " +
		"Paste the app's Client ID here, and sign in to Spotify. Log in. " + spotifyHint

	youtubeHint    = "YouTube Music needs no account: search, artists, albums, playlists and radios work signed out. Signed in, MixMusic shows your own home feed and library, and saves what you like and follow to YouTube Music. Tracks play as they do signed out: your session never goes to yt-dlp."
	youtubeSession = "You sign in on Google's own page, in a window of MixMusic: MixMusic never sees your password, and keeps the session encrypted on this computer. YouTube Music does not offer this to apps officially, so it may stop working when YouTube changes."
	youtubeForm    = "Sign in to YouTube Music. Log in. Google account. " + youtubeHint + " " + youtubeSession

	lastfmHint = "MixMusic keeps them in your system's credential store, with the connection."
	lastfmForm = "Create an API account on Last.fm; any name and description will do, and the callback URL can stay empty. " +
		"Paste its API key and shared secret here, and connect. Approve MixMusic on the page that opens in your browser. " +
		"Connect to Last.fm. Scrobble, scrobbling. Sign in, log in. " + lastfmHint
)

func (s *shell) spotifyAccount(c *ui.Context, sp app.SpotifyAccount) {
	s.group(c, "spotify", func() {
		if sp.SignedIn && sp.Error == "" {
			who := "Signed in"
			if sp.Profile != nil {
				who = "Signed in as " + sp.Profile.Name
			}
			s.row(c, who, "Search, your library and playback of Spotify.", func() {
				if s.button(c, "spotify-out", "sign out", false).Clicked() {
					s.signOutOfSpotify()
				}
			})
			return
		}
		if sp.Error != "" {
			s.found(c, "Spotify refused the sign-in", sp.Error, func() {
				s.alert(c, "Spotify refused the sign-in", sp.Error, nil).Margin(12, 0)
			})
		}
		// The form shows whole, as its button needs its field.
		s.found(c, "Sign in to Spotify", spotifyForm, func() {
			if sp.RedirectURI == "" {
				sp.RedirectURI = s.svc.System.Info().RedirectURI
			}
			s.steps(c,
				func() {
					ui.Text(c, "Create an app in the ")
					s.linkOf(c, "Spotify developer dashboard", "https://developer.spotify.com/dashboard")
					ui.Text(c, ", with the Web API, signed in with a Spotify account that has Premium.")
				},
				func() {
					ui.Text(c, "Add this redirect URI to it: ")
					ui.Text(c, sp.RedirectURI).Font(monospace())
					ui.Text(c, " ")
					if s.linkOf(c, "copy", "").Clicked() {
						c.WriteClipboard(sp.RedirectURI)
						s.notes = append(s.notes, notice("Copied the redirect URI"))
					}
				},
				func() {
					ui.Text(c, "In its settings, under Users Management, add your name and the email of the Spotify account you sign in with.")
				},
				func() { ui.Text(c, "Paste the app's Client ID here, and sign in.") },
			)
			var clientID *draft
			s.row(c, "Client ID", "", func() {
				var in ui.Element
				in, clientID = s.settingInput(c, "client-id", s.set.SpotifyClientID, func(id string) {
					s.set.SpotifyClientID = id
					s.save()
				})
				in.Placeholder("32 letters and digits").Width(260).Label("Client ID")
			})
			busy := s.signingIn == "spotify"
			label := "sign in to spotify"
			if busy {
				label = "waiting for the browser…"
			}
			ui.Row(c).Gap(8).Padding(12, 0).Children(func() {
				// The Client ID's draft, which its row's control made, unless
				// the search of the settings left the row out.
				id := s.set.SpotifyClientID
				if clientID == nil {
					clientID = s.drafts["client-id"]
				}
				if clientID != nil {
					id = strings.TrimSpace(clientID.text)
				}
				if s.button(c, "spotify-in", label, true).Disabled(busy || id == "").Clicked() {
					if id != s.set.SpotifyClientID {
						s.set.SpotifyClientID = id
						s.save()
					}
					s.notes = append(s.notes, notice("Finish signing in in your browser"))
					s.account("spotify", "Signed in to Spotify", func(ctx context.Context) error {
						_, err := s.svc.Accounts.SpotifyLogin(ctx)
						return err
					})
				}
				// A sign-in Spotify refused stays until the user drops it: every
				// request of Spotify fails meanwhile.
				if sp.SignedIn && s.button(c, "spotify-out", "sign out", false).Disabled(busy).Clicked() {
					s.signOutOfSpotify()
				}
			})
			s.hint(c, spotifyHint)
		})
	})
}

// signOutOfSpotify asks, then has MixMusic forget the Spotify sign-in.
func (s *shell) signOutOfSpotify() {
	s.ask("sign out of Spotify", "MixMusic forgets your Spotify sign-in.", "sign out", func() {
		s.account("", "", func(context.Context) error { return s.svc.Accounts.SpotifyLogout() })
	})
}

func (s *shell) youtubeAccount(c *ui.Context, yt app.YouTubeAccount) {
	s.group(c, "youtube music", func() {
		if !yt.SignedIn {
			s.found(c, "Sign in to YouTube Music", youtubeForm, func() {
				s.hint(c, youtubeHint)
				busy := s.signingIn == "youtube"
				label := "sign in to youtube music"
				if busy {
					label = "waiting for the sign-in window…"
				}
				if s.button(c, "youtube-in", label, true).Disabled(busy).Clicked() {
					s.account("youtube", "Signed in to YouTube Music", func(ctx context.Context) error {
						_, err := s.svc.Accounts.YouTubeLogin(ctx)
						return err
					})
				}
				s.hint(c, youtubeSession)
			})
			return
		}
		if yt.Error != "" {
			again := yt.Error + ". Sign out, then sign in again."
			s.found(c, "YouTube Music refused the sign-in", again, func() {
				s.alert(c, "YouTube Music refused the sign-in", again, nil).Margin(12, 0)
			})
		}
		who := "Signed in"
		if yt.Account != nil {
			who = "Signed in as " + yt.Account.Name
		}
		s.row(c, who, "Your home feed, your library, and likes that reach YouTube Music.", func() {
			if s.button(c, "youtube-out", "sign out", false).Clicked() {
				s.ask("sign out of YouTube Music", "MixMusic forgets your YouTube Music session; your library stays on YouTube Music.", "sign out", func() {
					s.account("", "", func(ctx context.Context) error { return s.svc.Accounts.YouTubeLogout(ctx) })
				})
			}
		})
		s.youtubeChannels(c)
		s.row(c, "Add what you play to your history", "YouTube Music makes its home and its radios of your history. Tracks join it once they played a while, as they do on Last.fm.", func() {
			if s.switchOf(c.Key("youtube-history"), &s.set.YouTubeHistory).Label("Add what you play to your YouTube Music history").Changed() {
				s.save()
			}
		})
	})
}

// youtubeChannels chooses the channel of the account YouTube Music is used
// as, when it has more than one.
func (s *shell) youtubeChannels(c *ui.Context) {
	list := fetch(s, "channels", s.svc.Accounts.YouTubeChannels).value
	if len(list) < 2 {
		return
	}
	labels := make([]string, len(list))
	chosen := ""
	for i, ch := range list {
		labels[i] = ch.Name
		if ch.Handle != "" {
			labels[i] += " (" + ch.Handle + ")"
		}
		if ch.Current {
			chosen = labels[i]
		}
	}
	s.row(c, "Channel", "Brand channels of your account have their own home feed, library and likes.", func() {
		if s.selectOf(c.Key("channel"), &chosen, labels).Width(260).Label("Channel").Changed() {
			if i := slices.Index(labels, chosen); i >= 0 {
				ch := list[i]
				s.account("", "Using YouTube Music as "+ch.Name, func(ctx context.Context) error {
					_, err := s.svc.Accounts.SetYouTubeChannel(ctx, ch.PageID)
					return err
				})
			}
		}
	})
}

func (s *shell) lastfmAccount(c *ui.Context, lf app.LastfmAccount) {
	s.group(c, "last.fm", func() {
		if lf.Connected {
			if lf.Expired {
				const title, again = "Last.fm no longer accepts this connection", "Disconnect, then connect again to go on scrobbling."
				s.found(c, title, again, func() { s.alert(c, title, again, nil).Margin(12, 0) })
			}
			s.row(c, "Scrobbling as "+lf.Name, "What you listen to joins your Last.fm profile: a track counts once half of it played, or four minutes.", func() {
				if s.switchOf(c.Key("scrobbling"), &s.set.Scrobbling).Label("Scrobble to Last.fm").Changed() {
					s.save()
				}
			})
			s.found(c, "disconnect", "sign out log out of Last.fm", func() {
				ui.Row(c).Padding(12, 0).Children(func() {
					if s.button(c, "lastfm-out", "disconnect", false).Clicked() {
						s.ask("disconnect Last.fm", "MixMusic forgets your Last.fm connection; your scrobbles stay on Last.fm.", "disconnect", func() {
							s.account("", "", func(ctx context.Context) error { return s.svc.Accounts.LastfmLogout(ctx) })
						})
					}
				})
			})
			return
		}
		// The form shows whole, as its button needs its fields.
		s.found(c, "Connect to Last.fm", lastfmForm, func() {
			s.steps(c,
				func() {
					ui.Text(c, "Create an API account on ")
					s.linkOf(c, "Last.fm", "https://www.last.fm/api/account/create")
					ui.Text(c, "; any name and description will do, and the callback URL can stay empty.")
				},
				func() { ui.Text(c, "Paste its API key and shared secret here, and connect.") },
				func() { ui.Text(c, "Approve MixMusic on the page that opens in your browser.") },
			)
			s.row(c, "API key", "", func() {
				ui.TextInput(c.Key("lastfm-key"), &s.lastfmKey).Placeholder("32 letters and digits").Width(260).Label("API key")
			})
			s.row(c, "Shared secret", "", func() {
				ui.TextInput(c.Key("lastfm-secret"), &s.lastfmSecret).Password().Placeholder("32 letters and digits").Width(260).Label("Shared secret")
			})
			busy := s.signingIn == "lastfm"
			label := "connect to last.fm"
			if busy {
				label = "waiting for Last.fm…"
			}
			ui.Row(c).Padding(12, 0).Children(func() {
				key, secret := strings.TrimSpace(s.lastfmKey), strings.TrimSpace(s.lastfmSecret)
				if s.button(c, "lastfm-in", label, true).Disabled(busy || key == "" || secret == "").Clicked() {
					s.notes = append(s.notes, notice("Approve MixMusic on Last.fm, in your browser"))
					s.account("lastfm", "Connected to Last.fm", func(ctx context.Context) error {
						_, err := s.svc.Accounts.LastfmLogin(ctx, key, secret)
						return err
					})
				}
			})
			s.hint(c, lastfmHint)
		})
	})
}

// steps lists what the user does, in order: paragraphs, which may hold
// links.
func (s *shell) steps(c *ui.Context, steps ...func()) {
	k := &s.lk
	ui.Column(c).Gap(8).Padding(12, 0).Children(func() {
		for i, step := range steps {
			ui.Row(c).Gap(10).AlignItems(ui.Start).Children(func() {
				ui.Text(c, fmt.Sprintf("%d.", i+1)).FontSize(k.bodySize()).TextColor(k.muted).Width(20).Shrink(0)
				ui.RichText(c).FontSize(k.bodySize()).Grow(1).MinWidth(0).Children(step)
			})
		}
	})
}

// account signs in or out beside the main thread, while kind, if any, says
// what waits; then it tells how it went, and the pages ask again for what
// they show, which the accounts change.
func (s *shell) account(kind, done string, do func(ctx context.Context) error) {
	s.signingIn = kind
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		err := do(ctx)
		s.update(func() {
			if s.signingIn == kind {
				s.signingIn = ""
			}
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				s.forget("accounts")
				return
			}
			if done != "" {
				s.notes = append(s.notes, notice(done))
			}
			if kind == "lastfm" {
				s.lastfmSecret = ""
			}
			clear(s.loads)
		})
	}()
}
