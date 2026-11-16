package library

// Tracks that became others (#340): files on this computer linked as
// copies of one song take the song's ID, which the liked and disliked
// songs, the mixes and the history keep from then on, so that they follow
// the song rather than one of its files, and stay when a copy goes.

import (
	"errors"
	"slices"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Retarget gives what the library keeps of the tracks of the IDs of to
// the track each became: in the liked and disliked songs, the mixes, the
// history and the play log, which keep when each played. A track that
// becomes one the list holds already stays where that one is: a song
// liked as two of its files is liked once. One liked and disliked stays
// liked. Nothing is written when the library holds none of them.
func (l *Library) Retarget(to map[string]media.Track) error {
	logged := l.plays.retarget(to)
	if len(to) == 0 || !holdsAny(l.file.Get(), to) {
		return logged
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
	return errors.Join(err, logged)
}

// retarget gives the plays of the tracks of the IDs of to the track each
// became, each when it played, and writes the log anew. Nothing is written
// when the log holds none of them; while the file can't be written, the
// next play writes it anew.
func (g *playLog) retarget(to map[string]media.Track) error {
	g.mu.Lock()
	if !slices.ContainsFunc(g.tracks, func(t loggedTrack) bool { _, ok := to[t.ID]; return ok }) {
		g.mu.Unlock()
		return nil
	}
	tracks := slices.Clone(g.tracks)
	byKey := make(map[string]int32, len(tracks))
	for i, t := range tracks {
		if n, ok := to[t.ID]; ok {
			tracks[i] = lineOf(time.Time{}, 0, loggedTrack{Track: n}).track()
		}
		byKey[lineOf(time.Time{}, 0, tracks[i]).key()] = int32(i)
	}
	g.tracks, g.byKey = tracks, byKey
	g.changes++
	g.rewrites++
	v, done := g.write()
	defer done()
	g.whole = true
	return g.flush(v)
}

// rewritten counts the changes to the plays that don't come with a change
// to the history, for the library's version.
func (g *playLog) rewritten() uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.rewrites
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
