package spotify

import (
	"context"
	"net/url"
	"strconv"

	"github.com/fzn0x/mixmusic/internal/media"
)

// releaseLimit is how many albums, and how many singles, ArtistReleases
// asks for: the most a page of an artist's albums holds since February
// 2026, the newest, as Spotify lists them.
const releaseLimit = 10

// release is an album as an artist's list of albums gives it, with how far
// its release date goes: "day", "month" or "year".
type release struct {
	album
	ReleaseDatePrecision string `json:"release_date_precision"`
}

// ArtistReleases returns an artist's newest albums, then their newest
// singles and EPs, each with the day it came out, or its month or its year
// where Spotify tells no more.
func (c *Client) ArtistReleases(ctx context.Context, id string) ([]media.Release, error) {
	out := []media.Release{}
	for _, group := range []string{"album", "single"} {
		var page paging[*release]
		q := url.Values{"include_groups": {group}, "limit": {strconv.Itoa(releaseLimit)}}
		if err := c.get(ctx, "/artists/"+url.PathEscape(id)+"/albums", q, &page); err != nil {
			return nil, err
		}
		for _, r := range page.Items {
			if r == nil || r.ID == "" {
				continue
			}
			out = append(out, media.Release{Album: toAlbum(&r.album), Released: releaseDate(r.ReleaseDate, r.ReleaseDatePrecision)})
		}
	}
	return out, nil
}

// releaseDate is a release date as far as its precision goes.
func releaseDate(date, precision string) string {
	n := map[string]int{"year": len("2006"), "month": len("2006-01")}[precision]
	if n > 0 && len(date) > n {
		return date[:n]
	}
	return date
}
