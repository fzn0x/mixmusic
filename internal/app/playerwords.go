package app

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/locales"
)

// The errors of what plays keep their English, for the logs and the Copy
// of a failure's details; the windows say those MixMusic knows in their
// language (#800). The engine's, the stream proxy's and the problems the
// page of MixMusic's own Spotify device names get their words here; the
// cast device's, the Spotify device's own and those of the files of this
// computer get theirs where they are made.

// engineWords is err, why the engine can't play a stream, or why the proxy
// couldn't serve it its audio, with the words the windows say of it, when
// MixMusic knows it: the errors the services and the system tell have
// theirs in errorText already.
func engineWords(err error) error {
	if err == nil {
		return nil
	}
	if said, ok := deckWords(err); ok {
		return worded(err.Error(), said, err)
	}
	var se *stream.ServeError
	switch {
	case errors.As(err, &se) && stream.Reason(err) == "":
		if said, ok := serveWords(se); ok {
			return worded(err.Error(), strings.Replace(err.Error(), se.Error(), said, 1), err)
		}
	case errors.Is(err, stream.ErrLoad):
		return worded(err.Error(), strings.Replace(err.Error(), stream.ErrLoad.Error(), locales.ErrorAudioNetwork(), 1), err)
	}
	return err
}

// deckWords are the words of err, a failure of the engine's decks, as the
// windows say it: the decoder's own words, which it may add, stay.
func deckWords(err error) (string, bool) {
	reason, part := audio.Reason(err)
	said, ok := deckText(reason)
	if !ok {
		return "", false
	}
	return strings.Replace(err.Error(), part, said, 1), true
}

// deckText is a reason of audio.Reason as the windows say it.
func deckText(reason string) (string, bool) {
	switch reason {
	case audio.ReasonNetwork:
		return locales.ErrorAudioNetwork(), true
	case audio.ReasonDecode:
		return locales.ErrorAudioDecode(), true
	case audio.ReasonUnsupported:
		return locales.ErrorAudioUnsupported(), true
	case audio.ReasonFile:
		return locales.ErrorAudioCantRead(), true
	}
	return "", false
}

// serveWords are the words of why the proxy couldn't serve a video's
// audio, as the windows say it, of what stream.Reason doesn't name.
func serveWords(se *stream.ServeError) (string, bool) {
	switch {
	case se.Status == http.StatusTooManyRequests:
		return locales.ErrorStreamThrottled(), true
	case se.Status != 0:
		return locales.ErrorStreamStatus(strconv.Itoa(se.Status)), true
	case se.Unreachable:
		return locales.ErrorStreamUnreachable(), true
	}
	return "", false
}

// hereProblems are the problems the page of MixMusic's own Spotify device
// names, public/spotify.html's, with their words in the windows.
var hereProblems = map[string]func() string{
	"drm":       locales.SpotifyHereNoDrm,
	"signIn":    locales.SpotifyHereSignInAgain,
	"premium":   locales.SpotifyHerePremium,
	"noSDK":     locales.SpotifyHereSdkNotLoaded,
	"noConnect": locales.SpotifyHereNotConnected,
}

// pageError is the error the page of MixMusic's own Spotify device threw,
// of message: one of its problems starts with its name in brackets, which
// the windows say in their language, and the error keeps its English.
func pageError(message string) error {
	if rest, ok := strings.CutPrefix(message, "["); ok {
		if name, text, ok := strings.Cut(rest, "] "); ok {
			if said, ok := hereProblems[name]; ok {
				return worded(text, said(), nil)
			}
			return errors.New(text)
		}
	}
	return errors.New(message)
}
