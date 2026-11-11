package main

// The queue of a computer controlled, on its page (remote control, #353):
// the tracks after the one that plays there, which play, move and go as
// those of this computer's queue do (panel.go), by the entries its states
// name.

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/remote"
)

// remoteDrag is an entry of another computer's queue dragged, by its name
// there.
type remoteDrag struct{ entry string }

// remoteQueue lists the tracks after the one that plays on the computer
// id: a click on one's cover, or a double click on it, plays it there; its
// cross, or its menu, takes it out; dragging it onto another moves it to
// that one's place; and Clear empties the queue there. Those of a
// computer whose MixMusic is older only show.
func (s *shell) remoteQueue(c *ui.Context, id string, v app.RemoteView) {
	k := &s.lk
	name := v.Computer.Name
	if len(v.State.Next) == 0 {
		return
	}
	s.sectionHead(c, "Next on "+name, func() {
		if !v.CanQueue {
			return
		}
		clear := s.buttonBase(c.Key("remote-clear")).Label("Clear the queue on " + name).Cursor(ui.CursorPointer)
		if clear.Clicked() {
			s.remoteDo(id, remote.Command{Op: remote.OpClear})
		}
		clear.Children(func() { ui.Text(c, k.words("Clear")).FontSize(k.smallSize()).TextColor(k.accent) })
	})
	ui.Column(c).Gap(2).MaxWidth(640).Children(func() {
		for _, t := range v.State.Next {
			if v.CanQueue && t.Entry != "" {
				s.remoteQueueRow(c, id, name, t)
				continue
			}
			line := t.Title
			if len(t.Artists) > 0 {
				line += " - " + strings.Join(t.Artists, ", ")
			}
			ui.Text(c, line).FontSize(k.smallSize()).SingleLine().MinWidth(0)
		}
	})
}

// remoteQueueRow is the track t of the queue of the computer id, named
// name, which the user plays, moves and takes out as an entry of this
// computer's queue.
func (s *shell) remoteQueueRow(c *ui.Context, id, name string, t remote.Track) {
	k := &s.lk
	play := func() { s.remoteDo(id, remote.Command{Op: remote.OpPlayEntry, Entry: t.Entry}) }
	remove := func() { s.remoteDo(id, remote.Command{Op: remote.OpRemove, Entry: t.Entry}) }
	row := ui.Row(c.Key("rq-"+t.Entry)).Height(queueRowHeight).Padding(0, 8).Gap(10).Radius(k.radiusSm).Cursor(ui.CursorDefault)
	if row.Hovered() {
		row.Background(k.hover)
	}
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item("Play").Chosen() {
			play()
		}
		if m.Item("Remove from the queue").Chosen() {
			remove()
		}
	})
	if row.DoubleClicked() {
		play()
	}
	row.Drag(remoteDrag{t.Entry})
	if row.Dragging() {
		row.Opacity(0.4)
	}
	if _, over := ui.DragOver[remoteDrag](row); over {
		row.InsetShadow(0, 2, 0, 0, k.accent)
	}
	if d, ok := ui.Drop[remoteDrag](row); ok && d.entry != t.Entry {
		s.remoteDo(id, remote.Command{Op: remote.OpMove, Entry: d.entry, To: t.Entry})
	}
	hovered := row.Hovered()
	row.Children(func() {
		art := s.buttonBase(c.Key("rq-art-"+t.Entry)).Size(40, 40).Shrink(0).Label("Play " + t.Title + " on " + name).
			Cursor(ui.CursorPointer)
		if art.Clicked() {
			play()
		}
		art.Children(func() {
			var images []media.Image
			if t.Artwork != "" {
				images = []media.Image{{URL: t.Artwork}}
			}
			s.cover(c, "rq-cover-"+t.Entry, images, 40, false)
			if hovered {
				ui.Box(c).Absolute().Top(0).Left(0).Size(40, 40).Background(k.scrim).Radius(k.scrimRadius()).Center().Children(func() {
					ui.Icon(c, icon("play")).FontSize(14).TextColor(k.text)
				})
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, t.Title).FontSize(k.bodySize()).SingleLine()
			if len(t.Artists) > 0 {
				ui.Text(c, strings.Join(t.Artists, ", ")).FontSize(k.smallSize()).TextColor(k.muted).SingleLine().MinWidth(0)
			}
		})
		if hovered {
			if s.iconButton(c, "rq-remove-"+t.Entry, "close", "Remove "+t.Title+" from the queue on "+name, false, 14).Clicked() {
				remove()
			}
		}
	})
}

// remoteOlder tells what the computer of v can't do from here, as its
// MixMusic is older.
func (s *shell) remoteOlder(c *ui.Context, v app.RemoteView) {
	if v.CanQueue && v.CanHandOver {
		return
	}
	ui.Text(c, "MixMusic on "+v.Computer.Name+" is older: update it there to change its queue from here, and to move what plays here to it.").
		FontSize(s.lk.smallSize()).TextColor(s.lk.muted).MaxWidth(640)
}
