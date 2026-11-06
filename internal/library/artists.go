package library

import (
	"slices"

	"github.com/fzn0x/mixmusic/internal/media"
)

// FollowedArtist is an artist the user follows in MixMusic, as their tile
// shows them.
type FollowedArtist struct {
	ID      string        `json:"id"`
	Source  media.Source  `json:"source"`
	Name    string        `json:"name"`
	Artwork []media.Image `json:"artwork,omitempty"`
	URL     string        `json:"url,omitempty"`
}

// media converts an artist followed, without the sections of their page.
func (a FollowedArtist) media() media.Artist {
	return media.Artist{ID: a.ID, Source: a.Source, Name: a.Name, Artwork: orEmpty(a.Artwork), URL: a.URL,
		TopTracks: []media.Track{}, Albums: []media.Album{}, Singles: []media.Album{}, Related: []media.Ref{}}
}

// Artists returns the artists the user follows in MixMusic, the last
// followed first.
func (l *Library) Artists() []media.Artist {
	followed := l.file.Get().Artists
	out := make([]media.Artist, len(followed))
	for i, a := range followed {
		out[i] = a.media()
	}
	return out
}

// FollowsArtist reports whether the user follows the artist id in
// MixMusic.
func (l *Library) FollowsArtist(id string) bool {
	return slices.ContainsFunc(l.file.Get().Artists, func(a FollowedArtist) bool { return a.ID == id })
}

// SetArtistFollowed follows an artist in MixMusic, or stops following
// them. An artist followed again moves first.
func (l *Library) SetArtistFollowed(a media.Artist, on bool) error {
	return l.file.Update(func(d *Data) error {
		d.Artists = slices.DeleteFunc(d.Artists, func(x FollowedArtist) bool { return x.ID == a.ID })
		if on {
			d.Artists = slices.Insert(d.Artists, 0, FollowedArtist{ID: a.ID, Source: a.Source, Name: a.Name, Artwork: a.Artwork, URL: a.URL})
		}
		return nil
	})
}
