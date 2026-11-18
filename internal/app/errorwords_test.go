package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"

	"github.com/fzn0x/mixmusic/locales"
)

// TestErrorWords says, in en-XA, the errors of the accounts' flows, the
// webviews and the desktop's portal as the windows show them, while the
// logs keep their English.
func TestErrorWords(t *testing.T) {
	was := i18n.Locale()
	i18n.SetLocaleFunc(func() string { return "en-XA" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
	bracketed := func(s string) bool {
		return strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") && !strings.Contains(s, "] [")
	}
	var errs []error
	for _, goos := range []string{"windows", "linux", "darwin"} {
		for _, what := range []string{webViewSignIn, webViewSpotify, webViewVideos} {
			errs = append(errs, webViewMissing(what, goos))
		}
	}
	errs = append(errs, errListenBrainzToken, fmt.Errorf("connecting: %w", errPlainRemote),
		worded("connect to Last.fm first", locales.AccountSettingsLastfmConnectFirst(), nil),
		errSpotifyEpisode, fmt.Errorf("opening: %w", errSpotifyShows), errSpotifyLinks)
	for _, err := range errs {
		if got := ErrorWords(err); !bracketed(got) {
			t.Errorf("%v says %q, which no catalog translates", err, got)
		}
		if strings.Contains(err.Error(), "[") {
			t.Errorf("the log of %q isn't English", err.Error())
		}
	}
	if got := ErrorWords(errors.New("dial tcp: refused")); got != "dial tcp: refused" {
		t.Errorf("an error of no words says %q", got)
	}
	// WordsOf says an error itself, not the ones it wraps, for the windows'
	// errorText to keep the words around it.
	if got, ok := WordsOf(webViewMissing(webViewSpotify, "windows")); !ok || !bracketed(got) {
		t.Errorf("a missing webview says %q, %v", got, ok)
	}
	if got, ok := WordsOf(fmt.Errorf("opening: %w", errSpotifyShows)); ok {
		t.Errorf("an error that wraps a worded one says %q", got)
	}
	if got := webViewMissing(webViewVideos, "linux").Error(); !strings.HasPrefix(got, "Showing music videos needs WebKitGTK") {
		t.Errorf("the log says %q", got)
	}
	for _, f := range []*TrackFile{readTrackFile(FileMusic, filepath.Join(t.TempDir(), "gone.mp3"), false)} {
		if !bracketed(f.Problem) {
			t.Errorf("track info's problem %q, which no catalog translates", f.Problem)
		}
	}
}
