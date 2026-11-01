package main

import (
	"context"
	"maps"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/lyrics"
)

// translateLanguages are the languages Settings › Playback offers to
// translate lyrics to, by their codes of ISO 639-1.
var translateLanguages = []struct{ code, name string }{
	{"af", "Afrikaans"}, {"ar", "Arabic"}, {"bg", "Bulgarian"}, {"bn", "Bengali"}, {"ca", "Catalan"},
	{"zh", "Chinese (Simplified)"}, {"zh-Hant", "Chinese (Traditional)"}, {"hr", "Croatian"}, {"cs", "Czech"},
	{"da", "Danish"}, {"nl", "Dutch"}, {"en", "English"}, {"et", "Estonian"}, {"tl", "Filipino"},
	{"fi", "Finnish"}, {"fr", "French"}, {"de", "German"}, {"el", "Greek"}, {"he", "Hebrew"},
	{"hi", "Hindi"}, {"hu", "Hungarian"}, {"id", "Indonesian"}, {"it", "Italian"}, {"ja", "Japanese"},
	{"ko", "Korean"}, {"lv", "Latvian"}, {"lt", "Lithuanian"}, {"ms", "Malay"}, {"no", "Norwegian"},
	{"fa", "Persian"}, {"pl", "Polish"}, {"pt", "Portuguese"}, {"ro", "Romanian"}, {"ru", "Russian"},
	{"sr", "Serbian"}, {"sk", "Slovak"}, {"sl", "Slovenian"}, {"es", "Spanish"}, {"sw", "Swahili"},
	{"sv", "Swedish"}, {"ta", "Tamil"}, {"te", "Telugu"}, {"th", "Thai"}, {"tr", "Turkish"},
	{"uk", "Ukrainian"}, {"ur", "Urdu"}, {"vi", "Vietnamese"},
}

// The tips of the switches above the lyrics, which tell what they send,
// as only they do.
const (
	translateTip = "Shows each line's translation under it. While it's on, the lyrics of the track playing and its video ID go to Unison, which has Google translate them."
	romanizeTip  = "Shows the lyrics in Latin letters too. While it's on, the lyrics of the track playing and its video ID go to Unison, which has Google romanize them."
)

// romanizeNames are the names of the switch that shows lyrics in Latin
// letters, and of its ways, for Japanese and for the other languages.
var romanizeNames = map[bool][3]string{
	true:  {"romaji", "romaji under", "romaji only"},
	false: {"romanize", "romanized under", "romanized only"},
}

// translateTarget is the language lyrics translate to: the one chosen in
// Settings › Playback, else the system's.
func (s *shell) translateTarget() string {
	if s.set.LyricsTranslateTo != "" {
		return s.set.LyricsTranslateTo
	}
	return s.systemLanguage()
}

// systemLanguage is the system's language, asked for once.
func (s *shell) systemLanguage() string {
	ls := &s.lyricsView
	if ls.system == "" {
		ls.system = s.svc.System.Language()
	}
	return ls.system
}

// lyricsExtra is what the lines of lyrics show besides their words, as
// the user chose for their language: their translation, and their words
// in Latin letters, under them or in their place.
type lyricsExtra struct {
	translate bool
	romanize  string
	// tr is the translation of the lyrics, once it came, which holds their
	// words in Latin letters too; nil until then, and while neither shows.
	tr *lyrics.Translation
}

// line returns what line i of the lyrics shows: its words, as written or
// in Latin letters, then those in Latin letters under it, and its
// translation, "" where there are none.
func (x lyricsExtra) line(i int, words string) (text, roman, meaning string) {
	if x.tr == nil {
		return words, "", ""
	}
	tl := x.tr.Lines[i]
	text = words
	switch {
	case tl.Romanized == "":
	case x.romanize == settings.RomanizeOnly:
		text = tl.Romanized
	case x.romanize == settings.RomanizeUnder:
		roman = tl.Romanized
	}
	if x.translate {
		meaning = tl.Text
	}
	return text, roman, meaning
}

// lyricsChoices shows how lyrics show, above them: synced or as plain
// text, translated, and in Latin letters, each where it can; and, while
// their translation loads or when it fails, says so. It tells what their
// lines show besides their words.
func (s *shell) lyricsChoices(c *ui.Context, t media.Track, ly lyrics.Lyrics, where string) lyricsExtra {
	k := &s.lk
	lang, to := ly.Language, s.translateTarget()
	// Lyrics of a language MixMusic can't tell may be in another.
	canTranslate, canRomanize := lang != to, lyrics.Romanizable(lang)
	x := lyricsExtra{translate: canTranslate && slices.Contains(s.set.LyricsTranslate, lang)}
	if canRomanize {
		x.romanize = s.set.LyricsRomanize[lang]
	}
	if ly.Synced || canTranslate || canRomanize {
		ui.Row(c).Padding(0, 24, 8).GapX(16).GapY(8).Wrap().AlignItems(ui.Center).Shrink(0).Children(func() {
			if ly.Synced {
				mode := 0
				if s.set.LyricsPlain {
					mode = 1
				}
				if s.segmented(c, "lyrics-mode-"+where, "Show lyrics as", &mode, "synced", "plain text") {
					s.set.LyricsPlain = mode == 1
					s.save()
				}
			}
			if canTranslate {
				on := x.translate
				if s.lyricsSwitch(c, "lyrics-translate-"+where, "translate", "Translate lyrics", translateTip, &on) {
					x.translate = on
					s.set.LyricsTranslate = slices.DeleteFunc(slices.Clone(s.set.LyricsTranslate), func(l string) bool { return l == lang })
					if on {
						s.set.LyricsTranslate = append(s.set.LyricsTranslate, lang)
					}
					s.save()
				}
			}
			if canRomanize {
				// The switch and its ways go on one line.
				ui.Row(c).Gap(16).AlignItems(ui.Center).Shrink(0).Children(func() {
					names := romanizeNames[lang == "ja"]
					on := x.romanize != ""
					if s.lyricsSwitch(c, "lyrics-romanize-"+where, names[0], "Show lyrics in Latin letters", romanizeTip, &on) {
						x.romanize = ""
						if on {
							x.romanize = settings.RomanizeUnder
						}
						s.setRomanize(lang, x.romanize)
					}
					if x.romanize == "" {
						return
					}
					mode := 0
					if x.romanize == settings.RomanizeOnly {
						mode = 1
					}
					if s.segmented(c, "lyrics-romanize-mode-"+where, "Show lyrics in Latin letters", &mode, names[1], names[2]) {
						x.romanize = settings.RomanizeUnder
						if mode == 1 {
							x.romanize = settings.RomanizeOnly
						}
						s.setRomanize(lang, x.romanize)
					}
				})
			}
		})
	}
	if !x.translate && x.romanize == "" {
		return x
	}
	// Romaji alone of lyrics in the language translated to come with a
	// translation all the same, to English, as Unison has nothing to
	// translate otherwise.
	if !x.translate && lang == to {
		to = "en"
	}
	key := "lyrics-translation:" + t.ID + ":" + to
	tr := fetch(s, key, func(ctx context.Context) (lyrics.Translation, error) {
		return s.svc.Catalog.TranslateLyrics(ctx, t, ly, to)
	})
	what := "Romanization"
	if x.translate {
		what = "Translation"
	}
	switch {
	case !tr.done:
		s.lyricsNote(c, what+" is on its way…")
	case tr.err != nil:
		// One quiet line; the lyrics show all the same.
		ui.Row(c).Padding(0, 24, 8).Gap(12).AlignItems(ui.Center).Shrink(0).Children(func() {
			ui.Text(c, what+" isn't available right now.").FontSize(k.smallSize()).TextColor(k.muted)
			if s.link(c, "lyrics-translate-again-"+where, "try again") {
				s.svc.Catalog.TranslateAgain()
				s.forget(key)
			}
		})
	case len(tr.value.Lines) == len(ly.Lines):
		x.tr = &tr.value
	}
	return x
}

// lyricsSwitch is a switch above the lyrics with its label, and tip,
// which tells what it does: it tells whether the user flipped it.
func (s *shell) lyricsSwitch(c *ui.Context, key, label, name, tip string, on *bool) bool {
	changed := false
	ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		changed = s.switchOf(c.Key(key), on).Label(name).Tooltip(tip).Changed()
		ui.Text(c, label).FontSize(s.lk.smallSize())
	})
	return changed
}

// setRomanize shows the lyrics of the language lang in Latin letters as
// how says, or as they are written when how is "".
func (s *shell) setRomanize(lang, how string) {
	m := maps.Clone(s.set.LyricsRomanize)
	if m == nil {
		m = map[string]string{}
	}
	if how == "" {
		delete(m, lang)
	} else {
		m[lang] = how
	}
	s.set.LyricsRomanize = m
	s.save()
}

func (s *shell) lyricsNote(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(s.lk.smallSize()).TextColor(s.lk.muted).Padding(0, 24, 8).Shrink(0)
}

// translateOptions are the choices of "Translate lyrics to", the system's
// language first, made once.
func (s *shell) translateOptions() []string {
	ls := &s.lyricsView
	if ls.languages == nil {
		system := s.systemLanguage()
		for _, l := range translateLanguages {
			if l.code == system {
				system = l.name
			}
		}
		ls.languages = []string{"System language (" + system + ")"}
		for _, l := range translateLanguages {
			ls.languages = append(ls.languages, l.name)
		}
	}
	return ls.languages
}

// lyricsSettings chooses the language lyrics translate to.
func (s *shell) lyricsSettings(c *ui.Context) {
	s.group(c, "lyrics", func() {
		s.row(c, "Translate lyrics to", "The translate switch beside the lyrics shows them in this language. While translate or romaji is on, the lyrics of the track playing and its video ID go to Unison, which has Google translate or romanize them; nothing is sent while both are off.", func() {
			options := s.translateOptions()
			chosen := options[0]
			for i, l := range translateLanguages {
				if l.code == s.set.LyricsTranslateTo {
					chosen = options[i+1]
				}
			}
			if s.selectOf(c.Key("translate-to"), &chosen, options).Width(220).Label("Translate lyrics to").Changed() {
				s.set.LyricsTranslateTo = ""
				if i := slices.Index(options, chosen); i > 0 {
					s.set.LyricsTranslateTo = translateLanguages[i-1].code
				}
				s.save()
			}
		})
	})
}
