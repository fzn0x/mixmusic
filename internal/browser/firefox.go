package browser

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// firefoxPrefs are the preferences of the profile, in its user.js: none
// of the pages and questions of a new profile's first run, Zen's welcome
// among them, no offer to keep the password the user types, and no
// restoring of a session the profile never has again.
var firefoxPrefs = []struct {
	name  string
	value any
}{
	{"browser.shell.checkDefaultBrowser", false},
	{"browser.startup.homepage_override.mstone", "ignore"},
	{"startup.homepage_welcome_url", ""},
	{"startup.homepage_welcome_url.additional", ""},
	{"browser.aboutwelcome.enabled", false},
	{"browser.preonboarding.enabled", false},
	{"termsofuse.bypassNotification", true},
	{"datareporting.policy.dataSubmissionPolicyBypassNotification", true},
	{"signon.rememberSignons", false},
	{"browser.sessionstore.resume_from_crash", false},
	{"toolkit.startup.max_resumed_crashes", -1},
	{"zen.welcome-screen.seen", true},
}

// userJS writes the profile's preferences as user.js has them.
func userJS() []byte {
	var b bytes.Buffer
	for _, p := range firefoxPrefs {
		switch v := p.value.(type) {
		case string:
			fmt.Fprintf(&b, "user_pref(%q, %q);\n", p.name, v)
		default:
			fmt.Fprintf(&b, "user_pref(%q, %v);\n", p.name, v)
		}
	}
	return b.Bytes()
}

// firefoxArgs are the arguments of a browser made from Firefox with the
// profile in dir, at url: an instance of its own, apart from the user's
// Firefox, if it runs. On Windows, Firefox's first process starts the
// browser's own and exits, unless told to wait for it.
func firefoxArgs(goos, dir, url string) []string {
	args := []string{"-no-remote", "-new-instance", "-profile", dir}
	if goos == "windows" {
		args = append(args, "-wait-for-browser")
	}
	args = append(args, extraArgs...)
	return append(args, url)
}

// firefox is a browser made from Firefox, whose cookies are read from its
// profile's cookie store.
type firefox struct {
	p   *process
	dir string
}

func startFirefox(ctx context.Context, b Browser, dir, url string) (session, error) {
	if err := os.WriteFile(filepath.Join(dir, "user.js"), userJS(), 0o600); err != nil {
		return nil, err
	}
	p, err := startProcess(b.Path, firefoxArgs(runtime.GOOS, dir, url), func(cmd *exec.Cmd) {
		// An update of Firefox the user's Firefox downloaded waits for
		// their Firefox to apply it, rather than this one, which would
		// start again once updated, apart from MixMusic. Nor does a crash
		// of this one, which only signs in, ask to be reported.
		setEnv(cmd, "MOZ_DISABLE_UPDATE_PROCESSING=1", "MOZ_CRASHREPORTER_DISABLE=1")
	})
	if err != nil {
		return nil, err
	}
	return &firefox{p: p, dir: dir}, nil
}

func (f *firefox) exited() <-chan struct{} { return f.p.done }

func (f *firefox) cookies(context.Context) ([]*http.Cookie, error) {
	return firefoxCookies(f.dir)
}

// quit ends the browser at once: its profile goes, and nothing of it is
// kept.
func (f *firefox) quit() { f.p.stop(0) }

// firefoxCookies reads the cookies of the profile in dir that the user's
// tabs send: those of no container or private window, and not kept for one
// site embedded in another. Firefox keeps those that outlast the browser
// in cookies.sqlite, whose last changes may be in its write-ahead log.
func firefoxCookies(dir string) ([]*http.Cookie, error) {
	db, err := readDatabase(filepath.Join(dir, "cookies.sqlite"))
	if err != nil {
		return nil, err
	}
	rows, err := db.table("moz_cookies")
	if err != nil {
		return nil, err
	}
	var cookies []*http.Cookie
	for _, r := range rows {
		if r.text("originAttributes") != "" {
			continue
		}
		c := &http.Cookie{
			Name: r.text("name"), Value: r.text("value"), Domain: r.text("host"), Path: r.text("path"),
			Secure: r.integer("isSecure") != 0, HttpOnly: r.integer("isHttpOnly") != 0,
		}
		// Firefox kept the time in seconds, then in milliseconds.
		if e := r.integer("expiry"); e > 1e11 {
			c.Expires = time.UnixMilli(e)
		} else if e > 0 {
			c.Expires = time.Unix(e, 0)
		}
		if c.Name != "" {
			cookies = append(cookies, c)
		}
	}
	return cookies, nil
}

// readDatabase reads an SQLite database with its write-ahead log, as
// SQLite would at that moment. Should SQLite move the log into the file
// and start a new one as it's read, it reads them again.
func readDatabase(file string) (*sqliteDB, error) {
	var err error
	for range 3 {
		var before, data, after []byte
		before, err = readOptional(file + "-wal")
		if err != nil {
			return nil, err
		}
		data, err = readOptional(file)
		if err != nil {
			return nil, err
		}
		if data == nil {
			return nil, os.ErrNotExist
		}
		after, err = readOptional(file + "-wal")
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(walHeader(before), walHeader(after)) {
			err = errors.New("the database's log started again as it was read")
			continue
		}
		return openSQLite(data, after)
	}
	return nil, err
}

// readOptional reads a file, or returns nil when it isn't there.
func readOptional(file string) ([]byte, error) {
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// walHeader returns the header of a write-ahead log, which changes when
// SQLite starts it again.
func walHeader(wal []byte) []byte {
	return wal[:min(len(wal), 32)]
}
