package browser

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// liveBrowser is the browser MIXMUSIC_SIGNIN_BROWSER names, by its
// program, for the tests that run one; its window shows only while
// MIXMUSIC_SIGNIN_HEADLESS isn't set: Firefox, Zen, or another made from
// Firefox.
func liveBrowser(t *testing.T) Browser {
	t.Helper()
	program := os.Getenv("MIXMUSIC_SIGNIN_BROWSER")
	if program == "" {
		t.Skip("MIXMUSIC_SIGNIN_BROWSER names no browser to run")
	}
	b := Browser{ID: "live", Name: "the browser", Engine: Firefox, Path: program}
	if os.Getenv("MIXMUSIC_SIGNIN_HEADLESS") != "" {
		saved := extraArgs
		extraArgs = []string{"--headless"}
		t.Cleanup(func() { extraArgs = saved })
	}
	return b
}

// signInPage is a sign-in of this computer, in place of Google's: its
// first page goes on to /in, as a sign-in comes back to the site, which
// sets the session's cookie, then another a moment later; /close closes
// its window, as the user would.
func signInPage(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`<title>Sign in</title><p>Signing in…<script>setTimeout(() => location = "/in", 1500)</script>`))
		case "/in":
			http.SetCookie(w, &http.Cookie{Name: "SAPISID", Value: "test-session", Path: "/", MaxAge: 600, HttpOnly: true})
			w.Write([]byte(`<title>Signed in</title><p>Signed in<script>setTimeout(() => document.cookie = "LATE=1; path=/; max-age=600", 500)</script>`))
		case "/close":
			w.Write([]byte(`<title>Closing</title><script>setTimeout(() => window.close(), 1000)</script>`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// leftProfiles returns the profiles left in the temporary folder.
func leftProfiles(t *testing.T) []string {
	t.Helper()
	left, err := filepath.Glob(filepath.Join(os.TempDir(), profilePrefix+"*"))
	if err != nil {
		t.Fatal(err)
	}
	return left
}

// TestLiveSignIn signs in, in a real browser, on a page of this computer,
// and takes its cookies; then the browser is closed, and its profile gone.
func TestLiveSignIn(t *testing.T) {
	b := liveBrowser(t)
	srv := signInPage(t)
	before := len(leftProfiles(t))
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	start := time.Now()
	cookies, err := SignIn(ctx, b, Options{URL: srv.URL, Every: 500 * time.Millisecond, SignedIn: func(cookies []*http.Cookie) bool {
		for _, c := range cookies {
			if c.Name == "SAPISID" && c.Value == "test-session" {
				return true
			}
		}
		return false
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("signed in after %s", time.Since(start).Round(time.Millisecond))
	var names []string
	for _, c := range cookies {
		names = append(names, c.Name+"@"+c.Domain+c.Path)
		if c.Name == "SAPISID" && (!c.HttpOnly || c.Expires.Before(time.Now().Add(5*time.Minute))) {
			t.Errorf("SAPISID came as %+v", c)
		}
	}
	t.Logf("cookies: %q", names)
	if len(cookies) == 0 {
		t.Error("no cookies")
	}
	if left := leftProfiles(t); len(left) != before {
		t.Errorf("profiles left: %q", left)
	}
}

// TestLiveClosed ends a sign-in once the user closes the browser's window,
// which leaves Firefox running on macOS: the user quits it there.
func TestLiveClosed(t *testing.T) {
	b := liveBrowser(t)
	if runtime.GOOS == "darwin" {
		t.Skip("Firefox goes on running on macOS once its last window closes")
	}
	srv := signInPage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	_, err := SignIn(ctx, b, Options{URL: srv.URL + "/close", Every: 500 * time.Millisecond, SignedIn: func([]*http.Cookie) bool { return false }})
	if !errors.Is(err, ErrClosed) {
		t.Errorf("closing the window gave %v", err)
	}
}
