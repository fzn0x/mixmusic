package app

// The notification of a track that starts shows its cover: the one the
// system's media controls show, from the artwork store or from the index
// of the music on this computer (#102).

import (
	"context"
	"log"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/artwork"
	"github.com/fzn0x/mixmusic/internal/media"
)

// noteCoverWait bounds the wait for the cover of a track's notification,
// which shows without one past it; tests shorten it.
var noteCoverWait = 2 * time.Second

// noteCoverSize is the most pixels across of a notification's cover: macOS
// shows it that large as the notification expands. MyGo shrinks it for
// Windows' balloons and Linux's notification servers.
const noteCoverSize = 512

// trackNote is the notification of t, which started: its title, its
// artists and its album, with its cover once it loaded, waiting
// noteCoverWait at most. It reports false once another track loads or
// started meanwhile, or nothing plays, so that a slow cover never shows
// over the next track's notification.
func (a *App) trackNote(t media.Track) (mygo.NotificationOptions, bool) {
	body := t.ArtistNames()
	if t.Album != nil && t.Album.Name != "" {
		body += " • " + t.Album.Name
	}
	opts := mygo.NotificationOptions{ID: "now-playing", Title: t.Title, Body: body, Silent: true, Image: a.noteCover(t)}
	a.mu.Lock()
	np := a.nowPlaying.Track
	current := a.started == t.ID && np != nil && np.ID == t.ID
	a.mu.Unlock()
	return opts, current
}

// noteCover is the cover of t's notification: the centre square of the
// cover the system's media controls show, at most noteCoverSize across,
// or nil when t has none, or it didn't load in noteCoverWait. The download
// goes on past the wait, for the controls and the windows.
func (a *App) noteCover(t media.Track) []byte {
	u := controlsArtwork(t.Artwork)
	if u == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), noteCoverWait)
	defer cancel()
	data, err := a.coverBytes(ctx, u)
	if err != nil || len(data) == 0 {
		return nil // the store logs why
	}
	square, err := artwork.Square(data, noteCoverSize)
	if err != nil {
		log.Printf("mixmusic: the cover of the notification of %s: %v", t.ID, err)
		return nil
	}
	return square
}
