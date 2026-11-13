package main

import (
	"context"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// The groups of Settings › Playback: the Spotify tracks and the sound
// here, the others in their features' files.

// spotifyPlaysHere tells whether MixMusic plays Spotify itself: on Windows
// and macOS, whose webviews decrypt Spotify's audio.
var spotifyPlaysHere = app.SpotifyPlaysHere()

func init() { addSettings("playback", 10, (*shell).spotifyTrackSettings) }

// spotifyTrackSettings choose how Spotify tracks play, and where.
func (s *shell) spotifyTrackSettings(c *ui.Context) {
	k := &s.lk
	sp := s.accounts().Spotify
	signedIn := sp.SignedIn
	s.group(c, locales.PlaybackSpotifyTracks(), func() {
		here := locales.PlaybackSpotifyHere()
		if !spotifyPlaysHere {
			here = locales.PlaybackSpotifyNoWebKit()
		}
		modes := []settings.SpotifyPlayback{settings.SpotifyPlaybackHere, settings.SpotifyPlaybackConnect, settings.SpotifyPlaybackMatch}
		mode := slices.Index(modes, s.set.SpotifyPlayback)
		if s.choices(c, "spotify-mode", locales.PlaybackHowSpotifyPlays(), &mode, []choice{
			{k.words(locales.PlaybackFromSpotify()), here, !spotifyPlaysHere},
			{k.words(locales.PlaybackOnConnect()), locales.PlaybackOnConnectDesc(), false},
			{k.words(locales.PlaybackFromYouTube()), locales.PlaybackFromYouTubeDesc(), false},
		}) {
			s.set.SpotifyPlayback = modes[mode]
			s.save()
		}
		switch {
		case !signedIn:
			s.found(c, "", locales.PlaybackSignInSpotify(), func() {
				ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Padding(12, 0).Children(func() {
					ui.Text(c, locales.PlaybackSignInSpotifyLead())
					s.linkOf(c, locales.SettingsTabAccounts(), "/settings/accounts")
					ui.Text(c, locales.PlaybackSignInSpotifyTail())
				})
			})
		case s.set.SpotifyPlayback == settings.SpotifyPlaybackHere && spotifyPlaysHere && !sp.Streaming:
			busy := s.signingIn == "spotify"
			label := locales.PlaybackSignInAgain()
			if busy {
				label = locales.PlaybackWaitingBrowser()
			}
			title, again := locales.PlaybackSignInAgainTitle(), locales.PlaybackSignInAgainBody()
			s.found(c, title, again, func() {
				s.alert(c, title, again, func() {
					if s.button(c, "spotify-again", label, true).Disabled(busy).Clicked() {
						s.notes = append(s.notes, notice(locales.PlaybackFinishSignIn()))
						s.account("spotify", locales.PlaybackSignedInSpotify(), func(ctx context.Context) error {
							_, err := s.svc.Accounts.SpotifyLogin(ctx)
							return err
						})
					}
				}).Margin(12, 0)
			})
		case s.set.SpotifyPlayback == settings.SpotifyPlaybackConnect:
			s.deviceRow(c)
		}
		s.hint(c, spotifyLossless(s.set.SpotifyPlayback))
	})
}

// spotifyLossless tells how Spotify's lossless audio plays with the way
// Spotify tracks play, mode: Spotify streams it to its own apps and some
// Connect devices alone, so only On Spotify Connect plays it.
func spotifyLossless(mode settings.SpotifyPlayback) string {
	if mode == settings.SpotifyPlaybackConnect {
		return locales.PlaybackLosslessConnect()
	}
	return locales.PlaybackLosslessOther()
}

// deviceRow chooses the Spotify Connect device that plays Spotify tracks.
func (s *shell) deviceRow(c *ui.Context) {
	s.row(c, locales.PlaybackDevice(), locales.PlaybackDeviceDesc(), func() {
		devices := fetch(s, "devices", s.svc.Player.Devices)
		labels, ids := []string{locales.PlaybackActiveDevice()}, []string{""}
		for _, d := range devices.value {
			labels, ids = append(labels, d.Name), append(ids, d.ID)
		}
		chosen := locales.PlaybackDeviceMissing()
		if i := slices.Index(ids, s.set.SpotifyDevice); i >= 0 {
			chosen = labels[i]
		}
		if s.selectOf(c.Key("device"), &chosen, labels).Width(220).Label(locales.PlaybackDevice()).Changed() {
			if i := slices.Index(labels, chosen); i >= 0 {
				s.setDevice(ids[i])
			}
		}
		if s.iconButton(c, "devices-again", "refresh", locales.PlaybackLookAgain(), false, 16).Clicked() {
			s.forget("devices")
		}
	})
}

// soundRows are the rows of Settings › Playback › Sound, which register
// from their features' files.
const soundRows = "playback › Sound"

func init() { addSettings("playback", 20, (*shell).soundSettings) }

// soundSettings show the rows of the sound.
func (s *shell) soundSettings(c *ui.Context) {
	s.group(c, locales.PlaybackSound(), func() { s.settingsOf(c, soundRows) })
}

func init() { addSettings(soundRows, 10, (*shell).qualityRow) }

// qualityRow chooses between the best audio and smaller streams.
func (s *shell) qualityRow(c *ui.Context) {
	s.row(c, locales.PlaybackQuality(), locales.PlaybackQualityDesc(), func() {
		saver := 0
		if s.set.Quality == settings.QualitySaver {
			saver = 1
		}
		if s.segmented(c, "quality", locales.PlaybackQuality(), &saver, locales.PlaybackQualityBest(), locales.PlaybackQualitySaver()) {
			s.set.Quality = settings.QualityBest
			if saver == 1 {
				s.set.Quality = settings.QualitySaver
			}
			s.save()
		}
	}, "best bitrate")
}

func init() { addSettings(soundRows, 60, (*shell).autoplayRow) }

// autoplayRow carries on with a radio as the queue runs out.
func (s *shell) autoplayRow(c *ui.Context) {
	s.row(c, locales.PlaybackAutoplay(), locales.PlaybackAutoplayDesc(), func() {
		if s.switchOf(c.Key("autoplay"), &s.set.Autoplay).Label(locales.PlaybackAutoplay()).Changed() {
			s.save()
		}
	})
}
