package lastfm

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/scrobble"
)

// TestGNUFMErrors reads the errors as GNU FM writes them: as Last.fm
// does, or, for a call it failed on its side, as an object of the code,
// a string, and the message. A temporary one keeps the plays, a session
// refused waits for a new one, and the rest reject the plays.
func TestGNUFMErrors(t *testing.T) {
	for _, c := range []struct {
		body      string
		temporary bool
		rejected  bool
		refused   bool
		text      string
	}{
		{`{"error":{"#text":"The service is temporarily unavailable, please try again.","code":"16"}}`, true, false, false,
			"librefm: The service is temporarily unavailable, please try again. (error 16)"},
		{`{"error":{"#text":"Invalid resource specified","code":"7"}}`, false, true, false, "librefm: Invalid resource specified (error 7)"},
		{`{"error":{"code":11}}`, true, false, false, "librefm: error 11"},
		{`{"error":9,"message":"Invalid session key - Please re-authenticate"}`, false, false, true, ""},
		{`{"error":6,"message":"Invalid parameters - Your request is missing a required parameter"}`, false, true, false,
			"librefm: Invalid parameters - Your request is missing a required parameter (error 6)"},
	} {
		cl := testServer(t, func(url.Values) (int, string) { return http.StatusOK, c.body })
		cl.Name = "librefm"
		err := cl.Scrobble(context.Background(), "SK", []Play{{Track: Track{Artist: "A", Title: "T"}}})
		if err == nil {
			t.Errorf("%s: no error", c.body)
			continue
		}
		if IsTemporary(err) != c.temporary || scrobble.IsRejected(err) != c.rejected || errors.Is(err, scrobble.ErrKeyRefused) != c.refused {
			t.Errorf("%s: %v is temporary %v, rejected %v, refused %v", c.body, err, IsTemporary(err), scrobble.IsRejected(err), errors.Is(err, scrobble.ErrKeyRefused))
		}
		if c.text != "" && err.Error() != c.text {
			t.Errorf("%s: %q, want %q", c.body, err, c.text)
		}
	}
	// An answer without an error is none, whatever else it holds.
	cl := testServer(t, func(url.Values) (int, string) {
		return http.StatusOK, `{"scrobbles":{"@attr":{"accepted":"1","ignored":"0"}}}`
	})
	if err := cl.Scrobble(context.Background(), "SK", []Play{{Track: Track{Artist: "A", Title: "T"}}}); err != nil {
		t.Error(err)
	}
}

// TestAuthPage approves on another server's page, with the client's key.
func TestAuthPage(t *testing.T) {
	c := NewClient("KEY", "SECRET")
	c.AuthPage = "https://gnufm.example.com/api/auth/"
	if got, want := c.AuthURL("a b"), "https://gnufm.example.com/api/auth/?api_key=KEY&token=a+b"; got != want {
		t.Errorf("AuthURL = %s, want %s", got, want)
	}
}

// TestServiceName names the service in the errors of a call that got no
// answer, or one that isn't the API's.
func TestServiceName(t *testing.T) {
	c := NewClient("KEY", "SECRET")
	c.BaseURL = "http://127.0.0.1:1/2.0/"
	c.Name = "librefm"
	if err := c.NowPlaying(context.Background(), "SK", Track{Artist: "A", Title: "T"}); err == nil || !strings.HasPrefix(err.Error(), "librefm: ") {
		t.Errorf("no answer: %v", err)
	}
	c = testServer(t, func(url.Values) (int, string) { return http.StatusNotFound, "<html>Not Found</html>" })
	c.Name = "librefm"
	if _, err := c.Token(context.Background()); err == nil || err.Error() != "librefm: HTTP 404" {
		t.Errorf("no API: %v", err)
	}
	if err := (&APIError{Status: 503}).Error(); err != "lastfm: HTTP 503" {
		t.Errorf("Last.fm's: %q", err)
	}
}
