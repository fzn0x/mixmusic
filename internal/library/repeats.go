package library

// Tracks a playlist holds already: an add leaves them out and tells which
// they were, and the user can add them again, a second time.

import (
	"slices"

	"github.com/fzn0x/mixmusic/internal/media"
)

// AddNewToPlaylist appends the tracks a playlist doesn't hold yet, as
// AddToPlaylist does, and returns those it held already, which stay out,
// as does a track given twice. A playlist that held them all stays as it
// was.
func (l *Library) AddNewToPlaylist(id string, tracks []media.Track) (held []media.Track, err error) {
	p, err := l.Playlist(id)
	if err != nil {
		return nil, err
	}
	var fresh []media.Track
	for _, t := range tracks {
		has := func(x media.Track) bool { return x.ID == t.ID }
		if slices.ContainsFunc(p.Tracks, has) || slices.ContainsFunc(fresh, has) {
			held = append(held, t)
		} else {
			fresh = append(fresh, t)
		}
	}
	if len(fresh) == 0 {
		return held, nil
	}
	return held, l.AddToPlaylist(id, fresh)
}

// AddToPlaylistAgain appends tracks to a playlist, those it holds already
// too, which it then holds twice.
func (l *Library) AddToPlaylistAgain(id string, tracks []media.Track) error {
	return l.editPlaylist(id, func(p *Playlist) { p.Tracks = append(p.Tracks, tracks...) })
}
