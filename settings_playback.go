package main

import (
	"context"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/settings"
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
	s.group(c, "Spotify tracks", func() {
		here := "MixMusic plays them itself, at Spotify's own quality, and shows among your Spotify devices. Needs Spotify Premium, and no Spotify app."
		if !spotifyPlaysHere {
			here = "This computer's webview, WebKitGTK, can't play Spotify's protected audio, so MixMusic can't play Spotify itself."
		}
		modes := []settings.SpotifyPlayback{settings.SpotifyPlaybackHere, settings.SpotifyPlaybackConnect, settings.SpotifyPlaybackMatch}
		mode := slices.Index(modes, s.set.SpotifyPlayback)
		if s.choices(c, "spotify-mode", "How Spotify tracks play", &mode, []choice{
			{k.words("In MixMusic, from Spotify"), here, !spotifyPlaysHere},
			{k.words("On Spotify Connect"), "Your Spotify app, or another Spotify device, plays them, at Spotify's own quality. Needs Spotify Premium and a device with Spotify open.", false},
			{k.words("In MixMusic, from YouTube Music"), "MixMusic plays their recording on YouTube Music, with the equalizer, crossfade and visualizer. Works without Premium.", false},
		}) {
			s.set.SpotifyPlayback = modes[mode]
			s.save()
		}
		switch {
		case !signedIn:
			s.found(c, "", "Sign in to Spotify in accounts to play your Spotify library.", func() {
				ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Padding(12, 0).Children(func() {
					ui.Text(c, "Sign in to Spotify in ")
					s.linkOf(c, "accounts", "/settings/accounts")
					ui.Text(c, " to play your Spotify library.")
				})
			})
		case s.set.SpotifyPlayback == settings.SpotifyPlaybackHere && spotifyPlaysHere && !sp.Streaming:
			busy := s.signingIn == "spotify"
			label := "sign in again"
			if busy {
				label = "waiting for the browser…"
			}
			const title, again = "Sign in to Spotify again", "You signed in before MixMusic played Spotify itself, which your sign-in doesn't allow yet. Until you sign in again, Spotify tracks play from YouTube Music."
			s.found(c, title, again, func() {
				s.alert(c, title, again, func() {
					if s.button(c, "spotify-again", label, true).Disabled(busy).Clicked() {
						s.notes = append(s.notes, notice("Finish signing in in your browser"))
						s.account("spotify", "Signed in to Spotify", func(ctx context.Context) error {
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

// spotifyLosslessPlan is the plan Spotify streams lossless audio to:
// Premium, and in the countries where Spotify sells Premium Lite,
// Standard and Platinum, as India and Indonesia, Platinum alone.
const spotifyLosslessPlan = "It needs Spotify Premium, or Premium Platinum in the countries where Spotify sells Lite, Standard and Platinum plans. "

// spotifyLosslessSteps turn Lossless on in Spotify's own app, as Spotify's
// help tells them, and on another Spotify device from the phone's app.
const spotifyLosslessSteps = "In the Spotify app, choose your profile picture › Settings and privacy › Media quality › Lossless (the desktop app 1.2.67 or later). For another Spotify device, in Spotify on your phone, tap the devices button while it plays, choose the device, then Change quality settings › Lossless."

// spotifyLossless tells how Spotify's lossless audio plays with the way
// Spotify tracks play, mode: Spotify streams it to its own apps and some
// Connect devices alone, so only On Spotify Connect plays it.
func spotifyLossless(mode settings.SpotifyPlayback) string {
	if mode == settings.SpotifyPlaybackConnect {
		return "Lossless: the device plays it once it's on there, in up to 24-bit quality. " + spotifyLosslessPlan + spotifyLosslessSteps
	}
	return "Lossless: Spotify streams it only to its own apps and some Spotify Connect devices, so it plays only with On Spotify Connect, with the Spotify app open. " + spotifyLosslessPlan + spotifyLosslessSteps
}

// deviceRow chooses the Spotify Connect device that plays Spotify tracks.
func (s *shell) deviceRow(c *ui.Context) {
	s.row(c, "Device", "Where Spotify tracks play. Open Spotify on a device for it to show here.", func() {
		devices := fetch(s, "devices", s.svc.Player.Devices)
		labels, ids := []string{"The active device"}, []string{""}
		for _, d := range devices.value {
			labels, ids = append(labels, d.Name), append(ids, d.ID)
		}
		chosen := "a device not found now"
		if i := slices.Index(ids, s.set.SpotifyDevice); i >= 0 {
			chosen = labels[i]
		}
		if s.selectOf(c.Key("device"), &chosen, labels).Width(220).Label("Device").Changed() {
			if i := slices.Index(labels, chosen); i >= 0 {
				s.setDevice(ids[i])
			}
		}
		if s.iconButton(c, "devices-again", "refresh", "Look for devices again", false, 16).Clicked() {
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
	s.group(c, "Sound", func() { s.settingsOf(c, soundRows) })
}

func init() { addSettings(soundRows, 10, (*shell).qualityRow) }

// qualityRow chooses between the best audio and smaller streams.
func (s *shell) qualityRow(c *ui.Context) {
	s.row(c, "Quality", "Data saver streams smaller files of YouTube Music.", func() {
		saver := 0
		if s.set.Quality == settings.QualitySaver {
			saver = 1
		}
		if s.segmented(c, "quality", "Quality", &saver, "best", "data saver") {
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
	s.row(c, "Autoplay", "When the queue runs out, a radio of what you played carries on.", func() {
		if s.switchOf(c.Key("autoplay"), &s.set.Autoplay).Label("Autoplay").Changed() {
			s.save()
		}
	})
}
