package native

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
)

// trackListOptions say what a track list shows.
type trackListOptions struct {
	// art shows each track's artwork; else its number.
	art bool
	// album shows the album column.
	album bool
	// plays shows how many times each track played, where its service
	// says: in a search, to tell the original from the covers.
	plays bool
	// menu adds items to a track's menu, as removing it from a playlist.
	menu  func(m *ui.Menu, i int, t media.Track)
	empty string
}

// trackList lists tracks: number or art, title and artists, album, source,
// like and duration. Playing a track plays the list from it; the playing
// row floods with the accent.
func (s *shell) trackList(c *ui.Context, key string, tracks []media.Track, o trackListOptions) {
	if len(tracks) == 0 {
		empty := o.empty
		if empty == "" {
			empty = "No tracks here yet."
		}
		s.emptyLine(c, empty)
		return
	}
	cur, hasCur := s.st.Current()
	list := ui.Column(c.Key(key))
	from, to := s.nearRows(c, key, list, len(tracks), s.lk.row)
	list.Children(func() {
		// The rows far from the view take their room, and build nothing.
		if from > 0 {
			ui.Box(c).Height(float32(from) * s.lk.row)
		}
		for i := from; i < to; i++ {
			t := tracks[i]
			s.trackRow(c, key, i, t, tracks, hasCur && cur.Track.ID == t.ID, o)
		}
		if to < len(tracks) {
			ui.Box(c).Height(float32(len(tracks)-to) * s.lk.row)
		}
	})
}

func (s *shell) trackRow(c *ui.Context, key string, i int, t media.Track, tracks []media.Track, current bool, o trackListOptions) {
	k := &s.lk
	row := ui.Row(c.Key(key+"-"+count(i)+"-"+t.ID)).Height(k.row).Padding(0, 8).Gap(16).Radius(k.radiusSm)
	ink, sub := k.text, k.muted
	switch {
	case current:
		row.Background(k.accent)
		ink, sub = k.accentText, k.accentText.Alpha(0.78)
	case row.Hovered():
		row.Background(k.hover)
	}
	play := func() {
		if current {
			s.p.Toggle()
		} else {
			s.p.PlayTracks(tracks, i)
		}
	}
	if row.DoubleClicked() {
		play()
	}
	menu := func(m *ui.Menu) {
		s.trackMenu(m, t)
		if o.menu != nil {
			o.menu(m, i, t)
		}
	}
	row.ContextMenu(menu)
	hovered := row.Hovered()
	row.Children(func() {
		lead := ui.ButtonBase(c.Key(key+"-lead-"+count(i))).Size(40, 40).Shrink(0).KeepFocus().Cursor(ui.CursorPointer)
		label := "Play " + t.Title
		if current && s.st.Playing {
			label = "Pause " + t.Title
		}
		lead.Label(label)
		if lead.Clicked() {
			play()
		}
		lead.Children(func() {
			switch {
			case o.art:
				s.cover(c, key+"-cover-"+count(i), t.Artwork, 40, false)
				if hovered || current {
					name := "play"
					if current && s.st.Playing {
						name = "pause"
					}
					ui.Box(c).Absolute().Top(0).Left(0).Size(40, 40).Background(k.scrim).Center().Children(func() {
						ui.Icon(c, icon(name)).FontSize(16).TextColor(k.text)
					})
				}
			case hovered || current:
				name := "play"
				if current && s.st.Playing {
					name = "pause"
				}
				ui.Icon(c, icon(name)).FontSize(16).TextColor(ink)
			default:
				ui.Text(c, count(i+1)).FontSize(k.smallSize()).TextColor(sub).FontFeatures("tnum")
			}
		})
		ui.Column(c).Grow(2).Basis(0).MinWidth(0).Gap(2).Children(func() {
			ui.Row(c).Gap(6).MinWidth(0).Children(func() {
				ui.Text(c, t.Title).FontSize(k.bodySize()).TextColor(ink).SingleLine().MinWidth(0)
				if t.Explicit {
					ui.Text(c, "e").FontSize(k.captionSize()).TextColor(sub).Padding(0, 4).Border(1, sub).
						Radius(2).Label("Explicit").Shrink(0)
				}
			})
			s.artistLinks(c, key+"-artists-"+count(i), t.Artists, sub)
		})
		if o.album {
			ui.Row(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
				if t.Album == nil {
					return
				}
				if t.Album.ID == "" {
					ui.Text(c, t.Album.Name).FontSize(k.smallSize()).TextColor(sub).SingleLine().MinWidth(0)
					return
				}
				// A button keeps its width; the link gives way, as its name.
				link := ui.ButtonBase(c.Key(key + "-album-" + count(i))).Shrink(1).MinWidth(0).Justify(ui.Start).Label(t.Album.Name).KeepFocus().Cursor(ui.CursorPointer)
				if link.Clicked() {
					s.navigate(albumPath(t.Album.ID))
				}
				link.Children(func() {
					ui.Text(c, t.Album.Name).FontSize(k.smallSize()).TextColor(sub).SingleLine().MinWidth(0)
				})
			})
		}
		if o.plays {
			// Its column stays where none is said, to keep the others in
			// line.
			ui.Text(c, playsLabel(t)).FontSize(k.smallSize()).TextColor(sub).FontFeatures("tnum").
				Width(88).TextAlign(ui.End).Shrink(0)
		}
		s.sourceMark(c, t.Source, current)
		if hovered || s.a.IsLiked(t.ID) {
			s.likeButton(c, key+"-like-"+count(i), t, 16)
		} else {
			ui.Box(c).Size(32, 32).Shrink(0)
		}
		duration := ""
		if t.DurationMs > 0 {
			duration = clock(ms(t.DurationMs))
		}
		ui.Text(c, duration).FontSize(k.smallSize()).TextColor(sub).FontFeatures("tnum").Width(48).TextAlign(ui.End)
		// The menu, without a right click: shown on the row the pointer is
		// on, and kept in its place on the others.
		more := s.iconButton(c, key+"-more-"+count(i), "more", "More for "+t.Title, false, 18).Menu(menu)
		if !hovered && !more.Focused() {
			more.Opacity(0)
		}
	})
}

// artistLinks writes artists, each a link to its page when it has one.
func (s *shell) artistLinks(c *ui.Context, key string, artists []media.Ref, ink ui.Color) {
	k := &s.lk
	// A text drops the space it ends with: the gap spaces the names.
	ui.Row(c).MinWidth(0).Gap(k.smallSize() / 4).Clip().Children(func() {
		for j, a := range artists {
			name := a.Name
			if j < len(artists)-1 {
				name += ","
			}
			if a.ID == "" {
				ui.Text(c, name).FontSize(k.smallSize()).TextColor(ink).SingleLine().Shrink(0)
				continue
			}
			link := ui.ButtonBase(c.Key(key + "-" + count(j))).Shrink(0).Label(a.Name).KeepFocus().Cursor(ui.CursorPointer)
			if link.Clicked() {
				s.navigate(artistPath(a.ID))
			}
			hovered := link.Hovered()
			link.Children(func() {
				t := ui.Text(c, name).FontSize(k.smallSize()).TextColor(ink).SingleLine()
				if hovered {
					t.Underline()
				}
			})
		}
	})
}
