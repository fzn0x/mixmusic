package spotify

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/fzn0x/mixmusic/internal/media"
)

// releasesAtOnce is how many releases of an artist ArtistSongs reads at
// once, one request each. do waits out a 429 once, for 5 s at most: more
// at once would meet Spotify's rate limit sooner.
const releasesAtOnce = 3

// artistReleases reads every page of an artist's albums and singles, in
// their order: the first, then the pages its count tells of, a few at a
// time, each tried again on media.PageWaits, up to maxListOffset. A page
// that fails after its tries leaves its releases out: missed counts them,
// as the page counts them, and err is the first error. The first page
// failing is an error alone.
func (c *Client) artistReleases(ctx context.Context, id string) (releases []*album, missed int, err error) {
	var first []*album
	var total int
	if err := media.TryPage(ctx, func(ctx context.Context) (err error) {
		first, total, err = c.artistAlbums(ctx, id, 0)
		return err
	}); err != nil {
		return nil, 0, err
	}
	pages := [][]*album{first}
	for offset := albumPageLimit; offset < total && offset <= maxListOffset; offset += albumPageLimit {
		pages = append(pages, nil)
	}
	errs := make([]error, len(pages))
	slots := make(chan struct{}, listPagesAtOnce)
	var wg sync.WaitGroup
	for i := 1; i < len(pages); i++ {
		slots <- struct{}{}
		wg.Go(func() {
			defer func() { <-slots }()
			errs[i] = media.TryPage(ctx, func(ctx context.Context) error {
				var err error
				pages[i], _, err = c.artistAlbums(ctx, id, i*albumPageLimit)
				return err
			})
		})
	}
	wg.Wait()
	for i, page := range pages {
		if errs[i] != nil {
			missed += min(albumPageLimit, total-i*albumPageLimit)
			if err == nil {
				err = errs[i]
			}
			continue
		}
		for _, al := range page {
			if al != nil && al.ID != "" {
				releases = append(releases, al)
			}
		}
	}
	return releases, missed, err
}

// songKey tells a song of one release from the same on another, as a
// single and its album: an album's tracks come without their ISRC, so it
// is the title, the artists and the length in seconds.
func songKey(t media.Track) string {
	ids := make([]string, len(t.Artists))
	for i, a := range t.Artists {
		ids[i] = a.ID
	}
	slices.Sort(ids)
	return strings.ToLower(strings.TrimSpace(t.Title)) + "\x00" + strings.Join(ids, ",") + "\x00" + strconv.Itoa((t.DurationMs+500)/1000)
}

// ArtistSongs reads the songs of all the albums and singles of the artist
// with Spotify's id, which Spotify gives apps no other way of: every page
// of their releases, then each release's tracks, releasesAtOnce at a
// time, up to media.MaxTracks songs. A song on a single and on its album
// comes once, from the release that comes first. each, if set, is given
// the new songs of each release as it is read, in the order of the
// releases, so that playing can start after the first. A release that
// fails, after its tries, is passed over: the songs read stay, and the
// error is a media.ShortList that tells how many releases of how many came.
// With no release read, the error is the first one, alone.
func (c *Client) ArtistSongs(ctx context.Context, id string, each func([]media.Track)) ([]media.Track, error) {
	releases, missed, err := c.artistReleases(ctx, id)
	if len(releases) == 0 {
		if err == nil {
			err = fmt.Errorf("spotify: no albums or singles of this artist")
		}
		return nil, err
	}
	total := len(releases) + missed
	type result struct {
		tracks []media.Track
		err    error
		done   chan struct{}
	}
	results := make([]*result, len(releases))
	for i := range results {
		results[i] = &result{done: make(chan struct{})}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	slots := make(chan struct{}, releasesAtOnce)
	go func() {
		for i, al := range releases {
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				for _, r := range results[i:] {
					r.err = ctx.Err()
					close(r.done)
				}
				return
			}
			go func() {
				defer func() { <-slots }()
				r := results[i]
				defer close(r.done)
				r.err = media.TryPage(ctx, func(ctx context.Context) error {
					a, err := c.Album(ctx, al.ID)
					r.tracks = a.Tracks
					return err
				})
			}()
		}
	}()

	var songs []media.Track
	seen := map[string]bool{}
	read := 0
	for _, r := range results {
		<-r.done
		if r.err != nil {
			if err == nil {
				err = r.err
			}
			continue
		}
		read++
		var fresh []media.Track
		for _, t := range r.tracks {
			if k := songKey(t); !seen[k] && len(songs)+len(fresh) < media.MaxTracks {
				seen[k] = true
				fresh = append(fresh, t)
			}
		}
		songs = append(songs, fresh...)
		if len(fresh) > 0 && each != nil {
			each(slices.Clone(fresh))
		}
	}
	if ctx.Err() != nil {
		return songs, ctx.Err()
	}
	switch {
	case read == 0:
		return nil, err
	case read < total:
		return songs, &media.ShortList{Got: read, Total: total, Err: err}
	}
	return songs, nil
}
