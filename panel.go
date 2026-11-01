package main

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/lyrics"
)

// sidePanel is the panel at the right of the main pane: the queue, or the
// lyrics.
func (s *shell) sidePanel(c *ui.Context) {
	k := &s.lk
	ui.Column(c).Width(s.panelWidth(c)).Shrink(0).Background(k.surface).BorderWidth(0, 0, 0, 1).BorderColor(k.border).Children(func() {
		if s.panel == "queue" {
			s.queuePanel(c)
		} else {
			s.lyricsPanel(c)
		}
	})
}

// panelHead titles the panel, with its actions and a close button.
func (s *shell) panelHead(c *ui.Context, title string, actions func()) {
	k := &s.lk
	ui.Row(c).Padding(16, 16, 12, 24).Gap(8).Shrink(0).Children(func() {
		ui.Text(c, title).FontSize(k.pivotSize()).FontWeight(300).Font(k.display).LetterSpacing(-0.02 * k.pivotSize()).
			SingleLine().Grow(1).MinWidth(0)
		if actions != nil {
			actions()
		}
		if s.iconButton(c, "close-panel", "close", "Close "+title, false, 16).Clicked() {
			s.showPanel("")
		}
	})
}

// queuePanel lists the track playing, and what plays next.
func (s *shell) queuePanel(c *ui.Context) {
	k := &s.lk
	q, at := s.st.Queue, s.st.Index
	upcoming := 0
	if at+1 < len(q) {
		upcoming = len(q) - at - 1
	}
	s.panelHead(c, "up next", func() {
		if upcoming > 0 {
			clear := s.buttonBase(c.Key("clear-queue")).Label("Clear the queue").Cursor(ui.CursorPointer)
			if clear.Clicked() {
				s.p.ClearUpcoming()
			}
			clear.Children(func() { ui.Text(c, "clear").FontSize(k.smallSize()).TextColor(k.accent) })
		}
	})
	ui.Scroll(c).Grow(1).MinHeight(0).Padding(0, 12, 24).Children(func() {
		if cur, ok := s.st.Current(); ok {
			ui.Text(c, "now playing").FontSize(k.smallSize()).TextColor(k.accent).Padding(0, 8, 6)
			s.queueRow(c, at, cur, true)
		}
		if upcoming == 0 {
			text := "Nothing queued. When the queue runs out, a radio of what you played carries on."
			if at < 0 {
				text = "Play something and the queue shows here."
			}
			ui.Text(c, text).FontSize(k.bodySize()).TextColor(k.muted).Padding(16, 8)
			return
		}
		ui.Column(c).Margin(12, 0, 0).Children(func() {
			for i := at + 1; i < len(q); i++ {
				s.queueRow(c, i, q[i], false)
			}
		})
	})
}

// queueRow is an entry of the queue; the current one floods with the
// accent.
func (s *shell) queueRow(c *ui.Context, i int, it player.Item, current bool) {
	k := &s.lk
	t := it.Track
	row := ui.Row(c.Key("q-"+it.UID)).Height(56).Padding(0, 8).Gap(10).Radius(k.radiusSm).Cursor(ui.CursorDefault)
	ink, sub := k.text, k.muted
	switch {
	case current:
		row.Background(k.accent)
		ink, sub = k.accentText, k.accentText.Alpha(0.78)
	case row.Hovered():
		row.Background(k.hover)
	}
	row.ContextMenu(func(m *ui.Menu) { s.trackMenuAt(m, t, i) })
	if row.DoubleClicked() && !current {
		s.p.PlayAt(i)
	}
	if !current {
		s.reorderable(row, it.UID, i)
	}
	hovered := row.Hovered()
	row.Children(func() {
		art := s.buttonBase(c.Key("q-art-"+it.UID)).Size(40, 40).Shrink(0).Label("Play " + t.Title).Cursor(ui.CursorPointer)
		if art.Clicked() {
			if current {
				s.p.Toggle()
			} else {
				s.p.PlayAt(i)
			}
		}
		art.Children(func() {
			s.cover(c, "q-cover-"+it.UID, t.Artwork, 40, false)
			if hovered && !current {
				ui.Box(c).Absolute().Top(0).Left(0).Size(40, 40).Background(k.scrim).Center().Children(func() {
					ui.Icon(c, icon("play")).FontSize(14).TextColor(k.text)
				})
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, t.Title).FontSize(k.bodySize()).TextColor(ink).SingleLine()
			ui.Row(c).Gap(6).MinWidth(0).Children(func() {
				s.sourceMark(c, t.Source, current)
				line := names(t.Artists)
				if it.Radio {
					line += " · radio"
				}
				ui.Text(c, line).FontSize(k.smallSize()).TextColor(sub).SingleLine().MinWidth(0)
			})
		})
		if t.DurationMs > 0 {
			ui.Text(c, clock(ms(t.DurationMs))).FontSize(k.captionSize()).TextColor(sub).FontFeatures("tnum")
		}
		if !current && hovered {
			if s.iconButton(c, "q-remove-"+it.UID, "close", "Remove "+t.Title+" from the queue", false, 14).Clicked() {
				s.p.RemoveAt(i)
			}
		}
	})
}

// queueDrag is an entry of the queue dragged, by its UID: what plays may
// move it meanwhile.
type queueDrag struct{ uid string }

// reorderable lets the user drag the upcoming entry of uid, at i, onto
// another, which takes it in its place, as the web interface's queue does:
// a line of the accent over an entry shows where it would go.
func (s *shell) reorderable(row ui.Element, uid string, i int) {
	row.Drag(queueDrag{uid})
	if row.Dragging() {
		row.Opacity(0.4)
	}
	if _, over := ui.DragOver[queueDrag](row); over {
		row.InsetShadow(0, 2, 0, 0, s.lk.accent)
	}
	if d, ok := ui.Drop[queueDrag](row); ok {
		from := slices.IndexFunc(s.st.Queue, func(it player.Item) bool { return it.UID == d.uid })
		if from >= 0 && from != i {
			s.p.Move(from, i)
		}
	}
}

// lyricsPanel shows the lyrics of the track playing.
func (s *shell) lyricsPanel(c *ui.Context) {
	s.panelHead(c, "lyrics", nil)
	s.lyrics(c, false)
}

// lyricsState is the lyrics views' own: the track they show, and where
// the panel's and the now playing page's lyrics scrolled, which show at
// once. Whether synced lyrics show as plain text is a setting.
type lyricsState struct {
	track       string
	panel, page lyricsScroll
}

// lyricsScroll is where a lyrics view scrolled, the line it last scrolled
// to, and when the user last scrolled it.
type lyricsScroll struct {
	list     ui.ListState
	scrolled int
	userAt   time.Time
}

// lyricsPause is how long lyrics the user scrolled don't follow the song.
const lyricsPause = 4 * time.Second

// lyrics follows the track playing: synchronized lines light up as they
// are sung. large sets them at the size of the now playing page.
func (s *shell) lyrics(c *ui.Context, large bool) {
	k := &s.lk
	ls := &s.lyricsView
	view, where := &ls.panel, "panel"
	if large {
		view, where = &ls.page, "page"
	}
	cur, ok := s.st.Current()
	if !ok {
		s.emptyIn(c, "Play a track to see its lyrics.")
		return
	}
	t := cur.Track
	l := fetch(s, "lyrics:"+t.ID, func(ctx context.Context) (lyrics.Lyrics, error) { return s.svc.Catalog.Lyrics(ctx, t) })
	switch {
	case !l.done:
		s.emptyIn(c, "Looking for the lyrics…")
		return
	case l.err != nil:
		s.emptyIn(c, l.err.Error())
		// A track without lyrics stays so; a failure may pass.
		if !errors.Is(l.err, lyrics.ErrNotFound) {
			ui.Row(c).Padding(0, 24, 8).Children(func() {
				if s.button(c, "lyrics-retry-"+where, "try again", false).Clicked() {
					s.forget("lyrics:" + t.ID)
				}
			})
		}
		return
	case l.value.Instrumental:
		s.emptyIn(c, "This track is instrumental.")
		return
	}
	ly := l.value
	if ls.track != t.ID {
		ls.track, ls.panel.scrolled, ls.page.scrolled = t.ID, -1, -1
	}
	synced := ly.Synced && !s.set.LyricsPlain
	if ly.Synced {
		ui.Row(c).Padding(0, 24, 8).Shrink(0).Children(func() {
			mode := 0
			if s.set.LyricsPlain {
				mode = 1
			}
			if s.segmented(c, "lyrics-mode-"+where, "Show lyrics as", &mode, "synced", "plain text") {
				s.set.LyricsPlain = mode == 1
				s.save()
			}
		})
	}
	active := -1
	if synced {
		pos := s.st.Position + 250*time.Millisecond
		for i, line := range ly.Lines {
			if ms(line.TimeMs) <= pos {
				active = i
			} else {
				break
			}
		}
	}
	size := k.bodySize()
	if large {
		size = k.size(2)
	}
	n := len(ly.Lines) + 1
	list := ui.List(c.Key("lyrics-"+where+"-"+t.ID), &view.list, n, func(i int) {
		if i == len(ly.Lines) {
			ui.Text(c, "lyrics from "+ly.Source).FontSize(k.captionSize()).TextColor(k.muted).Padding(16, 24)
			return
		}
		line := ly.Lines[i]
		text := line.Text
		if text == "" {
			text = "♪"
		}
		ink := k.text
		switch {
		case synced && i == active:
			ink = k.accent
		case synced && i < active:
			ink = k.muted
		}
		p := ui.Text(c, text).FontSize(size).FontWeight(300).TextColor(ink).Padding(4, 24)
		if synced {
			p.Cursor(ui.CursorPointer)
			if p.Clicked() {
				s.p.Seek(ms(line.TimeMs))
			}
		}
	}).Grow(1).MinHeight(0)
	// Lyrics the user scrolls, to read back say, stop following the song
	// for a while.
	list.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind == ui.InputScroll || ev.Kind == ui.InputPointerMove && ev.Button >= 0 {
			view.userAt = time.Now()
		}
		return false
	})
	if synced && active >= 0 && active != view.scrolled && time.Since(view.userAt) > lyricsPause {
		view.scrolled = active
		view.list.ScrollTo(active, ui.Center)
	}
}

func (s *shell) emptyIn(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(s.lk.bodySize()).TextColor(s.lk.muted).Padding(8, 24)
}

// trackMenu is the menu of a track: play it, queue it, start its radio,
// like it, keep it in a mix, go to its artist or album, open it on its
// service.
func (s *shell) trackMenu(m *ui.Menu, t media.Track) { s.trackMenuAt(m, t, -1) }

// trackMenuAt is the menu of a track of the queue, the entry at queued, or
// of another track when queued is -1: in the queue, Play plays that entry,
// and the entry can be taken out, but the current one.
func (s *shell) trackMenuAt(m *ui.Menu, t media.Track, queued int) {
	one := []media.Track{t}
	switch {
	case queued >= 0:
		if m.Item("Play").Chosen() {
			s.p.PlayAt(queued)
		}
		if queued != s.st.Index && m.Item("Remove from the queue").Chosen() {
			s.p.RemoveAt(queued)
		}
	default:
		if m.Item("Play").Chosen() {
			s.p.PlayTracks(one, 0)
		}
		if m.Item("Play next").Chosen() {
			s.p.PlayNext(one)
			s.notes = append(s.notes, notice(t.Title+" plays next"))
		}
		if m.Item("Add to queue").Chosen() {
			s.p.Enqueue(one)
			s.notes = append(s.notes, notice("Added "+t.Title+" to the queue"))
		}
	}
	if m.Item("Start radio").Chosen() {
		s.startRadio(t)
	}
	m.Separator()
	liked := s.a.IsLiked(t.ID)
	label := "Add to liked songs"
	if liked {
		label = "Remove from liked songs"
	}
	if m.Item(label).Chosen() {
		s.setLiked(t, !liked)
	}
	disliked := s.a.IsDisliked(t.ID)
	label = "Dislike"
	if disliked {
		label = "Remove the dislike"
	}
	if m.Item(label).Chosen() {
		s.setDisliked(t, !disliked)
	}
	s.addToMenus(m, one)
	m.Separator()
	for _, a := range t.Artists {
		if a.ID != "" {
			if m.Item("Go to " + a.Name).Chosen() {
				s.navigate(artistPath(a.ID))
			}
			break
		}
	}
	if t.Album != nil && t.Album.ID != "" {
		if m.Item("Go to album").Chosen() {
			s.navigate(albumPath(t.Album.ID))
		}
	}
	if t.URL != "" {
		u := t.URL
		if m.Item("Open in " + sourceName(t.Source)).Chosen() {
			go s.svc.System.OpenExternal(u)
		}
		s.copyLinkItem(m, u)
	}
	if t.Source == media.SourceSpotify {
		if m.Item("Find another YouTube Music match").Chosen() {
			if err := s.svc.Player.ForgetMatch(t.ID); err != nil {
				s.notes = append(s.notes, noticeOf(err))
			} else {
				s.notes = append(s.notes, notice("The next play looks for its recording again"))
			}
		}
	}
	// What plugins add to the menu of every track.
	if actions := s.plugins.TrackActions(); len(actions) > 0 {
		m.Separator()
		for _, a := range actions {
			if m.Item(a.Label).Chosen() {
				a.Run(t)
			}
		}
	}
}

// copyLinkItem is a menu's way to share what has a page on its service, at
// rawURL: it copies that link, which anyone can open. Without a link, the
// menu has no such item.
func (s *shell) copyLinkItem(m *ui.Menu, rawURL string) {
	if rawURL != "" && m.Item("Copy link").Chosen() {
		copyText(rawURL)
		s.notes = append(s.notes, notice("Copied the link"))
	}
}

// startRadio plays track, then tracks like it.
func (s *shell) startRadio(t media.Track) {
	s.notes = append(s.notes, notice("Starting a radio of "+t.Title+"…"))
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		found, err := s.svc.Catalog.Radio(ctx, t)
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.p.PlayTracks(append([]media.Track{t}, player.RadioTracks(found, []media.Track{t})...), 0)
		})
	}()
}
