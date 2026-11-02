package match

import (
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// Song returns the song of a music video: the one of YouTube Music's songs
// that is the same song, by one of the video's artists. YouTube Music
// makes the radio of a video out of videos, and the radio of its song out
// of songs. It returns ErrNoMatch when YouTube Music has no such song, as
// for covers, mashups and other uploads. Unlike Match, it remembers
// nothing.
func (m *Matcher) Song(ctx context.Context, video media.Track) (media.Track, error) {
	query := CleanTitle(video.Title)
	if len(video.Artists) > 0 {
		query += " " + video.Artists[0].Name
	}
	items, err := m.YouTube.Search(ctx, query, youtube.FilterSongs)
	if err != nil {
		return media.Track{}, err
	}
	// Of the songs that are this one: the one with the video's title, else
	// the one whose title differs from it by the fewest words, else the
	// first YouTube Music lists. Lengths tell nothing here, as a video can
	// run longer than its song, and a remix closer to it.
	title := titleWords(video)
	var best media.Track
	fewest := -1
	for _, it := range items {
		if it.Kind != media.ItemTrack || it.Track.Video || !SameSong(video, *it.Track) {
			continue
		}
		if n := extraWords(title, words(CleanTitle(it.Track.Title))); fewest < 0 || n < fewest {
			best, fewest = *it.Track, n
		}
	}
	if fewest < 0 {
		return media.Track{}, ErrNoMatch
	}
	return best, nil
}

// SameSong reports whether song is the song of video: one of its artists
// is one of the video's, or the one an "Artist - Title" upload names, and
// the words of one title are all in the other, once what marks a release
// or an upload is gone. Covers and karaoke versions, by other artists,
// are not, even when their titles name the artist.
func SameSong(video, song media.Track) bool {
	names := map[string]bool{}
	for _, a := range video.Artists {
		for _, n := range artistNames(a.Name) {
			names[n] = true
		}
	}
	if a, _, ok := strings.Cut(video.Title, " - "); ok {
		for _, n := range artistNames(a) {
			names[n] = true
		}
	}
	if !slices.ContainsFunc(song.Artists, func(a media.Ref) bool {
		return slices.ContainsFunc(artistNames(a.Name), func(n string) bool { return names[n] })
	}) {
		return false
	}
	v, s := titleWords(video), words(CleanTitle(song.Title))
	return holds(v, s) || holds(s, v)
}

// titleWords returns the words of a video's title without what marks the
// upload, and without its artists' names, which "Artist - Title" uploads
// carry, unless nothing else would be left.
func titleWords(video media.Track) []string {
	ws := words(CleanTitle(video.Title))
	for _, a := range video.Artists {
		name := words(a.Name)
		if len(ws) > len(name) {
			ws = slices.DeleteFunc(ws, func(w string) bool { return slices.Contains(name, w) })
		}
	}
	return ws
}

// joiners split a byline into the artists it joins.
var joiners = regexp.MustCompile(`(?i)\s+(?:ft\.?|feat\.?|featuring|&|x|with)\s+|\s*,\s*`)

// artistNames returns a byline's name, and the names it joins with "ft.",
// "feat.", "&", "," or "x", normalized.
func artistNames(s string) []string {
	var out []string
	for _, part := range append([]string{s}, joiners.Split(s, -1)...) {
		if n := normalize(part); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// holds reports whether part has words, and every one of them is in whole.
func holds(whole, part []string) bool {
	if len(part) == 0 {
		return false
	}
	for _, w := range part {
		if !slices.Contains(whole, w) {
			return false
		}
	}
	return true
}

// extraWords counts the words either title has and the other hasn't.
func extraWords(a, b []string) int {
	n := 0
	for _, w := range a {
		if !slices.Contains(b, w) {
			n++
		}
	}
	for _, w := range b {
		if !slices.Contains(a, w) {
			n++
		}
	}
	return n
}
