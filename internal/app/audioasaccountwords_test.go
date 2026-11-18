package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"

	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/locales"
)

// TestStreamAccountText says, in en-XA, why a track YouTube plays only to
// some signed-in viewers didn't play, with what to change for the
// account to be asked, or that it was, for each way asking as it went;
// and the words of today while the switch is off, signed in.
func TestStreamAccountText(t *testing.T) {
	was := i18n.Locale()
	i18n.SetLocaleFunc(func() string { return "en-XA" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
	for _, reason := range []string{stream.ReasonSignedIn, stream.ReasonAdults, stream.ReasonMembers, stream.ReasonPremium} {
		why, _ := streamReasonText(reason)
		for _, way := range []string{stream.AccountOff, stream.AccountSignIn, stream.AccountSignedOut, stream.AccountEnded,
			stream.AccountNotFirst, stream.AccountSaver, stream.AccountNoTools, stream.AccountRefused} {
			got, ok := streamAccountText(reason, way)
			switch {
			case !ok || !strings.HasPrefix(got, "["):
				t.Errorf("%s, %s says %q, which no catalog translates", reason, way, got)
			case reason == stream.ReasonSignedIn && way == stream.AccountOff:
				if got != locales.ErrorStreamSignedIn() {
					t.Errorf("signed in, off: %q", got)
				}
			case !strings.Contains(got, why):
				t.Errorf("%s, %s says %q, without why: %q", reason, way, got, why)
			}
		}
	}
	if _, ok := streamAccountText(stream.ReasonSignedIn, ""); ok {
		t.Error("words of no way")
	}
}
