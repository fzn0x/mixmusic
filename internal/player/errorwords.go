package player

import (
	"errors"
	"strings"
	"sync/atomic"
	"unicode"
	"unicode/utf8"

	"github.com/fzn0x/mixmusic/locales"
)

// errorWords says the errors the notices and a failure's details tell, as
// the windows do in their language (#800). The logs, the state's Error and
// the Copy of a failure's details keep the errors' English.
type errorWords struct {
	words atomic.Pointer[func(error) string]
}

// errSpotifyNotHere is why a Spotify track plays from YouTube Music where
// MixMusic can't play Spotify itself.
var errSpotifyNotHere = errors.New("Spotify can't play inside MixMusic on this computer")

// say is err in the words set, or as Go says it; the player's own errors
// it says in the interface's language.
func (w *errorWords) say(err error) string {
	if err == errSpotifyNotHere {
		return locales.PlayerSpotifyNotHere()
	}
	if fn := w.words.Load(); fn != nil && *fn != nil {
		return (*fn)(err)
	}
	return err.Error()
}

// SetErrorWords has the player say the errors of its notices with words,
// as the windows do in their language; nil says them as Go does.
func (p *Player) SetErrorWords(words func(error) string) { p.errorWords.words.Store(&words) }

// sentenceStart is text with its first letter a capital, for a reason
// that opens a notice: the reasons' messages start in lowercase, as most
// sit inside another. yt-dlp keeps its name.
func sentenceStart(text string) string {
	if strings.HasPrefix(text, "yt-dlp") {
		return text
	}
	r, n := utf8.DecodeRuneInString(text)
	return string(unicode.ToUpper(r)) + text[n:]
}
