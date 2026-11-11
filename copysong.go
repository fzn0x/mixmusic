package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
)

// copySongItems is Copy in a song's menu: a submenu that copies the song's
// link, its title, its artists and "Artist - Title". It is the one place a
// song's menu gets these, so the menus of rows, tiles, the player bar, the
// queue and the video page all offer the same. What the song lacks has no
// item: a file on this computer has no link, and a song may name no artist.
func (s *shell) copySongItems(m *ui.Menu, t media.Track) {
	artists := names(t.Artists)
	m.Submenu("Copy", func(m *ui.Menu) {
		if t.URL != "" && m.Item("Copy link").Chosen() {
			copyText(t.URL)
			s.notes = append(s.notes, notice("Copied the link"))
		}
		if t.Title != "" && m.Item("Copy title").Chosen() {
			copyText(t.Title)
			s.notes = append(s.notes, notice("Copied the title"))
		}
		if artists != "" && m.Item("Copy artist").Chosen() {
			copyText(artists)
			s.notes = append(s.notes, notice("Copied the artist"))
		}
		if t.Title != "" && artists != "" && m.Item(`Copy "Artist - Title"`).Chosen() {
			copyText(artists + " - " + t.Title)
			s.notes = append(s.notes, notice("Copied the artist and title"))
		}
	})
}
