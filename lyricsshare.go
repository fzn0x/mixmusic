package main

import (
	"image"
	"slices"
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/sharecard"
	"github.com/fzn0x/mixmusic/locales"
	"github.com/fzn0x/mixmusic/lyrics"
)

// Share lines, above the lyrics, lets the user choose lines of them, by
// clicking each, and save them as an image card: the lines large, and the
// song's cover, title and artist at its foot, with where the lyrics come
// from, which their licences ask for, and MixMusic's mark and name at the
// bottom right. While lines are being chosen, a
// click on a line chooses it rather than plays the song from there.

// lyricsPickMax is how many lines a card holds: the card shrinks its text
// to fit, and past this the words would be too small to read.
const lyricsPickMax = 12

// lyricsPick is the lines being chosen: of which track, by their index,
// and the words each shows.
type lyricsPick struct {
	on    bool
	track string
	lines map[int]string
}

// picking reports whether lines of the lyrics of track are being chosen.
func (p *lyricsPick) picking(track string) bool { return p.on && p.track == track }

// toggle chooses line i, which shows text, or lets it go, and says
// whether the line is chosen now. A card holds only so many lines.
func (p *lyricsPick) toggle(i int, text string) bool {
	if _, ok := p.lines[i]; ok {
		delete(p.lines, i)
		return false
	}
	if len(p.lines) >= lyricsPickMax {
		return false
	}
	if p.lines == nil {
		p.lines = map[int]string{}
	}
	p.lines[i] = text
	return true
}

// chosen are the words of the lines chosen, in the order of the lyrics.
func (p *lyricsPick) chosen() []string {
	keys := make([]int, 0, len(p.lines))
	for i := range p.lines {
		keys = append(keys, i)
	}
	slices.Sort(keys)
	out := make([]string, len(keys))
	for n, i := range keys {
		out[n] = p.lines[i]
	}
	return out
}

// lyricsShareBar is the row above the lyrics of t that starts choosing
// lines, and, while lines are chosen, saves them or stops.
func (s *shell) lyricsShareBar(c *ui.Context, t media.Track, ly lyrics.Lyrics, where string) {
	k := &s.lk
	pick := &s.lyricPick
	if pick.on && pick.track != t.ID {
		*pick = lyricsPick{}
	}
	ui.Row(c.Key("lyrics-share-"+where)).Gap(8).AlignItems(ui.Center).Wrap().Padding(0, 24, 8).Children(func() {
		if !pick.picking(t.ID) {
			if s.button(c, "lyrics-share-start-"+where, locales.LyricsShareLines(), false).Clicked() {
				*pick = lyricsPick{on: true, track: t.ID}
			}
			return
		}
		n := len(pick.lines)
		hint := locales.LyricsShareHint()
		if n > 0 {
			hint = locales.LyricsShareChosen(n)
		}
		ui.Text(c, hint).FontSize(k.smallSize()).TextColor(k.muted)
		if n > 0 {
			if s.button(c, "lyrics-share-save-"+where, locales.LyricsShareSave(), true).Clicked() {
				s.saveLyricsCard(t, ly, sharecard.Square)
			}
			if s.button(c, "lyrics-share-story-"+where, locales.LyricsShareStory(), false).Clicked() {
				s.saveLyricsCard(t, ly, sharecard.Story)
			}
		}
		if s.button(c, "lyrics-share-cancel-"+where, locales.LyricsShareDone(), false).Clicked() {
			*pick = lyricsPick{}
		}
	})
}

// pickLine makes the element of line i, which shows text, choose the line
// when clicked, and shows it chosen.
func (s *shell) pickLine(p ui.Element, i int, text string) {
	k := &s.lk
	pick := &s.lyricPick
	p.Cursor(ui.CursorPointer)
	if _, ok := pick.lines[i]; ok {
		p.Background(k.accent.Alpha(0.2)).Radius(k.radiusSm)
	}
	if p.Clicked() {
		pick.toggle(i, text)
	}
}

// saveLyricsCard saves the lines chosen of the lyrics ly of t as a card.
func (s *shell) saveLyricsCard(t media.Track, ly lyrics.Lyrics, size sharecard.Size) {
	lines := s.lyricPick.chosen()
	colors := s.lk.cardColors()
	artist := names(t.Artists)
	credit := ""
	switch {
	case ly.Source == "":
	case ly.By == "":
		credit = locales.LyricsShareCredit(ly.Source)
	default:
		credit = locales.LyricsShareCreditBy(ly.Source, ly.By)
	}
	s.saveCard(t.Title, t.Artwork, size, func(cover image.Image, size sharecard.Size) sharecard.Card {
		c := sharecard.Card{Size: size, Colors: colors, Cover: cover, CoverAt: sharecard.CoverFoot}
		for n, text := range lines {
			gap := 30.0
			if n == 0 {
				gap = 0
			}
			c.Lines = append(c.Lines, sharecard.Line{Text: text, Size: 76, Weight: 700, MaxLines: 4, Gap: gap})
		}
		c.Foot = []sharecard.Line{{Text: t.Title, Size: 44, Weight: 700, MaxLines: 2}}
		if artist != "" {
			c.Foot = append(c.Foot, sharecard.Line{Text: artist, Size: 34, Weight: 400, Ink: colors.Muted})
		}
		if credit != "" {
			c.Foot = append(c.Foot, sharecard.Line{Text: credit, Size: 26, Weight: 400, Ink: colors.Muted, MaxLines: 2})
		}
		c.Mark = &sharecard.Mark{Logo: cardLogo(), Name: sharecard.Line{Text: "mixmusic", Size: 30, Weight: 700}}
		return c
	})
}

// cardLogo is MixMusic's mark in its pink, as the sidebar shows it beside
// the name, drawn once for the cards that carry it.
var cardLogo = sync.OnceValue(func() image.Image { return brandMark.Image(256, 256, nil) })
