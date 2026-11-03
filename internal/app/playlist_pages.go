package app

// Long playlists: the first page of a playlist's tracks comes at once, and
// the rest are read a page at a time, up to media.MaxTracks.

import (
	"context"
	"fmt"

	"github.com/fzn0x/mixmusic/internal/media"
)

// PlaylistStart returns a playlist with its first page of tracks, of any
// service or made in MixMusic, as soon as it comes: while it has more,
// ReadOn reads them.
func (c *Catalog) PlaylistStart(ctx context.Context, id string) (media.Playlist, error) {
	return c.a.playlist(ctx, id)
}

// ReadOn reads the tracks of p, a playlist PlaylistStart gave, past those
// it holds, from its service, as media.Playlist.ReadOn does: onPage, if
// set, is given p after each page. When a page still fails after a few
// tries, p holds the tracks read before it, with the error, and ReadOn of
// that p reads on from the page that failed.
func (c *Catalog) ReadOn(ctx context.Context, p media.Playlist, onPage func(media.Playlist)) (media.Playlist, error) {
	return c.a.readOn(ctx, p, onPage)
}

func (a *App) readOn(ctx context.Context, p media.Playlist, onPage func(media.Playlist)) (media.Playlist, error) {
	var read media.PageFunc
	switch p.Source {
	case media.SourceYouTube:
		read = a.youtube.PlaylistPage
	case media.SourceSpotify:
		read = a.spotify.PlaylistPage
	}
	if read == nil || p.More == "" {
		return p, nil
	}
	err := p.ReadOn(ctx, read, onPage)
	return p, err
}

// wholePlaylist returns a playlist with all its tracks, up to
// media.MaxTracks: a page that still fails after a few tries fails it.
func (a *App) wholePlaylist(ctx context.Context, id string) (media.Playlist, error) {
	p, err := a.playlist(ctx, id)
	if err != nil {
		return p, err
	}
	if p, err = a.readOn(ctx, p, nil); err != nil {
		return p, fmt.Errorf("only %d of the tracks of %s could be read: %w", len(p.Tracks), p.Title, err)
	}
	return p, nil
}
