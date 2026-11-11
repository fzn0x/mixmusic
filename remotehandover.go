package main

// Handing what plays here over to a computer controlled, from its page
// (remote control, #353): its queue, the track that plays and where it
// is move there, and this computer pauses (internal/app's
// remotehandover.go).

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
)

// remoteHandOver shows what plays on this computer, on the page of the
// computer id, with a button that moves it there.
func (s *shell) remoteHandOver(c *ui.Context, id string, v app.RemoteView) {
	cur, ok := s.st.Current()
	if !ok || !v.CanHandOver {
		return
	}
	k := &s.lk
	name := v.Computer.Name
	t := cur.Track
	ui.Row(c).Gap(12).AlignItems(ui.Center).MaxWidth(640).Padding(12, 16).Radius(k.radiusSm).Background(k.surface).Children(func() {
		s.cover(c, "remote-here-cover", t.Artwork, 40, false)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, "On this computer").FontSize(k.captionSize()).TextColor(k.muted)
			line := t.Title
			if a := names(t.Artists); a != "" {
				line += " · " + a
			}
			ui.Text(c, line).FontSize(k.smallSize()).SingleLine().MinWidth(0)
		})
		label := "move to " + name
		if s.remote.handing {
			label = "moving to " + name + "…"
		}
		if s.button(c, "remote-hand-over", label, false).Disabled(s.remote.handing).Clicked() && !s.remote.handing {
			s.handOver(id, name)
		}
	})
}

// handOver moves what plays here to the computer id, named name, beside
// the main thread, and tells how it went.
func (s *shell) handOver(id, name string) {
	s.remote.handing = true
	go func() {
		got, err := s.a.Remote().HandOver(id)
		s.update(func() {
			s.remote.handing = false
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.notes = append(s.notes, notice(handedOverNote(name, got)))
		})
	}()
}

// handedOverNote tells how moving what plays here to the computer named
// name went.
func handedOverNote(name string, got app.HandedOver) string {
	switch {
	case got.Kept == 0:
		return name + " has none of these tracks, files of this computer: they play on here"
	case got.Left == 0:
		return "Moved to " + name + ", and paused here"
	}
	tracks := "tracks"
	if got.Left == 1 {
		tracks = "track"
	}
	note := fmt.Sprintf("Moved to %s, and paused here, but for %d %s, files of this computer it doesn't have", name, got.Left, tracks)
	if got.CurrentLeft {
		note += ": it plays from the next one"
	}
	return note
}
