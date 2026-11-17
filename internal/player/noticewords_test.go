package player

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"

	"github.com/fzn0x/mixmusic/internal/sponsorblock"
	"github.com/fzn0x/mixmusic/locales"
)

// TestNoticeWords says the player's notices in the app's language, each
// one message (#775): under the pseudo locale, en-XA, in brackets; and in
// English as before, but for the count of kinds of parts, now in digits.
func TestNoticeWords(t *testing.T) {
	en := locales.In("en")
	for _, tc := range []struct{ got, want string }{
		{en.PlayerSkippedParts(0, "", ""), "Skipped a part"},
		{en.PlayerSkippedParts(1, en.PlayerPart("intro"), ""), "Skipped the intro"},
		{en.PlayerSkippedParts(2, en.PlayerPart("intro"), en.PlayerPart("sponsor")), "Skipped the intro and a sponsor read"},
		{en.PlayerSkippedParts(3, en.PlayerPart("intro"), en.PlayerPart("sponsor")), "Skipped parts of 3 kinds"},
		{en.PlayerRestPassedOver("disliked"), "The rest of the queue is songs you dislike or YouTube labels as made with AI"},
		{en.PlayerRestPassedOver("ai"), "The rest of the queue is songs YouTube labels as made with AI"},
		{en.PlayerCouldntPlayStopped("Teardrop", "this track is private", 3), "Couldn't play Teardrop: this track is private. Stopped, as 3 tracks in a row couldn't play"},
	} {
		if tc.got != tc.want {
			t.Errorf("%q, want %q", tc.got, tc.want)
		}
	}

	was := i18n.Locale()
	i18n.SetLocaleFunc(func() string { return "en-XA" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
	said := []string{pausedChecks(), allPassedOver([]Item{{Skip: SkipDisliked}}), allPassedOver([]Item{{Skip: SkipExplicit}}),
		allPassedOver([]Item{{Skip: SkipMadeWithAI}}), allPassedOver([]Item{{Skip: SkipMadeWithAI}, {Skip: SkipDisliked}})}
	for k := range partKinds {
		said = append(said, partOf(k))
	}
	said = append(said, partOf(sponsorblock.Kind("new")))
	for _, s := range said {
		if !strings.HasPrefix(s, "[") {
			t.Errorf("%q isn't a message of the catalog", s)
		}
	}
}
