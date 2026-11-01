package native

import (
	"context"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// playerBar shows what plays, the transport and where it is in the track,
// and the volume; the next track waits in view under the current one.
func (s *shell) playerBar(c *ui.Context) {
	k := &s.lk
	ui.Row(c).Height(k.player).Shrink(0).Padding(0, 24, 0, 16).Gap(24).Background(k.surface).
		BorderWidth(1, 0, 0, 0).BorderColor(k.border).Children(func() {
		ui.Row(c).Grow(1).Basis(0).MinWidth(220).Gap(12).Children(func() { s.nowPlayingInfo(c) })
		ui.Column(c).Grow(1.6).Basis(0).MinWidth(320).Gap(6).AlignItems(ui.Center).Children(func() {
			s.transport(c)
			s.progress(c)
		})
		ui.Row(c).Grow(1).Basis(0).MinWidth(220).Gap(4).Justify(ui.End).Children(func() { s.playerSide(c) })
	})
}

func (s *shell) nowPlayingInfo(c *ui.Context) {
	k := &s.lk
	cur, ok := s.st.Current()
	if !ok {
		ui.Text(c, "pick something to play").FontSize(k.smallSize()).TextColor(k.muted)
		return
	}
	t := cur.Track
	art := s.buttonBase(c.Key("player-art")).Size(62, 62).Shrink(0).Label("Open now playing").Cursor(ui.CursorPointer)
	art.ContextMenu(func(m *ui.Menu) { s.trackMenu(m, t) })
	if art.Clicked() {
		s.navigate("/now-playing")
	}
	art.Children(func() { s.cover(c, "player-cover", t.Artwork, 62, false) })
	ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
		if t.Album != nil && t.Album.ID != "" {
			link := s.buttonBase(c.Key("player-title")).Justify(ui.Start).MinWidth(0).Label(t.Title).Cursor(ui.CursorPointer)
			if link.Clicked() {
				s.navigate(albumPath(t.Album.ID))
			}
			link.Children(func() { ui.Text(c, t.Title).FontSize(k.bodySize()).SingleLine().MinWidth(0) })
		} else {
			ui.Text(c, t.Title).FontSize(k.bodySize()).SingleLine()
		}
		ui.Row(c).Gap(6).MinWidth(0).Children(func() {
			s.sourceMark(c, t.Source, false)
			line := names(t.Artists)
			if src := s.st.Source; src != nil {
				switch {
				case src.Mode == player.ModeStream && src.Track.ID != t.ID:
					line += " · via YouTube Music"
				case src.Mode == player.ModeConnect:
					line += " · on Spotify"
				}
			}
			if s.st.Waiting {
				line += " · waiting for the network"
			}
			ui.Text(c, line).FontSize(k.smallSize()).TextColor(k.muted).SingleLine().MinWidth(0)
		})
		if next, ok := s.st.UpNext(); ok && next.Track.ID != t.ID && s.panel != "queue" {
			if w, _ := c.Size(); w >= 1100 {
				ui.Text(c, "next · "+next.Track.Title+" — "+names(next.Track.Artists)).FontSize(k.captionSize()).
					TextColor(k.muted).SingleLine()
			}
		}
	})
	s.likeButton(c, "player-like", t, 18)
}

// likeButton likes a track, or no longer.
func (s *shell) likeButton(c *ui.Context, key string, t media.Track, size float32) {
	on := s.a.IsLiked(t.ID)
	label := "Like"
	if on {
		label = "Remove from liked songs"
	}
	k := &s.lk
	b := s.buttonBase(c.Key(key)).Size(32, 32).Radius(k.radiusSm).Label(label).Tooltip(label).Cursor(ui.CursorPointer)
	ink := k.muted
	switch {
	case on:
		ink = k.accent
	case b.Hovered():
		ink = k.text
	}
	if b.Hovered() {
		b.Background(k.hover)
	}
	if b.Clicked() {
		s.setLiked(t, !on)
	}
	b.Children(func() {
		svg := icons["heart"]
		if on {
			svg = solidIcons["heart"]
		}
		ui.Icon(c, svg).FontSize(size).TextColor(ink)
	})
}

// setLiked likes a track or no longer, beside the main thread: liking a
// Spotify track asks Spotify.
func (s *shell) setLiked(t media.Track, on bool) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := s.svc.Collection.SetLiked(ctx, t, on)
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
			}
		})
	}()
}

func (s *shell) transport(c *ui.Context) {
	k := &s.lk
	_, hasTrack := s.st.Current()
	ui.Row(c).Gap(12).Children(func() {
		if s.iconButton(c, "shuffle", "shuffle", "Shuffle", s.st.Shuffle, 18).Clicked() {
			s.p.ToggleShuffle()
		}
		if s.roundButton(c, "previous", "previous", "Previous", false, false).Clicked() {
			s.p.Previous()
		}
		name, label := toggleOf(s.st)
		main := s.roundButton(c, "toggle", name, label, true, s.st.Loading || !hasTrack && len(s.st.Queue) == 0)
		if pulses(s.st) {
			main.Opacity(0.55 + 0.45*main.Loop("busy", 900*time.Millisecond, ui.EaseInOut))
		}
		if main.Clicked() {
			s.p.Toggle()
		}
		if s.roundButton(c, "next", "next", "Next", false, false).Clicked() {
			s.p.Next()
		}
		rep, label := "repeat", "Repeat: off"
		switch s.st.Repeat {
		case player.RepeatAll:
			label = "Repeat: all"
		case player.RepeatOne:
			rep, label = "repeatOne", "Repeat: one"
		}
		if s.iconButton(c, "repeat", rep, label, s.st.Repeat != player.RepeatOff, 18).Clicked() {
			s.p.CycleRepeat()
		}
	})
	_ = k
}

// toggleOf is the icon and the label of the play button, which tells
// what it does: a track that loads can't be paused, and one that waits for
// the network plays once it's back unless paused.
func toggleOf(st player.State) (name, label string) {
	switch {
	case st.Waiting && st.WillPlay:
		return "pause", "Pause, waiting for the network"
	case st.Waiting:
		return "play", "Play once the network is back"
	case st.Loading:
		return "play", "Loading"
	case st.Playing:
		return "pause", "Pause"
	}
	return "play", "Play"
}

// pulses reports whether the play button pulses: while the track buffers,
// or waits for the network to play.
func pulses(st player.State) bool { return st.Buffering || st.Waiting && st.WillPlay }

// progress shows where the track is, and seeks once the user lets go.
func (s *shell) progress(c *ui.Context) {
	k := &s.lk
	cur, ok := s.st.Current()
	dur := s.st.Duration
	if dur <= 0 && ok {
		dur = ms(cur.Track.DurationMs)
	}
	if !s.seeking {
		s.seekTo = s.st.Position.Seconds()
	}
	ui.Row(c).Gap(8).FillWidth().MaxWidth(640).Children(func() {
		ui.Text(c, clock(time.Duration(s.seekTo*float64(time.Second)))).FontSize(k.captionSize()).TextColor(k.muted).
			FontFeatures("tnum").Width(44).TextAlign(ui.End)
		seek := ui.Slider(c.Key("seek"), &s.seekTo, 0, max(dur.Seconds(), 1)).Grow(1).MinWidth(0).KeepFocus().
			Label("Seek").Disabled(!ok)
		_, _, held := seek.Dragged()
		if ok && !held && (s.seeking || seek.Changed()) {
			s.p.Seek(time.Duration(s.seekTo * float64(time.Second)))
		}
		s.seeking = held
		ui.Text(c, clock(dur)).FontSize(k.captionSize()).TextColor(k.muted).FontFeatures("tnum").Width(44)
	})
}

func (s *shell) playerSide(c *ui.Context) {
	if s.iconButton(c, "lyrics", "lyrics", "Lyrics", s.panel == "lyrics", 18).Clicked() {
		s.togglePanel("lyrics")
	}
	if s.iconButton(c, "queue", "queue", "Queue", s.panel == "queue", 18).Clicked() {
		s.togglePanel("queue")
	}
	if s.set.SpotifyPlayback == settings.SpotifyPlaybackConnect {
		s.devicesButton(c)
	}
	s.volumeControl(c)
	if s.iconButton(c, "mini", "mini", "Open the mini player", false, 18).Clicked() {
		s.showMini()
	}
}

func (s *shell) togglePanel(p string) {
	if s.panel == p {
		p = ""
	}
	s.showPanel(p)
}

// showPanel shows the side panel p, "" for none, as the next start does.
func (s *shell) showPanel(p string) {
	s.panel, s.set.Panel = p, p
	if p == "" {
		s.set.Panel = "none"
	}
	s.save()
}

// volumeControl mutes, and sets the volume.
func (s *shell) volumeControl(c *ui.Context) {
	v := s.st.Volume
	if s.st.Muted {
		v = 0
	}
	ui.Row(c).Gap(4).Width(140).Children(func() {
		name, label := "volume", "Mute"
		if s.st.Muted {
			label = "Unmute"
		}
		if v == 0 {
			name = "volumeMute"
		}
		if s.iconButton(c, "mute", name, label, false, 18).Clicked() {
			s.p.ToggleMute()
		}
		s.volume = v
		if ui.Slider(c.Key("volume"), &s.volume, 0, 1).Grow(1).MinWidth(0).KeepFocus().Label("Volume").Changed() {
			s.p.SetVolume(s.volume)
		}
	})
}

// devicesButton chooses the Spotify Connect device that plays Spotify
// tracks.
func (s *shell) devicesButton(c *ui.Context) {
	b := s.iconButton(c, "devices", "devices", "Spotify Connect devices", false, 18)
	b.Menu(func(m *ui.Menu) {
		chosen := s.set.SpotifyDevice
		devices := fetch(s, "devices", s.svc.Player.Devices)
		if m.Item("The active device").Checked(chosen == "").Chosen() {
			s.setDevice("")
		}
		switch {
		case !devices.done:
			m.Item("Looking for devices…").Disabled(true)
		case devices.err != nil:
			m.Item(devices.err.Error()).Disabled(true)
		case len(devices.value) == 0:
			m.Item("Open Spotify on a device to see it here").Disabled(true)
		}
		for _, d := range devices.value {
			label := d.Name
			if d.Active {
				label += " · active"
			}
			if m.Item(label).Checked(chosen == d.ID).Disabled(d.Restricted).Chosen() {
				s.setDevice(d.ID)
			}
		}
	})
	if b.Clicked() {
		s.forget("devices")
	}
}

func (s *shell) setDevice(id string) {
	st := s.svc.Preferences.Get()
	st.SpotifyDevice = id
	if _, err := s.svc.Preferences.Set(st); err != nil {
		s.notes = append(s.notes, noticeOf(err))
	}
}
