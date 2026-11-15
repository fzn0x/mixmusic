package settings

import (
	"math"
	"regexp"
	"slices"
)

// The ways of showing lyrics in Latin letters (LyricsRomanize).
const (
	// RomanizeUnder shows each line in Latin letters under it.
	RomanizeUnder = "under"
	// RomanizeOnly shows the lines in Latin letters in place of theirs.
	RomanizeOnly = "only"
)

// languageRe matches the code of a language, as "ja" or "zh-Hant".
var languageRe = regexp.MustCompile(`^[a-z]{2,3}(-[A-Z][a-z]{3})?$`)

// LyricStyle chooses how the full-screen lyrics look: their face, weight
// and size, how the lines away from the one sung fade and blur, and the glow
// of the words sung. The stage has its own ink over the cover, so the
// theme's colours don't change them.
type LyricStyle string

const (
	// LyricStyleGlow is Quicksand, round and bold, with a soft glow on the
	// words as they are sung and the lines away from them blurred.
	LyricStyleGlow LyricStyle = "glow"
	// LyricStyleFocus is Roboto Flex in bold, the lines away from the one
	// sung fading and blurring in steps.
	LyricStyleFocus LyricStyle = "focus"
	// LyricStyleSimple is Selawik, as the rest of MixMusic is set: no
	// blur, the lines away from the one sung only fading.
	LyricStyleSimple LyricStyle = "simple"
	// LyricStyleBold is Archivo SemiCondensed ExtraBold, large and tight,
	// as a poster is.
	LyricStyleBold LyricStyle = "bold"
	// LyricStyleCalm is Roboto Flex in medium, small and calm: a light fade
	// and no glow.
	LyricStyleCalm LyricStyle = "calm"
	// LyricStyleSpotlight is Selawik Bold, very large, the lines away from
	// the one sung blurred hard.
	LyricStyleSpotlight LyricStyle = "spotlight"
)

// LyricStyles are the lyrics' styles, in the order the settings list them.
// The first is the one a new install has, and the one a style the settings
// don't know becomes.
var LyricStyles = []LyricStyle{LyricStyleSpotlight, LyricStyleGlow, LyricStyleFocus, LyricStyleSimple, LyricStyleBold, LyricStyleCalm}

// The bounds of the scale of the full-screen lyrics' text (LyricsSize), and
// the step the settings move it by.
const (
	MinLyricsSize  = 0.6
	MaxLyricsSize  = 1.8
	LyricsSizeStep = 0.1
)

// LyricBallMotion chooses how the ball of the full-screen lyrics goes from the
// last word of a line to the first of the next, where it hops from word to
// word over each line as the words are sung.
type LyricBallMotion string

const (
	// LyricBallMix has a different motion for each line, in an order the
	// line's place in the song sets.
	LyricBallMix LyricBallMotion = "mix"
	// LyricBallHop is one arc, as the hops between words are.
	LyricBallHop LyricBallMotion = "hop"
	// LyricBallBoomerang throws the ball out past the end of the line, and has
	// it curve back to the next.
	LyricBallBoomerang LyricBallMotion = "boomerang"
	// LyricBallLoop has it turn a loop over the last word, and dive to the
	// next line.
	LyricBallLoop LyricBallMotion = "loop"
	// LyricBallDive has it rise a little, as to gather itself, and drop to the
	// next line in one swoop.
	LyricBallDive LyricBallMotion = "dive"
	// LyricBallRicochet has it bounce off the right edge of the lyrics and
	// their floor on its way to the next line.
	LyricBallRicochet LyricBallMotion = "ricochet"
)

// LyricBallMotions are the ball's motions between lines, in the order the
// settings list them. The first is the one a new install has, and the one a
// motion the settings don't know becomes.
var LyricBallMotions = []LyricBallMotion{LyricBallMix, LyricBallHop, LyricBallBoomerang, LyricBallLoop, LyricBallDive, LyricBallRicochet}

// Lyrics is how the lyrics show, as the user leaves the switches beside
// them.
type Lyrics struct {
	// LyricsPlain shows synchronized lyrics as plain text.
	LyricsPlain bool `json:"lyricsPlain"`
	// LyricsStyle is the style of the full-screen lyrics, one of
	// LyricStyles.
	LyricsStyle LyricStyle `json:"lyricsStyle"`
	// LyricsSize scales the text of the full-screen lyrics, from
	// MinLyricsSize to MaxLyricsSize, in steps of LyricsSizeStep.
	LyricsSize float64 `json:"lyricsSize"`
	// LyricsBall has a ball bounce from word to word over the line of the
	// full-screen lyrics that is sung, for lyrics timed word by word.
	LyricsBall bool `json:"lyricsBall"`
	// LyricsBallMotion is how the ball goes from one line to the next, one of
	// LyricBallMotions.
	LyricsBallMotion LyricBallMotion `json:"lyricsBallMotion"`
	// LyricsTranslate are the languages of the lyrics the user translates,
	// codes of ISO 639-1 as "ja": their translation shows under each line.
	// "" stands for lyrics whose language MixMusic can't tell.
	LyricsTranslate []string `json:"lyricsTranslate"`
	// LyricsRomanize shows the lyrics of a language in Latin letters,
	// RomanizeUnder or RomanizeOnly, by the language's code; lyrics of
	// other languages show as they are written.
	LyricsRomanize map[string]string `json:"lyricsRomanize"`
}

// defaultLyrics is how a new install shows the lyrics.
func defaultLyrics() Lyrics {
	var l Lyrics
	l.LyricsStyle = LyricStyles[0]
	l.LyricsSize = 1
	l.LyricsBall = true
	l.LyricsBallMotion = LyricBallMotions[0]
	l.LyricsTranslate = []string{}
	l.LyricsRomanize = map[string]string{}
	return l
}

// clean is l without the languages that aren't one.
func (l Lyrics) clean() Lyrics {
	if !slices.Contains(LyricStyles, l.LyricsStyle) {
		l.LyricsStyle = LyricStyles[0]
	}
	if !(l.LyricsSize >= MinLyricsSize && l.LyricsSize <= MaxLyricsSize) {
		l.LyricsSize = 1
	}
	l.LyricsSize = math.Round(l.LyricsSize*10) / 10
	if !slices.Contains(LyricBallMotions, l.LyricsBallMotion) {
		l.LyricsBallMotion = LyricBallMotions[0]
	}
	translate := []string{}
	for _, lang := range l.LyricsTranslate {
		if (lang == "" || languageRe.MatchString(lang)) && !slices.Contains(translate, lang) && len(translate) < 200 {
			translate = append(translate, lang)
		}
	}
	l.LyricsTranslate = translate
	romanize := map[string]string{}
	for lang, how := range l.LyricsRomanize {
		if (lang == "" || languageRe.MatchString(lang)) && (how == RomanizeUnder || how == RomanizeOnly) && len(romanize) < 200 {
			romanize[lang] = how
		}
	}
	l.LyricsRomanize = romanize
	return l
}

// Translation is the language lyrics translate to, as Settings › Playback
// › Lyrics sets it. It is apart from Lyrics, as settings.json has its key
// after the playback's.
type Translation struct {
	// LyricsTranslateTo is the language lyrics translate to, a code of ISO
	// 639-1; "" is the system's.
	LyricsTranslateTo string `json:"lyricsTranslateTo"`
}

// clean is t with the system's language in place of one that isn't one.
func (t Translation) clean() Translation {
	if !languageRe.MatchString(t.LyricsTranslateTo) {
		t.LyricsTranslateTo = ""
	}
	return t
}
