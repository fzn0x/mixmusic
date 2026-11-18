package main

// Adding to the queue in one step (#618): the rows of song lists and of
// episodes show a queue button in the length's slot while the pointer is
// over them, Shift+Enter and Alt+Enter queue a row or play it next while
// the keyboard's focus is in it, and song tiles have Add to queue and Play
// next beside their play circle. Every Add to queue of the window plays
// what it adds when no track is current (Player.AddToQueue).

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// addToQueue is the window's Add to queue, of a track, the tracks selected
// or a collection: tracks go at the end of the queue, before the radio's,
// or play when no track is current. The toast says queued, or playing when
// they play.
func (s *shell) addToQueue(tracks []media.Track, queued, playing string) {
	if len(tracks) == 0 {
		return
	}
	msg := queued
	if s.playsAdded(tracks) {
		msg = playing
	}
	s.p.AddToQueue(tracks)
	s.notes = append(s.notes, notice(msg))
}

// playsAdded reports whether Add to queue plays tracks, as the player
// decides: no track is current, nor one loading, and the settings hide not
// all of them.
func (s *shell) playsAdded(tracks []media.Track) bool {
	_, ok := s.st.Current()
	return !ok && !s.st.Loading && slices.ContainsFunc(tracks, func(t media.Track) bool { return !s.hides(t) })
}

// queueTrack is a track's Add to queue: its menu's item, its row's button
// and Shift+Enter, and its tile's button.
func (s *shell) queueTrack(t media.Track) {
	s.addToQueue([]media.Track{t}, locales.PanelAddedToQueue(t.Title), locales.QueueButtonPlaying(t.Title))
}

// playNextTrack is a track's Play next: its menu's item, Alt+Enter on its
// row and its tile's button.
func (s *shell) playNextTrack(t media.Track) {
	s.p.PlayNext([]media.Track{t})
	s.notes = append(s.notes, notice(locales.PanelPlaysNext(t.Title)))
}

// queueKeys are the keys of the row of t while the keyboard's focus is in
// it: Shift+Enter adds t to the queue, and Alt+Enter (Option+Enter on
// macOS) plays it next. They are the row's own, which go before the
// window's shortcuts.
func (s *shell) queueKeys(row ui.Element, t media.Track) {
	row.OnShortcut(ui.Shift, ui.KeyEnter, func() { s.queueTrack(t) })
	row.OnShortcut(ui.Alt, ui.KeyEnter, func() { s.playNextTrack(t) })
}

// queueButtonShows reports whether a row of the list of key, hovered or
// not, shows its queue button in place of its length: while the pointer is
// over it, and the list has no tracks selected, whose bar has Add to queue.
func (s *shell) queueButtonShows(key string, hovered bool) bool {
	return hovered && !s.picking(key)
}

// rowQueueButton is the button of a row, rowHeight tall, that adds t to
// the queue, in the length's slot, slot wide: a 32px icon, as the other
// icon buttons are, whose pointer target is 48px square, or as tall as a
// shorter row. Built only while it shows, it adds no Tab stop.
func (s *shell) rowQueueButton(c *ui.Context, key string, t media.Track, slot, rowHeight float32) {
	k := &s.lk
	label := locales.QueueButtonAdd(t.Title)
	ui.Row(c).Width(slot).Shrink(0).Justify(ui.End).Children(func() {
		b := s.buttonBase(c.Key(key)).Size(48, min(48, rowHeight)).Shrink(0).Center().Label(label).Tooltip(label).
			Cursor(ui.CursorPointer)
		if b.Clicked() {
			s.queueTrack(t)
		}
		s.pressMotion(b, 0.9)
		ink := k.muted
		if b.Hovered() {
			ink = k.text
		}
		b.Children(func() {
			face := ui.Box(c).Size(32, 32).Radius(k.round(32)).Center()
			switch {
			case b.Pressed():
				face.Background(k.press)
			case b.Hovered():
				face.Background(k.hover)
			}
			face.Children(func() { ui.Icon(c, lineIcon(c, "queueAdd")).FontSize(18).TextColor(ink) })
		})
	})
}

// songTileButtons are the buttons that rise beside the play circle of the
// tile of the song s.songTile, keyed key, as the circle rises (rise), or
// while the keyboard's focus in the tile shows its ring: Add to queue and
// Play next. A click on either neither opens the tile nor plays it.
func (s *shell) songTileButtons(c *ui.Context, tile ui.Element, key string, rise float32) {
	t := s.songTile
	if t == nil {
		return
	}
	// The focus Tab gives the tile, then its buttons, keeps them; the one a
	// click leaves shows no ring, and they go with the pointer.
	ring := ui.Local(tile, "queue-ring", func() bool { return false })
	if tile.FocusVisible() || *ring && tile.FocusWithin() {
		rise = 1
	}
	*ring = tile.FocusVisible()
	if rise <= 0.01 {
		return
	}
	// Beside the circle, 42px wide and 8px from the artwork's end and
	// foot, and centred on it.
	row := ui.Row(c).Absolute().End(8 + 42 + 6).Bottom(8 + 5).Gap(6)
	if rise < 1 {
		row.Translate(0, 12*(1-rise)).Opacity(min(1, rise))
	}
	row.Children(func() {
		add := s.tileButton(c, key+"-queue", "queueAdd", locales.QueueButtonAdd(t.Title))
		next := s.tileButton(c, key+"-next", "queueNext", locales.QueueButtonPlayNext(t.Title))
		if add.Clicked() {
			s.queueTrack(*t)
		}
		if next.Clicked() {
			s.playNextTrack(*t)
		}
		*ring = *ring || add.FocusVisible() || next.FocusVisible()
	})
}

// tileButton is a 32px round button over a tile's artwork, of the icon
// name: ink on the scrim, which turns raised while hovered.
func (s *shell) tileButton(c *ui.Context, key, name, label string) ui.Element {
	k := &s.lk
	b := s.buttonBase(c.Key(key)).Size(32, 32).Radius(16).Center().Label(label).Tooltip(label).Cursor(ui.CursorPointer)
	if b.Hovered() || b.Pressed() {
		b.Background(k.raised)
	} else {
		b.Background(k.scrim)
	}
	s.pressMotion(b, 0.9)
	b.Children(func() { ui.Icon(c, lineIcon(c, name)).FontSize(16).TextColor(k.text) })
	return b
}
