package main

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// speedSteps are the speeds the speed button's menu offers, which its
// slider goes between and past, from settings.MinSpeed to
// settings.MaxSpeed.
var speedSteps = []float64{0.75, 1, 1.1, 1.25, 1.5, 1.75, 2}

// playingSpeed is the setting of the speed of the kind of track that plays,
// and whether it's the podcasts': an episode plays at theirs, a song, or
// nothing, at the songs'.
func (s *shell) playingSpeed() (speed *float64, podcasts bool) {
	if cur, ok := s.st.Current(); ok && cur.Track.IsEpisode() {
		return &s.set.PodcastSpeed, true
	}
	return &s.set.SongSpeed, false
}

// speedTitle names the speed of songs, or of podcasts.
func speedTitle(podcasts bool) string {
	if podcasts {
		return "Speed of podcasts"
	}
	return "Speed of songs"
}

// setSpeed sets the speed of the kind of track that plays, which the
// settings keep and the player takes in.
func (s *shell) setSpeed(v float64) {
	speed, _ := s.playingSpeed()
	*speed = settings.CleanSpeed(v)
	s.save()
}

// speedLocked tells why the speed of what plays can't change, or "" when
// it can: Spotify's players, in MixMusic or on Spotify Connect, play its
// tracks and episodes at their own speed, as a live stream plays as it's
// broadcast. A Spotify track not on its device yet, as it loads, goes
// there, unless Spotify tracks play from YouTube Music.
func (s *shell) speedLocked() string {
	const spotify = "Spotify plays this one at its own speed"
	src := s.st.Source
	switch {
	case src != nil && src.Mode != player.ModeStream:
		return spotify
	case s.st.Live:
		return "this live stream plays at its own speed"
	case src == nil && s.set.SpotifyPlayback != settings.SpotifyPlaybackMatch:
		if cur, ok := s.st.Current(); ok && cur.Track.Source == media.SourceSpotify {
			return spotify
		}
	}
	return ""
}

// speedButton shows the speed of the kind of track that plays, beside the
// volume, unless it's 1×, and opens speedMenu, which changes it. While it
// can't change, the button is dimmed, and its tip tells why: MyGo shows no
// tip on a disabled button, so the box around it shows the button's.
func (s *shell) speedButton(c *ui.Context) {
	k := &s.lk
	speed, podcasts := s.playingSpeed()
	why := s.speedLocked()
	label := "Speed"
	switch {
	case why != "":
		label += ": " + why
	case *speed != 1:
		label += ": " + speedLabel(*speed)
	}
	tip := ui.Box(c.Key("speed-tip")).Shrink(0)
	open := ui.Local(tip, "open", func() bool { return false })
	if why != "" {
		*open = false
	}
	// The tip would cover the menu.
	if !*open {
		tip.Tooltip(label)
	}
	tip.Children(func() {
		b := s.buttonBase(c.Key("speed")).Height(32).MinWidth(32).PaddingX(6).Radius(k.round(32)).Label(label).
			Cursor(ui.CursorPointer).Disabled(why != "").Expanded(*open)
		ink := k.muted
		switch {
		case why != "":
			b.Opacity(0.45)
		case *speed != 1:
			ink = k.accent
		case *open || b.Hovered():
			ink = k.text
		}
		if b.Pressed() {
			b.Background(k.press)
		} else if *open || b.Hovered() {
			b.Background(k.hover)
		}
		s.pressMotion(b, 0.9)
		if b.Clicked() {
			*open = !*open
		}
		b.Children(func() {
			if why != "" || *speed == 1 {
				ui.Icon(c, icon("speed")).FontSize(18).TextColor(ink)
				return
			}
			ui.Text(c, speedLabel(*speed)).FontSize(k.smallSize()).FontWeight(600).FontFeatures("tnum").TextColor(ink).
				SingleLine().Shrink(0)
		})
		s.speedMenu(c, b, open, speed, podcasts)
	})
}

// speedMenu is the speed button's menu, above it while *open: the steps
// of speedSteps, the one set checked, which set the speed and close it,
// and a slider for the speeds between and past them.
func (s *shell) speedMenu(c *ui.Context, anchor ui.Element, open *bool, speed *float64, podcasts bool) {
	k := &s.lk
	title := speedTitle(podcasts)
	ui.PopoverBase(c, anchor, open, func(panel ui.Element) {
		panel.AttachTo(anchor, ui.AnchorTopEnd, ui.AnchorBottomEnd).MarginY(8).Width(220).Padding(6).Gap(2).
			Radius(k.radiusSm).Background(k.raised).Border(1, k.border).Shadow(0, 12, 32, -8, ui.RGBA(0, 0, 0, 0.55)).
			TextColor(k.text).Font(k.body)
		panel.Children(func() {
			ui.Text(c, k.words(title)).FontSize(k.captionSize()).TextColor(k.muted).Padding(4, 10, 6).SingleLine()
			for _, step := range speedSteps {
				on := step == *speed
				label := speedLabel(step)
				b := s.buttonBase(c.Key("speed-step-" + label)).Height(32).PaddingX(10).Gap(10).Radius(k.radiusSm).
					Justify(ui.Start).AlignItems(ui.Center).Label(label).Role(ui.RoleMenuItemRadio).Checked(on).
					Cursor(ui.CursorPointer)
				if b.Pressed() {
					b.Background(k.press)
				} else if b.Hovered() {
					b.Background(k.hover)
				}
				if b.Clicked() {
					s.setSpeed(step)
					*open = false
				}
				b.Children(func() {
					ui.Box(c).Size(16, 16).Shrink(0).Children(func() {
						if on {
							ui.Icon(c, icon("check")).FontSize(16).TextColor(k.accent)
						}
					})
					ui.Text(c, label).FontSize(k.bodySize()).FontFeatures("tnum").SingleLine()
					if step == 1 {
						ui.Text(c, k.sentence("normal")).FontSize(k.smallSize()).TextColor(k.muted).SingleLine()
					}
				})
			}
			ui.Box(c).Height(1).Margin(4, 6).Background(k.border)
			ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(4, 10).Children(func() {
				// The window's arrows keep seeking and setting the volume.
				x := *speed
				slider := ui.Slider(c.Key("speed-slider"), &x, settings.MinSpeed, settings.MaxSpeed).KeepFocus()
				if slider.Step(settings.SpeedStep).Grow(1).MinWidth(0).Label(title).Changed() {
					s.setSpeed(x)
				}
				ui.Text(c, speedLabel(*speed)).FontSize(k.smallSize()).FontFeatures("tnum").TextColor(k.muted).
					Width(40).TextAlign(ui.End).SingleLine()
			})
		})
	})
}

// speedItems are the speed of the kind of track that plays in a menu, as
// the mini player's: a submenu of speedSteps, and of the speed set when
// it's none of them, the one set checked; or, while the speed can't
// change, why.
func (s *shell) speedItems(m *ui.Menu) {
	if why := s.speedLocked(); why != "" {
		m.Item("Speed: " + why).Disabled(true)
		return
	}
	speed, podcasts := s.playingSpeed()
	set := *speed
	m.Submenu(speedTitle(podcasts), func(m *ui.Menu) {
		for _, v := range menuSpeeds(set) {
			if m.Item(speedLabel(v)).Radio(v == set).Chosen() {
				s.setSpeed(v)
			}
		}
	})
}

// menuSpeeds are speedSteps, and v among them, in order, when it's none.
func menuSpeeds(v float64) []float64 {
	i, found := slices.BinarySearch(speedSteps, v)
	if found {
		return speedSteps
	}
	return slices.Insert(slices.Clone(speedSteps), i, v)
}
