package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/internal/youtube"
	"github.com/fzn0x/mixmusic/lyrics"
)

// The sources of lyrics, which Settings › Playback › Lyrics puts in order
// and switches off (settings.Lyrics.SourceOrder), and how Catalog.Lyrics
// asks them: one at a time, in the user's order, those that are on.

// lyricsLookup asks a source of lyrics for those of track, which q names:
// the lyrics, or lyrics.ErrNotFound for a track it has none of, or
// errNotAsked for one it isn't asked for; other errors, as
// lyrics.ErrUnavailable, tell that it couldn't be reached.
type lyricsLookup func(c *Catalog, ctx context.Context, track media.Track, q lyrics.Query) (lyrics.Lyrics, error)

// lyricsLookups are the sources of lyrics, by their keys in
// settings.LyricsSourceKeys.
var lyricsLookups = map[string]lyricsLookup{
	settings.LyricsBetterLyrics: func(c *Catalog, ctx context.Context, _ media.Track, q lyrics.Query) (lyrics.Lyrics, error) {
		if !c.a.betterLyrics.Asks(q) {
			return lyrics.Lyrics{}, errNotAsked
		}
		return c.a.betterLyrics.Get(ctx, q)
	},
	settings.LyricsUnison: func(c *Catalog, ctx context.Context, _ media.Track, q lyrics.Query) (lyrics.Lyrics, error) {
		return c.a.unison.Get(ctx, q)
	},
	settings.LyricsLRCLIB: (*Catalog).lrclibLyrics,
	settings.LyricsYouTubeMusic: func(c *Catalog, ctx context.Context, track media.Track, _ lyrics.Query) (lyrics.Lyrics, error) {
		return c.youtubeLyrics(ctx, track)
	},
	settings.LyricsCaptions: (*Catalog).captionLyrics,
}

// errNotAsked is a source of lyrics a track isn't looked up on, as Better
// Lyrics for one without an album.
var errNotAsked = errors.New("lyrics: not asked")

// askLyrics asks the sources of lyrics that are on, in the order set
// puts them, one at a time, for the lyrics of track, and returns the best
// of their answers (betterLyrics). It stops at lyrics no later answer
// would beat: synced, timed word by word while the user prefers those,
// or a track the source says is instrumental. Plain lyrics that show for
// a music video, in place of its song's synced lyrics, keep the note of
// those, which says why.
func (c *Catalog) askLyrics(ctx context.Context, track media.Track, q lyrics.Query, set settings.Lyrics) (lyrics.Lyrics, error) {
	var best lyrics.Lyrics
	found, note := false, ""
	var errs []error
	for _, s := range set.SourceOrder() {
		ask := lyricsLookups[s.Key]
		if s.Off || ask == nil {
			continue
		}
		l, err := ask(c, ctx, track, q)
		if err != nil {
			if !lyricsNone(err) && !errors.Is(err, lyrics.ErrNotFoundYet) {
				log.Printf("mixmusic: the lyrics of %s, on %s: %v", track.ID, s.Key, err)
			}
			errs = append(errs, err)
			continue
		}
		if l.Note != "" && !l.Synced {
			note = l.Note
		}
		if !found || betterLyrics(l, best, set.LyricsPreferWords) {
			best, found = l, true
		}
		if best.Instrumental || best.Synced && (!set.LyricsPreferWords || best.WordTimed()) {
			break
		}
	}
	if !found {
		return lyrics.Lyrics{}, lookupFailure(errs)
	}
	if note != "" && !best.Synced && best.Note == "" {
		best.Note = note
	}
	return best, nil
}

// betterLyrics reports whether the lyrics l, of a source asked after
// those of best, beat them: synced lyrics beat plain ones, and, while
// words come first, lyrics timed word by word beat those timed line by
// line. Of two plain answers, the longer wins. Else the source asked
// first does.
func betterLyrics(l, best lyrics.Lyrics, words bool) bool {
	rank := func(l lyrics.Lyrics) int {
		switch {
		case l.Synced && words && l.WordTimed():
			return 2
		case l.Synced:
			return 1
		}
		return 0
	}
	if a, b := rank(l), rank(best); a != b {
		return a > b
	}
	return !l.Synced && len(l.Lines) > len(best.Lines)
}

// lyricsNone reports whether err tells that a source has no lyrics of a
// track, or wasn't asked for them, rather than that it couldn't answer.
func lyricsNone(err error) bool {
	return errors.Is(err, lyrics.ErrNotFound) || errors.Is(err, errNotAsked) ||
		errors.Is(err, youtube.ErrNotFound) || errors.Is(err, match.ErrNoMatch) || errors.Is(err, stream.ErrNoCaptions)
}

// lookupFailure is why no source had the lyrics of a track, of the errors
// of those asked: lyrics.ErrNotFound when every one that looked had none,
// lyrics.ErrUnavailable when none could be reached, and
// lyrics.ErrNotFoundYet when some had none while others couldn't be
// reached, which may have some: trying again may find them.
func lookupFailure(errs []error) error {
	var none, down []error
	for _, err := range errs {
		switch {
		case errors.Is(err, errNotAsked):
		case errors.Is(err, lyrics.ErrNotFoundYet):
			// LRCLIB had none in its own database, too busy to look further.
			none, down = append(none, err), append(down, err)
		case lyricsNone(err):
			none = append(none, err)
		default:
			down = append(down, err)
		}
	}
	switch {
	case len(down) == 0:
		return lyrics.ErrNotFound
	case len(none) == 0:
		return unreached(down)
	}
	return fmt.Errorf("%w (%v)", lyrics.ErrNotFoundYet, unreached(errs))
}

// unreached is why the sources of lyrics couldn't be reached: each one's
// failure. It is lyrics.ErrUnavailable.
type unreached []error

func (e unreached) Error() string {
	parts := make([]string, len(e))
	for i, err := range e {
		parts[i] = err.Error()
	}
	return strings.Join(parts, "; ")
}
func (e unreached) Unwrap() []error      { return e }
func (e unreached) Is(target error) bool { return target == lyrics.ErrUnavailable }

// lyricsSetup is what of the settings set changes the lyrics a track
// gets: its sources' order and switches, whether words come first, and
// whether music videos' lyrics are timed to them.
// The lyrics kept of a track are kept by it, so that a change looks the
// track up again.
func lyricsSetup(set settings.Lyrics) string {
	var b strings.Builder
	for _, s := range set.SourceOrder() {
		b.WriteString(s.Key)
		if s.Off {
			b.WriteString("-")
		}
		b.WriteString(" ")
	}
	if set.LyricsPreferWords {
		b.WriteString("words ")
	}
	if set.LyricsTimeVideos {
		b.WriteString("timed ")
	}
	b.WriteString(set.LyricsCaptionLanguage)
	return b.String()
}
