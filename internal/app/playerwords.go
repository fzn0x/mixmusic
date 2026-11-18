package app

import (
	"errors"
	"strings"

	"github.com/fzn0x/mixmusic/locales"
)

// The errors of what plays keep their English, for the logs and the Copy
// of a failure's details; the windows say those MixMusic knows in their
// language (#800). The engine's, the stream proxy's and the problems the
// page of MixMusic's own Spotify device names get their words here; the
// cast device's, the Spotify device's own and those of the files of this
// computer get theirs where they are made.

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
