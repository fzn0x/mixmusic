// Package match finds a Spotify track's recording on YouTube Music, so
// that Spotify tracks play through MixMusic's own player, with its
// equalizer, crossfade and visualizer, and without Spotify Premium.
package match

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// threshold is the lowest score of a match MixMusic plays.
const threshold = 0.6

// ErrNoMatch is returned when YouTube Music has no recording close enough.
var ErrNoMatch = errors.New("no matching recording found on YouTube Music")

// Searcher searches YouTube Music.
type Searcher interface {
	Search(ctx context.Context, query string, filter youtube.Filter) ([]media.Item, error)
}

// Matcher matches tracks, remembering the matches it found.
type Matcher struct {
	YouTube Searcher
	// Known keeps the matches found, by the ID of the matched track.
	Known *store.File[map[string]media.Track]

	mu       sync.Mutex
	inflight map[string]chan struct{}
}

// New returns a matcher searching yt and remembering matches in known.
func New(yt Searcher, known *store.File[map[string]media.Track]) *Matcher {
	return &Matcher{YouTube: yt, Known: known, inflight: map[string]chan struct{}{}}
}

// Match returns the YouTube Music track of the same recording as t.
func (m *Matcher) Match(ctx context.Context, t media.Track) (media.Track, error) {
	if t.Source == media.SourceYouTube {
		return t, nil
	}
	if found, ok := m.known(t.ID); ok {
		return found, nil
	}
	// One search per track, however many ask at once.
	m.mu.Lock()
	wait, running := m.inflight[t.ID]
	if !running {
		wait = make(chan struct{})
		m.inflight[t.ID] = wait
	}
	m.mu.Unlock()
	if running {
		select {
		case <-wait:
			if found, ok := m.known(t.ID); ok {
				return found, nil
			}
			return media.Track{}, ErrNoMatch
		case <-ctx.Done():
			return media.Track{}, ctx.Err()
		}
	}
	defer func() {
		m.mu.Lock()
		delete(m.inflight, t.ID)
		m.mu.Unlock()
		close(wait)
	}()

	found, err := m.search(ctx, t)
	if err != nil {
		return media.Track{}, err
	}
	if m.Known != nil {
		m.Known.Update(func(known *map[string]media.Track) error {
			if *known == nil {
				*known = map[string]media.Track{}
			}
			(*known)[t.ID] = found
			return nil
		})
	}
	return found, nil
}

// Forget drops the match of a track, for when the user says it is wrong.
func (m *Matcher) Forget(trackID string) error {
	if m.Known == nil {
		return nil
	}
	return m.Known.Update(func(known *map[string]media.Track) error {
		delete(*known, trackID)
		return nil
	})
}

// KnownMatch returns the recording found before of a track, without
// searching: for a Spotify track, its match, if one was found; a YouTube
// Music track is its own.
func (m *Matcher) KnownMatch(t media.Track) (media.Track, bool) {
	if t.Source == media.SourceYouTube {
		return t, true
	}
	return m.known(t.ID)
}

func (m *Matcher) known(id string) (media.Track, bool) {
	if m.Known == nil {
		return media.Track{}, false
	}
	found, ok := m.Known.Get()[id]
	return found, ok
}

// search looks for t among songs, then among videos.
func (m *Matcher) search(ctx context.Context, t media.Track) (media.Track, error) {
	query := t.Title
	if len(t.Artists) > 0 {
		query += " " + t.Artists[0].Name
	}
	var errs []error
	for _, filter := range []youtube.Filter{youtube.FilterSongs, youtube.FilterVideos} {
		items, err := m.YouTube.Search(ctx, query, filter)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if best, ok := Best(t, items); ok {
			return best, nil
		}
	}
	if len(errs) == 2 {
		return media.Track{}, errors.Join(errs...)
	}
	return media.Track{}, ErrNoMatch
}

// Best returns the candidate closest to t, if one is close enough.
func Best(t media.Track, candidates []media.Item) (media.Track, bool) {
	var best media.Track
	bestScore := 0.0
	for _, c := range candidates {
		if c.Kind != media.ItemTrack {
			continue
		}
		if s := Score(t, *c.Track); s > bestScore {
			best, bestScore = *c.Track, s
		}
	}
	return best, bestScore >= threshold
}

// Score rates how likely candidate is the recording of t, from 0 to 1, by
// their titles, artists and durations.
func Score(t, candidate media.Track) float64 {
	title := similarity(words(CleanTitle(t.Title)), words(CleanTitle(candidate.Title)))
	artist := artistScore(t.Artists, candidate)
	duration := durationScore(t.DurationMs, candidate.DurationMs)
	score := 0.45*title + 0.35*artist + 0.2*duration
	if !candidate.Video {
		score += 0.05 // audio tracks are the released recordings
	}
	if title < 0.5 {
		score *= 0.5 // another song, whoever sings it
	}
	return min(score, 1)
}

// artistScore rates whether the candidate is by one of the artists, by its
// artists, or failing that its title, as "Artist - Song" videos have it.
func artistScore(artists []media.Ref, candidate media.Track) float64 {
	if len(artists) == 0 {
		return 0.5
	}
	theirs := normalize(candidate.ArtistNames() + " " + candidate.Title)
	best := 0.0
	for _, a := range artists {
		name := normalize(a.Name)
		if name == "" {
			continue
		}
		if strings.Contains(theirs, name) {
			return 1
		}
		best = max(best, similarity(words(name), words(theirs)))
	}
	return best
}

// durationScore rates how close two durations are; 0.5 when one is
// unknown.
func durationScore(a, b int) float64 {
	if a == 0 || b == 0 {
		return 0.5
	}
	d := max(a-b, b-a)
	switch {
	case d <= 2000:
		return 1
	case d <= 5000:
		return 0.8
	case d <= 10000:
		return 0.5
	case d <= 20000:
		return 0.2
	}
	return 0
}

// noise matches the parts of titles that name a release rather than the
// song: "(Remastered 2011)", "- Radio Edit", "[Official Video]".
var noise = regexp.MustCompile(`(?i)\s*([(\[][^)\]]*(remaster|version|edit|mono|stereo|deluxe|official|video|audio|lyrics?|visuali[sz]er|feat\.?|ft\.?|with )[^)\]]*[)\]]|\s-\s.*(remaster|version|edit|mono|stereo)\b.*$)`)

// CleanTitle removes from a title what names the release rather than the
// song, such as "(Remastered 2011)" or "[Official Video]".
func CleanTitle(s string) string { return noise.ReplaceAllString(s, "") }

// normalize lowercases s, strips its accents and punctuation, and
// collapses its spaces.
func normalize(s string) string {
	var b strings.Builder
	space := false
	for _, r := range norm.NFKD.String(strings.ToLower(s)) {
		switch {
		case unicode.Is(unicode.Mn, r): // accents
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			b.WriteRune(r)
			space = false
		case !space && b.Len() > 0:
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

func words(s string) []string { return strings.Fields(normalize(s)) }

// similarity is the share of the words of the shorter list that the other
// has.
func similarity(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	set := map[string]bool{}
	for _, w := range b {
		set[w] = true
	}
	shared := 0
	for _, w := range a {
		if set[w] {
			shared++
		}
	}
	return float64(shared) / float64(len(a))
}
