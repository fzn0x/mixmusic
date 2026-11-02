package app

import (
	"context"
	"path/filepath"

	"github.com/fzn0x/mixmusic/internal/romaji"
	"github.com/fzn0x/mixmusic/lyrics"
)

// newRomaji returns the dictionary of the readings of kanji, which romaji
// downloads the first time it is turned on for Japanese lyrics, kept in
// the cache's romaji folder.
func newRomaji(opts Options, userAgent string) *romaji.Dictionary {
	d := romaji.NewDictionary(filepath.Join(opts.CacheDir, "romaji"), userAgent)
	if opts.RomajiURL != "" {
		d.URL = opts.RomajiURL
	}
	return d
}

// LyricsRomaji returns the lines of Japanese lyrics in romaji, made on the
// computer: nothing of the lyrics is sent anywhere. Their kanji are read
// once the dictionary of their readings is there, which the first lyrics
// start the download of; until it is, they stay as they are, and the
// lines' Readings tell why.
func (c *Catalog) LyricsRomaji(_ context.Context, l lyrics.Lyrics) (romaji.Lines, error) {
	lines := make([]string, len(l.Lines))
	for i, line := range l.Lines {
		lines[i] = line.Text
	}
	return c.a.romaji.Romanize(lines), nil
}

// WaitRomaji waits for the download of the dictionary of the readings of
// kanji, when one is under way, to end, or ctx to.
func (c *Catalog) WaitRomaji(ctx context.Context) error { return c.a.romaji.Wait(ctx) }

// RomajiAgain has the next romaji download the dictionary of the readings
// of kanji, though its download failed not long ago: the user asked to
// try again.
func (c *Catalog) RomajiAgain() { c.a.romaji.Retry() }
