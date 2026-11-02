package main

import (
	"context"
	"fmt"
	"runtime"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// spotifyPlaysHere tells whether MixMusic plays Spotify itself: where its
// webview, WebView2, decrypts Spotify's audio.
var spotifyPlaysHere = runtime.GOOS == "windows"

func (s *shell) playbackSettings(c *ui.Context) {
	k := &s.lk
	sp := s.accounts().Spotify
	signedIn := sp.SignedIn
	s.group(c, "Spotify tracks", func() {
		here := "MixMusic plays them itself, at Spotify's own quality, and shows among your Spotify devices. Needs Spotify Premium, and no Spotify app."
		if !spotifyPlaysHere {
			here = "This computer's webview can't play Spotify's protected audio, so MixMusic can't play Spotify itself."
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
	})
	s.group(c, "Sound", func() {
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
		desc := "Off: tracks follow each other."
		if s.set.Crossfade > 0 {
			desc = fmt.Sprintf("Tracks overlap by %d seconds.", s.set.Crossfade)
		}
		s.row(c, "Crossfade", desc, func() {
			v := float64(s.set.Crossfade)
			if ui.StepSlider(c.Key("crossfade"), &v, 0, 12, 1).Width(200).Label("Crossfade").Changed() {
				s.set.Crossfade = int(v + 0.5)
				s.save()
			}
		})
		even := "Turns loud tracks down to one level, from YouTube's measure of each, so none jumps out. Quiet recordings keep their level."
		if s.set.SpotifyPlayback == settings.SpotifyPlaybackConnect || s.set.SpotifyPlayback == settings.SpotifyPlaybackHere && spotifyPlaysHere {
			even += " Spotify plays its own tracks at its own loudness."
		}
		s.row(c, "Even loudness", even, func() {
			if s.switchOf(c.Key("loudness"), &s.set.Loudness).Label("Even loudness").Changed() {
				s.save()
			}
		})
		s.row(c, "Volume levelling", "Squeezes loud and quiet moments closer together as they play, and lifts quiet recordings.", func() {
			if s.switchOf(c.Key("normalize"), &s.set.Normalize).Label("Volume levelling").Changed() {
				s.save()
			}
		}, "normalize loudness")
		s.row(c, "Play 5.1 surround when a track has it", surroundDescription, func() {
			if s.switchOf(c.Key("surround"), &s.set.Surround).Label("Play 5.1 surround when a track has it").Changed() {
				s.save()
			}
		})
		s.row(c, "Autoplay", "When the queue runs out, a radio of what you played carries on.", func() {
			if s.switchOf(c.Key("autoplay"), &s.set.Autoplay).Label("Autoplay").Changed() {
				s.save()
			}
		})
		s.row(c, "Skip songs made with AI", "Skips the tracks YouTube labels as made with AI. YouTube labels only some of them, so others still play.", func() {
			if s.switchOf(c.Key("skip-ai"), &s.set.SkipAI).Label("Skip songs made with AI").Changed() {
				s.save()
			}
		})
		s.row(c, "Skip songs you dislike", "Songs you dislike don't play when the queue moves on, and radios leave them out.", func() {
			if s.switchOf(c.Key("skip-disliked"), &s.set.SkipDisliked).Label("Skip songs you dislike").Changed() {
				s.save()
			}
		})
		// Where dislikes go, and, once there are any, a link to them.
		note, see := dislikesNote, len(s.svc.Collection.Disliked()) > 0
		if see {
			note += " See the songs you dislike."
		}
		s.found(c, "", note, func() {
			ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Padding(12, 0).MaxWidth(720).Children(func() {
				ui.Text(c, dislikesNote)
				if see {
					ui.Text(c, " ")
					s.linkOf(c, "See the songs you dislike", "/library/disliked")
					ui.Text(c, ".")
				}
			})
		})
	})
	s.speedSettings(c)
	s.group(c, "Music videos", func() {
		desc := "On the now playing page, a music video plays its picture in place of the cover, muted and in time with the sound. The picture takes about 3 MB a minute more, under 1 MB with data saver."
		problem := s.videoProblem()
		if problem != "" {
			desc = problem
		}
		s.row(c, "Show the video of music videos", desc, func() {
			if s.switchOf(c.Key("music-videos"), &s.set.MusicVideos).Label("Show the video of music videos").Disabled(problem != "").Changed() {
				s.save()
			}
		})
	})
	s.equalizerSettings(c)
	s.lyricsSettings(c)
	s.cacheSettings(c)
	s.downloadSettings(c)
	s.toolSettings(c)
}

// surroundDescription tells what 5.1 surround does, in Settings › Playback.
const surroundDescription = "Some tracks have a 5.1 surround mix beside their stereo one. It plays on six speakers where your output has them, " +
	"else mixed down to stereo. The 5.1 button by the player turns it on and off too."

// dislikesNote is what the note under Skip songs you dislike says of where
// dislikes go.
const dislikesNote = "Signed in to YouTube Music, your dislikes of its songs go there too. Spotify takes no dislikes from apps: those of its tracks stay in MixMusic."

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
