// Package librefmtest is a fake server of GNU FM, the software Libre.fm
// runs, for tests: the calls of its API 2.0 that MixMusic makes, and its
// approval page, answered as GNU FM's code answers them.
package librefmtest

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
)

// Server is a GNU FM server whose users approve tokens as the test says.
type Server struct {
	// URL is the server's address, without a trailing slash.
	URL string

	mu sync.Mutex
	// tokens are the tokens given, with the user who approved each, ""
	// until one did; a token exchanged for a session goes.
	tokens map[string]string
	// sessions are the session keys given, with their users.
	sessions map[string]string
	// fail, when not 0, is the error every call of the API fails with,
	// as GNU FM writes the errors of the calls it fails on its side.
	fail int
	// heard are the calls of the API, as "method user artist track",
	// and pages the approval pages asked for.
	heard []string
	pages []url.Values
	keys  []string
}

// New starts a server, which closes when the test ends.
func New(t testing.TB) *Server {
	t.Helper()
	s := &Server{tokens: map[string]string{}, sessions: map[string]string{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/2.0/", s.api)
	mux.HandleFunc("/api/auth/", s.auth)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	s.URL = srv.URL
	return s
}

// Approve has user approve token on the approval page, as signing in
// there does. It reports whether the server gave the token.
func (s *Server) Approve(token, user string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tokens[token]; !ok {
		return false
	}
	s.tokens[token] = user
	return true
}

// Revoke ends the sessions of user, as GNU FM refuses a key it doesn't
// know.
func (s *Server) Revoke(user string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, u := range s.sessions {
		if u == user {
			delete(s.sessions, key)
		}
	}
}

// Fail has every call of the API fail with code, as GNU FM writes the
// errors of the calls it fails on its side, or answer again with 0.
func (s *Server) Fail(code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fail = code
}

// Heard returns the calls of the API that the server took, each as
// "method user artist track", in order; a scrobble of several plays is
// one of each.
func (s *Server) Heard() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.heard)
}

// Pages returns the queries of the approval pages asked for.
func (s *Server) Pages() []url.Values {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.pages)
}

// Keys returns the API keys the calls came with.
func (s *Server) Keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.keys)
}

// auth is the approval page, which takes the API key and the token.
func (s *Server) auth(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	s.mu.Lock()
	s.pages = append(s.pages, q)
	user, ok := s.tokens[q.Get("token")]
	s.mu.Unlock()
	if q.Get("api_key") == "" || !ok || user != "" {
		io.WriteString(w, "<p>Invalid token</p>")
		return
	}
	io.WriteString(w, "<form method=post><input name=username><input name=password type=password></form>")
}

// api answers a call of the API 2.0, as nixtape/2.0/index.php does.
func (s *Server) api(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	form := r.Form
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys = append(s.keys, form.Get("api_key"))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if form.Get("format") != "json" {
		io.WriteString(w, `<lfm status="failed"><error code="5">Invalid format</error></lfm>`)
		return
	}
	if s.fail != 0 {
		writeJSON(w, map[string]any{"error": map[string]string{"#text": "The service is temporarily unavailable, please try again.", "code": fmt.Sprint(s.fail)}})
		return
	}
	switch method := strings.ToLower(form.Get("method")); method {
	case "auth.gettoken":
		token := newKey()
		s.tokens[token] = ""
		writeJSON(w, map[string]any{"token": token})
	case "auth.getsession":
		user := s.tokens[form.Get("token")]
		if user == "" {
			failure(w, 14, "This token has not yet been authorised")
			return
		}
		delete(s.tokens, form.Get("token"))
		key := newKey()
		s.sessions[key] = user
		writeJSON(w, map[string]any{"session": map[string]any{"name": user, "key": key, "subscriber": 0}})
	case "track.updatenowplaying", "track.love", "track.unlove":
		if r.PostForm.Get("artist") == "" || r.PostForm.Get("track") == "" {
			failure(w, 6, "Invalid parameters - Your request is missing a required parameter")
			return
		}
		user, ok := s.sessions[form.Get("sk")]
		if !ok {
			failure(w, 9, "Invalid session key - Please re-authenticate")
			return
		}
		s.heard = append(s.heard, method+" "+user+" "+r.PostForm.Get("artist")+" "+r.PostForm.Get("track"))
		if method == "track.updatenowplaying" {
			writeJSON(w, map[string]any{"nowplaying": map[string]any{"track": map[string]string{"#text": r.PostForm.Get("track"), "corrected": "0"}}})
			return
		}
		writeJSON(w, map[string]any{})
	case "track.scrobble":
		if r.PostForm.Get("artist[0]") == "" || r.PostForm.Get("timestamp[0]") == "" {
			failure(w, 6, "Invalid parameters - Your request is missing a required parameter")
			return
		}
		user, ok := s.sessions[form.Get("sk")]
		if !ok {
			failure(w, 9, "Invalid session key - Please re-authenticate")
			return
		}
		n := 0
		for ; r.PostForm.Has(fmt.Sprintf("artist[%d]", n)); n++ {
			s.heard = append(s.heard, method+" "+user+" "+r.PostForm.Get(fmt.Sprintf("artist[%d]", n))+" "+r.PostForm.Get(fmt.Sprintf("track[%d]", n)))
		}
		writeJSON(w, map[string]any{"scrobbles": map[string]any{"@attr": map[string]string{"accepted": fmt.Sprint(n), "ignored": "0"}}})
	default:
		failure(w, 3, "Invalid Method - No method with that name in this package")
	}
}

// failure writes an error as report_failure does.
func failure(w http.ResponseWriter, code int, message string) {
	writeJSON(w, map[string]any{"error": code, "message": message})
}

func writeJSON(w http.ResponseWriter, v any) {
	json.NewEncoder(w).Encode(v)
}

// newKey is a token or a session key, 32 hexadecimal digits as GNU FM's.
func newKey() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
