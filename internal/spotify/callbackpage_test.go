package spotify

import (
	"errors"
	"html"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"

	"github.com/fzn0x/mixmusic/locales"
)

// TestCallbackPageInTheLanguage tells in the browser, in the interface's
// language, how signing in to Spotify went: Spotify's own words stay.
func TestCallbackPageInTheLanguage(t *testing.T) {
	was := i18n.Locale()
	i18n.SetLocaleFunc(func() string { return "en-XA" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
	for err, want := range map[error][]string{
		nil:                         {locales.SpotifyPageSignedIn(), locales.SpotifyPageClose()},
		errors.New("access_denied"): {locales.SpotifyPageFailed(), "access_denied"},
	} {
		w := httptest.NewRecorder()
		writeCallbackPage(w, err)
		page := html.UnescapeString(w.Body.String())
		for _, text := range want {
			if !strings.Contains(page, text) {
				t.Errorf("%v: the page doesn't say %q: %s", err, text, page)
			}
		}
		if err == nil && strings.Contains(page, "Signed in to Spotify") {
			t.Errorf("the page is English: %s", page)
		}
	}
}
