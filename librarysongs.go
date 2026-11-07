package main

import (
	"context"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/fzn0x/mixmusic/internal/media"
)

// saveOnYouTube saves a song to the user's library on YouTube Music, or
// takes it out; tests answer for YouTube.
var saveOnYouTube = func(s *shell, ctx context.Context, t media.Track, on bool) error {
	return s.svc.Collection.SetSongSaved(ctx, t, on)
}

// youtubeSongsKey is the load of the songs the user saved to their
// library on YouTube Music, apart from those they liked.
const youtubeSongsKey = "youtube:songs"

func (s *shell) youtubeSongs() *loaded[[]media.Track] {
	return fetch(s, youtubeSongsKey, s.svc.Collection.YouTubeSongs)
}

// keepYouTubeSongs fetches the songs of the user's library on YouTube
// Music once they're signed in, before any track menu opens, which tells
// from them whether a song is there: the system's menus show what they
// held as they opened, as keepYouTubePlaylists says.
func (s *shell) keepYouTubeSongs() {
	if s.accounts().YouTube.SignedIn {
		s.youtubeSongs()
	}
}

// songInLibrary reports whether a song is in the user's library on
// YouTube Music, as far as the songs fetched say.
func (s *shell) songInLibrary(id string) bool {
	l := s.youtubeSongs()
	return l.done && slices.ContainsFunc(l.value, func(t media.Track) bool { return t.ID == id })
}

// librarySongItem is the item of a YouTube Music song's menu that saves it
// to the user's library there, without liking it, or takes it out, for a
// user signed in. It waits while a change of the song goes.
func (s *shell) librarySongItem(m *ui.Menu, t media.Track) {
	if t.Source != media.SourceYouTube || t.IsEpisode() || !s.signedIn(media.SourceYouTube) {
		return
	}
	saved := s.songInLibrary(t.ID)
	label := "Add to library"
	if saved {
		label = "Remove from library"
	}
	if m.Item(label).Disabled(s.keeping["song:"+t.ID]).Chosen() {
		s.saveSong(t, !saved)
	}
}

// saveSong saves a song to the user's library on YouTube Music, or takes
// it out. The library and the menus show it at once, and put it back as
// it was if YouTube refuses; the songs aren't read again, which YouTube
// may answer as they were a moment before.
func (s *shell) saveSong(t media.Track, on bool) {
	key := "song:" + t.ID
	s.keeping[key] = true
	s.listSong(t, on)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := saveOnYouTube(s, ctx, t, on)
		s.update(func() {
			delete(s.keeping, key)
			switch {
			case err != nil:
				s.listSong(t, !on)
				s.notes = append(s.notes, noticeOf(err))
			case on:
				s.notes = append(s.notes, notice("Added "+t.Title+" to your library on YouTube Music"))
			default:
				s.notes = append(s.notes, notice("Removed "+t.Title+" from your library on YouTube Music"))
			}
		})
	}()
}

// listSong puts a song first among those fetched of the user's library,
// or takes it out of them. Songs still on their way are asked for again.
func (s *shell) listSong(t media.Track, on bool) {
	l, ok := s.loads[youtubeSongsKey].(*loaded[[]media.Track])
	switch {
	case !ok:
	case !l.done:
		s.refetch(youtubeSongsKey)
	default:
		songs := slices.DeleteFunc(slices.Clone(l.value), func(x media.Track) bool { return x.ID == t.ID })
		if on {
			songs = slices.Insert(songs, 0, t)
		}
		l.value = songs
	}
}

// youtubeSongsList lists the songs of the user's library on YouTube
// Music, in Library › YouTube Music.
func (s *shell) youtubeSongsList(c *ui.Context) {
	s.sectionTitle(c, "Songs in your library")
	songs := s.youtubeSongs()
	switch {
	case !songs.done:
		s.loading(c)
	case songs.err != nil:
		s.problem(c, songs.err, func() { s.forget(youtubeSongsKey) })
	default:
		s.trackList(c, "youtube-songs", songs.value, trackListOptions{art: true, album: true,
			empty: "No songs in your library on YouTube Music. A song's Add to library keeps it here without liking it."})
	}
}
