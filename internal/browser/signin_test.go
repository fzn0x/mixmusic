package browser

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSession is a browser whose cookies reads gives, read after read, the
// last again once they run out, failing as errs say; it exits when exit is
// closed.
type fakeSession struct {
	mu     sync.Mutex
	reads  [][]*http.Cookie
	errs   []error
	exit   chan struct{}
	quits  int
	dir    string
	served int
}

func (f *fakeSession) cookies(context.Context) ([]*http.Cookie, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	i := min(f.served, len(f.reads)-1)
	var err error
	if f.served < len(f.errs) {
		err = f.errs[f.served]
	}
	f.served++
	if i < 0 {
		return nil, err
	}
	return f.reads[i], err
}

func (f *fakeSession) exited() <-chan struct{} { return f.exit }

func (f *fakeSession) quit() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.quits++
}

// fakeStart has SignIn start s in place of a browser, in a temporary folder
// of the test's own, and checks the profile it gets.
func fakeStart(t *testing.T, s *fakeSession, fail error) {
	t.Helper()
	tmp := t.TempDir()
	for _, v := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(v, tmp)
	}
	saved := starters[Firefox]
	starters[Firefox] = func(_ context.Context, b Browser, dir, url string) (session, error) {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() || !strings.HasPrefix(filepath.Base(dir), profilePrefix) {
			t.Errorf("the profile is %q: %v", dir, err)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("the new profile holds %d files", len(entries))
		}
		if url != "https://example.com/sign-in" {
			t.Errorf("the browser opened at %q", url)
		}
		s.dir = dir
		os.WriteFile(filepath.Join(dir, "Cookies"), []byte("the session"), 0o600)
		return s, fail
	}
	t.Cleanup(func() { starters[Firefox] = saved })
}

func cookie(name, value string) *http.Cookie {
	return &http.Cookie{Name: name, Value: value, Domain: ".example.com", Path: "/"}
}

// signedIn tells a sign-in by its SAPISID.
func signedIn(cookies []*http.Cookie) bool {
	return slices.ContainsFunc(cookies, func(c *http.Cookie) bool { return c.Name == "SAPISID" })
}

var testBrowser = Browser{ID: "firefox", Name: "Firefox", Engine: Firefox, Path: "firefox"}

func signIn(ctx context.Context) ([]*http.Cookie, error) {
	return SignIn(ctx, testBrowser, Options{URL: "https://example.com/sign-in", SignedIn: signedIn, Every: time.Millisecond})
}

// TestSignIn waits for the cookies of a sign-in, then for them to stop
// changing, as the site sets more once back from the sign-in; then closes
// the browser and deletes its profile.
func TestSignIn(t *testing.T) {
	s := &fakeSession{exit: make(chan struct{}), reads: [][]*http.Cookie{
		{cookie("PREF", "1")},
		{cookie("PREF", "1"), cookie("SAPISID", "a")},
		{cookie("PREF", "1"), cookie("SAPISID", "a"), cookie("LOGIN_INFO", "b")},
		{cookie("PREF", "1"), cookie("SAPISID", "a"), cookie("LOGIN_INFO", "b")},
	}, errs: []error{errors.New("the store is being written")}}
	fakeStart(t, s, nil)
	got, err := signIn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range got {
		names = append(names, c.Name)
	}
	if !slices.Equal(names, []string{"PREF", "SAPISID", "LOGIN_INFO"}) || s.served != 4 {
		t.Errorf("took %q after %d reads", names, s.served)
	}
	if s.quits != 1 {
		t.Errorf("the browser was closed %d times", s.quits)
	}
	if _, err := os.Stat(s.dir); !os.IsNotExist(err) {
		t.Errorf("the profile is left: %v", err)
	}
}

// TestSignInKeepsChanging takes the cookies once they kept changing for a
// few reads: the site may change one each time.
func TestSignInKeepsChanging(t *testing.T) {
	s := &fakeSession{exit: make(chan struct{})}
	for i := range 20 {
		s.reads = append(s.reads, []*http.Cookie{cookie("SAPISID", "a"), cookie("SIDCC", strings.Repeat("x", i))})
	}
	fakeStart(t, s, nil)
	if _, err := signIn(context.Background()); err != nil || s.served != settleReads+1 {
		t.Errorf("signing in gave %v after %d reads", err, s.served)
	}
}

// TestSignInEnds ends a sign-in when the user closes the browser, or its
// last window, when the caller cancels it, or its time is up, and when the
// browser can't start: each time the profile goes.
func TestSignInEnds(t *testing.T) {
	waiting := [][]*http.Cookie{{cookie("PREF", "1")}}
	for _, c := range []struct {
		name  string
		s     *fakeSession
		fail  error
		ctx   func() (context.Context, context.CancelFunc)
		want  error
		quits int
	}{
		{name: "exited", s: &fakeSession{exit: closed(), reads: waiting}, want: ErrClosed, quits: 1},
		{name: "windows closed", s: &fakeSession{exit: make(chan struct{}), reads: waiting, errs: []error{nil, errWindowsClosed}}, want: ErrClosed, quits: 1},
		{name: "cancelled", s: &fakeSession{exit: make(chan struct{}), reads: waiting}, want: context.Canceled, quits: 1,
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				time.AfterFunc(20*time.Millisecond, cancel)
				return ctx, cancel
			}},
		{name: "time is up", s: &fakeSession{exit: make(chan struct{}), reads: waiting}, want: context.DeadlineExceeded, quits: 1,
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 20*time.Millisecond)
			}},
		{name: "doesn't start", s: &fakeSession{exit: make(chan struct{})}, fail: errors.New("no such program"), want: errors.New("no such program")},
	} {
		t.Run(c.name, func(t *testing.T) {
			fakeStart(t, c.s, c.fail)
			ctx, cancel := context.Background(), context.CancelFunc(func() {})
			if c.ctx != nil {
				ctx, cancel = c.ctx()
			}
			defer cancel()
			_, err := signIn(ctx)
			if !errors.Is(err, c.want) && (err == nil || err.Error() != c.want.Error()) {
				t.Errorf("ended with %v, want %v", err, c.want)
			}
			if c.s.quits != c.quits {
				t.Errorf("the browser was closed %d times", c.s.quits)
			}
			if _, err := os.Stat(c.s.dir); !os.IsNotExist(err) {
				t.Errorf("the profile is left: %v", err)
			}
		})
	}
}

func closed() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

// TestSnapProfile makes the profile of a snap's browser in the snap's own
// folder, which its sandbox sees, and the leftovers of both kinds go at
// the next start.
func TestSnapProfile(t *testing.T) {
	h := t.TempDir()
	saved := home
	home = func() (string, error) { return h, nil }
	t.Cleanup(func() { home = saved })
	tmp := t.TempDir()
	for _, v := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(v, tmp)
	}
	dir, err := newProfile(Browser{Snap: "firefox"})
	if err != nil {
		t.Fatal(err)
	}
	if parent := filepath.Dir(dir); parent != filepath.Join(h, "snap", "firefox", "common") {
		t.Errorf("the snap's profile is in %q", parent)
	}
	other, err := newProfile(Browser{})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(other) != filepath.Clean(os.TempDir()) {
		t.Errorf("the profile is in %q, not the temporary folder %q", filepath.Dir(other), os.TempDir())
	}
	keep := filepath.Join(tmp, "someone-elses")
	os.Mkdir(keep, 0o700)
	os.WriteFile(filepath.Join(dir, "cookies.sqlite"), []byte("x"), 0o600)
	RemoveLeftovers()
	for _, d := range []string{dir, other} {
		if _, err := os.Stat(d); !os.IsNotExist(err) {
			t.Errorf("%s is left: %v", d, err)
		}
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("another folder went: %v", err)
	}
}

// TestArgs opens Firefox with its own profile, apart from the user's
// browser, at the page, without the questions of a first run.
func TestArgs(t *testing.T) {
	if got := strings.Join(firefoxArgs("windows", `C:\p`, "u"), " "); got != `-no-remote -new-instance -profile C:\p -wait-for-browser u` {
		t.Errorf("Firefox's arguments on Windows are %q", got)
	}
	if got := strings.Join(firefoxArgs("linux", "/p", "u"), " "); got != "-no-remote -new-instance -profile /p u" {
		t.Errorf("Firefox's arguments on Linux are %q", got)
	}
	js := string(userJS())
	for _, want := range []string{`user_pref("signon.rememberSignons", false);`, `user_pref("browser.startup.homepage_override.mstone", "ignore");`,
		`user_pref("zen.welcome-screen.seen", true);`, `user_pref("toolkit.startup.max_resumed_crashes", -1);`} {
		if !strings.Contains(js, want) {
			t.Errorf("user.js lacks %s:\n%s", want, js)
		}
	}
}
