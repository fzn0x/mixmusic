package main

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// songsReader reads all the songs of an artist, giving each the new ones
// as they come: Catalog.ArtistSongs.
type songsReader func(ctx context.Context, id string, each func([]media.Track)) ([]media.Track, error)

// artistSongsWait is the longest the songs of an artist are read for.
const artistSongsWait = 15 * time.Minute

// artistPlaysAll tells whether Play on the artist's page plays all of
// their songs, read from their releases, rather than their top songs.
func artistPlaysAll(a media.Artist) bool {
	return a.Source == media.SourceSpotify && len(a.Albums)+len(a.Singles) > 0
}

// playArtist plays all the songs of a, read with read: sound starts with
// the first release read, and the songs of the others join the end of the
// queue, in the order of the releases, while the queue is still the one it
// started. A read that gave no song plays their top songs, with a notice;
// one that came in part says how many releases came.
func (s *shell) playArtist(a media.Artist, read songsReader) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), artistSongsWait)
		defer cancel()
		var first string
		put := func(batch []media.Track) {
			s.update(func() {
				switch {
				case first == "":
					first = batch[0].ID
					s.p.PlayAll(batch, false)
				// A queue that is still empty is the one PlayAll hasn't made yet.
				case len(s.p.State().Queue) == 0 || slices.ContainsFunc(s.p.State().Queue, func(it player.Item) bool { return it.Track.ID == first }):
					s.p.Enqueue(batch)
				}
			})
		}
		_, err := read(ctx, a.ID, put)
		s.update(func() {
			if short, ok := media.Short(err); ok {
				s.notes = append(s.notes, noticeOf(errors.New("Only "+strconv.Itoa(short.Got)+" of "+strconv.Itoa(short.Total)+
					" of "+a.Name+"'s albums and singles could be read: "+short.Err.Error())))
				return
			}
			if err == nil || first != "" {
				return
			}
			if len(a.TopTracks) == 0 {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.notes = append(s.notes, noticeOf(errors.New("Couldn't read all of "+a.Name+"'s songs, so their top songs play: "+err.Error())))
			s.p.PlayAll(a.TopTracks, false)
		})
	}()
}
