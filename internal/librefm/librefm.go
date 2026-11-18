// Package librefm scrobbles what MixMusic plays to Libre.fm, or to
// another server of GNU FM, the free software Libre.fm runs.
//
// GNU FM takes the calls of Last.fm's API 2.0, which lastfm.Client makes:
// the package gives a client the server's addresses and an API account.
// GNU FM's own pages don't give the addresses; they are the paths of its
// code, /2.0/ for the API and /api/auth/ for the page where the user
// approves MixMusic.
package librefm

import (
	"errors"
	"net/url"
	"strings"

	"github.com/fzn0x/mixmusic/internal/lastfm"
)

// DefaultServer is Libre.fm's address.
const DefaultServer = "https://libre.fm"

// APIKey and Secret are MixMusic's own API account. GNU FM checks only
// that a key and a signature are 32 characters long, as it can't know
// Last.fm's shared secrets, so there is no account to create: the key is
// the MD5 of "MixMusic", and the secret signs calls as Last.fm's would.
const (
	APIKey = "8245b80b7b552009058dd9e2dca8b949"
	Secret = "9d7059ddcec20d9d190d1a9aa1b1f7ed"
)

// API is the address of the API of the server at server.
func API(server string) string { return server + "/2.0/" }

// AuthPage is the address of the page where the user approves a token on
// the server at server.
func AuthPage(server string) string { return server + "/api/auth/" }

// New returns a client of the API of the server at server, with the API
// account of apiKey and secret, or MixMusic's own when apiKey is "". Its
// calls introduce MixMusic as userAgent.
func New(server, apiKey, secret, userAgent string) *lastfm.Client {
	if apiKey == "" {
		apiKey, secret = APIKey, Secret
	}
	c := lastfm.NewClient(apiKey, secret)
	c.BaseURL, c.AuthPage = API(server), AuthPage(server)
	c.UserAgent, c.Name = userAgent, "librefm"
	return c
}

// ErrNotServer is ParseServer's error for an address that isn't a web
// address.
var ErrNotServer = errors.New("this isn't the address of a server: write it as https://gnufm.example.com")

// ParseServer returns the address of the server that addr, as the user
// wrote it, names: "" is DefaultServer, and an address without a scheme
// is one of https. A trailing /2.0 or /api/auth goes, so that addr may be
// the API's address or the approval page's. plain reports whether calls
// go over plain http, unencrypted.
func ParseServer(addr string) (server string, plain bool, err error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return DefaultServer, false, nil
	}
	if !strings.Contains(addr, "://") {
		addr = "https://" + addr
	}
	u, err := url.Parse(addr)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", false, ErrNotServer
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", false, ErrNotServer
	}
	u.Host = strings.ToLower(u.Host)
	path := strings.TrimRight(u.Path, "/")
	for _, end := range []string{"/2.0", "/api/auth"} {
		path = strings.TrimSuffix(path, end)
	}
	u.Path, u.RawPath = path, ""
	server = strings.TrimRight(u.String(), "/")
	if server == "https://www.libre.fm" {
		server = DefaultServer
	}
	return server, u.Scheme == "http", nil
}
