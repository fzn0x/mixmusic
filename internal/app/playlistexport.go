package app

// Exporting lists (#613): a mix, a playlist, an album or Liked songs, or
// every mix and Liked songs at once, to a file other players read: CSV,
// M3U8 or JSON, as internal/playlistio writes them. MixMusic reads its
// own back as they were, without a search.

import (
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/playlistio"
	"github.com/fzn0x/mixmusic/locales"
)

// ExportList is a list to export: its name and its tracks.
type ExportList struct {
	Name   string
	Tracks []media.Track
}

// Export writes lists to path, in the format of its extension: M3U8 for
// .m3u8 and .m3u, JSON for .json, else CSV.
func (i *Imports) Export(path string, lists []ExportList) error {
	out := make([]playlistio.ExportList, len(lists))
	for k, l := range lists {
		out[k].Name = l.Name
		out[k].Rows = make([]playlistio.Row, len(l.Tracks))
		for n, t := range l.Tracks {
			out[k].Rows[n] = i.a.exportRow(t)
		}
	}
	return writeLists(path, playlistio.FormatOf(path), out)
}

// Library is every list of the library, for Export the library: Liked
// songs, then the mixes, in the order the user keeps them.
func (i *Imports) Library() []ExportList {
	a := i.a
	out := []ExportList{{Name: locales.RailLikedSongs(), Tracks: a.library.Liked()}}
	for _, p := range (&Collection{a}).Playlists() {
		native, err := localID(p.ID)
		if err != nil {
			continue
		}
		if whole, err := a.library.Playlist(native); err == nil {
			out = append(out, ExportList{Name: whole.Title, Tracks: whole.Tracks})
		}
	}
	return out
}

// exportRow is a track as the files of lists write it: of a song on this
// computer, its file.
func (a *App) exportRow(t media.Track) playlistio.Row {
	r := playlistio.Row{Title: t.Title, Duration: time.Duration(t.DurationMs) * time.Millisecond, ISRC: t.ISRC,
		MixMusicID: t.ID, URL: t.URL}
	for _, ar := range t.Artists {
		r.Artists = append(r.Artists, ar.Name)
	}
	if t.Album != nil {
		r.Album = t.Album.Name
	}
	source, native, _ := media.ParseID(t.ID)
	switch source {
	case media.SourceYouTube:
		r.Service = "YouTube Music"
		if r.URL == "" || !strings.Contains(r.URL, "watch?v=") {
			r.URL = "https://music.youtube.com/watch?v=" + url.QueryEscape(native)
		}
	case media.SourceSpotify:
		r.Service = "Spotify"
		r.URL = "https://open.spotify.com/track/" + url.PathEscape(native)
	case media.SourceFile:
		r.Service = "File"
		path, ok := "", false
		if a.localFiles != nil {
			path, ok = a.localFiles.FileOf(t.ID)
		}
		if !ok {
			path, ok = localfiles.PathOf(t.ID)
		}
		if ok {
			r.Path, r.MixMusicID = path, localfiles.TrackID(path)
			r.URL = fileURL(path)
		}
	}
	return r
}

// fileURL is the file:// URL of a path.
func fileURL(path string) string {
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}
