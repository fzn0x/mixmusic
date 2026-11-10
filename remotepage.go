package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/remote"
)

// remotePage is the page of the computer id, with MixMusic's remote control
// on: it pairs with it, by the code it shows, then shows what plays there,
// with play, pause, next, previous, seeking, the volume and the tracks
// after it.
func (s *shell) remotePage(c *ui.Context, id string) {
	k := &s.lk
	rc := s.a.Remote()
	v := rc.View(id)
	name := v.Computer.Name
	s.pageScroll(c, func() {
		s.pageHeader(c, name, nil, nil)
		ui.Column(c).MaxWidth(720).Gap(16).Children(func() {
			say := func(text string) {
				ui.Text(c, text).FontSize(k.bodySize()).MaxWidth(640)
			}
			if v.Problem != "" {
				ui.Text(c, v.Problem).FontSize(k.bodySize()).TextColor(k.danger).MaxWidth(640)
			}
			switch v.Step {
			case app.RemoteAway:
				say(name + " isn't on the network now. It shows here while MixMusic runs there, with Settings › Desktop › " +
					"Remote control on, on the same network as this computer.")
			case app.RemoteUnpaired:
				say("Pair with " + name + " to see what plays there and control it from here. " +
					name + " shows a code of 6 digits, which you type here.")
				if s.button(c, "remote-pair", "pair", true).Clicked() {
					rc.Pair(id)
				}
			case app.RemotePairing:
				say("Asking " + name + " to pair…")
				s.remoteCancel(c, id)
			case app.RemoteCodeAsked:
				s.remoteCodeEntry(c, id, name)
			case app.RemoteChecking:
				say("Checking the code with " + name + "…")
			case app.RemoteConnecting:
				say("Connecting to " + name + "…")
			case app.RemoteConnected:
				if !v.HasState {
					say("Connecting to " + name + "…")
					return
				}
				s.remotePlayer(c, id, v)
			}
		})
	})
}

// remoteCancel is the button that ends a pairing under way.
func (s *shell) remoteCancel(c *ui.Context, id string) {
	if s.button(c, "remote-cancel", "cancel", false).Clicked() {
		s.a.Remote().CancelPairing(id)
		s.remote.code = ""
	}
}

// remoteCodeEntry asks for the code the computer named name shows.
func (s *shell) remoteCodeEntry(c *ui.Context, id, name string) {
	k := &s.lk
	ui.Text(c, "Type the code "+name+" shows.").FontSize(k.bodySize())
	v := &s.remote
	var pair bool
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		in := ui.TextInput(c.Key("remote-code"), &v.code).Width(160).Label("Code").Placeholder("000 000").AutoFocus()
		pair = in.Submitted()
		_, ok := remote.CodeValue(v.code)
		if s.button(c, "remote-code-pair", "pair", true).Disabled(!ok).Clicked() {
			pair = true
		}
		s.remoteCancel(c, id)
		if pair && ok {
			s.a.Remote().EnterCode(id, v.code)
			v.code = ""
		}
	})
}

// remotePlayer shows what plays on the computer id, as v's state tells,
// with its controls: the cover, the title and the artists, where the track
// is, which moves on as it plays, the transport, the volume, what plays
// here, to move there (remotehandover.go), and the tracks after it
// (remotequeue.go).
func (s *shell) remotePlayer(c *ui.Context, id string, v app.RemoteView) {
	k := &s.lk
	st := v.State
	if st.Track == nil {
		ui.Text(c, "Nothing plays on "+v.Computer.Name+".").FontSize(k.bodySize())
	} else {
		t := *st.Track
		ui.Row(c).Gap(20).AlignItems(ui.Center).Children(func() {
			var art []media.Image
			if t.Artwork != "" {
				art = []media.Image{{URL: t.Artwork}}
			}
			s.cover(c, "remote-cover", art, 120, false)
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				ui.Text(c, t.Title).FontSize(k.subtitleSize()).FontWeight(k.leadWeight())
				line := strings.Join(t.Artists, ", ")
				if t.Album != "" {
					line += " · " + t.Album
				}
				ui.Text(c, line).FontSize(k.smallSize()).TextColor(k.muted)
			})
		})
	}
	s.remoteProgress(c, id, v)
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		has := st.Track != nil
		if s.roundButton(c, "remote-previous", "previous", "Previous", false, !has).Clicked() {
			s.remoteDo(id, remote.Command{Op: remote.OpPrevious})
		}
		name, label, op := "play", "Play", remote.OpPlay
		if st.Playing {
			name, label, op = "pause", "Pause", remote.OpPause
		}
		if s.roundButton(c, "remote-toggle", name, label, true, !has || st.Loading).Clicked() {
			s.remoteDo(id, remote.Command{Op: op})
		}
		if s.roundButton(c, "remote-next", "next", "Next", false, !has).Clicked() {
			s.remoteDo(id, remote.Command{Op: remote.OpNext})
		}
		ui.Box(c).Width(24)
		s.remoteVolume(c, id, st)
	})
	s.remoteHandOver(c, id, v)
	s.remoteQueue(c, id, v)
	s.remoteOlder(c, v)
}

// remoteHold is how long the page shows a seek or a volume the user set,
// before the state of the other computer, which tells it a moment after.
const remoteHold = 1500 * time.Millisecond

// remotePosition is where the track that plays there is now, as the state
// that came at at tells, moving on while it plays.
func remotePosition(st remote.State, at, now time.Time) time.Duration {
	pos := time.Duration(st.PositionMs) * time.Millisecond
	if st.Playing && !st.Loading && !at.IsZero() && now.After(at) {
		pos += now.Sub(at)
	}
	if d := time.Duration(st.DurationMs) * time.Millisecond; d > 0 && pos > d {
		pos = d
	}
	return pos
}

// remoteProgress shows where the track that plays there is, and seeks once
// the user lets go; it moves on each second while the track plays.
func (s *shell) remoteProgress(c *ui.Context, id string, v app.RemoteView) {
	k := &s.lk
	st := v.State
	r := &s.remote
	now := c.Now()
	dur := time.Duration(st.DurationMs) * time.Millisecond
	// A seek shows where it went until the state of it comes.
	if !r.seeking && now.Sub(r.seekAt) > remoteHold {
		r.seekTo = remotePosition(st, v.At, now).Seconds()
	}
	if st.Playing && !r.seeking {
		// The next whole second of the track.
		c.After(time.Second - time.Duration(r.seekTo*float64(time.Second))%time.Second)
	}
	ui.Row(c).Gap(8).FillWidth().MaxWidth(640).Children(func() {
		ui.Text(c, clock(time.Duration(r.seekTo*float64(time.Second)))).FontSize(k.captionSize()).TextColor(k.muted).
			FontFeatures("tnum").Width(44).TextAlign(ui.End)
		label := "Seek"
		if st.Live {
			label = "Live: no seeking"
		}
		seek := ui.Slider(c.Key("remote-seek"), &r.seekTo, 0, max(dur.Seconds(), 1)).Grow(1).MinWidth(0).KeepFocus().
			Label(label).Disabled(st.Track == nil || st.Live || dur <= 0)
		_, _, held := seek.Dragged()
		if st.Track != nil && !held && (r.seeking || seek.Changed()) {
			s.remoteDo(id, remote.Command{Op: remote.OpSeek, PositionMs: int64(r.seekTo * 1000)})
			r.seekAt = now
		}
		r.seeking = held
		ui.Text(c, clock(dur)).FontSize(k.captionSize()).TextColor(k.muted).FontFeatures("tnum").Width(44)
	})
}

// remoteVolume shows the volume there, and sets it.
func (s *shell) remoteVolume(c *ui.Context, id string, st remote.State) {
	r := &s.remote
	ui.Row(c).Gap(4).Width(180).AlignItems(ui.Center).Children(func() {
		name := "volume"
		if st.Muted || st.Volume == 0 {
			name = "volumeMute"
		}
		ui.Icon(c, icon(name)).FontSize(18).TextColor(s.lk.muted)
		// The volume the user sets shows until the state of it comes.
		if now := c.Now(); now.Sub(r.volumeAt) > remoteHold {
			r.volume = st.Volume
			if st.Muted {
				r.volume = 0
			}
		}
		if ui.Slider(c.Key("remote-volume"), &r.volume, 0, 1).Grow(1).MinWidth(0).KeepFocus().Label("Volume").Changed() {
			s.remoteDo(id, remote.Command{Op: remote.OpVolume, Volume: r.volume})
			r.volumeAt = c.Now()
			c.After(remoteHold)
		}
	})
}
