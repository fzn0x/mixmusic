package main

import (
	"context"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// wholeListTime is how long a list of the user's library on YouTube Music
// may take to come: it's read a page of about two dozen at a time, up to
// 1,000 pages, each tried again a few times, which fetch's minute doesn't
// give a large library.
const wholeListTime = 20 * time.Minute

// wholeList fetches for key a list of the user's library on YouTube Music
// with get, which reads it page after page, giving get wholeListTime
// rather than fetch's minute. What get has read comes to the page as it
// goes (loaded.shown); what needs the whole list waits for done.
func wholeList[T any](s *shell, key string, get func(ctx context.Context) (T, error)) *loaded[T] {
	return fetchAs(s, key, func(ctx context.Context) (T, error) {
		// fetch's context ends at its minute, and only then. Its values
		// stay, as the one that hands over what was read so far.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), wholeListTime)
		defer cancel()
		return get(ctx)
	}, true)
}

// waiting reports whether a list has nothing to show yet: its first page
// hasn't come.
func (l *loaded[T]) waiting() bool { return !l.done && !l.coming }

// growing reports whether a list shows what it has read so far, and reads
// on.
func (l *loaded[T]) growing() bool { return !l.done && l.coming }

// shown is what a page of the library shows of a list: all of it once it
// came, and what was read so far while it comes.
func (l *loaded[T]) shown() T {
	if l.growing() {
		return l.part
	}
	return l.value
}

// moreComing is the foot of a list of the library that is still being
// read: Loading more…, under key.
func (s *shell) moreComing(c *ui.Context, key string) {
	ui.Row(c.Key(key+"-coming")).PaddingY(12).Margin(0, 0, 16).Children(func() {
		ui.Spinner(c).Label(locales.LibraryLoadingMore())
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
