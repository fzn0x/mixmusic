package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"

	"github.com/fzn0x/mixmusic/internal/listenbrainz"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/internal/tools"
	"github.com/fzn0x/mixmusic/locales"
)

// TestStreamReasonText says each reason of internal/stream in en-XA.
func TestStreamReasonText(t *testing.T) {
	was := i18n.Locale()
	i18n.SetLocaleFunc(func() string { return "en-XA" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
	for _, reason := range []string{stream.ReasonBot, stream.ReasonThrottled, stream.ReasonAdults, stream.ReasonSignedIn,
		stream.ReasonCountry, stream.ReasonPrivate, stream.ReasonMembers, stream.ReasonPremium, stream.ReasonNoFormat,
		stream.ReasonGone, stream.ReasonRefused, stream.ReasonStalled} {
		if got, ok := streamReasonText(reason); !ok || !strings.HasPrefix(got, "[") {
			t.Errorf("the reason %q says %q", reason, got)
		}
	}
}

// TestErrorWordsKnowsServices says, through ErrorWords, the errors of the
// services and the system ErrorText knows, as the accounts' flows meet
// them, and a worded error's words first.
func TestErrorWordsKnowsServices(t *testing.T) {
	was := i18n.Locale()
	i18n.SetLocaleFunc(func() string { return "en-XA" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
	for _, err := range []error{listenbrainz.ErrNotServer, fmt.Errorf("importing: %w", tools.ErrNoYTDLP)} {
		if got := ErrorWords(err); !strings.Contains(got, "[") {
			t.Errorf("%v says %q, which no catalog translates", err, got)
		}
	}
	w := worded("connect to Last.fm first", locales.AccountSettingsLastfmConnectFirst(), tools.ErrNoYTDLP)
	if got := ErrorWords(w); got != locales.AccountSettingsLastfmConnectFirst() {
		t.Errorf("a worded error says %q", got)
	}
	if got := ErrorWords(errors.New("dial tcp: refused")); got != "dial tcp: refused" {
		t.Errorf("an error of no words says %q", got)
	}
}
