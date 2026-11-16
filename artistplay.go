package main

// An artist's Play and Shuffle play all their songs (#543): on YouTube
// Music, the list of them that the title of their Top songs opens, and
// Show all beside it; on Spotify, those of all their releases (#571);
// on this computer, theirs. An artist's tile and its menu play them too
// (#572). Without such a list, Play plays their top songs.

import (
	"context"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// playAllSongs plays all the songs of a, an artist's page, shuffled or
// not, as the app does (Catalog.PlayArtist), which reads them meanwhile;
// tests answer for it.
var playAllSongs = func(s *shell, a media.Artist, shuffled bool) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), artistSongsWait)
		defer cancel()
		if err := s.svc.Catalog.PlayArtist(ctx, s.p, a, shuffled); err != nil {
			s.update(func() { s.notes = append(s.notes, noticeOf(err)) })
		}
	}()
}

// playArtistID plays all the songs of the artist with the id given, as an
// artist's tile and its menu do, which don't hold the songs: the app reads
// the artist first, as their page does (Catalog.PlayArtistID). A read that
// fails says why in a notice, as a playlist's tile does; tests answer for
// it.
var playArtistID = func(s *shell, id string, shuffled bool) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), artistSongsWait)
		defer cancel()
		if err := s.svc.Catalog.PlayArtistID(ctx, s.p, id, shuffled); err != nil {
			s.update(func() { s.notes = append(s.notes, noticeOf(err)) })
		}
	}()
}

// artistPlay gives the head of a's page Play and Shuffle of all their
// songs, where MixMusic knows them: Play then says so. It leaves the head
// as it was for the others.
func (s *shell) artistPlay(h *headProps, a media.Artist) {
	if a.SongsID != "" || a.Source == media.SourceFile && len(a.TopTracks) > 0 ||
		a.Source == media.SourceSpotify && len(a.Albums)+len(a.Singles) > 0 {
		h.playLabel = locales.LibraryPlay()
		h.play = func() { playAllSongs(s, a, false) }
		h.shuffle = func() { playAllSongs(s, a, true) }
	}
}

// topSongsTitle heads an artist's top songs, with Show all, which opens
// the list of all their songs, where they have one.
func (s *shell) topSongsTitle(c *ui.Context, a media.Artist) {
	if a.SongsID == "" {
		s.sectionTitle(c, locales.CollectionTopSongs())
		return
	}
	s.sectionHead(c, locales.CollectionTopSongs(), func() {
		if s.link(c, "artist-songs", locales.DownloadsShowAll()) {
			s.navigate(playlistPath(a.SongsID))
		}
	})
}

// artistSongsWait is the longest the songs of an artist are read for: a
// Spotify artist of many releases takes minutes.
const artistSongsWait = 15 * time.Minute
