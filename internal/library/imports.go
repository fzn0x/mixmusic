package library

import (
	"slices"

	"github.com/fzn0x/mixmusic/internal/media"
)

// LikeAll likes the tracks an import brings at once (#613), in their
// order, first among the liked songs, and returns those it liked: a track
// liked already keeps its place. Liking takes their dislikes back, as
// SetLiked does.
func (l *Library) LikeAll(tracks []media.Track) ([]media.Track, error) {
	var added []media.Track
	err := l.file.Update(func(d *Data) error {
		added = nil
		seen := map[string]bool{}
		for _, t := range d.Liked {
			seen[t.ID] = true
		}
		for _, t := range tracks {
			if !seen[t.ID] {
				seen[t.ID] = true
				added = append(added, t)
			}
		}
		if len(added) == 0 {
			return nil
		}
		d.Liked = append(slices.Clone(added), d.Liked...)
		d.Disliked = slices.DeleteFunc(d.Disliked, func(x media.Track) bool {
			return slices.ContainsFunc(added, func(t media.Track) bool { return t.ID == x.ID })
		})
		return nil
	})
	l.indexDisliked()
	if err != nil {
		return nil, err
	}
	return added, nil
}

// PlaylistTrackIDs returns the IDs of the tracks of a playlist, for an
// import to leave out those it has.
func (l *Library) PlaylistTrackIDs(id string) (map[string]bool, error) {
	for _, p := range l.file.Get().Playlists {
		if p.ID == id {
			ids := make(map[string]bool, len(p.Tracks))
			for _, t := range p.Tracks {
				ids[t.ID] = true
			}
			return ids, nil
		}
	}
	return nil, ErrNoPlaylist
}

// TrackIDs returns the IDs of the tracks the library holds: the liked
// ones and those of the playlists.
func (l *Library) TrackIDs() map[string]bool {
	d := l.file.Get()
	ids := make(map[string]bool, len(d.Liked))
	for _, t := range d.Liked {
		ids[t.ID] = true
	}
	for _, p := range d.Playlists {
		for _, t := range p.Tracks {
			ids[t.ID] = true
		}
	}
	return ids
}

// Tracks returns the tracks the library holds, each once: the liked ones,
// then those of the playlists.
func (l *Library) Tracks() []media.Track {
	d := l.file.Get()
	seen := map[string]bool{}
	var out []media.Track
	add := func(ts []media.Track) {
		for _, t := range ts {
			if !seen[t.ID] {
				seen[t.ID] = true
				out = append(out, t)
			}
		}
	}
	add(d.Liked)
	for _, p := range d.Playlists {
		add(p.Tracks)
	}
	return out
}
