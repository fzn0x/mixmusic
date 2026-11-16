package youtube

import (
	"context"
	"errors"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
)

// ErrNoSongList is why ArtistSongs reads nothing of an artist whose page
// links no list of their songs.
var ErrNoSongList = errors.New("YouTube Music lists no more than the top songs of this artist")

// songsList is the ID of the playlist of all an artist's songs, which the
// title of the Top songs of their page and its Show all link open, or ""
// when neither opens a playlist (#543).
func songsList(shelf node) string {
	for _, end := range []node{shelf.get("title", "runs", 0, "navigationEndpoint"), shelf.get("bottomEndpoint")} {
		id := end.get("browseEndpoint", "browseId").str()
		if strings.HasPrefix(id, "VL") && pageType(end) == pageTypePlaylist {
			return media.ID(media.SourceYouTube, strings.TrimPrefix(id, "VL"))
		}
	}
	return ""
}

// ArtistSongs reads all the songs of the artist of channelID (UC…): the
// list of them that their page links (media.Artist.SongsID), as SongList
// reads it. An artist without a list fails with ErrNoSongList.
func (c *Client) ArtistSongs(ctx context.Context, channelID string, each func([]media.Track)) ([]media.Track, error) {
	var a media.Artist
	if err := media.TryPage(ctx, func(ctx context.Context) (err error) {
		a, err = c.Artist(ctx, channelID)
		return err
	}); err != nil {
		return nil, err
	}
	if a.SongsID == "" {
		return nil, ErrNoSongList
	}
	_, id, _ := media.ParseID(a.SongsID)
	return c.SongList(ctx, id, each)
}

// SongList reads the playlist of listID, the list of an artist's songs, a
// page at a time, up to media.MaxTracks, a page that fails tried again
// on media.PageWaits. each, if set, is given the new songs of each page
// in order, each song once by its ID, so that playing can start with the
// first. A page past the first that still fails returns the songs that
// came with a media.ShortList, whose Total is the count of the list's
// header, which counts songs that can't play too.
func (c *Client) SongList(ctx context.Context, listID string, each func([]media.Track)) ([]media.Track, error) {
	var p media.Playlist
	if err := media.TryPage(ctx, func(ctx context.Context) (err error) {
		p, err = c.Playlist(ctx, listID)
		return err
	}); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var songs []media.Track
	give := func(tracks []media.Track) {
		var fresh []media.Track
		for _, t := range tracks {
			if !seen[t.ID] {
				seen[t.ID] = true
				fresh = append(fresh, t)
			}
		}
		songs = append(songs, fresh...)
		if each != nil && len(fresh) > 0 {
			each(fresh)
		}
	}
	give(p.Tracks)
	n := len(p.Tracks)
	if err := p.ReadOn(ctx, c.PlaylistPage, func(now media.Playlist) {
		give(now.Tracks[n:])
		n = len(now.Tracks)
	}); err != nil {
		return songs, &media.ShortList{Got: len(songs), Total: p.TrackCount, Err: err}
	}
	return songs, nil
}
