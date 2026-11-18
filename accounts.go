package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/youtube"
	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings("accounts", 10, (*shell).accountSettings) }

// accountSettings sign in and out of Spotify, YouTube Music and Last.fm.
func (s *shell) accountSettings(c *ui.Context) {
	st := s.accounts()
	s.spotifyAccount(c, st.Spotify)
	s.youtubeAccount(c, st.YouTube)
	s.lastfmAccount(c, st.Lastfm)
}

func (s *shell) spotifyAccount(c *ui.Context, sp app.SpotifyAccount) {
	s.group(c, "Spotify", func() {
		if sp.SignedIn && sp.Error == "" {
			who := locales.AccountSettingsSignedIn()
			if sp.Profile != nil {
				who = locales.AccountSettingsSignedInAs(sp.Profile.Name)
			}
			s.row(c, who, locales.AccountSettingsSpotifyDesc(), func() {
				if s.button(c, "spotify-out", locales.AccountSettingsSignOut(), false).Clicked() {
					s.signOutOfSpotify()
				}
			}, "sign out, log out")
			return
		}
		if sp.Error != "" {
			why := faultText(sp.Fault, sp.Error)
			s.found(c, locales.AccountSettingsSpotifyRefused(), why, func() {
				s.alert(c, locales.AccountSettingsSpotifyRefused(), why, nil).Margin(12, 0)
			})
		}
		// The form shows whole, as its button needs its field.
		s.found(c, locales.AccountSettingsSpotifySignIn(), locales.AccountSettingsSpotifyForm(), func() {
			if sp.RedirectURI == "" {
				sp.RedirectURI = s.svc.System.Info().RedirectURI
			}
			s.steps(c,
				func() {
					inlineMessage(c, locales.AccountSettingsSpotifyStep1(mark(0, locales.AccountSettingsSpotifyDashboard())), func(text string) {
						s.linkOf(c, text, "https://developer.spotify.com/dashboard")
					})
				},
				func() {
					inlineMessage(c, locales.AccountSettingsSpotifyRedirect(mark(0, sp.RedirectURI), mark(1, locales.AccountSettingsCopy())),
						func(text string) { ui.Text(c, text).Font(monospace()) },
						func(text string) {
							if s.linkOf(c, text, "").Clicked() {
								c.WriteClipboard(sp.RedirectURI)
								s.notes = append(s.notes, notice(locales.AccountSettingsCopiedRedirect()))
							}
						})
				},
				func() {
					ui.Text(c, locales.AccountSettingsSpotifyUsers())
				},
				func() { ui.Text(c, locales.AccountSettingsSpotifyPaste()) },
			)
			var clientID *draft
			s.row(c, locales.AccountsClientID(), "", func() {
				var in ui.Element
				in, clientID = s.settingInput(c, "client-id", s.set.SpotifyClientID, func(id string) {
					s.set.SpotifyClientID = id
					s.save()
				})
				in.Placeholder(locales.AccountSettingsCharsPlaceholder()).Width(260).Label(locales.AccountsClientID())
			})
			busy := s.signingIn == "spotify"
			label := locales.AccountSettingsSignInSpotify()
			if busy {
				label = locales.AccountSettingsWaitingBrowser()
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
					s.notes = append(s.notes, notice(locales.AccountSettingsFinishSpotify()))
					s.account("spotify", locales.AccountSettingsSignedInSpotify(), func(ctx context.Context) error {
						_, err := s.svc.Accounts.SpotifyLogin(ctx)
						return err
					})
				}
				// A sign-in Spotify refused stays until the user drops it: every
				// request of Spotify fails meanwhile.
				if sp.SignedIn && s.button(c, "spotify-out", locales.AccountSettingsSignOut(), false).Disabled(busy).Clicked() {
					s.signOutOfSpotify()
				}
			})
			s.hint(c, locales.AccountSettingsSpotifyHint())
		})
	})
}

// signOutOfSpotify asks, then has MixMusic forget the Spotify sign-in.
func (s *shell) signOutOfSpotify() {
	s.ask(locales.AccountSettingsSpotifySignOutTitle(), locales.AccountSettingsSpotifySignOutBody(), locales.AccountSettingsSignOut(), func() {
		s.account("", "", func(context.Context) error { return s.svc.Accounts.SpotifyLogout() })
	})
}

func (s *shell) youtubeAccount(c *ui.Context, yt app.YouTubeAccount) {
	s.group(c, "YouTube Music", func() {
		if !yt.SignedIn {
			// One sign-in at a time, of the three ways.
			busy := strings.HasPrefix(s.signingIn, "youtube")
			s.found(c, locales.AccountSettingsYouTubeSignInTitle(), locales.AccountSettingsYouTubeForm(), func() {
				s.hint(c, locales.AccountSettingsYouTubeHint())
				label := locales.AccountSettingsYouTubeSignInButton()
				if s.signingIn == "youtube" {
					label = locales.AccountSettingsWaitingWindow()
				}
				if s.button(c, "youtube-in", label, true).Disabled(busy).Clicked() {
					s.youtubeCookies.problem, s.youtubePick = "", false
					s.account("youtube", locales.AccountSettingsYouTubeSignedIn(), func(ctx context.Context) error {
						_, err := s.svc.Accounts.YouTubeLogin(ctx)
						return err
					})
				}
				s.hint(c, locales.AccountSettingsYouTubeSession())
			})
			s.youtubeBrowserSignIn(c, busy)
			s.youtubeCookieSignIn(c, busy)
			return
		}
		if yt.Error != "" {
			again := locales.AccountSettingsYouTubeAgain(faultText(yt.Fault, yt.Error))
			s.found(c, locales.AccountSettingsYouTubeRefused(), again, func() {
				s.alert(c, locales.AccountSettingsYouTubeRefused(), again, nil).Margin(12, 0)
			})
		}
		who := locales.AccountSettingsSignedIn()
		if yt.Account != nil {
			who = locales.AccountSettingsSignedInAs(yt.Account.Name)
		}
		s.row(c, who, locales.AccountSettingsYouTubeSignedInDesc(), func() {
			if s.button(c, "youtube-out", locales.AccountSettingsSignOut(), false).Clicked() {
				s.ask(locales.AccountSettingsYouTubeSignOutTitle(), locales.AccountSettingsYouTubeSignOutBody(), locales.AccountSettingsSignOut(), func() {
					s.youtubePick = false
					s.account("", "", func(ctx context.Context) error { return s.svc.Accounts.YouTubeLogout(ctx) })
				})
			}
		}, "sign out, log out")
		s.youtubeChannels(c)
		s.row(c, locales.AccountSettingsYouTubeHistoryTitle(), locales.AccountSettingsYouTubeHistoryDesc(), func() {
			if s.switchOf(c.Key("youtube-history"), &s.set.YouTubeHistory).Label(locales.AccountSettingsYouTubeHistoryLabel()).Changed() {
				s.save()
			}
		})
	})
}

// youtubeCookieSignIn signs in to YouTube Music with the session of a
// browser the user signed in with, or with a Cookie header they paste:
// where the sign-in window can't open, or Google refuses it. busy is set
// while a sign-in waits. The search of the settings shows the two ways
// whole, with why the last try failed: the buttons need the fields.
func (s *shell) youtubeCookieSignIn(c *ui.Context, busy bool) {
	k, yc := &s.lk, &s.youtubeCookies
	if yc.browsers == nil {
		yc.browsers = s.svc.Accounts.YouTubeBrowsers()
		for _, b := range yc.browsers {
			yc.names = append(yc.names, b.Name)
		}
		yc.browser = yc.names[0]
		yc.words = strings.Join(yc.names, " ")
	}
	if !s.findable(locales.AccountSettingsYouTubeCookieTitle(), locales.AccountSettingsYouTubeCookieForm(), yc.words, yc.problem) {
		return
	}
	s.show(c, func() {
		if yc.problem != "" {
			s.alert(c, locales.AccountSettingsYouTubeDidntSignIn(), yc.problem, nil).Margin(12, 0)
		}
		s.row(c, locales.AccountSettingsYouTubeImportTitle(), locales.AccountSettingsYouTubeImportDesc(), func() {
			// Wide enough for "Chromium (Flatpak)".
			s.selectOf(c.Key("youtube-browser"), &yc.browser, yc.names).Width(200).Label(locales.AccountSettingsYouTubeBrowser())
		})
		s.row(c, locales.AccountSettingsYouTubeProfileTitle(), locales.AccountSettingsYouTubeProfileDesc(), func() {
			ui.TextInput(c.Key("youtube-profile"), &yc.profile).Placeholder(k.sentence(locales.AccountSettingsYouTubeProfilePlaceholder())).Width(260).Label(locales.AccountSettingsYouTubeProfileTitle())
		})
		ui.Row(c).Padding(12, 0).Children(func() {
			label := locales.AccountSettingsYouTubeImportButton()
			if s.signingIn == "youtube-import" {
				label = locales.AccountSettingsYouTubeImporting()
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
		s.hint(c, locales.AccountSettingsYouTubeImportHint())
		ui.Column(c).Gap(8).Padding(14, 0).Children(func() {
			ui.Text(c, locales.AccountSettingsYouTubePasteTitle()).FontSize(k.bodySize())
			ui.Text(c, locales.AccountSettingsYouTubePasteDesc()).FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(720)
			ui.TextArea(c.Key("youtube-cookies"), &yc.pasted).Placeholder("SAPISID=…; SID=…").Height(88).MaxWidth(720).Label(locales.AccountSettingsYouTubeCookieLabel()).
				HandleInput(s.scrollsInside)
			label := locales.AccountSettingsYouTubePasteButton()
			if s.signingIn == "youtube-paste" {
				label = locales.AccountSettingsYouTubeSigningIn()
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
		s.hint(c, locales.AccountSettingsYouTubePasteHint())
	})
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
				msg := app.ErrorWords(err)
				if msg != "" && !strings.HasPrefix(msg, "yt-dlp") {
					msg = strings.ToUpper(msg[:1]) + msg[1:]
				}
				s.youtubeCookies.problem = msg
				s.forget("accounts")
				return
			}
			s.youtubePick = true
			s.notes = append(s.notes, notice(locales.AccountSettingsYouTubeSignedIn()))
			clear(s.loads)
		})
	}()
}

// youtubeChannels chooses the account and channel YouTube Music is used
// as, when the session has more than one, or when the one used is no
// longer of it.
func (s *shell) youtubeChannels(c *ui.Context) {
	// The search of the settings finds the row by what it says, whichever
	// it says, without asking YouTube for the session's channels.
	words := locales.AccountSettingsYouTubeChannelTitle() + " " + locales.AccountSettingsYouTubeChannelAbout() + " " + locales.AccountSettingsYouTubeAccountTitle() + " " + locales.AccountSettingsYouTubeAccountAbout()
	if _, asked := s.loads["channels"]; !asked && !s.findable("", "", words) {
		return
	}
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
		if ch.Unswitchable {
			labels[i] += " · " + locales.AccountSettingsYouTubeUnswitchable()
		}
		if i == current {
			chosen = labels[i]
		}
	}
	title, about := locales.AccountSettingsYouTubeChannelTitle(), locales.AccountSettingsYouTubeChannelAbout()
	if accounts {
		title, about = locales.AccountSettingsYouTubeAccountTitle(), locales.AccountSettingsYouTubeAccountAbout()
	}
	switch {
	case current < 0:
		about = locales.AccountSettingsYouTubeNoAccount()
	case s.youtubePick:
		about = locales.AccountSettingsYouTubeMultiple()
	}
	s.row(c, title, about, func() {
		if s.selectOf(c.Key("channel"), &chosen, labels).Width(260).Label(title).Changed() {
			switch i := slices.Index(labels, chosen); {
			case i < 0:
			case list[i].Unswitchable:
				// It stays listed, and the one used stays chosen.
				s.notes = append(s.notes, noticeOf(errors.New(locales.AccountSettingsYouTubeSwitchNamed(list[i].Name))))
			default:
				ch := list[i]
				s.youtubePick = false
				// The notice names the channel once switched, unless YouTube
				// Music answered as the account itself, which the app tells.
				s.account("", locales.AccountSettingsYouTubeUsingAs(ch.Name), func(ctx context.Context) error {
					_, err := s.svc.Accounts.SetYouTubeChannel(ctx, ch)
					return err
				})
			}
		}
	}, words)
}

// waitingText says how many plays wait to be sent to a scrobbling
// service, by the word it names them by: "scrobble" or "listen".
func waitingText(n int, word, service string) string {
	if word == "listen" {
		return locales.AccountSettingsWaitingListens(n, service)
	}
	return locales.AccountSettingsWaitingScrobbles(n, service)
}

// lastfmExpiredText tells what to do when Last.fm refuses the connection.
func lastfmExpiredText(lf app.LastfmAccount) string {
	text := locales.AccountSettingsLastfmAgain()
	if lf.Waiting > 0 {
		text += " " + locales.AccountSettingsLastfmWaiting(lf.Waiting)
	}
	return text
}

// lastfmOtherText asks what becomes of the waiting scrobbles when another
// Last.fm user approved the new connection.
func lastfmOtherText(lf app.LastfmAccount) string {
	return locales.AccountSettingsLastfmOther(lf.Waiting, lf.Name, lf.Other)
}

// lastfmDisconnectText is the question disconnecting asks.
func lastfmDisconnectText(lf app.LastfmAccount) string {
	text := locales.AccountSettingsDisconnectAskBody()
	if lf.Waiting > 0 {
		text += " " + locales.AccountSettingsLastfmDropped(lf.Waiting)
	}
	return text
}

func (s *shell) lastfmAccount(c *ui.Context, lf app.LastfmAccount) {
	s.group(c, "Last.fm", func() {
		if lf.Connected {
			if lf.Other != "" {
				title, text := locales.AccountSettingsLastfmSendTitle(lf.Other), lastfmOtherText(lf)
				s.found(c, title, text, func() {
					s.alert(c, title, text, func() {
						if s.button(c, "lastfm-send", locales.AccountSettingsLastfmSend(), true).Clicked() {
							s.account("", locales.AccountSettingsConnectedLastfm(), func(ctx context.Context) error { return s.svc.Accounts.LastfmResolve(ctx, true) })
						}
						if s.button(c, "lastfm-drop", locales.AccountSettingsLastfmDrop(), false).Clicked() {
							s.account("", locales.AccountSettingsConnectedLastfm(), func(ctx context.Context) error { return s.svc.Accounts.LastfmResolve(ctx, false) })
						}
					}).Margin(12, 0)
				})
			} else if lf.Expired {
				title := locales.AccountSettingsLastfmGone()
				text := lastfmExpiredText(lf)
				s.found(c, title, text, func() {
					s.alert(c, title, text, func() {
						busy := s.signingIn == "lastfm"
						label := locales.AccountSettingsLastfmConnectAgain()
						if busy {
							label = locales.AccountSettingsWaitingLastfm()
						}
						if s.button(c, "lastfm-again", label, true).Disabled(busy).Clicked() {
							s.notes = append(s.notes, notice(locales.AccountSettingsApproveLastfm()))
							s.account("lastfm", locales.AccountSettingsConnectedLastfm(), func(ctx context.Context) error { return s.svc.Accounts.LastfmReconnect(ctx) })
						}
					}).Margin(12, 0)
				})
			}
			s.row(c, locales.AccountSettingsScrobblingAs(lf.Name), locales.AccountSettingsScrobbleDesc(), func() {
				if s.switchOf(c.Key("scrobbling"), &s.set.Scrobbling).Label(locales.AccountSettingsScrobbleSwitch()).Changed() {
					s.save()
				}
			})
			if lf.Waiting > 0 && !lf.Expired && lf.Other == "" {
				s.hint(c, waitingText(lf.Waiting, "scrobble", "Last.fm"))
			}
			s.row(c, locales.AccountSettingsLastfmLoves(), locales.AccountSettingsLastfmLovesDesc(), func() {
				if s.switchOf(c.Key("lastfm-loves"), &s.set.LastfmLoves).Label(locales.AccountSettingsLastfmLovesLabel()).Changed() {
					s.save()
				}
			})
			s.found(c, locales.AccountSettingsDisconnect(), "sign out log out of Last.fm", func() {
				ui.Row(c).Padding(12, 0).Children(func() {
					if s.button(c, "lastfm-out", locales.AccountSettingsDisconnect(), false).Clicked() {
						s.ask(locales.AccountSettingsDisconnectAskTitle(), lastfmDisconnectText(lf), locales.AccountSettingsDisconnect(), func() {
							s.account("", "", func(ctx context.Context) error { return s.svc.Accounts.LastfmLogout(ctx) })
						})
					}
				})
			})
			return
		}
		// The form shows whole, as its button needs its fields.
		s.found(c, locales.AccountSettingsConnectLastfm(), locales.AccountSettingsLastfmForm(), func() {
			s.steps(c,
				func() {
					inlineMessage(c, locales.AccountSettingsLastfmStep1(mark(0, "Last.fm")), func(text string) {
						s.linkOf(c, text, "https://www.last.fm/api/account/create")
					})
				},
				func() { ui.Text(c, locales.AccountSettingsLastfmStep2()) },
				func() { ui.Text(c, locales.AccountSettingsLastfmStep3()) },
			)
			s.row(c, locales.AccountSettingsAPIKey(), "", func() {
				ui.TextInput(c.Key("lastfm-key"), &s.lastfmKey).Placeholder(locales.AccountSettingsCharsPlaceholder()).Width(260).Label(locales.AccountSettingsAPIKey())
			})
			s.row(c, locales.AccountSettingsSharedSecret(), "", func() {
				ui.TextInput(c.Key("lastfm-secret"), &s.lastfmSecret).Password().Placeholder(locales.AccountSettingsCharsPlaceholder()).Width(260).Label(locales.AccountSettingsSharedSecret())
			})
			busy := s.signingIn == "lastfm"
			label := locales.AccountSettingsConnectButton()
			if busy {
				label = locales.AccountSettingsWaitingLastfm()
			}
			ui.Row(c).Padding(12, 0).Children(func() {
				key, secret := strings.TrimSpace(s.lastfmKey), strings.TrimSpace(s.lastfmSecret)
				if s.button(c, "lastfm-in", label, true).Disabled(busy || key == "" || secret == "").Clicked() {
					s.notes = append(s.notes, notice(locales.AccountSettingsApproveLastfm()))
					s.account("lastfm", locales.AccountSettingsConnectedLastfm(), func(ctx context.Context) error {
						_, err := s.svc.Accounts.LastfmLogin(ctx, key, secret)
						return err
					})
				}
			})
			s.hint(c, locales.AccountSettingsLastfmHint())
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
				s.notes = append(s.notes, wordsNotice(err))
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

// faultText is fault, the error a state's text tells of, as the windows
// show it, or text when the state has no fault, as one read from a
// window's earlier state.
func faultText(fault error, text string) string {
	if fault != nil {
		return errorText(fault)
	}
	return text
}
