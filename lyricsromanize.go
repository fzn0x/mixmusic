package main

import (
	"context"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/romaji"
	"github.com/fzn0x/mixmusic/internal/romanize"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/lyrics"
)

// Lyrics in the other scripts than Latin letters, but Japanese's, are
// romanized on the computer (internal/romanize): their switch shows at
// once, and nothing of them is sent. Arabic's stays Unison's, whose
// romanization by Google has the short vowels Arabic doesn't write, which
// no rule can tell; the computer's shows until it comes, and when it
// can't.

// unisonFirst are the languages romanized on the computer whose lines in
// Latin letters are Unison's, once they come, as they're better: those
// romanizedFirst names, of a script that writes no short vowels.
var unisonFirst = map[string]bool{"ar": true}

// romanizeView is what the lyrics views know of the lyrics romanized on
// the computer: the lyrics of the lines of, n of them, and the language
// they're romanized as, which lang tells.
type romanizeView struct {
	of   *lyrics.Line
	n    int
	lang string
}

// romanizeLanguage returns the language of lyrics that MixMusic romanizes
// on the computer, which keeps the user's choice: theirs, when their
// lines are in its script, else that of the script of their lines, as
// Korean for lyrics in English with lines in Hangul; "" for lyrics it
// doesn't romanize, as Japanese, romaji's. It looks at their lines once,
// while the views show the same lyrics.
func (s *shell) romanizeLanguage(ly lyrics.Lyrics) string {
	if len(ly.Lines) == 0 || s.japanese(ly) {
		return ""
	}
	v := &s.lyricsView.local
	if v.of != &ly.Lines[0] || v.n != len(ly.Lines) {
		v.of, v.n = &ly.Lines[0], len(ly.Lines)
		writes := func(lang string) bool {
			return lang != "" && lang != "ja" && slices.ContainsFunc(ly.Lines, func(l lyrics.Line) bool { return romanize.Romanizes(l.Text, lang) })
		}
		lang := ly.Language
		if !lyrics.Romanizable(lang) || !writes(lang) {
			lang = lyrics.ScriptLanguage(ly.Lines)
		}
		if !writes(lang) {
			lang = ""
		}
		v.lang = lang
	}
	return v.lang
}

// romanizedLines are lines of lyrics in Latin letters, made on the
// computer: each line, "" for those with nothing to romanize, and the
// words of those synced word by word.
type romanizedLines struct {
	lines []string
	words [][]lyrics.Word
}

// romanizedKey is the key fetch keeps the romanization of a track's
// lyrics in the language lang under.
func romanizedKey(t media.Track, lang string) string { return "lyrics-romanized:" + t.ID + ":" + lang }

// lyricsRomanized returns the lines of lyrics romanized on the computer
// as the language lang, made once, beside the main thread; nil until they
// are.
func (s *shell) lyricsRomanized(t media.Track, ly lyrics.Lyrics, lang string) *romanizedLines {
	r := fetch(s, romanizedKey(t, lang), func(context.Context) (romanizedLines, error) { return romanizeLyrics(ly, lang), nil })
	if !r.done || len(r.value.lines) != len(ly.Lines) {
		return nil
	}
	return &r.value
}

// romanizeLyrics writes the lines of lyrics in Latin letters, those synced
// word by word word by word, as the language lang.
func romanizeLyrics(ly lyrics.Lyrics, lang string) romanizedLines {
	out := romanizedLines{lines: make([]string, len(ly.Lines)), words: make([][]lyrics.Word, len(ly.Lines))}
	for i, l := range ly.Lines {
		if len(l.Words) == 0 {
			out.lines[i] = romanize.Line(l.Text, lang)
			continue
		}
		texts := make([]string, len(l.Words))
		for k, w := range l.Words {
			texts[k] = w.Text
		}
		line, words := romanize.Words(l.Text, lang, texts)
		out.lines[i] = line
		if line == "" {
			continue
		}
		out.words[i] = make([]lyrics.Word, len(words))
		for k, w := range words {
			out.words[i][k] = lyrics.Word{Text: w, StartMs: l.Words[k].StartMs, EndMs: l.Words[k].EndMs}
		}
	}
	return out
}

// words returns the words of line i that light as they're sung, those of
// the text it shows, and of the line in Latin letters under it: the line's
// own as written, and those romanized on the computer; none for lines
// shown in Unison's Latin letters, whose words aren't known.
func (x lyricsExtra) words(i int, line lyrics.Line, text, roman string) (shown, under []lyrics.Word) {
	var latin []lyrics.Word
	if x.roman != nil && x.roman.words[i] != nil && (roman == x.roman.lines[i] || text == x.roman.lines[i]) {
		latin = x.roman.words[i]
	}
	if text == line.Text {
		shown = line.Words
	} else if roman == "" {
		shown = latin
	}
	if roman != "" {
		under = latin
	}
	return shown, under
}

// lyricsRoman shows a line in Latin letters under its line, smaller and
// muted; the words given, of the line sung now, light up as they are
// sung.
func (s *shell) lyricsRoman(c *ui.Context, text string, size float32, words []lyrics.Word) {
	k := &s.lk
	e := ui.Text(c, text).FontSize(size).TextColor(k.muted)
	if len(words) > 0 {
		s.sungText(c, e, text, ui.Font{Family: k.body, Size: size, Weight: 400}, k.accent, k.muted, words, 0, 0)
	}
}

// latinLines returns the lines of lyrics in Latin letters, as far as they
// are made, and how the user shows them, for the notch player: romaji,
// and those romanized on the computer; nil while there are none, as for
// lyrics shown as they are written.
func (s *shell) latinLines(t media.Track, ly lyrics.Lyrics) ([]string, string) {
	if s.japanese(ly) {
		how := s.set.LyricsRomanize["ja"]
		if how == "" {
			return nil, ""
		}
		r := fetch(s, romajiKey(t), func(ctx context.Context) (romaji.Lines, error) { return s.svc.Catalog.LyricsRomaji(ctx, ly) })
		if !r.done || r.err != nil || len(r.value.Romaji) != len(ly.Lines) {
			return nil, how
		}
		return r.value.Romaji, how
	}
	lang := s.romanizeLanguage(ly)
	how := s.set.LyricsRomanize[lang]
	if lang == "" || how == "" {
		return nil, ""
	}
	if r := s.lyricsRomanized(t, ly, lang); r != nil {
		return r.lines, how
	}
	return nil, how
}

// notchLine is line i of lyrics as the notch player shows it: in Latin
// letters in place of its words when the user shows them only so.
func notchLine(lines []lyrics.Line, latin []string, how string, i int) string {
	if i < 0 || i >= len(lines) || lines[i].Text == "" {
		return "♪"
	}
	if how == settings.RomanizeOnly && i < len(latin) && latin[i] != "" {
		return latin[i]
	}
	return lines[i].Text
}

// notchUnder is the line in Latin letters the notch player shows under
// line i, sung now, when the user shows them under it; "" for none.
func notchUnder(latin []string, how string, i int) string {
	if how != settings.RomanizeUnder || i < 0 || i >= len(latin) {
		return ""
	}
	return latin[i]
}
