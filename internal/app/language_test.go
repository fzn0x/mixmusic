package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestSaidOutsideTheWindows says the tray's menu and tooltip, the
// taskbar's buttons, the main window's title, the notifications of likes
// and the lyrics' notes in the app's language: under the pseudo locale, en-XA, each is a
// message of the catalog, in brackets (#775).
func TestSaidOutsideTheWindows(t *testing.T) {
	was := i18n.Locale()
	i18n.SetLocaleFunc(func() string { return "en-XA" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
	var said []string
	for _, item := range (&App{}).trayMenu() {
		if item.Label != "" {
			said = append(said, item.Label)
		}
	}
	tips := taskbarTips()
	said = append(said, tips.Play, tips.Pause, tips.Previous, tips.Next)
	liked, _ := likedNote(media.Track{Title: "Teardrop"}, true)
	unliked, _ := likedNote(media.Track{Title: "Teardrop"}, false)
	disliked, _ := dislikedNote(media.Track{Title: "Teardrop"}, true)
	undisliked, _ := dislikedNote(media.Track{Title: "Teardrop"}, false)
	said = append(said, liked, unliked, disliked, undisliked)
	paused := playingLine(NowPlaying{Track: &media.Track{Title: "Teardrop", Artists: []media.Ref{{Name: "Massive Attack"}}}})
	said = append(said, paused, videoNote(), translationError{}.Error())
	for _, s := range said {
		if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
			t.Errorf("%q isn't a message of the catalog", s)
		}
	}
	if !strings.Contains(paused, "Teardrop - Massive Attack") {
		t.Errorf("paused, the title is %q", paused)
	}
}
