package app

import (
	"errors"

	"github.com/fzn0x/mixmusic/locales"
)

// The errors MixMusic makes of the accounts' flows, the webviews and the
// desktop's portal keep their English for the logs, and carry the words
// the windows show, in the interface's language (#800).

// wordedError is an error of text, in English, which the windows show as
// said, wrapping the error it tells of, if any.
type wordedError struct {
	text, said string
	err        error
}

func worded(text, said string, err error) error { return &wordedError{text, said, err} }
func (e *wordedError) Error() string            { return e.text }
func (e *wordedError) Unwrap() error            { return e.err }

// ErrorWords is err as the windows show it: the words of the first error
// of its chain that MixMusic says in the interface's language, or err's
// own.
func ErrorWords(err error) string {
	var w *wordedError
	switch {
	case err == nil:
		return ""
	case errors.As(err, &w):
		return w.said
	case errors.Is(err, errListenBrainzToken):
		return locales.ListenBrainzSettingsTokenRefused()
	case errors.Is(err, errPlainRemote):
		return locales.ListenBrainzSettingsPlainRemote()
	}
	return err.Error()
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
