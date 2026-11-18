package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// sessionServer answers the account's menu of a browser session signed in
// to several Google accounts, as menus says of each index: a name, signed
// out ("" for a visitor's answer, "401" for a 401), or "500" for a server
// that fails. An index menus doesn't name answers as a visitor. It lists
// the session's accounts as list. It records the indexes asked as, and
// the page IDs of those requests.
type sessionServer struct {
	menus map[int]string
	list  string

	mu    sync.Mutex
	asked []int
	pages []string
}

func (s *sessionServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/accounts_list") {
		w.Write([]byte(s.list))
		return
	}
	user, err := strconv.Atoi(r.Header.Get("X-Goog-AuthUser"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	data, _ := io.ReadAll(r.Body)
	var body struct {
		Context struct {
			User map[string]any `json:"user"`
		} `json:"context"`
	}
	json.Unmarshal(data, &body)
	page := r.Header.Get("X-Goog-PageId")
	if behalf, _ := body.Context.User["onBehalfOfUser"].(string); behalf != "" {
		page += "+" + behalf
	}
	s.mu.Lock()
	s.asked = append(s.asked, user)
	s.pages = append(s.pages, page)
	s.mu.Unlock()
	switch name := s.menus[user]; name {
	case "":
		json.NewEncoder(w).Encode(loggedIn("0"))
	case "401":
		w.WriteHeader(http.StatusUnauthorized)
	case "500":
		w.WriteHeader(http.StatusInternalServerError)
	default:
		menu := loggedIn("1")
		menu["actions"] = []any{map[string]any{"openPopupAction": map[string]any{"popup": map[string]any{"multiPageMenuRenderer": map[string]any{
			"header": map[string]any{"activeAccountHeaderRenderer": map[string]any{"accountName": map[string]any{"runs": []any{map[string]any{"text": name}}}}},
		}}}}}
		json.NewEncoder(w).Encode(menu)
	}
}

// askedAs returns the indexes asked as, in order, and the page IDs sent.
func (s *sessionServer) askedAs() ([]int, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.asked), slices.Clone(s.pages)
}

// sessionClient is a client signed in to s's session.
func sessionClient(t *testing.T, s *sessionServer) *Client {
	t.Helper()
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	c := New()
	c.BaseURL = srv.URL
	c.SetCookies(session)
	return c
}

// TestUseFirstAccount signs in as the first account of the session still
// signed in, itself, when those before it were signed out of: the session
// goes on.
func TestUseFirstAccount(t *testing.T) {
	s := &sessionServer{menus: map[int]string{0: "401", 2: "Cy", 3: "Dee"}}
	c := sessionClient(t, s)
	rejected := make(chan struct{}, 1)
	c.OnRejected = func() { rejected <- struct{}{} }
	c.SetAccount(4, "123456789012345678901")
	got, err := c.UseFirstAccount(context.Background())
	if err != nil || got.Name != "Cy" {
		t.Fatalf("UseFirstAccount() = %+v, %v", got, err)
	}
	if c.AuthUser() != 2 || c.Channel() != "" || c.Rejected() {
		t.Errorf("using account %d, channel %q, rejected %v", c.AuthUser(), c.Channel(), c.Rejected())
	}
	asked, pages := s.askedAs()
	if !slices.Equal(asked, []int{0, 1, 2}) || slices.ContainsFunc(pages, func(p string) bool { return p != "" }) {
		t.Errorf("asked as %v, of pages %q", asked, pages)
	}
	select {
	case <-rejected:
		t.Error("a signed-out account ended the session")
	case <-time.After(50 * time.Millisecond):
	}
}

// TestUseFirstAccountNone finds the session ended when no account of it is
// signed in.
func TestUseFirstAccountNone(t *testing.T) {
	s := &sessionServer{menus: map[int]string{0: "401", 1: "401"}}
	c := sessionClient(t, s)
	_, err := c.UseFirstAccount(context.Background())
	if !errors.Is(err, ErrSessionRejected) || !c.Rejected() {
		t.Errorf("UseFirstAccount() = %v, rejected %v", err, c.Rejected())
	}
	if asked, _ := s.askedAs(); !slices.Equal(asked, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}) {
		t.Errorf("asked as %v", asked)
	}
	if c.AuthUser() != 0 {
		t.Errorf("using account %d", c.AuthUser())
	}
}

// TestUseFirstAccountFails stops at a failure: the account may be signed in
// still, and one after it is not the first.
func TestUseFirstAccountFails(t *testing.T) {
	s := &sessionServer{menus: map[int]string{0: "401", 1: "500", 2: "Cy"}}
	c := sessionClient(t, s)
	c.SetAccount(3, "123456789012345678901")
	_, err := c.UseFirstAccount(context.Background())
	var api *APIError
	if !errors.As(err, &api) || api.Status != http.StatusInternalServerError || c.Rejected() {
		t.Errorf("UseFirstAccount() = %v, rejected %v", err, c.Rejected())
	}
	if asked, _ := s.askedAs(); !slices.Equal(asked, []int{0, 1}) {
		t.Errorf("asked as %v", asked)
	}
	if c.AuthUser() != 3 || c.Channel() != "123456789012345678901" {
		t.Errorf("went on as account %d, channel %q", c.AuthUser(), c.Channel())
	}
}
