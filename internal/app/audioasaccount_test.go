package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/stream"
)

// premiumMenu answers as YouTube Music does to a session it takes, of an
// account with Premium when premium is set, as account-menu-premium.json
// of the youtube package has it: made-up names, the tracking parameters
// of a recorded answer.
func premiumMenu(premium bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"responseContext":{"serviceTrackingParams":[{"service":"GFEEDBACK","params":[
			{"key":"has_unlimited_entitlement","value":%q},{"key":"logged_in","value":"1"}]}]},
			"actions":[{"openPopupAction":{"popup":{"multiPageMenuRenderer":{"header":{"activeAccountHeaderRenderer":{
				"accountName":{"runs":[{"text":"Ada"}]}}}}}}}]}`, map[bool]string{true: "True", false: "False"}[premium])
	}
}

// asAccountYTDLP is a fake yt-dlp that keeps the cookies each run as the
// account read, and answers a stream of Opus.
type asAccountYTDLP struct {
	mu       sync.Mutex
	sessions []string // "" for a run signed out
}

func (y *asAccountYTDLP) run(_ context.Context, args ...string) ([]byte, error) {
	session := ""
	if i := slices.Index(args, "--cookies"); i >= 0 {
		data, err := os.ReadFile(args[i+1])
		if err != nil {
			return nil, err
		}
		session = string(data)
	}
	y.mu.Lock()
	y.sessions = append(y.sessions, session)
	y.mu.Unlock()
	return fmt.Appendf(nil, `{"url": "https://example.com/opus?expire=%d", "ext": "webm", "acodec": "opus", "abr": 256, "filesize": 1000}`,
		time.Now().Add(6*time.Hour).Unix()), nil
}

func (y *asAccountYTDLP) last() string {
	y.mu.Lock()
	defer y.mu.Unlock()
	if len(y.sessions) == 0 {
		return "no run"
	}
	return y.sessions[len(y.sessions)-1]
}

// TestAudioAsAccount asks for streams as the account signed in to YouTube
// Music, with its session, once "Play audio as your account" is on, and
// signed out while it's off, signed out, and while MixMusic uses another
// Google account of the session, which yt-dlp wouldn't ask as.
func TestAudioAsAccount(t *testing.T) {
	a, _ := cookieApp(t, premiumMenu(true))
	// No request leaves the computer, and whatever the computer has
	// installed, yt-dlp is the test's, with a runtime.
	a.resolver.Direct = nil
	a.resolver.Tools = nil
	y := &asAccountYTDLP{}
	a.resolver.Run = y.run
	ctx := context.Background()
	videos := []string{"aaaaaaaaaaa", "bbbbbbbbbbb", "ccccccccccc", "ddddddddddd", "eeeeeeeeeee"}
	resolve := func(i int) stream.Stream {
		t.Helper()
		s, err := a.resolver.Resolve(ctx, videos[i])
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	set := func(on bool) {
		t.Helper()
		if _, err := (&Preferences{a}).Update(func(s *settings.Settings) { s.AudioAsAccount = on }); err != nil {
			t.Fatal(err)
		}
	}

	// Signed out, on: signed out.
	set(true)
	if s := resolve(0); s.AsAccount || y.last() != "" {
		t.Errorf("signed out: as the account %v, session %q", s.AsAccount, y.last())
	}

	if _, err := (&Accounts{a}).YouTubePasteCookies(ctx, "SAPISID=madeup-sapisid; LOGIN_INFO=madeup-login; SID=madeup-sid"); err != nil {
		t.Fatal(err)
	}
	if st := a.accountsStatus(ctx); !st.YouTube.Premium {
		t.Errorf("the accounts don't tell Premium: %+v", st.YouTube)
	}
	s := resolve(1)
	if !s.AsAccount || !strings.Contains(y.last(), "\tLOGIN_INFO\tmadeup-login\n") || !strings.Contains(y.last(), "\tSAPISID\tmadeup-sapisid\n") {
		t.Errorf("signed in, on: as the account %v, session %q", s.AsAccount, y.last())
	}

	set(false)
	if s := resolve(1); s.AsAccount || y.last() != "" {
		t.Errorf("off: as the account %v, session %q", s.AsAccount, y.last())
	}

	set(true)
	a.youtube.SetAuthUser(1)
	if s := resolve(2); s.AsAccount || y.last() != "" {
		t.Errorf("another account of the session: as the account %v, session %q", s.AsAccount, y.last())
	}
	a.youtube.SetAuthUser(0)
	if s := resolve(3); !s.AsAccount {
		t.Errorf("the first account again: as the account %v, session %q", s.AsAccount, y.last())
	}

	// Signed out, as YouTubeLogout signs the client out; the rest of it
	// reaches the webview's cookies, which tests have none of.
	a.youtube.SetCookies(nil)
	if s := resolve(4); s.AsAccount || y.last() != "" {
		t.Errorf("signed out again: as the account %v, session %q", s.AsAccount, y.last())
	}
	if st := a.accountsStatus(ctx); st.YouTube.Premium {
		t.Errorf("Premium signed out: %+v", st.YouTube)
	}
}

// TestAccountsTellNoPremium tells no Premium of an account YouTube Music
// says hasn't it.
func TestAccountsTellNoPremium(t *testing.T) {
	a, _ := cookieApp(t, premiumMenu(false))
	if _, err := (&Accounts{a}).YouTubePasteCookies(context.Background(), "SAPISID=madeup; LOGIN_INFO=madeup"); err != nil {
		t.Fatal(err)
	}
	if st := a.accountsStatus(context.Background()); !st.YouTube.SignedIn || st.YouTube.Premium {
		t.Errorf("a free account: %+v", st.YouTube)
	}
}

// TestSessionFilesGoAtStart names the files that hand the session to
// yt-dlp as the copies of browsers' cookies are named, which MixMusic
// removes as it starts (TestRemoveCookieExports): so one a MixMusic that
// stopped during a run left goes too.
func TestSessionFilesGoAtStart(t *testing.T) {
	if stream.SessionFilePrefix != cookieExportPrefix {
		t.Errorf("the session's files begin %q, the copies of cookies %q", stream.SessionFilePrefix, cookieExportPrefix)
	}
}
