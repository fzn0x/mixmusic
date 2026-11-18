package app

import (
	"errors"

	"github.com/fzn0x/mixmusic/locales"
)

// The errors MixMusic makes of the accounts' flows, the webviews and the
// desktop's portal keep their English for the logs, and carry the words
// the windows show, in the interface's language (#800).

// wordedError is an error of text, in English, which the windows show as
// say says, wrapping the error it tells of, if any.
type wordedError struct {
	text string
	say  func() string
	err  error
}

func worded(text, said string, err error) error {
	return &wordedError{text, func() string { return said }, err}
}

// wordedLater is worded, whose words say gives when the windows show it, in
// the language of then: for an error a state keeps, which the windows show
// later.
func wordedLater(text string, say func() string, err error) error {
	return &wordedError{text, say, err}
}
func (e *wordedError) Error() string { return e.text }
func (e *wordedError) Unwrap() error { return e.err }

// ErrorWords is err as the windows show it: the words of the first error
// of its chain that MixMusic says in the interface's language, alone; or,
// for an error of the services or the system, ErrorText; or err's own
// words.
func ErrorWords(err error) string {
	var w *wordedError
	switch {
	case err == nil:
		return ""
	case errors.As(err, &w):
		return w.say()
	}
	for _, known := range ownErrors {
		if errors.Is(err, known) {
			said, _ := WordsOf(known)
			return said
		}
	}
	return ErrorText(err)
}

// ownErrors are MixMusic's errors WordsOf has words for, besides the
// worded ones.
var ownErrors = []error{errListenBrainzToken, errPlainRemote, errPlainGNUFM, errSpotifyEpisode, errSpotifyShows, errNotCover, errNoLogs,
	errSignInToCollaborate, errNotSignedIn, ErrNoJSONApp}

// WordsOf is err itself, not the errors it wraps, as the windows show it,
// when MixMusic says it in the interface's language: the words of a worded
// error, or the message of one of ownErrors. The windows' errorText asks it
// of each error of a chain.
func WordsOf(err error) (string, bool) {
	if w, ok := err.(*wordedError); ok {
		return w.say(), true
	}
	switch err {
	case errListenBrainzToken:
		return locales.ListenBrainzSettingsTokenRefused(), true
	case errPlainRemote:
		return locales.ListenBrainzSettingsPlainRemote(), true
	case errPlainGNUFM:
		return locales.LibrefmSettingsPlainRemote(), true
	case errSpotifyEpisode:
		return locales.PodcastsSpotifyOnly(locales.PodcastsSettingsPlayback()), true
	case errSpotifyShows:
		return locales.PodcastsSpotifySignIn(), true
	case errNotCover:
		return locales.MixCoverNotImage(), true
	case errNoLogs:
		return locales.AboutNoLogs(), true
	case errSignInToCollaborate:
		return locales.CollabSignInFirst(), true
	case errNotSignedIn:
		return locales.LibrarySignInToKeep(), true
	case ErrNoJSONApp:
		return locales.DesktopSettingsNoJsonApp(), true
	}
	return "", false
}

// What needs a webview, which needWebView names.
const (
	webViewSignIn  = "signing in to YouTube Music"
	webViewSpotify = "playing Spotify in MixMusic"
	webViewVideos  = "Showing music videos"
)

// webViewMissing tells that what needs the webview of the system goos,
// which this computer doesn't have.
func webViewMissing(what, goos string) error {
	feature := map[string]string{webViewSignIn: "signIn", webViewSpotify: "spotify", webViewVideos: "videos"}[what]
	switch goos {
	case "windows":
		return worded(what+" needs Microsoft's WebView2 Runtime, which this computer doesn't have: get it from https://go.microsoft.com/fwlink/p/?LinkId=2124703",
			locales.WebViewMissingWindows(feature), nil)
	case "linux":
		return worded(what+" needs WebKitGTK, which this computer doesn't have: install libwebkit2gtk-4.1-0 on Debian and Ubuntu, webkit2gtk4.1 on Fedora",
			locales.WebViewMissingLinux(feature), nil)
	}
	return worded(what+" needs the system's webview, which this computer doesn't have", locales.WebViewMissing(feature), nil)
}
