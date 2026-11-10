package main

// The new releases of the artists the user follows, as a shelf on Home and
// in Library › Artists.

import (
	"context"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
)

// newReleases returns what the artists the user follows put out lately,
// as the services last told: once the look of this run came, what it
// found, and before, what the last run's found.
func (s *shell) newReleases() []media.Release {
	if l := fetch(s, "releases", s.svc.Catalog.NewReleases); l.done && l.err == nil {
		return l.value
	}
	return fetch(s, "releases:known", func(context.Context) ([]media.Release, error) {
		return s.svc.Catalog.KnownReleases(), nil
	}).value
}

// releasesShelf is a shelf of the new releases of the artists the user
// follows, those of source, or all for "" or the user's own, when there
// are any. It returns how many it shows.
func (s *shell) releasesShelf(c *ui.Context, key string, source media.Source) int {
	var shown []media.Release
	for _, r := range s.newReleases() {
		if (source == "" || source == media.SourceLocal || r.Album.Source == source) && !s.hidesAlbum(r.Album) {
			shown = append(shown, r)
		}
	}
	if len(shown) == 0 {
		return 0
	}
	s.shelf(c, key, "New from artists you follow", func() {
		// The tiles far to either side take their room, and build nothing.
		n := len(shown)
		from, to := s.nearTiles(c, key, n)
		step := s.lk.tileWidth() + s.lk.gap
		if from > 0 {
			ui.Box(c).Width(float32(from)*step - s.lk.gap).Shrink(0)
		}
		now := time.Now()
		for i := from; i < to; i++ {
			s.releaseTile(c, key+"-"+count(i), shown[i], now)
		}
		if to < n {
			ui.Box(c).Width(float32(n-to)*step - s.lk.gap).Shrink(0)
		}
	})
	return len(shown)
}

// releaseTile is the tile of a release: its album's, whose subtitle tells
// the day it came out, as Nov 1, where an album's tells its year, or its
// year alone, when that is all its service tells.
func (s *shell) releaseTile(c *ui.Context, key string, r media.Release, now time.Time) {
	al := r.Album
	al.Year = releasedLabel(r.Released, now)
	s.albumTile(c, key, al)
}

// releasesFollowed has the new releases show those of the artist a, whom
// the user just followed, once they're asked again, or drops theirs at
// once, for one they stopped following.
func (s *shell) releasesFollowed(a media.Artist, on bool) {
	if on {
		s.refetch("releases")
		return
	}
	for _, key := range []string{"releases", "releases:known"} {
		if l, ok := s.loads[key].(*loaded[[]media.Release]); ok && l.done {
			l.value = slices.DeleteFunc(slices.Clone(l.value), func(r media.Release) bool { return r.Of.ID == a.ID })
		}
	}
}
