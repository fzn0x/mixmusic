package library

import (
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
)

// The cover a user chose for a playlist is an image file the app keeps in
// a folder of its own, beside the library. The playlist holds its name,
// and its artwork is the file's address, which the windows show from the
// file, and nothing fetches.

// coverScheme starts the addresses of the covers chosen for playlists.
const coverScheme = "mixcover:"

// CoverImage is the artwork of a playlist whose cover is the file name.
func CoverImage(name string) media.Image { return media.Image{URL: coverScheme + name} }

// CoverName returns the name of the file of a playlist's cover by its
// address, unless it isn't one, or names a file outside the folder.
func CoverName(url string) (string, bool) {
	name, ok := strings.CutPrefix(url, coverScheme)
	if !ok || name == "" || strings.ContainsAny(name, `/\:`) || strings.Contains(name, "..") {
		return "", false
	}
	return name, true
}

// PlaylistCover returns the name of the file of the cover chosen for a
// playlist, or "" while it shows its tracks' covers.
func (l *Library) PlaylistCover(id string) (string, error) {
	for _, p := range l.file.Get().Playlists {
		if p.ID == id {
			return p.Cover, nil
		}
	}
	return "", ErrNoPlaylist
}

// SetPlaylistCover gives a playlist the cover kept in the file name, or
// its tracks' covers again for "", and returns the name of the file it
// had, for the caller to remove.
func (l *Library) SetPlaylistCover(id, name string) (old string, err error) {
	err = l.editPlaylist(id, func(p *Playlist) { old, p.Cover = p.Cover, name })
	return old, err
}
