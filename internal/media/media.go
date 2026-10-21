// Package media defines the catalog that every music source shares:
// tracks, albums, artists and playlists, whichever service they come from.
//
// Every item has an ID made of a source prefix and the service's own ID,
// such as "yt:w3XzgjlKFjc" or "sp:4uLU6hMCjMI75M1A2tKUQC", so items of
// different services can sit side by side in one queue or playlist.
package media

import (
	"errors"
	"strings"
)

// Source is a service that items come from.
type Source string

const (
	// SourceYouTube is YouTube Music.
	SourceYouTube Source = "youtube"
	// SourceSpotify is Spotify.
	SourceSpotify Source = "spotify"
	// SourceLocal is MixMusic itself: playlists made in the app.
	SourceLocal Source = "local"
)

// prefixes maps sources to the prefix of their IDs.
var prefixes = map[Source]string{
	SourceYouTube: "yt",
	SourceSpotify: "sp",
	SourceLocal:   "mx",
}

// ErrBadID is returned for IDs that name no source.
var ErrBadID = errors.New("media: malformed id")

// ID returns the ID of an item of source whose own ID is native.
func ID(source Source, native string) string {
	return prefixes[source] + ":" + native
}

// ParseID splits an ID into its source and the service's own ID.
func ParseID(id string) (Source, string, error) {
	prefix, native, ok := strings.Cut(id, ":")
	if !ok || native == "" {
		return "", "", ErrBadID
	}
	for source, p := range prefixes {
		if p == prefix {
			return source, native, nil
		}
	}
	return "", "", ErrBadID
}

// Image is an artwork at one size.
type Image struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// Ref links to an artist or an album by name, and by ID when it has one.
type Ref struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
}

// Track is a song or a music video.
type Track struct {
	ID         string  `json:"id"`
	Source     Source  `json:"source"`
	Title      string  `json:"title"`
	Artists    []Ref   `json:"artists"`
	Album      *Ref    `json:"album,omitempty"`
	DurationMs int     `json:"durationMs"`
	Artwork    []Image `json:"artwork"`
	Explicit   bool    `json:"explicit,omitempty"`
	// Video is set for music videos, rather than audio tracks.
	Video bool `json:"video,omitempty"`
	// Plays is how many times the track played, as its service counts
	// them: a song's plays on YouTube Music, a video's views. 0 is unknown,
	// as for Spotify, which gives apps no such count.
	Plays int64 `json:"plays,omitempty"`
	// URL opens the track on the service's website.
	URL string `json:"url,omitempty"`
	// EntryID names the track's place in the playlist it was read from,
	// which taking it out of the playlist names; only YouTube Music sets it.
	EntryID string `json:"entryId,omitempty"`
}

// ArtistNames returns the names of the track's artists, joined by commas.
func (t Track) ArtistNames() string {
	names := make([]string, len(t.Artists))
	for i, a := range t.Artists {
		names[i] = a.Name
	}
	return strings.Join(names, ", ")
}

// AlbumKind tells albums apart from singles and EPs.
type AlbumKind string

const (
	AlbumKindAlbum  AlbumKind = "album"
	AlbumKindSingle AlbumKind = "single"
	AlbumKindEP     AlbumKind = "ep"
)

// Album is a release of an artist. Lists of albums leave Tracks empty.
type Album struct {
	ID         string    `json:"id"`
	Source     Source    `json:"source"`
	Title      string    `json:"title"`
	Artists    []Ref     `json:"artists"`
	Kind       AlbumKind `json:"kind,omitempty"`
	Year       string    `json:"year,omitempty"`
	Artwork    []Image   `json:"artwork"`
	TrackCount int       `json:"trackCount,omitempty"`
	Tracks     []Track   `json:"tracks"`
	URL        string    `json:"url,omitempty"`
	// PlaylistID is the playlist of an album's tracks on YouTube Music,
	// which saving the album saves.
	PlaylistID string `json:"playlistId,omitempty"`
	// Saved is set when the album is in the user's library of its
	// service; only an album page says so.
	Saved bool `json:"saved,omitempty"`
}

// Artist is a performer. Lists of artists leave the sections empty.
type Artist struct {
	ID          string  `json:"id"`
	Source      Source  `json:"source"`
	Name        string  `json:"name"`
	Artwork     []Image `json:"artwork"`
	Subtitle    string  `json:"subtitle,omitempty"`
	Description string  `json:"description,omitempty"`
	TopTracks   []Track `json:"topTracks"`
	Albums      []Album `json:"albums"`
	Singles     []Album `json:"singles"`
	Related     []Ref   `json:"related"`
	URL         string  `json:"url,omitempty"`
	// Following is set when the user follows the artist on its service;
	// only an artist page says so.
	Following bool `json:"following,omitempty"`
}

// Playlist is a list of tracks curated by someone. Lists of playlists
// leave Tracks empty.
type Playlist struct {
	ID          string  `json:"id"`
	Source      Source  `json:"source"`
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	Owner       string  `json:"owner,omitempty"`
	Artwork     []Image `json:"artwork"`
	TrackCount  int     `json:"trackCount,omitempty"`
	Tracks      []Track `json:"tracks"`
	// Editable is set for playlists the user may change.
	Editable bool   `json:"editable,omitempty"`
	URL      string `json:"url,omitempty"`
	// Saved is set when the user saved or follows the playlist on its
	// service; only a playlist page says so.
	Saved bool `json:"saved,omitempty"`
}

// ItemKind is the kind of an Item.
type ItemKind string

const (
	ItemTrack    ItemKind = "track"
	ItemAlbum    ItemKind = "album"
	ItemArtist   ItemKind = "artist"
	ItemPlaylist ItemKind = "playlist"
)

// Item is one entry of a shelf: a track, an album, an artist or a
// playlist, as Kind says.
type Item struct {
	Kind     ItemKind  `json:"kind"`
	Track    *Track    `json:"track,omitempty"`
	Album    *Album    `json:"album,omitempty"`
	Artist   *Artist   `json:"artist,omitempty"`
	Playlist *Playlist `json:"playlist,omitempty"`
}

// Shelf is a titled row of items, as the home feed shows them.
type Shelf struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
	Source   Source `json:"source"`
	Items    []Item `json:"items"`
}

// SearchResults holds what a search found, by kind.
type SearchResults struct {
	Tracks    []Track    `json:"tracks"`
	Albums    []Album    `json:"albums"`
	Artists   []Artist   `json:"artists"`
	Playlists []Playlist `json:"playlists"`
}
