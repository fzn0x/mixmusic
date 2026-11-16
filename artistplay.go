package main

// An artist's Play and Shuffle play all their songs (#543): on YouTube
// Music, the list of them that the title of their Top songs opens, and
// Show all beside it; on this computer, theirs. Without such a list,
// Play plays their top songs.

import (
	"context"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
)

// playAllSongs plays all the songs of a, an artist's page, shuffled or
// not, as the app does (Catalog.PlayArtist), which reads them meanwhile;
// tests answer for it.
var playAllSongs = func(s *shell, a media.Artist, shuffled bool) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), wholeWait)
		defer cancel()
		if err := s.svc.Catalog.PlayArtist(ctx, s.p, a, shuffled); err != nil {
			s.update(func() { s.notes = append(s.notes, noticeOf(err)) })
		}
	}()
}

// artistPlay gives the head of a's page Play and Shuffle of all their
// songs, where MixMusic knows them: Play then says so. It leaves the head
// as it was for the others.
func (s *shell) artistPlay(h *headProps, a media.Artist) {
	if a.SongsID != "" || a.Source == media.SourceFile && len(a.TopTracks) > 0 {
		h.playLabel = "play"
		h.play = func() { playAllSongs(s, a, false) }
		h.shuffle = func() { playAllSongs(s, a, true) }
	}
}

// topSongsTitle heads an artist's top songs, with Show all, which opens
// the list of all their songs, where they have one.
func (s *shell) topSongsTitle(c *ui.Context, a media.Artist) {
	if a.SongsID == "" {
		s.sectionTitle(c, "Top songs")
		return
	}
	s.sectionHead(c, "Top songs", func() {
		if s.link(c, "artist-songs", "show all") {
			s.navigate(playlistPath(a.SongsID))
		}
	})
}
