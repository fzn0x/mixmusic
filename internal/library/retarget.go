package library

// Tracks that became others (#340): files on this computer linked as
// copies of one song take the song's ID, which the liked and disliked
// songs, the mixes and the history keep from then on, so that they follow
// the song rather than one of its files, and stay when a copy goes.

import (
	"slices"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Retarget gives what the library keeps of the tracks of the IDs of to
// the track each became: in the liked and disliked songs, the mixes and
// the history, which keeps when each played. A track that becomes one the
// list holds already stays where that one is: a song liked as two of its
// files is liked once. One liked and disliked stays liked. Nothing is
// written when the library holds none of them.
func (l *Library) Retarget(to map[string]media.Track) error {
	if len(to) == 0 || !holdsAny(l.file.Get(), to) {
		return nil
	}
	err := l.file.Update(func(d *Data) error {
		d.Liked = retargetList(d.Liked, to)
		d.Disliked = retargetList(d.Disliked, to)
		d.Disliked = slices.DeleteFunc(d.Disliked, func(t media.Track) bool {
			return slices.ContainsFunc(d.Liked, func(x media.Track) bool { return x.ID == t.ID })
		})
		for i := range d.Playlists {
			d.Playlists[i].Tracks = retargetList(d.Playlists[i].Tracks, to)
		}
		for i, p := range d.History {
			if t, ok := to[p.Track.ID]; ok {
				d.History[i].Track = t
			}
		}
		return nil
	})
	l.indexDisliked()
	return err
}

// retargetList gives the tracks of list the tracks of to, leaving out
// one that became a track the list holds before it, or that the list
// holds after one that became it. Those it held twice before, as a mix
// may, stay.
func retargetList(list []media.Track, to map[string]media.Track) []media.Track {
	out := make([]media.Track, 0, len(list))
	seen, moved := map[string]bool{}, map[string]bool{}
	for _, t := range list {
		n, ok := to[t.ID]
		if !ok {
			n = t
		}
		if seen[n.ID] && (ok || moved[n.ID]) {
			continue
		}
		out = append(out, n)
		seen[n.ID] = true
		moved[n.ID] = moved[n.ID] || ok
	}
	return out
}

// holdsAny reports whether d holds a track of the IDs of to.
func holdsAny(d Data, to map[string]media.Track) bool {
	in := func(ts []media.Track) bool {
		return slices.ContainsFunc(ts, func(t media.Track) bool { _, ok := to[t.ID]; return ok })
	}
	if in(d.Liked) || in(d.Disliked) {
		return true
	}
	for _, p := range d.Playlists {
		if in(p.Tracks) {
			return true
		}
	}
	return slices.ContainsFunc(d.History, func(p Play) bool { _, ok := to[p.Track.ID]; return ok })
}
