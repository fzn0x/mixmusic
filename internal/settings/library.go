package settings

import "slices"

// The sorts of the mixes, as MixesSort names them: the user's own order,
// the last changed or made first, by title, and by how many tracks they
// hold.
const (
	MixesByOrder   = "order"
	MixesByChanged = "changed"
	MixesByAdded   = "added"
	MixesByTitle   = "title"
	MixesByTracks  = "tracks"
)

// MixesSorts are the sorts of the mixes, in the order the Library offers
// them.
var MixesSorts = []string{MixesByOrder, MixesByChanged, MixesByAdded, MixesByTitle, MixesByTracks}

// The sorts of the playlists of a service, as PlaylistsSort names them:
// the service's own order, by title, and by how many tracks they hold.
const (
	PlaylistsByService = "service"
	PlaylistsByTitle   = "title"
	PlaylistsByTracks  = "tracks"
)

// PlaylistsSorts are the sorts of a service's playlists, in the order the
// Library offers them.
var PlaylistsSorts = []string{PlaylistsByService, PlaylistsByTitle, PlaylistsByTracks}

// The places of the sidebar's Your mixes that aren't mixes, by the keys
// SidebarPlaces names them by.
const (
	PlaceLiked     = "liked"
	PlaceDownloads = "downloads"
)

// Library is how the library lists what it holds: the user's mixes, the
// playlists of the services, its tabs and the sidebar's Your mixes.
type Library struct {
	// MixesSort is how the mixes list, everywhere they do: one of
	// MixesSorts. The pinned ones lead each.
	MixesSort string `json:"mixesSort"`
	// MixesDescending lists the mixes the other way round of what
	// MixesSort says first: the oldest first of the times, Z to A of the
	// titles, the most tracks first of the counts. Their own order has
	// no other way.
	MixesDescending bool `json:"mixesDescending"`
	// PlaylistsSort is how Library › Spotify and Library › YouTube Music
	// list their playlists: one of PlaylistsSorts.
	PlaylistsSort string `json:"playlistsSort"`
	// PlaylistsDescending lists them the other way round: Z to A, or the
	// most tracks first.
	PlaylistsDescending bool `json:"playlistsDescending"`
	// LibraryTabs are the Library's tabs as the user put them in order and
	// hid them, by their keys, as "liked"; none leaves them as they come.
	// The tabs it doesn't name show after those it does, in their own
	// order.
	LibraryTabs []Arranged `json:"libraryTabs"`
	// SidebarPlaces are where Liked songs and Downloads show among the
	// pins and mixes of the sidebar's Your mixes, and whether they do;
	// none leaves them first.
	SidebarPlaces []SidebarPlace `json:"sidebarPlaces"`
}

// SidebarPlace is where an entry of the sidebar's Your mixes that isn't a
// mix shows: Liked songs, or Downloads.
type SidebarPlace struct {
	// Key names the entry: PlaceLiked or PlaceDownloads, or one of a
	// later version, which this one keeps.
	Key string `json:"key"`
	// After is the key of the entry it follows: another place's, or
	// "pin:" or "mix:" and the ID of a pin or a mix. None puts it first,
	// as does an entry that's gone.
	After string `json:"after,omitempty"`
	// Hidden keeps it out of the sidebar, until the user brings it back.
	Hidden bool `json:"hidden,omitempty"`
}

// These bound what LibraryTabs and SidebarPlaces hold.
const (
	maxLibraryTabs    = 64
	maxSidebarPlaces  = 32
	maxSidebarPlaceID = 512
)

// defaultLibrary is the library of a new install: the mixes in the
// user's own order, the playlists in their services', the tabs as they
// come.
func defaultLibrary() Library {
	return Library{MixesSort: MixesByOrder, PlaylistsSort: PlaylistsByService, LibraryTabs: []Arranged{}, SidebarPlaces: []SidebarPlace{}}
}

// clean is l with the sorts of d in place of those unknown, and the tabs
// and places each named once, by a key of their own, without those past
// their bounds.
func (l Library) clean(d Library) Library {
	if !slices.Contains(MixesSorts, l.MixesSort) {
		l.MixesSort, l.MixesDescending = d.MixesSort, d.MixesDescending
	}
	if !slices.Contains(PlaylistsSorts, l.PlaylistsSort) {
		l.PlaylistsSort, l.PlaylistsDescending = d.PlaylistsSort, d.PlaylistsDescending
	}
	l.LibraryTabs = cleanArranged(l.LibraryTabs, maxLibraryTabs)
	places := make([]SidebarPlace, 0, len(l.SidebarPlaces))
	for _, p := range l.SidebarPlaces {
		if p.Key == "" || len(p.Key) > maxSidebarPlaceID || len(p.After) > maxSidebarPlaceID || len(places) == maxSidebarPlaces ||
			slices.ContainsFunc(places, func(o SidebarPlace) bool { return o.Key == p.Key }) {
			continue
		}
		places = append(places, p)
	}
	l.SidebarPlaces = places
	return l
}
