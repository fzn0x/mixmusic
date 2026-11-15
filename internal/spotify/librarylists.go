package spotify

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"sync"

	"github.com/fzn0x/mixmusic/internal/media"
)

// maxListOffset is the largest offset of a page that Spotify's lists of
// the library take, 100,000 in its reference: MixMusic reads none past
// it. Tests lower it.
var maxListOffset = 100_000

// listPagesAtOnce is how many pages of a list of the library are read at
// once. do waits out a 429 once, for 5 s at most: more pages at once would
// meet Spotify's rate limit sooner.
const listPagesAtOnce = 3

// errListEnded is why a list stopped at a page of nothing, or of nothing
// new, that named a page after it: read on, it might be read forever.
var errListEnded = errors.New("spotify: a page of the list held nothing new")

// errListBound is why a list stopped at the most of it Spotify gives.
var errListBound = errors.New("spotify gives apps no more of a list")

// listPage reads the page of a list of the user's library, path's, that
// starts at offset.
func listPage[T any](ctx context.Context, c *Client, path string, offset int) (paging[T], error) {
	var page paging[T]
	q := url.Values{"limit": {strconv.Itoa(pageLimit)}, "offset": {strconv.Itoa(offset)}}
	err := c.get(ctx, path, q, &page)
	return page, err
}

// listReading is a list of the library as its pages come: the items that
// have an ID, each once.
type listReading[T any] struct {
	id    func(T) string
	items []T
	seen  map[string]bool
}

// take adds the items of page that weren't on the pages before, and
// reports whether the list goes on past it: while a page names a next,
// but for a page of nothing, or of nothing but what came before, which
// stops it with errListEnded. Items without an ID, as null ones, are
// left out.
func (l *listReading[T]) take(page paging[T]) (bool, error) {
	known, fresh := 0, 0
	for _, it := range page.Items {
		switch id := l.id(it); {
		case id == "":
		case l.seen[id]:
			known++
		default:
			l.seen[id] = true
			l.items = append(l.items, it)
			fresh++
		}
	}
	switch {
	case page.Next == "":
		return false, nil
	case len(page.Items) == 0, known > 0 && fresh == 0:
		return false, errListEnded
	}
	return true, nil
}

// readList reads the whole of a list of the user's library, path's, that
// pages by offset: its first page, then the pages its count tells of,
// listPagesAtOnce at a time, each tried again on media.PageWaits, then
// one at a time while the last names a next, up to maxListOffset. id
// tells an item's ID, by which each comes once, "" for one to leave out.
// A list that stopped short comes in part, with a media.ShortList; one
// whose first page failed, as signed out, comes with the error alone.
func readList[T any](ctx context.Context, c *Client, path string, id func(T) string) ([]T, error) {
	first, err := listPage[T](ctx, c, path, 0)
	if err != nil {
		return nil, err
	}
	l := &listReading[T]{id: id, seen: map[string]bool{}}
	more, end := l.take(first)
	for offset := pageLimit; more; {
		if offset > maxListOffset {
			end = errListBound
			break
		}
		// The pages the count tells of, a few at once, and past it, as of
		// a list that grew meanwhile, one at a time.
		n := min(max((first.Total-offset+pageLimit-1)/pageLimit, 1), listPagesAtOnce, (maxListOffset-offset)/pageLimit+1)
		pages, errs := make([]paging[T], n), make([]error, n)
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() {
				errs[i] = media.TryPage(ctx, func(ctx context.Context) error {
					var err error
					pages[i], err = listPage[T](ctx, c, path, offset+i*pageLimit)
					return err
				})
			})
		}
		wg.Wait()
		for i := 0; i < n && more; i++ {
			if errs[i] != nil {
				more, end = false, errs[i]
				break
			}
			more, end = l.take(pages[i])
		}
		offset += n * pageLimit
	}
	return l.items, shortList(len(l.items), first.Total, end)
}

// shortList is the error of a list of the library that came in part, got
// of its total items, as end, why the reading stopped, tells: none for
// one read to its end, or of which all its count tells of came before a
// page that ended it early.
func shortList(got, total int, end error) error {
	if end == nil || end == errListEnded && total > 0 && got >= total {
		return nil
	}
	return &media.ShortList{Got: got, Total: total, Cut: end == errListBound, Err: end}
}

// came tells whether a list of the library came, whole or in part, by the
// error of its reading: none, or a media.ShortList.
func came(err error) bool {
	_, short := media.Short(err)
	return err == nil || short
}

// homePlaylists returns the first page of the playlists the user made or
// follows, which Home shows: it doesn't wait for the rest.
func (c *Client) homePlaylists(ctx context.Context, userID string) ([]media.Playlist, error) {
	page, err := listPage[*playlist](ctx, c, "/me/playlists", 0)
	if err != nil {
		return nil, err
	}
	out := []media.Playlist{}
	for _, p := range page.Items {
		if p != nil {
			out = append(out, toPlaylist(p, userID))
		}
	}
	return out, nil
}

// savedAlbum is an item of the albums the user saved.
type savedAlbum struct {
	Album *album `json:"album"`
}

// savedAlbums are the albums of items, without their tracks.
func savedAlbums(items []savedAlbum) []media.Album {
	out := []media.Album{}
	for _, it := range items {
		if it.Album != nil {
			al := toAlbum(it.Album)
			al.Tracks = []media.Track{}
			out = append(out, al)
		}
	}
	return out
}

// homeAlbums returns the first page of the albums the user saved, which
// Home shows.
func (c *Client) homeAlbums(ctx context.Context) ([]media.Album, error) {
	page, err := listPage[savedAlbum](ctx, c, "/me/albums", 0)
	if err != nil {
		return nil, err
	}
	return savedAlbums(page.Items), nil
}
