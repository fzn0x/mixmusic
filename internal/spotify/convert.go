package spotify

import (
	"slices"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
)

// The objects of the Web API, as far as MixMusic reads them.

type image struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type externalURLs struct {
	Spotify string `json:"spotify"`
}

type artist struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Images       []image      `json:"images"`
	Genres       []string     `json:"genres"`
	ExternalURLs externalURLs `json:"external_urls"`
}

type album struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	AlbumType    string          `json:"album_type"`
	ReleaseDate  string          `json:"release_date"`
	TotalTracks  int             `json:"total_tracks"`
	Images       []image         `json:"images"`
	Artists      []artist        `json:"artists"`
	ExternalURLs externalURLs    `json:"external_urls"`
	Tracks       *paging[*track] `json:"tracks"`
}

type track struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Type         string       `json:"type"`
	DurationMs   int          `json:"duration_ms"`
	Explicit     bool         `json:"explicit"`
	IsLocal      bool         `json:"is_local"`
	Artists      []artist     `json:"artists"`
	Album        *album       `json:"album"`
	ExternalURLs externalURLs `json:"external_urls"`
	// ExternalIDs are of a whole track, as a playlist's or a search's;
	// an album's page leaves them out.
	ExternalIDs struct {
		ISRC string `json:"isrc"`
	} `json:"external_ids"`
}

type owner struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// playlistItem is an entry of a playlist. The API named its track "track"
// before February 2026 and "item" since.
type playlistItem struct {
	Item  *track `json:"item"`
	Track *track `json:"track"`
}

func (p playlistItem) track() *track {
	if p.Item != nil {
		return p.Item
	}
	return p.Track
}

// playlist is a playlist. The API named its entries "tracks" before
// February 2026 and "items" since; lists of playlists only give their
// count.
type playlist struct {
	ID            string                `json:"id"`
	Name          string                `json:"name"`
	Description   string                `json:"description"`
	Images        []image               `json:"images"`
	Owner         owner                 `json:"owner"`
	Collaborative bool                  `json:"collaborative"`
	Public        *bool                 `json:"public"`
	ExternalURLs  externalURLs          `json:"external_urls"`
	Items         *paging[playlistItem] `json:"items"`
	Tracks        *paging[playlistItem] `json:"tracks"`
}

func (p playlist) entries() *paging[playlistItem] {
	if p.Items != nil {
		return p.Items
	}
	return p.Tracks
}

type paging[T any] struct {
	Items []T    `json:"items"`
	Next  string `json:"next"`
	Total int    `json:"total"`
	// Cursors pages lists of followed artists.
	Cursors struct {
		After string `json:"after"`
	} `json:"cursors"`
}

// images returns the images of the API, smallest first.
func images(in []image) []media.Image {
	out := make([]media.Image, 0, len(in))
	for _, i := range in {
		out = append(out, media.Image{URL: i.URL, Width: i.Width, Height: i.Height})
	}
	slices.SortStableFunc(out, func(a, b media.Image) int { return a.Width - b.Width })
	return out
}

func refs(artists []artist) []media.Ref {
	out := make([]media.Ref, 0, len(artists))
	for _, a := range artists {
		if a.Name == "" {
			continue
		}
		r := media.Ref{Name: a.Name}
		if a.ID != "" {
			r.ID = media.ID(media.SourceSpotify, a.ID)
		}
		out = append(out, r)
	}
	return out
}

// toTrack converts a track; on albums' track lists, which leave the album
// out of each track, from is the album.
func toTrack(t *track, from *album) (media.Track, bool) {
	if t == nil || t.ID == "" || t.IsLocal || (t.Type != "" && t.Type != "track") {
		return media.Track{}, false
	}
	al := t.Album
	if al == nil {
		al = from
	}
	mt := media.Track{
		ID:         media.ID(media.SourceSpotify, t.ID),
		Source:     media.SourceSpotify,
		Title:      t.Name,
		Artists:    refs(t.Artists),
		DurationMs: t.DurationMs,
		Explicit:   t.Explicit,
		Artwork:    []media.Image{},
		URL:        t.ExternalURLs.Spotify,
		ISRC:       t.ExternalIDs.ISRC,
	}
	if al != nil {
		mt.Album = &media.Ref{ID: media.ID(media.SourceSpotify, al.ID), Name: al.Name}
		mt.Artwork = images(al.Images)
	}
	return mt, true
}

func toTracks(in []*track, from *album) []media.Track {
	out := []media.Track{}
	for _, t := range in {
		if mt, ok := toTrack(t, from); ok {
			out = append(out, mt)
		}
	}
	return out
}

func toAlbum(a *album) media.Album {
	kind := media.AlbumKind(a.AlbumType)
	if kind == "compilation" {
		kind = media.AlbumKindAlbum
	}
	year, _, _ := strings.Cut(a.ReleaseDate, "-")
	out := media.Album{
		ID:         media.ID(media.SourceSpotify, a.ID),
		Source:     media.SourceSpotify,
		Title:      a.Name,
		Artists:    refs(a.Artists),
		Kind:       kind,
		Year:       year,
		Artwork:    images(a.Images),
		TrackCount: a.TotalTracks,
		Tracks:     []media.Track{},
		URL:        a.ExternalURLs.Spotify,
	}
	if a.Tracks != nil {
		out.Tracks = toTracks(a.Tracks.Items, a)
	}
	return out
}

func toArtist(a *artist) media.Artist {
	return media.Artist{
		ID:        media.ID(media.SourceSpotify, a.ID),
		Source:    media.SourceSpotify,
		Name:      a.Name,
		Artwork:   images(a.Images),
		Subtitle:  strings.Join(a.Genres, ", "),
		TopTracks: []media.Track{},
		Albums:    []media.Album{},
		Singles:   []media.Album{},
		Related:   []media.Ref{},
		URL:       a.ExternalURLs.Spotify,
	}
}

// toPlaylist converts a playlist; userID is the signed-in user, who may
// edit the playlists they own.
func toPlaylist(p *playlist, userID string) media.Playlist {
	out := media.Playlist{
		ID:          media.ID(media.SourceSpotify, p.ID),
		Source:      media.SourceSpotify,
		Title:       p.Name,
		Description: p.Description,
		Owner:       p.Owner.DisplayName,
		Artwork:     images(p.Images),
		Tracks:      []media.Track{},
		Editable:    userID != "" && (p.Owner.ID == userID || p.Collaborative),
		URL:         p.ExternalURLs.Spotify,
	}
	out.Mine, out.Collaborative = userID != "" && p.Owner.ID == userID, p.Collaborative
	if p.Public != nil {
		out.Privacy = media.Private
		if *p.Public {
			out.Privacy = media.Public
		}
	}
	if e := p.entries(); e != nil {
		out.TrackCount = e.Total
		out.Tracks = playlistTracks(e.Items)
	}
	return out
}

func playlistTracks(items []playlistItem) []media.Track {
	out := []media.Track{}
	for _, it := range items {
		if mt, ok := toTrack(it.track(), nil); ok {
			out = append(out, mt)
		}
	}
	return out
}
