package main

import (
	"context"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/fzn0x/mixmusic/internal/media"
)

// topBar holds the history arrows, the refresh button and the search box,
// then the head of the page shown once the page scrolled it away
// (topBarHead). Under a hidden title bar, it keeps clear of the window's
// controls and drags the window.
func (s *shell) topBar(c *ui.Context) {
	k := &s.lk
	bar := c.TitleBar()
	row := ui.Row(c).Height(max(bar.Height, 48)).Shrink(0).Padding(0, bar.Right+16, 0, k.gutter).Gap(16)
	if bar.Height > 0 {
		row.DragWindow()
	}
	row.Children(func() {
		ui.Row(c).Gap(4).Children(func() {
			if s.iconButton(c, "back", "back", "Back", false, 18).Disabled(!s.router.CanGoBack()).Clicked() {
				s.router.Back()
			}
			if s.iconButton(c, "forward", "forward", "Forward", false, 18).Disabled(!s.router.CanGoForward()).Clicked() {
				s.router.Forward()
			}
			s.refreshButton(c)
		})
		s.searchField(c)
		s.topBarHead(c)
		palette := s.buttonBase(c.Key("palette")).Height(32).PaddingX(8).Radius(k.round(32)).Label("Open the command palette").Cursor(ui.CursorPointer)
		ink := k.muted
		if palette.Hovered() {
			palette.Background(k.hover)
			ink = k.text
		}
		if palette.Clicked() {
			s.paletteOpen = true
		}
		palette.Children(func() { ui.Text(c, shortcutLabel("Mod+K")).FontSize(k.captionSize()).TextColor(ink) })
	})
}

// suggestPause is how long the typing pauses before the search box asks
// for suggestions.
const suggestPause = 180 * time.Millisecond

// searchField is the 36px outlined box that searches both services, or,
// with tonal controls, a filled one, outlined in the accent while focused. It
// suggests searches below it: what was typed, which Enter searches, then
// YouTube Music's suggestions for it, or the recent searches while it is
// empty. Below the searches come the songs, albums, artists and playlists
// YouTube Music suggests: a song plays at once, with its radio after it,
// and the others open their page.
func (s *shell) searchField(c *ui.Context) {
	k := &s.lk
	box := ui.Row(c).Height(36).MaxWidth(520).Grow(1).Shrink(1).MinWidth(160).Gap(8).PaddingX(12).
		Radius(k.round(36)).Background(k.bg).Cursor(ui.CursorText)
	if k.pill {
		box.PaddingX(16)
	}
	if box.Clicked() {
		s.searchBox.Focus()
	}
	focused := s.searchBox.Focused(c)
	ink := k.muted
	switch {
	case focused:
		box.Border(2, k.accent)
		ink = k.text
	case k.tonal:
		box.Background(k.surface2)
	default:
		box.Border(2, k.outline)
	}
	if focused && k.tonal {
		box.Background(k.surface2)
	}
	search := func(q string) {
		if q = strings.TrimSpace(q); q != "" {
			s.query = q
			s.forget("suggest:")
			s.navigate(searchPath(q))
		}
	}
	box.Children(func() {
		ui.Icon(c, icon("search")).FontSize(18).TextColor(ink)
		cb := ui.ComboboxBase(c.Key("search"), &s.query)
		in := cb.Input.Placeholder(k.words("Search Spotify and YouTube Music")).Grow(1).MinWidth(0).Bind(&s.searchBox).
			Label("Search").FontSize(k.bodySize())
		if in.Changed() {
			s.queryAt = c.Now()
		}
		var (
			options []string
			items   []media.Item
		)
		if cb.Open() {
			options, items = s.suggestions(c)
		}
		cb.Popup(func(panel ui.Element) {
			// A menu, below the box and as wide, rather than below the text:
			// a long title ends in an ellipsis rather than widen it, but in a
			// box too narrow to read one.
			b := box.Bounds()
			panel.AttachTo(box, ui.AnchorBottomStart, ui.AnchorTopStart).MarginY(6).MinWidth(b.W).MaxWidth(max(b.W, 360)).Padding(4).
				Radius(k.radiusSm).Background(k.raised).Border(1, k.border).Shadow(0, 12, 32, -8, ui.RGBA(0, 0, 0, 0.55))
			// In a window too short for them all, the rows scroll, and the
			// one the arrows reach comes into view.
			_, h := c.Size()
			ui.Scroll(c.Key("search-suggestions")).MaxHeight(max(h-b.Y-b.H-6-8-12, 120)).Children(func() {
				for _, q := range options {
					item := cb.Item(q).Height(36).PaddingX(12).Gap(12).AlignItems(ui.Center).Radius(k.radiusSm).Cursor(ui.CursorPointer)
					ink, mark := k.text, k.muted
					if item.Highlighted() {
						item.Background(k.accent)
						ink, mark = k.accentText, k.accentText
					}
					item.Children(func() {
						ui.Icon(c, icon("search")).FontSize(14).TextColor(mark)
						ui.Text(c, q).FontSize(k.bodySize()).TextColor(ink).SingleLine().MinWidth(0)
					})
				}
				if len(options) > 0 && len(items) > 0 {
					ui.Divider(c).MarginY(4).Background(k.border)
				}
				for _, it := range items {
					s.suggestedItem(c, cb, it)
				}
			})
		})
		if v, ok := cb.Chosen(); ok {
			// The choice comes as the popup closes, which then built no rows.
			if it, ok := suggestedOf(s.suggested.Items, v); ok {
				s.openSuggested(it)
			} else {
				search(v)
			}
		} else if in.Submitted() {
			search(s.query)
		}
	})
}

// suggestions are what the box suggests: the searches, what was typed,
// then up to seven of YouTube Music's suggestions for it, and up to five
// of its songs, albums, artists and playlists, asked once the typing
// pauses; or the recent searches while nothing is typed.
func (s *shell) suggestions(c *ui.Context) (searches []string, items []media.Item) {
	q := strings.TrimSpace(s.query)
	if wait := suggestPause - c.Now().Sub(s.queryAt); wait > 0 {
		c.After(wait)
	} else if l := fetch(s, "suggest:"+q, func(ctx context.Context) (media.Suggestions, error) { return s.svc.Catalog.Suggestions(ctx, q) }); l.done {
		s.suggestedFor, s.suggested = q, l.value
	}
	if q != "" {
		searches = append(searches, q)
	}
	// Those of what was typed before show until those of the text come.
	if s.suggestedFor == q || s.suggestedFor != "" && strings.HasPrefix(q, s.suggestedFor) {
		for _, v := range s.suggested.Searches {
			if len(searches) == 8 {
				break
			}
			if v = strings.TrimSpace(v); v != "" && !strings.EqualFold(v, q) {
				searches = append(searches, v)
			}
		}
		if q != "" {
			items = s.suggested.Items[:min(len(s.suggested.Items), 5)]
		}
	}
	return searches, items
}

// suggestedKey is the value of an item's row among the suggestions, which
// no search typed can be.
func suggestedKey(it media.Item) string {
	id := ""
	switch {
	case it.Track != nil:
		id = it.Track.ID
	case it.Album != nil:
		id = it.Album.ID
	case it.Artist != nil:
		id = it.Artist.ID
	case it.Playlist != nil:
		id = it.Playlist.ID
	case it.Podcast != nil:
		id = it.Podcast.ID
	}
	return "\x00" + string(it.Kind) + ":" + id
}

// suggestedOf returns the item of the row chosen, value, or false when it
// is a search's.
func suggestedOf(items []media.Item, value string) (media.Item, bool) {
	for _, it := range items {
		if suggestedKey(it) == value {
			return it, true
		}
	}
	return media.Item{}, false
}

// suggestedItem is the 48px row of a song, album, artist or playlist the
// box suggests: its artwork, round for an artist, then its title over
// what it is and by whom.
func (s *shell) suggestedItem(c *ui.Context, cb ui.ComboboxParts, it media.Item) {
	k := &s.lk
	key := suggestedKey(it)
	title, line, art := s.suggestedLine(it)
	item := cb.Item(key).Height(48).PaddingX(8).Gap(12).AlignItems(ui.Center).Radius(k.radiusSm).Cursor(ui.CursorPointer).
		Label(title + ", " + line)
	ink, mark := k.text, k.muted
	if item.Highlighted() {
		item.Background(k.accent)
		ink, mark = k.accentText, k.accentText
	}
	item.Children(func() {
		s.cover(c, "suggested-art"+key, art, 32, it.Artist != nil)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, title).FontSize(k.bodySize()).TextColor(ink).SingleLine().MinWidth(0)
			ui.Text(c, line).FontSize(k.captionSize()).TextColor(mark).SingleLine().MinWidth(0)
		})
	})
}

// suggestedLine is the title of an item the box suggests, what it is and
// by whom, as "Song · Daft Punk", and its artwork.
func (s *shell) suggestedLine(it media.Item) (title, line string, art []media.Image) {
	k := &s.lk
	kind, by := "", ""
	switch {
	case it.Track != nil:
		t := it.Track
		title, art, by = t.Title, t.Artwork, names(t.Artists)
		switch {
		case t.IsEpisode():
			kind = "Episode"
		case t.Video:
			kind = "Video"
		default:
			kind = "Song"
		}
	case it.Album != nil:
		a := it.Album
		title, art, by = a.Title, a.Artwork, names(a.Artists)
		switch a.Kind {
		case media.AlbumKindSingle:
			kind = "Single"
		case media.AlbumKindEP:
			kind = "EP"
		default:
			kind = "Album"
		}
	case it.Artist != nil:
		title, art, kind = it.Artist.Name, it.Artist.Artwork, "Artist"
	case it.Playlist != nil:
		p := it.Playlist
		title, art, kind, by = p.Title, p.Artwork, "Playlist", p.Owner
	case it.Podcast != nil:
		p := it.Podcast
		title, art, kind, by = p.Title, p.Artwork, "Podcast", names(p.Authors)
	}
	line = k.words(kind)
	if by != "" {
		line += " · " + by
	}
	return title, line, art
}

// openSuggested plays a song the box suggested at once, with its radio
// after it, or opens the page of an album, artist or playlist.
func (s *shell) openSuggested(it media.Item) {
	if it.Track != nil {
		s.p.PlayWithRadio(*it.Track)
		return
	}
	if path := itemPath(it); path != "" {
		s.navigate(path)
	}
}
