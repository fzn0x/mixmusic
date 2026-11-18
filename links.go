package main

// Links: a link to a song, album, playlist, artist or podcast of YouTube,
// YouTube Music or Spotify, pasted in the search box or the command
// palette, opens what it names, rather than search its text (#614).

import (
	"context"
	"errors"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/locales"
)

// linkValue is the value of the row of a link's item among the search
// box's suggestions, which no search typed, nor other row, can be.
const linkValue = "\x00link"

// readLink is what l names, as the catalog reads it: once for each link,
// whatever tracked who shared it.
func (s *shell) readLink(l media.Link) *loaded[app.Linked] {
	return fetch(s, "link:"+l.URL(), func(ctx context.Context) (app.Linked, error) { return s.svc.Catalog.Link(ctx, l) })
}

// openLink opens what l names, once read: a song plays at once, as a
// suggested song does, and the song of a radio link with that radio after
// it; an album, a playlist or an artist opens its page, and a show or an
// episode its podcast's. A link that can't be read says why, and nothing
// is searched.
func (s *shell) openLink(l media.Link) {
	s.linkOpens = &l
	s.followLink()
}

// followLink opens the link openLink was given, once it is read.
func (s *shell) followLink() {
	if s.linkOpens == nil {
		return
	}
	l := *s.linkOpens
	res := s.readLink(l)
	if !res.done {
		return
	}
	s.linkOpens = nil
	if res.err != nil {
		// Opened again, it is read again.
		s.forget("link:" + l.URL())
		s.linkFailed(res.err)
		return
	}
	if t := res.value.Item.Track; t != nil && l.Kind == media.LinkRadio {
		s.p.PlayRadio(*t, res.value.Radio)
		return
	}
	s.openSuggested(res.value.Item)
}

// linkFailed says why a link didn't open: signed out of Spotify, with a
// way to Settings › Accounts.
func (s *shell) linkFailed(err error) {
	if errors.Is(err, spotify.ErrNotSignedIn) {
		s.actionToasts = append(s.actionToasts, actionToast{errorText(err), locales.ShellToastSettings(), func() { s.navigate("/settings/accounts") }})
		return
	}
	s.notes = append(s.notes, player.Notice{Message: locales.TopbarLinkCantOpen(errorText(err)), Error: true})
}

// linkSuggestion is what the search box suggests for l, a link it opens:
// the item it names, once read, as its only row; none while it is read,
// or when it can't be. It asks once the typing pauses.
func (s *shell) linkSuggestion(c *ui.Context, l media.Link) *app.Linked {
	if wait := suggestPause - c.Now().Sub(s.queryAt); wait > 0 {
		c.After(wait)
		return nil
	}
	if res := s.readLink(l); res.done && res.err == nil {
		return &res.value
	}
	return nil
}

// linkedItem is the row of v, what the link l names, among the search
// box's suggestions: as a suggested item's, but that of a radio says so.
func (s *shell) linkedItem(c *ui.Context, cb ui.ComboboxParts, l media.Link, v app.Linked) {
	it := v.Item
	title, line, art := s.suggestedLine(it)
	if t := it.Track; t != nil && l.Kind == media.LinkRadio {
		line = s.lk.words(locales.TopbarKindRadio())
		if by := names(t.Artists); by != "" {
			line += " · " + by
		}
	}
	// A playlist of Spotify's own, which it shows apps nothing of.
	if title == "" && it.Playlist != nil {
		title = locales.HiddenSpotifyPlaylist()
	}
	s.suggestedRow(c, cb, linkValue, it, title, line, art)
}
