package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"

	"github.com/fzn0x/mixmusic/internal/listenbrainz"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/mcp"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/internal/tools"
	"github.com/fzn0x/mixmusic/locales"
)

// TestOwnErrorsInTheLanguage says MixMusic's own errors in en-XA, as the
// windows show them, and keeps their English for the logs: those of
// updates, mix covers, the library, sign-ins, remote control and AI
// assistants.
func TestOwnErrorsInTheLanguage(t *testing.T) {
	was := i18n.Locale()
	i18n.SetLocaleFunc(func() string { return "en-XA" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
	limited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer limited.Close()
	_, githubLimit := latestRelease(context.Background(), limited.Client(), limited.URL, "deb", "amd64", "0.5.0")
	gone := cantReadImage(&fs.PathError{Op: "open", Path: filepath.Join(t.TempDir(), "gone.png"), Err: fs.ErrNotExist})
	for _, err := range []error{
		githubLimit, gone, errNotCover, errNoLogs, ErrNoJSONApp, errSignInToCollaborate, errNotSignedIn,
	} {
		got := ErrorText(err)
		if err == nil || !strings.HasPrefix(got, "[") {
			t.Errorf("%v says %q, which no catalog translates", err, got)
		}
		if err != nil && strings.Contains(err.Error(), "[") {
			t.Errorf("the log of %q isn't English", err.Error())
		}
	}
	listen := fmt.Errorf("%w: %w", mcp.ErrListen, errors.New("bind: address already in use"))
	if got := assistantProblem(listen); !strings.HasPrefix(got, "[") || !strings.Contains(got, "bind: address already in use") {
		t.Errorf("the AI assistants' server says %q", got)
	}
	for _, err := range []error{errKeyBusy, fmt.Errorf("starting: %w", errKeyBusy)} {
		if got := RoomErrorText(err); !strings.HasPrefix(got, "[") {
			t.Errorf("a room's %v says %q", err, got)
		}
	}
	why := fmt.Errorf("%w (fs.inotify.max_user_watches)", localfiles.ErrWatchLimit).Error()
	if rest, ok := strings.CutPrefix(why, localfiles.ErrWatchLimit.Error()); !ok || rest != " (fs.inotify.max_user_watches)" {
		t.Errorf("the watch limit's detail: %q", rest)
	}
}

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
