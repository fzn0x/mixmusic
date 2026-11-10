package main

import (
	"context"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/player"
)

// roomCast is the width the cast button takes in the player bar, beside
// its gap.
const roomCast = 40

// castShown reports whether the player bar shows the cast button: while a
// cast device, or a computer with MixMusic's remote control on, is on the
// network, or the player plays on a cast device.
func (s *shell) castShown() bool {
	if len(s.svc.Player.CastDevices()) > 0 || len(s.a.Remote().Computers()) > 0 {
		return true
	}
	_, on := s.svc.Player.CastingTo()
	return on
}

// castButton chooses where the player's tracks play: on this computer, or
// on a cast device on the network, listed by its name, which tells its
// room, and its model. Choosing one moves what plays there, from where it
// was. Spotify tracks that play on Spotify Connect, or in MixMusic as a
// Spotify device, stay where they play. Below the devices, the computers
// with MixMusic's remote control on open their pages (remote.go).
func (s *shell) castButton(c *ui.Context) {
	if !s.castShown() {
		return
	}
	devices := s.svc.Player.CastDevices()
	casting, on := s.svc.Player.CastingTo()
	label := "Cast to a device"
	if on {
		label = "Playing on " + casting.Name
	}
	b := s.iconButton(c, "cast", "cast", label, on, 18)
	b.Menu(func(m *ui.Menu) {
		starting, connecting := s.svc.Player.CastStarting()
		if m.Item("This computer").Radio(!on).Disabled(connecting).Chosen() && on {
			go s.svc.Player.StopCasting()
		}
		for _, d := range devices {
			label := d.Name
			if d.Model != "" {
				label += " · " + d.Model
			}
			if connecting && starting.ID == d.ID {
				label += " · connecting…"
			}
			if m.Item(label).Radio(on && casting.ID == d.ID).Disabled(connecting).Chosen() {
				s.castTo(d)
			}
		}
		s.remoteItems(m)
	})
	if b.Clicked() {
		// The menu opens on what was found, and finds again beside it.
		s.svc.Player.FindCastDevices()
		s.a.Remote().Find()
	}
}

// castTo moves what plays to the cast device d, and tells why it can't.
func (s *shell) castTo(d app.CastDevice) {
	if cur, on := s.svc.Player.CastingTo(); on && cur.ID == d.ID {
		return
	}
	go func() {
		if err := s.svc.Player.Cast(context.Background(), d.ID); err != nil {
			s.update(func() { s.notes = append(s.notes, noticeOf(err)) })
		}
	}()
}

// followCast redraws the window as the cast devices found, or the cast,
// change, and tells why a cast stopped by itself.
func (s *shell) followCast() func() {
	return s.a.OnCast(func(why string) {
		s.update(func() {
			if why != "" {
				s.notes = append(s.notes, player.Notice{Message: why, Error: true})
			}
		})
	})
}
