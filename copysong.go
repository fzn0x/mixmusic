package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// copySongItems is Copy in a song's menu: a submenu that copies the song's
// link, its title, its artists and "Artist - Title". It is the one place a
// song's menu gets these, so the menus of rows, tiles, the player bar, the
// queue and the video page all offer the same. What the song lacks has no
// item: a file on this computer has no link, and a song may name no artist.
func (s *shell) copySongItems(m *ui.Menu, t media.Track) {
	artists := names(t.Artists)
	m.Submenu(locales.CopySongMenu(), func(m *ui.Menu) {
		if t.URL != "" && m.Item(locales.CopySongLink()).Chosen() {
			copyText(t.URL)
			s.notes = append(s.notes, notice(locales.CopySongLinkCopied()))
		}
		if t.Title != "" && m.Item(locales.CopySongTitle()).Chosen() {
			copyText(t.Title)
			s.notes = append(s.notes, notice(locales.CopySongTitleCopied()))
		}
		if artists != "" && m.Item(locales.CopySongArtist()).Chosen() {
			copyText(artists)
			s.notes = append(s.notes, notice(locales.CopySongArtistCopied()))
		}
		if t.Title != "" && artists != "" && m.Item(locales.CopySongArtistTitle()).Chosen() {
			copyText(artists + " - " + t.Title)
			s.notes = append(s.notes, notice(locales.CopySongArtistTitleCopied()))
		}
	})
}
