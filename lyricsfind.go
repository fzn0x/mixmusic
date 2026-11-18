package main

// Find in the lyrics (#603). A search button in the lyrics panel's head,
// beside Close, and beside Share lines on the now playing page, opens a box
// above the lyrics, with the focus; the palette's Find in the lyrics opens
// it too, with no keys of its own: Ctrl+F stays the music search. A line
// matches when its words as written, or as they show in Latin letters or
// translated, hold what is typed, as lyricsFold reads both. The lines found
// show tinted, the current one more, and no line dims; while the box holds
// text, synced lyrics stop following the song, so that the line found stays
// in view. Up and Down step through the lines found, and wrap. Enter plays
// the song from the current one, as a click on it does, or chooses it while
// lines are chosen to share; lyrics shown without times only scroll to it.
// Escape clears the box, then closes it. The full-screen lyrics, the mini
// player and the notch have no box.

import (
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
	"golang.org/x/text/unicode/norm"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
	"github.com/fzn0x/mixmusic/lyrics"
)

// lyricsFind is the box that finds lines of a lyrics view by their words.
type lyricsFind struct {
	// open is set while the box shows, and focus has it take the focus once
	// it does.
	open, focus bool
	query       string
	// matches are the lines that hold the words typed, as worked out for
	// key, in order, and at which of them is the current one (current).
	key     lyricsFindKey
	matches []int
	at      int
	// moved has the lyrics scroll the current line found into view, and
	// stepped has screen readers hear it, after a step to it.
	moved, stepped bool
	// finding is whether the box held text in the last frame: once it
	// doesn't, synced lyrics follow the song again.
	finding bool
	// typedAt is when the text changed, and told the text whose count
	// screen readers heard.
	typedAt time.Time
	told    string
}

// lyricsFindKey is what the lines found are worked out from: the text
// typed, the lyrics, and what their lines show besides their words. The
// lyrics and what they show are kept as they came: the same lyrics are the
// same lines.
type lyricsFindKey struct {
	query, track string
	lines        *lyrics.Line
	n            int
	translate    bool
	romanize     string
	tr           *lyrics.Translation
	romaji       *string
	roman        *romanizedLines
}

// findKeyOf is the key of the lines found of the lyrics ly of track, which
// show what x says besides their words, for the text query.
func findKeyOf(query, track string, ly lyrics.Lyrics, x lyricsExtra) lyricsFindKey {
	k := lyricsFindKey{query: query, track: track, n: len(ly.Lines), translate: x.translate, romanize: x.romanize, tr: x.tr, roman: x.roman}
	if len(ly.Lines) > 0 {
		k.lines = &ly.Lines[0]
	}
	if len(x.romaji) > 0 {
		k.romaji = &x.romaji[0]
	}
	return k
}

// lyricsFindOf is the box of the lyrics view where: the panel's, or the
// now playing page's.
func (s *shell) lyricsFindOf(where string) *lyricsFind {
	if where == "page" {
		return &s.lyricsView.page.find
	}
	return &s.lyricsView.panel.find
}

// close closes the box, and lets its text and its lines found go.
func (f *lyricsFind) close() { *f = lyricsFind{finding: f.finding} }

// sync works the lines found out again, by match, when the key changed:
// the current one stays while only what the lines show besides their words
// changed, as when a translation comes, and is otherwise the first at or
// after active, the line sung now, else the first.
func (f *lyricsFind) sync(key lyricsFindKey, match func() []int, active int) {
	if key == f.key {
		return
	}
	was := -1
	if f.at >= 0 && f.at < len(f.matches) && key.query == f.key.query && key.track == f.key.track && key.lines == f.key.lines {
		was = f.matches[f.at]
	}
	f.key, f.matches, f.at = key, match(), -1
	if len(f.matches) == 0 {
		return
	}
	if i := slices.Index(f.matches, was); was >= 0 && i >= 0 {
		f.at = i
		return
	}
	f.at = 0
	for i, m := range f.matches {
		if m >= active {
			f.at = i
			break
		}
	}
	f.moved = true
}

// step goes on to the next line found, by 1, or back, by -1, wrapping at
// the ends.
func (f *lyricsFind) step(by int) {
	n := len(f.matches)
	if n == 0 {
		return
	}
	f.at = ((f.at+by)%n + n) % n
	f.moved, f.stepped = true, true
}

// current is the line found now, and whether there is one.
func (f *lyricsFind) current() (int, bool) {
	if f.at < 0 || f.at >= len(f.matches) {
		return 0, false
	}
	return f.matches[f.at], true
}

// tint is how strongly line i shows the accent behind it: some for a line
// found, more for the current one, none for the others.
func (f *lyricsFind) tint(i int) float32 {
	n, ok := slices.BinarySearch(f.matches, i)
	switch {
	case !ok:
		return 0
	case n == f.at:
		return 0.3
	}
	return 0.12
}

// lyricsMatching returns the lines of the lyrics ly whose words, as written
// or as x shows them, in Latin letters or translated, hold query, as
// lyricsFold reads both.
func lyricsMatching(ly lyrics.Lyrics, x lyricsExtra, query string) []int {
	q := lyricsFold(query)
	if q == "" {
		return nil
	}
	out := []int{}
	for i, line := range ly.Lines {
		text, roman, meaning := x.line(i, line.Text)
		for _, words := range [...]string{line.Text, text, roman, meaning} {
			if words != "" && strings.Contains(lyricsFold(words), q) {
				out = append(out, i)
				break
			}
		}
	}
	return out
}

// lyricsFold reads text as Find in the lyrics compares it: in lowercase,
// with curly quotes straight, the spaces between words one, and Latin,
// Greek and Cyrillic letters without their accents, so that "cafe" finds
// "café", "Ελλαδα" "Ελλάδα" and "ежик" "ёжик". Other scripts keep their
// marks, which make other letters there: か doesn't find が, and a Hangul
// syllable stays whole. Arabic and Hebrew drop the vowel signs and the
// hamza and maddah of their letters, which lines may or may not be written
// with, and Arabic its tatweel, which only stretches words. Some letters
// fold as their languages read them: German ß as ss, the Greek final ς as
// σ, the Turkish dotless ı as i, and ł, ø, đ, ħ, æ and œ as their Latin
// letters; Cyrillic й and ў, letters of their own wherever they're used,
// keep their breve. Width folds too: a full-width A is an A.
func lyricsFold(text string) string {
	text = norm.NFKC.String(text)
	var b strings.Builder
	b.Grow(len(text))
	space, strip := false, false
	put := func(r rune) {
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		switch r = unicode.ToLower(r); r {
		case 'ß':
			b.WriteString("ss")
		case 'ς':
			b.WriteRune('σ')
		case 'ı':
			b.WriteByte('i')
		case 'ł':
			b.WriteByte('l')
		case 'ø':
			b.WriteByte('o')
		case 'đ':
			b.WriteByte('d')
		case 'ħ':
			b.WriteByte('h')
		case 'æ':
			b.WriteString("ae")
		case 'œ':
			b.WriteString("oe")
		case '‘', '’', '‚', '‛', 'ʼ':
			b.WriteByte('\'')
		case '“', '”', '„', '‟':
			b.WriteByte('"')
		default:
			b.WriteRune(r)
		}
	}
	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			space, strip = true, false
		case unicode.In(r, unicode.Mn, unicode.Me):
			// A mark that no letter took in: those of the letters whose marks
			// go, go too.
			if !strip {
				b.WriteRune(r)
			}
		case r == 'ـ', unicode.Is(unicode.Cf, r):
			// Tatweel, and the marks of direction and joiners that don't show.
		case r == 'й', r == 'Й', r == 'ў', r == 'Ў':
			put(r)
			strip = true
		case unicode.In(r, unicode.Latin, unicode.Greek, unicode.Cyrillic, unicode.Arabic, unicode.Hebrew):
			for _, d := range norm.NFD.String(string(r)) {
				if !unicode.In(d, unicode.Mn, unicode.Me) {
					put(d)
				}
			}
			strip = true
		default:
			put(r)
			strip = false
		}
	}
	return b.String()
}

// findableLyrics tells whether the lyrics of t came, with lines to find.
func (s *shell) findableLyrics(t media.Track) bool {
	l, ok := s.loads["lyrics:"+t.ID].(*loaded[lyrics.Lyrics])
	return ok && l.done && l.err == nil && !l.value.Instrumental && len(l.value.Lines) > 0
}

// lyricsFindButton opens the box that finds lines of the lyrics of where,
// focused, or closes it, as it shows on.
func (s *shell) lyricsFindButton(c *ui.Context, where string, size float32) {
	f := s.lyricsFindOf(where)
	if !s.iconButton(c, "lyrics-find-button-"+where, "search", locales.LyricsFind(), f.open, size).Clicked() {
		return
	}
	if f.open {
		f.close()
	} else {
		f.open, f.focus = true, true
	}
}

// findInLyrics is the palette's Find in the lyrics: it focuses the box of
// the now playing page, there, and else opens the lyrics panel with its
// box. The full-screen lyrics, which have none, close.
func (s *shell) findInLyrics() {
	cur, ok := s.st.Current()
	switch {
	case !ok:
		s.notes = append(s.notes, notice(locales.CmdPlayFirst()))
		return
	case cur.Track.IsEpisode():
		s.notes = append(s.notes, notice(locales.LyricsStagePodcast()))
		return
	}
	s.stage.open = false
	where := "page"
	if s.router.Path() != "/now-playing" {
		where = "panel"
		if s.panel != "lyrics" {
			s.showPanel("lyrics")
		}
	}
	f := s.lyricsFindOf(where)
	f.open, f.focus = true, true
}

// lyricsFindIn is what the box of a lyrics view finds in: the view's key;
// the track t, and its lyrics ly, whose lines show what x says besides
// their words; whether they show synced; the line sung now, active; and
// how far they show moved, shift (lyricsShift).
type lyricsFindIn struct {
	where  string
	t      media.Track
	ly     lyrics.Lyrics
	x      lyricsExtra
	synced bool
	active int
	shift  time.Duration
}

// text is what line i of the lyrics shows, in Latin letters in its place
// where they show so.
func (in lyricsFindIn) text(i int) string {
	text, _, _ := in.x.line(i, in.ly.Lines[i].Text)
	if text == "" {
		text = "♪"
	}
	return text
}

// lyricsFindOn shows the box of a lyrics view, view, while it's open, and
// tells whether it holds words: while it does, the lines found show tinted,
// none dims, and the lyrics don't follow the song. Once it holds none again,
// synced lyrics follow it again, at once.
func (s *shell) lyricsFindOn(c *ui.Context, view *lyricsScroll, in lyricsFindIn) bool {
	f := &view.find
	if f.open {
		s.lyricsFindBox(c, f, in)
	}
	finding := f.open && strings.TrimSpace(f.query) != ""
	if f.finding && !finding {
		view.scrolled = -1
	}
	f.finding = finding
	return finding
}

// scroll scrolls list, the lyrics', to the current line found, after a step
// to it, or as it is found.
func (f *lyricsFind) scroll(list *ui.ListState) {
	if line, ok := f.current(); ok && f.moved {
		list.ScrollTo(line, ui.Center)
	}
	f.moved = false
}

// findEnter is Enter in the box, on line i, the line found, as a click on
// it: it plays the song from there, at its time less the lyrics' timing,
// unless the track still loads, and a song paused stays paused. While lines
// are chosen to share, it chooses the line. Lyrics shown without times only
// scroll to it.
func (s *shell) findEnter(in lyricsFindIn, i int) {
	switch {
	case s.lyricPick.picking(in.t.ID):
		s.lyricPick.toggle(i, in.text(i))
	case in.synced && !s.st.Loading:
		s.seekLine(in.ly.Lines[i], in.shift)
	}
}

// lyricsFindBox is the box above the lyrics that finds lines by their words,
// while it's open, as lyricsFind says: what is typed, then how many lines
// hold it and which is current, "2 of 5", or that none does. Screen readers
// hear the count once typing pauses, as in the search of the settings, and
// each line Up or Down goes to. Under lyrics shown without times, it says
// that Enter only scrolls.
func (s *shell) lyricsFindBox(c *ui.Context, f *lyricsFind, in lyricsFindIn) {
	k := &s.lk
	ui.Column(c.Key("lyrics-find-"+in.where)).Padding(0, 24, 8).Gap(6).Shrink(0).Children(func() {
		ui.Row(c).GapX(12).GapY(6).Wrap().AlignItems(ui.Center).Children(func() {
			focus := f.focus
			f.focus = false
			box := s.filterBox(c, "lyrics-find-input-"+in.where, locales.LyricsFindPlaceholder(), locales.LyricsFind(), &f.query, focus, func() { f.query = "" })
			if focus {
				box.ScrollIntoView()
			}
			if box.Changed() {
				f.typedAt = c.Now()
			}
			// The lines found, of what the box holds now.
			f.sync(findKeyOf(f.query, in.t.ID, in.ly, in.x), func() []int { return lyricsMatching(in.ly, in.x, f.query) }, in.active)
			switch {
			case f.query == "" && box.Shortcut(0, ui.KeyEscape):
				f.close()
				return
			case box.Shortcut(0, ui.KeyDown):
				f.step(1)
			case box.Shortcut(0, ui.KeyUp):
				f.step(-1)
			}
			if line, ok := f.current(); ok && box.Submitted() {
				s.findEnter(in, line)
				f.moved = true
			}
			if strings.TrimSpace(f.query) == "" {
				return
			}
			count := locales.LyricsFindNone()
			if line, ok := f.current(); ok {
				count = locales.LyricsFindCount(f.at+1, len(f.matches))
				if f.stepped {
					c.Announce(locales.LyricsFindAt(f.at+1, len(f.matches), in.text(line)))
				}
			}
			ui.Text(c, count).FontSize(k.smallSize()).TextColor(k.muted).FontFeatures("tnum")
		})
		f.stepped = false
		if f.open && !in.synced && !s.lyricPick.picking(in.t.ID) {
			ui.Text(c, locales.LyricsFindPlain()).FontSize(k.smallSize()).TextColor(k.muted)
		}
	})
	s.lyricsFindTell(c, f)
}

// lyricsFindTell tells screen readers how many lines hold the words typed,
// once the typing pauses for tellAfter: "3 lines hold these words", or that
// none does.
func (s *shell) lyricsFindTell(c *ui.Context, f *lyricsFind) {
	q := strings.TrimSpace(f.query)
	if f.told == q {
		return
	}
	if wait := tellAfter - c.Now().Sub(f.typedAt); wait > 0 {
		c.After(wait)
		return
	}
	f.told = q
	switch {
	case q == "":
	case len(f.matches) == 0:
		c.Announce(locales.LyricsFindNone())
	default:
		c.Announce(locales.LyricsFindSpoken(len(f.matches)))
	}
}
