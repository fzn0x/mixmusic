package main

import (
	"context"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/romaji"
	"github.com/fzn0x/mixmusic/lyrics"
)

// romajiTip is the tip of the romaji switch above Japanese lyrics, which
// sends nothing.
const romajiTip = "Shows the lyrics in romaji too, made on this computer: nothing is sent. The first time, it downloads the dictionary of the readings of kanji, 11.7 MB, from GitHub."

// romajiView is what the lyrics views know of romaji: whether the lyrics
// of the lines of, n of them, are Japanese, which ja tells; and whether
// they wait for the dictionary of the readings of kanji to download.
type romajiView struct {
	of      *lyrics.Line
	n       int
	ja      bool
	waiting bool
}

// japanese tells whether lyrics are in Japanese, whose romaji MixMusic
// makes: lines in kana or kanji, of lyrics in Japanese, or of a language
// MixMusic couldn't tell, with kana among them. It looks at their lines
// once, while the views show the same lyrics.
func (s *shell) japanese(ly lyrics.Lyrics) bool {
	if ly.Language != "ja" && ly.Language != "" || len(ly.Lines) == 0 {
		return false
	}
	v := &s.lyricsView.romaji
	if v.of != &ly.Lines[0] || v.n != len(ly.Lines) {
		is := romaji.Japanese
		if ly.Language == "" {
			is = romaji.HasKana
		}
		v.of, v.n = &ly.Lines[0], len(ly.Lines)
		v.ja = slices.ContainsFunc(ly.Lines, func(l lyrics.Line) bool { return is(l.Text) })
	}
	return v.ja
}

// romajiKey is the key fetch keeps the romaji of a track's lyrics under.
func romajiKey(t media.Track) string { return "lyrics-romaji:" + t.ID }

// lyricsRomaji returns the lines of Japanese lyrics in romaji, made once,
// nil until they are; and says, above the lyrics, while the readings of
// their kanji download, or that they couldn't, with try again.
func (s *shell) lyricsRomaji(c *ui.Context, t media.Track, ly lyrics.Lyrics, where string) []string {
	key := romajiKey(t)
	r := fetch(s, key, func(ctx context.Context) (romaji.Lines, error) { return s.svc.Catalog.LyricsRomaji(ctx, ly) })
	switch {
	case !r.done:
		s.lyricsNote(c, "Romaji is on its way…")
		return nil
	case r.err != nil || len(r.value.Romaji) != len(ly.Lines):
		// The romaji of other lyrics of the track.
		return nil
	}
	switch r.value.Readings {
	case romaji.Downloading:
		s.lyricsNote(c, "The readings of kanji are downloading: kanji show as they are until then.")
		s.awaitRomaji(key)
	case romaji.Unavailable:
		k := &s.lk
		ui.Row(c).Padding(0, 24, 8).Gap(12).AlignItems(ui.Center).Shrink(0).Children(func() {
			ui.Text(c, "The readings of kanji couldn't be downloaded: kanji show as they are.").FontSize(k.smallSize()).TextColor(k.muted)
			if s.link(c, "lyrics-romaji-again-"+where, "try again") {
				s.svc.Catalog.RomajiAgain()
				s.refetch(key)
			}
		})
	}
	return r.value.Romaji
}

// awaitRomaji makes the romaji of key again once the dictionary of the
// readings of kanji has downloaded, or failed to: one wait for the views.
func (s *shell) awaitRomaji(key string) {
	v := &s.lyricsView.romaji
	if v.waiting {
		return
	}
	v.waiting = true
	go func() {
		s.svc.Catalog.WaitRomaji(context.Background())
		s.update(func() {
			v.waiting = false
			s.refetch(key)
		})
	}()
}
