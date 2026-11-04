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

// romanizedFirst are the languages whose lyrics Unison romanizes: Korean,
// Chinese, Russian and Arabic lines came back romanized on 2026-11-02, as
// Google's translation of lyrics romanizes them, and Japanese, Thai,
// Hebrew, Hindi, Greek and Ukrainian ones didn't. MixMusic romanizes all
// of these itself (lyricsromaji.go, lyricsromanize.go): Unison is asked
// only for Arabic's, which are better (unisonFirst), and, for lyrics in
// a script MixMusic doesn't romanize, its switch shows once a translation
// of them brought some Latin letters.
var romanizedFirst = map[string]bool{"ar": true}

// translated is the translation of lyrics, and whether any line of it came
// in Latin letters, found beside the main thread.
type translated struct {
	lyrics.Translation
	romanized bool
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
	// ja is set for lyrics in Japanese, whose words in Latin letters are
	// their romaji, made on the computer: nil until made.
	ja     bool
	romaji []string
	// local is the language of lyrics romanized on the computer
	// (lyricsromanize.go), and roman their lines romanized: nil until made.
	local string
	roman *romanizedLines
}

// line returns what line i of the lyrics shows: its words, as written or
// in Latin letters, then those in Latin letters under it, and its
// translation, "" where there are none.
func (x lyricsExtra) line(i int, words string) (text, roman, meaning string) {
	text = words
	latin := ""
	switch {
	case x.ja:
		if x.romaji != nil {
			latin = x.romaji[i]
		}
	case x.local != "":
		// Unison's, where it is better, once it came.
		if x.tr != nil && unisonFirst[x.local] {
			latin = x.tr.Lines[i].Romanized
		}
		if latin == "" && x.roman != nil {
			latin = x.roman.lines[i]
		}
	case x.tr != nil:
		latin = x.tr.Lines[i].Romanized
	}
	switch {
	case latin == "":
	case x.romanize == settings.RomanizeOnly:
		text = latin
	case x.romanize == settings.RomanizeUnder:
		roman = latin
	}
	if x.translate && x.tr != nil {
		meaning = x.tr.Lines[i].Text
	}
	return text, roman, meaning
}

// lyricsChoices shows how lyrics show, above them: synced or as plain
// text, translated, and in Latin letters, each where it can; and, while
// their translation loads or when it fails, says so. It tells what their
// lines show besides their words.
func (s *shell) lyricsChoices(c *ui.Context, t media.Track, ly lyrics.Lyrics, where string) lyricsExtra {
	lang, to := ly.Language, s.translateTarget()
	// Lyrics of a language MixMusic can't tell may be in another.
	canTranslate := lang != to
	x := lyricsExtra{translate: canTranslate && slices.Contains(s.set.LyricsTranslate, lang), ja: s.japanese(ly), local: s.romanizeLanguage(ly)}
	// Lyrics in another script show in Latin letters made on the computer,
	// which asks Unison for nothing: Japanese in romaji, and the others
	// as their scripts are romanized, but Arabic, as Unison has it once it
	// came. Lyrics in a script MixMusic doesn't romanize show as Unison
	// has them, with their translation, if it brought them so.
	romanize := ""
	switch {
	case x.ja:
		romanize = s.set.LyricsRomanize["ja"]
	case x.local != "":
		romanize = s.set.LyricsRomanize[x.local]
	case lyrics.Romanizable(lang):
		romanize = s.set.LyricsRomanize[lang]
	}
	var tr *loaded[translated]
	if x.translate || !x.ja && romanize != "" && (x.local == "" && romanizedFirst[lang] || unisonFirst[x.local] && romanizedFirst[x.local]) {
		tr = s.translation(t, ly, to, x.translate)
		if tr.done && tr.err == nil && len(tr.value.Lines) == len(ly.Lines) {
			x.tr = &tr.value.Translation
		}
	}
	canRomanize := x.ja || x.local != "" || lyrics.Romanizable(lang) && (romanizedFirst[lang] || x.translate && x.tr != nil && tr.value.romanized)
	if canRomanize {
		x.romanize = romanize
	}
	if ly.Synced || canTranslate || canRomanize {
		s.lyricsSwitches(c, ly, lang, where, canTranslate, canRomanize, &x)
	}
	if x.ja && x.romanize != "" {
		x.romaji = s.lyricsRomaji(c, t, ly, where)
	}
	if x.local != "" && x.romanize != "" {
		x.roman = s.lyricsRomanized(t, ly, x.local)
	}
	if tr == nil {
		return x
	}
	what := "Romanization"
	if x.translate {
		what = "Translation"
	}
	switch {
	case !tr.done && (x.translate || x.roman == nil):
		s.lyricsNote(c, what+" is on its way…")
	case tr.err != nil && (x.translate || x.local == ""):
		// One quiet line; the lyrics show all the same.
		k := &s.lk
		ui.Row(c).Padding(0, 24, 8).Gap(12).AlignItems(ui.Center).Shrink(0).Children(func() {
			ui.Text(c, what+" isn't available right now.").FontSize(k.smallSize()).TextColor(k.muted)
			if s.link(c, "lyrics-translate-again-"+where, "try again") {
				s.svc.Catalog.TranslateAgain()
				s.forget(translationKey(t, translationInto(ly, to, x.translate)))
			}
		})
	}
	return x
}

// translation fetches the translation of a track's lyrics into the
// language to, as translationInto has it.
func (s *shell) translation(t media.Track, ly lyrics.Lyrics, to string, translate bool) *loaded[translated] {
	to = translationInto(ly, to, translate)
	return fetch(s, translationKey(t, to), func(ctx context.Context) (translated, error) {
		tr, err := s.svc.Catalog.TranslateLyrics(ctx, t, ly, to)
		return translated{tr, slices.ContainsFunc(tr.Lines, func(l lyrics.TranslatedLine) bool { return l.Romanized != "" })}, err
	})
}

// translationInto is the language to ask a translation of lyrics into,
// for their translation into to: to, but, for their romanization alone,
// translate false, English when to is their own language, as Unison
// translates nothing into the language lyrics are in.
func translationInto(ly lyrics.Lyrics, to string, translate bool) string {
	if !translate && ly.Language == to {
		return "en"
	}
	return to
}

// translationKey is the key translation fetches the translation of a
// track's lyrics into the language into under.
func translationKey(t media.Track, into string) string {
	return "lyrics-translation:" + t.ID + ":" + into
}

// lyricsSwitches shows the choices above lyrics in the language lang:
// synced or as plain text, when they are synced; translated, when they
// can be; and in Latin letters, under each line or in its place, when
// they can be. It changes x as the user chooses.
func (s *shell) lyricsSwitches(c *ui.Context, ly lyrics.Lyrics, lang, where string, canTranslate, canRomanize bool, x *lyricsExtra) {
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
		if !canRomanize {
			return
		}
		// The switch and its ways go on one line. Japanese is romaji, kept
		// as "ja" though MixMusic couldn't tell the lyrics' language.
		ui.Row(c).Gap(16).AlignItems(ui.Center).Shrink(0).Children(func() {
			as, tip := lang, romanizeTip
			switch {
			case x.ja:
				as, tip = "ja", romajiTip
			case unisonFirst[x.local]:
				as, tip = x.local, unisonFirstTip
			case x.local != "":
				as, tip = x.local, localTip
			}
			names := romanizeNames[as == "ja"]
			on := x.romanize != ""
			if s.lyricsSwitch(c, "lyrics-romanize-"+where, names[0], "Show lyrics in Latin letters", tip, &on) {
				x.romanize = ""
				if on {
					x.romanize = settings.RomanizeUnder
				}
				s.setRomanize(as, x.romanize)
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
				s.setRomanize(as, x.romanize)
			}
		})
	})
}

// lyricsSwitch is a switch above the lyrics with its label, and tip,
// which tells what it does: it tells whether the user flipped it.
func (s *shell) lyricsSwitch(c *ui.Context, key, label, name, tip string, on *bool) bool {
	changed := false
	ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		changed = s.switchOf(c.Key(key), on).Label(name).Tooltip(tip).Changed()
		ui.Text(c, s.lk.sentence(label)).FontSize(s.lk.smallSize())
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

func init() { addSettings("playback", 60, (*shell).lyricsSettings) }

// lyricsSettings chooses the language lyrics translate to.
func (s *shell) lyricsSettings(c *ui.Context) {
	s.group(c, "Lyrics", func() {
		s.row(c, "Translate lyrics to", "The translate switch beside the lyrics shows them in this language. While it's on, the lyrics of the track playing and its video ID go to Unison, which has Google translate them; nothing is sent while it's off. Lyrics in other scripts show in Latin letters made on this computer, which sends nothing: Japanese in romaji, and the others by the standard of their language. Only Arabic's go to Unison, which romanizes them with the vowels Arabic doesn't write.", func() {
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
