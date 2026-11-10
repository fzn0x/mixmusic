package media

import (
	"slices"
	"strings"
)

// Explicit content, which Hide explicit content keeps out of the lists the
// services make (#427): the songs, videos and albums their service marks
// explicit. A podcast's episode isn't a song: the setting leaves it alone.

// ExplicitSong reports whether t is a song or a video its service, or its
// file's tags, mark explicit, rather than a podcast's episode.
func (t Track) ExplicitSong() bool { return t.Explicit && t.Episode == nil }

// ExplicitAlbum reports whether a is an album its service marks explicit,
// by a badge or by its tracks, or whose title does, as "Kites (Explicit)".
func (a Album) ExplicitAlbum() bool {
	if a.Explicit {
		return true
	}
	// Only a title that names it is read for its marks.
	return (strings.Contains(a.Title, "xplicit") || strings.Contains(a.Title, "XPLICIT")) && VersionOf(a.Title).Explicit
}

// WithoutExplicit returns tracks without their explicit songs and videos:
// tracks itself when it holds none.
func WithoutExplicit(tracks []Track) []Track { return without(tracks, Track.ExplicitSong) }

// WithoutExplicitAlbums returns albums without their explicit ones: albums
// itself when it holds none.
func WithoutExplicitAlbums(albums []Album) []Album { return without(albums, Album.ExplicitAlbum) }

// WithoutExplicitItems returns the items of a shelf, or of suggestions,
// without their explicit songs, videos and albums: items itself when it
// holds none.
func WithoutExplicitItems(items []Item) []Item {
	return without(items, func(it Item) bool {
		return it.Track != nil && it.Track.ExplicitSong() || it.Album != nil && it.Album.ExplicitAlbum()
	})
}

// WithoutExplicit returns what a search found without its explicit songs,
// videos and albums.
func (r SearchResults) WithoutExplicit() SearchResults {
	r.Tracks = WithoutExplicit(r.Tracks)
	r.Videos = WithoutExplicit(r.Videos)
	r.Albums = WithoutExplicitAlbums(r.Albums)
	return r
}

// WithoutExplicit returns an artist's page without its explicit songs,
// albums and singles.
func (a Artist) WithoutExplicit() Artist {
	a.TopTracks = WithoutExplicit(a.TopTracks)
	a.Albums = WithoutExplicitAlbums(a.Albums)
	a.Singles = WithoutExplicitAlbums(a.Singles)
	return a
}

// ExplicitVersions tells which of versions, GroupVersions' group, are
// explicit: those their service, their titles or their tracks mark so,
// and, of an edition of which a version is marked clean and none
// explicit, those not marked.
func ExplicitVersions(versions []Album) []bool {
	_, explicit := versionRatings(versions)
	return explicit
}

// without returns list without the values drop reports, in their order:
// list itself when it drops none, else a list of its own.
func without[T any](list []T, drop func(T) bool) []T {
	i := slices.IndexFunc(list, drop)
	if i < 0 {
		return list
	}
	out := make([]T, i, len(list)-1)
	copy(out, list[:i])
	for _, v := range list[i+1:] {
		if !drop(v) {
			out = append(out, v)
		}
	}
	return out
}
