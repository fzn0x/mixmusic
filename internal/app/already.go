package app

// Tracks a playlist holds already: YouTube Music refuses to add one again,
// and a mix leaves them out. Either adds them anyway, a second time, when
// the user asks.

import (
	"context"
	"errors"

	"github.com/fzn0x/mixmusic/internal/media"
)

// ErrAlreadyInPlaylist is an add of tracks a playlist or a mix holds
// already, which it left out: AddToYouTubePlaylistAnyway and
// AddToMixAnyway add them a second time.
var ErrAlreadyInPlaylist = errors.New("that playlist has it already")

// AddToYouTubePlaylistAnyway adds a track of any service to a playlist of
// the user on YouTube Music even when it holds the track already, a second
// time, as YouTube Music's Add anyway does.
func (c *Collection) AddToYouTubePlaylistAnyway(ctx context.Context, playlistID string, track media.Track) error {
	return c.addToYouTube(ctx, playlistID, track, c.a.youtube.AddToPlaylistAnyway)
}

// AddToMix appends tracks to a mix, leaving out those it holds already,
// which it returns: when it held them all, nothing changes, and the error
// is ErrAlreadyInPlaylist.
func (c *Collection) AddToMix(id string, tracks []media.Track) (held []media.Track, err error) {
	native, err := localID(id)
	if err != nil {
		return nil, err
	}
	held, err = c.a.library.AddNewToPlaylist(native, c.a.asSongs(tracks))
	if err == nil && len(tracks) > 0 && len(held) == len(tracks) {
		err = ErrAlreadyInPlaylist
	}
	return held, err
}

// AddToMixAnyway appends tracks to a mix, those it holds already too, which
// it then holds twice.
func (c *Collection) AddToMixAnyway(id string, tracks []media.Track) error {
	native, err := localID(id)
	if err != nil {
		return err
	}
	return c.a.library.AddToPlaylistAgain(native, c.a.asSongs(tracks))
}
