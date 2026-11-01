package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// accountSettings sign in and out of Spotify, YouTube Music and Last.fm.
func (s *shell) accountSettings(c *ui.Context) {
	st := s.accounts()
	s.spotifyAccount(c, st.Spotify)
	s.youtubeAccount(c, st.YouTube)
	s.lastfmAccount(c, st.Lastfm)
}

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
			s.alert(c, "Spotify refused the sign-in", sp.Error, nil).Margin(12, 0)
		}
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
			id := strings.TrimSpace(clientID.text)
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
		s.hint(c, "Spotify lets an app in development mode serve up to five accounts added under Users Management, while its owner has Premium. Search shows ten results of each kind, and the tracks of playlists you do not own stay hidden to apps.")
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
			s.hint(c, "YouTube Music needs no account: search, artists, albums, playlists and radios work signed out. Signed in, MixMusic shows your own home feed and library, and saves what you like and follow to YouTube Music. Tracks play as they do signed out: your session never goes to yt-dlp.")
			// One sign-in at a time, of the three ways.
			busy := strings.HasPrefix(s.signingIn, "youtube")
			label := "sign in to youtube music"
			if s.signingIn == "youtube" {
				label = "waiting for the sign-in window…"
			}
			if s.button(c, "youtube-in", label, true).Disabled(busy).Clicked() {
				s.youtubeCookies.problem, s.youtubePick = "", false
				s.account("youtube", "Signed in to YouTube Music", func(ctx context.Context) error {
					_, err := s.svc.Accounts.YouTubeLogin(ctx)
					return err
				})
			}
			s.hint(c, "You sign in on Google's own page, in a window of MixMusic: MixMusic never sees your password, and keeps the session encrypted on this computer. YouTube Music does not offer this to apps officially, so it may stop working when YouTube changes.")
			s.youtubeCookieSignIn(c, busy)
			return
		}
		if yt.Error != "" {
			s.alert(c, "YouTube Music refused the sign-in", yt.Error+". Sign out, then sign in again.", nil).Margin(12, 0)
		}
		who := "Signed in"
		if yt.Account != nil {
			who = "Signed in as " + yt.Account.Name
		}
		s.row(c, who, "Your home feed, your library, and likes that reach YouTube Music.", func() {
			if s.button(c, "youtube-out", "sign out", false).Clicked() {
				s.ask("sign out of YouTube Music", "MixMusic forgets your YouTube Music session; your library stays on YouTube Music.", "sign out", func() {
					s.youtubePick = false
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

// youtubeCookieSignIn signs in to YouTube Music with the session of a
// browser the user signed in with, or with a Cookie header they paste:
// where the sign-in window can't open, or Google refuses it. busy is set
// while a sign-in waits.
func (s *shell) youtubeCookieSignIn(c *ui.Context, busy bool) {
	k, yc := &s.lk, &s.youtubeCookies
	if yc.browsers == nil {
		yc.browsers = s.svc.Accounts.YouTubeBrowsers()
		for _, b := range yc.browsers {
			yc.names = append(yc.names, b.Name)
		}
		yc.browser = yc.names[0]
	}
	if yc.problem != "" {
		s.alert(c, "MixMusic didn't sign in to YouTube Music", yc.problem, nil).Margin(12, 0)
	}
	s.row(c, "Import from a browser", "Sign in with the session of a browser you signed in to YouTube Music with.", func() {
		s.selectOf(c.Key("youtube-browser"), &yc.browser, yc.names).Width(160).Label("Browser")
	})
	s.row(c, "Profile", "Another profile than the browser's own, by its name or its folder. MixMusic finds those of Zen and Helium in their folders.", func() {
		ui.TextInput(c.Key("youtube-profile"), &yc.profile).Placeholder("the browser's own").Width(260).Label("Profile")
	})
	ui.Row(c).Padding(12, 0).Children(func() {
		label := "import"
		if s.signingIn == "youtube-import" {
			label = "importing…"
		}
		if s.button(c, "youtube-import", label, false).Disabled(busy).Clicked() {
			browser, profile := "", yc.profile
			if i := slices.Index(yc.names, yc.browser); i >= 0 {
				browser = yc.browsers[i].ID
			}
			s.youtubeSignIn("youtube-import", func(ctx context.Context) error {
				_, err := s.svc.Accounts.YouTubeImportCookies(ctx, browser, profile)
				return err
			})
		}
	})
	s.hint(c, "MixMusic reads the browser's cookies with yt-dlp, keeps YouTube's alone, encrypted on this computer, and deletes the rest at once. Close the browser first: while it's open, it may hold its cookies, or not have saved a new sign-in yet. Recent Chrome on Windows seals its cookies against other programs: use Firefox there, or paste a cookie header.")
	ui.Column(c).Gap(8).Padding(14, 0).Children(func() {
		ui.Text(c, "Paste a cookie header").FontSize(k.bodySize())
		ui.Text(c, "In a tab signed in to music.youtube.com, open the developer tools, and copy the Cookie header of a request to music.youtube.com from the Network panel. The text of a cookies.txt file will do too.").
			FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(720)
		ui.TextArea(c.Key("youtube-cookies"), &yc.pasted).Placeholder("SAPISID=…; SID=…").Height(88).MaxWidth(720).Label("Cookie header")
		label := "sign in with it"
		if s.signingIn == "youtube-paste" {
			label = "signing in…"
		}
		if s.button(c, "youtube-paste", label, false).Disabled(busy || strings.TrimSpace(yc.pasted) == "").Clicked() {
			pasted := yc.pasted
			yc.pasted = "" // it holds the session: it stays no longer than this try
			s.youtubeSignIn("youtube-paste", func(ctx context.Context) error {
				_, err := s.svc.Accounts.YouTubePasteCookies(ctx, pasted)
				return err
			})
		}
	})
	s.hint(c, "A cookie header is as good as your password: paste it here alone. YouTube may end a session that goes on in the browser; one from a private window you then close lasts longest.")
}

// youtubeSignIn signs in to YouTube Music with cookies beside the main
// thread, as do says, while kind says which way waits. A sign-in that
// fails says why above the ways, until the next try; one that works asks
// for the account and channel, when the session has more than one.
func (s *shell) youtubeSignIn(kind string, do func(ctx context.Context) error) {
	s.signingIn, s.youtubeCookies.problem = kind, ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		err := do(ctx)
		s.update(func() {
			if s.signingIn == kind {
				s.signingIn = ""
			}
			if err != nil {
				// A sentence of the alert, but yt-dlp's name as it is.
				msg := err.Error()
				if msg != "" && !strings.HasPrefix(msg, "yt-dlp") {
					msg = strings.ToUpper(msg[:1]) + msg[1:]
				}
				s.youtubeCookies.problem = msg
				s.forget("accounts")
				return
			}
			s.youtubePick = true
			s.notes = append(s.notes, notice("Signed in to YouTube Music"))
			clear(s.loads)
		})
	}()
}

// youtubeChannels chooses the account and channel YouTube Music is used
// as, when the session has more than one, or when the one used is no
// longer of it.
func (s *shell) youtubeChannels(c *ui.Context) {
	list := fetch(s, "channels", s.svc.Accounts.YouTubeChannels).value
	current := slices.IndexFunc(list, func(ch youtube.AccountChannel) bool { return ch.Current })
	if len(list) < 2 && (len(list) == 0 || current >= 0) {
		return
	}
	accounts := slices.ContainsFunc(list, func(ch youtube.AccountChannel) bool { return ch.Email != list[0].Email })
	labels := make([]string, len(list))
	chosen := ""
	for i, ch := range list {
		labels[i] = ch.Name
		if ch.Handle != "" {
			labels[i] += " (" + ch.Handle + ")"
		}
		if accounts && ch.Email != "" {
			labels[i] += " · " + ch.Email
		}
		if i == current {
			chosen = labels[i]
		}
	}
	title, about := "Channel", "Brand channels of your account have their own home feed, library and likes."
	if accounts {
		title, about = "Account and channel", "Each Google account of the session, and each of their brand channels, has its own home feed, library and likes."
	}
	switch {
	case current < 0:
		about = "The account or channel MixMusic used is no longer one of this session: choose one."
	case s.youtubePick:
		about = "Your session reaches more than one account or channel. MixMusic uses the first account: choose another here."
	}
	s.row(c, title, about, func() {
		if s.selectOf(c.Key("channel"), &chosen, labels).Width(260).Label(title).Changed() {
			if i := slices.Index(labels, chosen); i >= 0 {
				ch := list[i]
				s.youtubePick = false
				s.account("", "Using YouTube Music as "+ch.Name, func(ctx context.Context) error {
					_, err := s.svc.Accounts.SetYouTubeChannel(ctx, ch.AuthUser, ch.PageID)
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
				s.alert(c, "Last.fm no longer accepts this connection", "Disconnect, then connect again to go on scrobbling.", nil).Margin(12, 0)
			}
			s.row(c, "Scrobbling as "+lf.Name, "What you listen to joins your Last.fm profile: a track counts once half of it played, or four minutes.", func() {
				if s.switchOf(c.Key("scrobbling"), &s.set.Scrobbling).Label("Scrobble to Last.fm").Changed() {
					s.save()
				}
			})
			ui.Row(c).Padding(12, 0).Children(func() {
				if s.button(c, "lastfm-out", "disconnect", false).Clicked() {
					s.ask("disconnect Last.fm", "MixMusic forgets your Last.fm connection; your scrobbles stay on Last.fm.", "disconnect", func() {
						s.account("", "", func(ctx context.Context) error { return s.svc.Accounts.LastfmLogout(ctx) })
					})
				}
			})
			return
		}
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
		s.hint(c, "MixMusic keeps them in your system's credential store, with the connection.")
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
