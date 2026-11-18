package downloads

import (
	"cmp"
	"slices"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Listing is what the downloads hold, as they were asked for, without
// their audio: what a backup keeps, to download them again (#592).
type Listing struct {
	// Tracks are the tracks downloaded on their own, in the order they
	// were asked for.
	Tracks []media.Track
	// Collections are the albums, playlists, mixes and liked songs
	// downloaded whole, the first asked for first, each with its tracks.
	Collections []ListedCollection
}

// ListedCollection is a collection downloaded whole, with its tracks in
// its order: Collection.Tracks are their IDs.
type ListedCollection struct {
	Collection
	Items []media.Track
}

// Listing returns what the downloads hold, as they were asked for.
func (m *Manager) Listing() Listing {
	var l Listing
	if m == nil {
		return l
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.idx == nil {
		return l
	}
	own := make([]*entry, 0, len(m.idx.Tracks))
	for _, e := range m.idx.Tracks {
		if e.pinned(pinTrack) {
			own = append(own, e)
		}
	}
	slices.SortFunc(own, func(a, b *entry) int { return cmp.Compare(a.Seq, b.Seq) })
	for _, e := range own {
		l.Tracks = append(l.Tracks, e.Track)
	}
	for _, c := range m.idx.Collections {
		lc := ListedCollection{Collection: *c}
		lc.Tracks = slices.Clone(c.Tracks)
		lc.Art = ""
		for _, id := range c.Tracks {
			if e, ok := m.idx.Tracks[id]; ok {
				lc.Items = append(lc.Items, e.Track)
			}
		}
		l.Collections = append(l.Collections, lc)
	}
	slices.SortFunc(l.Collections, func(a, b ListedCollection) int {
		return cmp.Or(a.Added.Compare(b.Added), cmp.Compare(a.ID, b.ID))
	})
	return l
}
