package match

import (
	"context"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// Found is what Find found of a song: the track, and how sure it is.
type Found struct {
	Track   media.Track
	Verdict Verdict
	// Library is set when the track is of the user's library.
	Library bool
}

// FindOptions tell where Find looks before YouTube Music.
type FindOptions struct {
	// Library are the tracks of the user's library that may be the song,
	// which Find judges first.
	Library []media.Track
}

// Find looks for the recording of t, a song an import names (#613): among
// o.Library, then among YouTube Music's songs, then its videos, searched
// by its title and its artists, and, when that finds none sure, by its
// title and its first artist alone. It returns the first sure candidate,
// else the closest to check, else a Found of NotFound. A search that
// fails stops it, with that error: the song is looked for again later.
// Unlike Match, it remembers nothing.
func (m *Matcher) Find(ctx context.Context, t media.Track, o FindOptions) (Found, error) {
	best := Found{}
	bestScore := 0.0
	consider := func(c media.Track, library bool) bool {
		v := Judge(t, c)
		if v == NotFound {
			return false
		}
		s := Score(t, c)
		if v > best.Verdict || (v == best.Verdict && s > bestScore) {
			best, bestScore = Found{Track: c, Verdict: v, Library: library}, s
		}
		return v == Sure
	}
	for _, c := range o.Library {
		if consider(c, true) {
			return best, nil
		}
	}
	queries := []string{t.Title}
	if len(t.Artists) > 0 {
		var names []string
		for _, a := range t.Artists {
			names = append(names, a.Name)
		}
		queries[0] = t.Title + " " + strings.Join(names, " ")
		if first := FirstArtist(t.Artists[0].Name); first != "" && t.Title+" "+first != queries[0] {
			queries = append(queries, t.Title+" "+first)
		}
	}
	for _, q := range queries {
		for _, filter := range []youtube.Filter{youtube.FilterSongs, youtube.FilterVideos} {
			items, err := m.YouTube.Search(ctx, q, filter)
			if err != nil {
				return best, err
			}
			for _, it := range items {
				if it.Kind == media.ItemTrack && it.Track != nil && consider(*it.Track, false) {
					return best, nil
				}
			}
		}
		if best.Verdict == Sure {
			break
		}
	}
	return best, nil
}
