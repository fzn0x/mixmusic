package main

import (
	"context"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// wholeListTime is how long a list of the user's library on YouTube Music
// may take to come: it's read a page of about two dozen at a time, up to
// 1,000 pages, each tried again a few times, which fetch's minute doesn't
// give a large library.
const wholeListTime = 20 * time.Minute

// wholeList fetches for key a list of the user's library on YouTube Music
// with get, which reads it page after page, giving get wholeListTime
// rather than fetch's minute.
func wholeList[T any](s *shell, key string, get func(ctx context.Context) (T, error)) *loaded[T] {
	return fetch(s, key, func(ctx context.Context) (T, error) {
		// fetch's context ends at its minute, and only then.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), wholeListTime)
		defer cancel()
		return get(ctx)
	})
}

// youtubeAlbums are the albums the user saved on YouTube Music.
func (s *shell) youtubeAlbums() *loaded[[]media.Album] {
	return wholeList(s, "youtube:albums", s.svc.Collection.YouTubeAlbums)
}

// youtubeArtists are the artists the user subscribes to on YouTube Music.
func (s *shell) youtubeArtists() *loaded[[]media.Artist] {
	return wholeList(s, "youtube:artists", s.svc.Collection.YouTubeArtists)
}
