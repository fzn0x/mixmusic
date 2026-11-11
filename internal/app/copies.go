package app

// Copies of one song on this computer (#340): files of the same title and
// artists, a few seconds apart at most, or that the user linked, are one
// song, which plays its best copy. internal/localfiles groups them; here
// the liked and disliked songs, the mixes and the history follow the song,
// not one of its files: what was kept of a file before it became a copy
// goes to its song, and what the user does with a copy, as one an album
// shows, or one played alone, is done with its song.

import (
	"io"
	"log"
	"slices"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
)

// followCopies has the index read what the audio of the copies is, which
// tells the best of each song, and the library follow the songs with
// copies: from the first grouping on, once each run, and as they change.
func (a *App) followCopies() {
	a.localFiles.Inspect(inspectCopy)
	a.localFiles.OnLinked(func(to map[string]media.Track) {
		if err := a.library.Retarget(to); err != nil {
			log.Printf("mixmusic: moving the plays and likes of copies to their songs: %v", err)
		}
	})
}

// inspectCopy reads what the audio of a copy is, as the engine opens it.
func inspectCopy(r io.ReadSeeker) (localfiles.Facts, error) {
	f, err := audio.Inspect(r)
	return localfiles.Facts{Codec: f.Codec, SampleRate: f.SampleRate, BitDepth: f.BitDepth}, err
}

// asSong returns the track of the song t is a copy of, else t.
func (a *App) asSong(t media.Track) media.Track {
	if a.localFiles == nil || t.Source != media.SourceFile {
		return t
	}
	return a.localFiles.SongTrack(t)
}

// asSongs returns tracks, each copy as its song.
func (a *App) asSongs(tracks []media.Track) []media.Track {
	var out []media.Track
	for i, t := range tracks {
		if s := a.asSong(t); s.ID != t.ID {
			if out == nil {
				out = append([]media.Track{}, tracks...)
			}
			out[i] = s
		}
	}
	if out == nil {
		return tracks
	}
	return out
}

// songID returns the ID of the song of a copy's ID, else id.
func (a *App) songID(id string) string {
	if a.localFiles == nil {
		return id
	}
	return a.localFiles.SongIDOf(id)
}

// Song returns the song of a song's ID or of one of its copies', with its
// copies on this computer, best first, the one it plays marked.
func (l *LocalFiles) Song(id string) (localfiles.Song, bool) { return l.a.localFiles.Song(id) }

// FileOf returns the path of the file that plays a track on this
// computer: a file's own, and of a song with copies, its best copy that's
// there.
func (l *LocalFiles) FileOf(id string) (string, bool) { return l.a.localFiles.FileOf(id) }

// Link makes the tracks of ids, files on this computer and songs with
// copies, copies of one song, whatever their tags say: what was played,
// liked and mixed of each goes to the song.
func (l *LocalFiles) Link(ids []string) error { return l.a.localFiles.Link(ids) }

// Unlink takes a copy, by its file's ID, out of its song, which keeps what
// was played, liked and mixed of it.
func (l *LocalFiles) Unlink(id string) error { return l.a.localFiles.Unlink(id) }

// withCopies adds to counts, of tracks by their IDs, the count of each
// song for each of its copies, as an album shows them.
func (a *App) withCopies(counts map[string]int) map[string]int {
	if a.localFiles == nil {
		return counts
	}
	for copyID, songID := range a.localFiles.SongIDs() {
		if n, ok := counts[songID]; ok {
			counts[copyID] = n
		}
	}
	return counts
}

// withSongs returns tracks, and after them the song of each that is a
// copy of one, which the queue holds as the song.
func (a *App) withSongs(tracks []media.Track) []media.Track {
	out := slices.Clone(tracks)
	for _, t := range tracks {
		if s := a.asSong(t); s.ID != t.ID && !slices.ContainsFunc(out, func(x media.Track) bool { return x.ID == s.ID }) {
			out = append(out, s)
		}
	}
	return out
}

// fileID returns the ID of the file a track on this computer plays: of a
// song with copies, its best copy's, else id.
func (a *App) fileID(id string) string {
	if !localfiles.IsSong(id) {
		return id
	}
	if p, ok := a.localFiles.FileOf(id); ok {
		return localfiles.TrackID(p)
	}
	return id
}

// copyIDs returns ids, each song with copies as the IDs of its copies on
// this computer, which all take a song's new tags.
func (a *App) copyIDs(ids []string) []string {
	var out []string
	for _, id := range ids {
		s, ok := a.localFiles.Song(id)
		if !localfiles.IsSong(id) || !ok {
			out = append(out, id)
			continue
		}
		for _, c := range s.Copies {
			out = append(out, c.Track.ID)
		}
	}
	return out
}
