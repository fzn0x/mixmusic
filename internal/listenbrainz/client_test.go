package listenbrainz

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/scrobble"
)

// request is a request the test server had.
type request struct {
	method, path, query, token, agent string
	body                              map[string]any
}

// server is a ListenBrainz that records its requests and answers each as
// answer says.
type server struct {
	mu   sync.Mutex
	got  []request
	srv  *httptest.Server
	next func(r request) (status int, header http.Header, body string)
}

func newServer(t *testing.T, answer func(r request) (int, http.Header, string)) (*server, *Client) {
	t.Helper()
	s := &server{next: answer}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := request{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, agent: r.Header.Get("User-Agent")}
		req.token, _ = strings.CutPrefix(r.Header.Get("Authorization"), "Token ")
		if data, _ := io.ReadAll(r.Body); len(data) > 0 {
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type %q", r.Header.Get("Content-Type"))
			}
			if err := json.Unmarshal(data, &req.body); err != nil {
				t.Errorf("body %s: %v", data, err)
			}
		}
		s.mu.Lock()
		s.got = append(s.got, req)
		s.mu.Unlock()
		status, header, body := s.next(req)
		for k, v := range header {
			w.Header()[k] = v
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
	t.Cleanup(s.srv.Close)
	c := New(s.srv.URL, "MixMusic/1.2.3 (https://github.com/fzn0x/mixmusic)", "1.2.3")
	c.Gap = 0
	return s, c
}

func (s *server) requests() []request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.got
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func ok(body string) func(request) (int, http.Header, string) {
	return func(request) (int, http.Header, string) { return http.StatusOK, nil, body }
}

const token = "0f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0"

func TestValidateToken(t *testing.T) {
	s, c := newServer(t, ok(fixture(t, "validate-valid.json")))
	name, err := c.ValidateToken(context.Background(), token)
	if err != nil || name != "ann" {
		t.Fatalf("name %q, %v", name, err)
	}
	r := s.requests()[0]
	if r.method != http.MethodGet || r.path != "/1/validate-token" || r.token != token || r.agent != "MixMusic/1.2.3 (https://github.com/fzn0x/mixmusic)" {
		t.Errorf("request %+v", r)
	}
	for _, answer := range []func(request) (int, http.Header, string){
		ok(fixture(t, "validate-invalid.json")),
		func(request) (int, http.Header, string) {
			return http.StatusUnauthorized, nil, `{"code":401,"error":"Invalid authorization token."}`
		},
		func(request) (int, http.Header, string) { return http.StatusForbidden, nil, `` },
	} {
		s.next = answer
		if _, err := c.ValidateToken(context.Background(), token); !errors.Is(err, ErrTokenRefused) {
			t.Errorf("err = %v", err)
		}
	}
}

func TestNowPlaying(t *testing.T) {
	s, c := newServer(t, ok(fixture(t, "playing-now.json")))
	var heard []string
	c.OnRecording = func(t Track, msid string) { heard = append(heard, t.ID+" "+msid) }
	track := Track{ID: "yt:abc", Artist: "Ann", Title: "Song", Album: "Album", Duration: 3*time.Minute + 500*time.Millisecond,
		Service: "music.youtube.com", URL: "https://music.youtube.com/watch?v=abc"}
	if err := c.NowPlaying(context.Background(), token, track); err != nil {
		t.Fatal(err)
	}
	if want := []string{"yt:abc 00000000-0000-4000-8000-0000000000d1"}; len(heard) != 1 || heard[0] != want[0] {
		t.Errorf("heard %q", heard)
	}
	r := s.requests()[0]
	if r.method != http.MethodPost || r.path != "/1/submit-listens" || r.query != "return_msid=true" {
		t.Errorf("request %+v", r)
	}
	want := `{"listen_type":"playing_now","payload":[{"track_metadata":{"additional_info":{"duration_ms":180500,"media_player":"MixMusic","media_player_version":"1.2.3","music_service":"music.youtube.com","origin_url":"https://music.youtube.com/watch?v=abc","submission_client":"MixMusic","submission_client_version":"1.2.3"},"artist_name":"Ann","release_name":"Album","track_name":"Song"}}]}`
	if got, _ := json.Marshal(r.body); string(got) != want {
		t.Errorf("body\n%s\nwant\n%s", got, want)
	}
}

func TestScrobbleSingleAndImport(t *testing.T) {
	s, c := newServer(t, ok(`{"status": "ok"}`))
	start := time.Unix(1_790_000_000, 0)
	sp := Track{Artist: "Bo", Title: "Tune", Duration: time.Minute, Service: "spotify.com", URL: "https://open.spotify.com/track/xyz",
		SpotifyURL: "https://open.spotify.com/track/xyz", ISRC: "USRC17607839"}
	if err := c.Scrobble(context.Background(), token, []Play{{Track: sp, Start: start}}); err != nil {
		t.Fatal(err)
	}
	plays := make([]Play, MaxListens+1)
	for i := range plays {
		plays[i] = Play{Track: Track{Artist: "Ann", Title: "Song"}, Start: start.Add(time.Duration(i) * time.Minute)}
	}
	if err := c.Scrobble(context.Background(), token, plays); err != nil {
		t.Fatal(err)
	}
	got := s.requests()
	if len(got) != 3 {
		t.Fatalf("%d requests", len(got))
	}
	want := `{"listen_type":"single","payload":[{"listened_at":1790000000,"track_metadata":{"additional_info":{"duration_ms":60000,"isrc":"USRC17607839","media_player":"MixMusic","media_player_version":"1.2.3","music_service":"spotify.com","origin_url":"https://open.spotify.com/track/xyz","spotify_id":"https://open.spotify.com/track/xyz","submission_client":"MixMusic","submission_client_version":"1.2.3"},"artist_name":"Bo","track_name":"Tune"}}]}`
	if body, _ := json.Marshal(got[0].body); string(body) != want {
		t.Errorf("single\n%s\nwant\n%s", body, want)
	}
	for i, n := range []int{MaxListens, 1} {
		r := got[i+1]
		if r.body["listen_type"] != "import" || len(r.body["payload"].([]any)) != n {
			t.Errorf("submission %d: %v listens of %v", i+1, len(r.body["payload"].([]any)), r.body["listen_type"])
		}
	}
	if r := got[2]; r.query != "" {
		t.Errorf("query %q", r.query)
	}
}

func TestErrors(t *testing.T) {
	c := New("", "MixMusic", "1.2.3")
	c.Gap = 0
	for _, cs := range []struct {
		name     string
		status   int
		header   http.Header
		body     string
		refused  bool
		rejected bool
		wait     time.Duration
	}{
		{"no token", http.StatusBadRequest, nil, fixture(t, "no-token.json"), false, true, 0},
		{"refused", http.StatusUnauthorized, nil, `{"code":401,"error":"Invalid authorization token."}`, true, false, 0},
		{"too many", http.StatusTooManyRequests, http.Header{"X-Ratelimit-Reset-In": {"5"}}, ``, false, false, 5 * time.Second},
		{"down", http.StatusServiceUnavailable, nil, `<html>`, false, false, 0},
		{"moved", http.StatusNotFound, nil, ``, false, false, 0},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for k, v := range cs.header {
				w.Header()[k] = v
			}
			w.WriteHeader(cs.status)
			io.WriteString(w, cs.body)
		}))
		c.Server = srv.URL
		err := c.Scrobble(context.Background(), token, []Play{{Track: Track{Artist: "A", Title: "T"}, Start: time.Unix(1_790_000_000, 0)}})
		srv.Close()
		c.next = time.Time{}
		if got := errors.Is(err, scrobble.ErrKeyRefused); got != cs.refused {
			t.Errorf("%s: refused %v: %v", cs.name, got, err)
		}
		if got := scrobble.IsRejected(err); got != cs.rejected {
			t.Errorf("%s: rejected %v: %v", cs.name, got, err)
		}
		if got := scrobble.RetryAfter(err); got != cs.wait {
			t.Errorf("%s: waits %v", cs.name, got)
		}
	}
	var apiErr *APIError
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, fixture(t, "no-token.json"))
	}))
	defer srv.Close()
	c.Server = srv.URL
	if _, err := c.ValidateToken(context.Background(), ""); !errors.As(err, &apiErr) || apiErr.Message != "You need to provide an Authorization token." {
		t.Errorf("err = %v", err)
	}
	c.Server = "http://127.0.0.1:1"
	if err := c.Scrobble(context.Background(), token, []Play{{}}); err == nil || scrobble.IsRejected(err) || errors.Is(err, scrobble.ErrKeyRefused) {
		t.Errorf("no answer: %v", err)
	}
}

// TestOneCallASecond spaces the calls, and waits as the server asked
// after a 429.
func TestOneCallASecond(t *testing.T) {
	_, c := newServer(t, ok(fixture(t, "validate-valid.json")))
	c.Gap = 40 * time.Millisecond
	start := time.Now()
	for range 3 {
		if _, err := c.ValidateToken(context.Background(), token); err != nil {
			t.Fatal(err)
		}
	}
	if d := time.Since(start); d < 80*time.Millisecond {
		t.Errorf("3 calls in %v", d)
	}
	c.hold(time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := c.ValidateToken(ctx, token); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a call during the wait: %v", err)
	}
}

func TestLove(t *testing.T) {
	s, c := newServer(t, ok(`{"status": "ok"}`))
	ctx := context.Background()
	if err := c.Love(ctx, token, Recording{MSID: "msid"}, true); err != nil {
		t.Fatal(err)
	}
	if err := c.Love(ctx, token, Recording{MBID: "mbid"}, false); err != nil {
		t.Fatal(err)
	}
	got := s.requests()
	for i, want := range []string{`{"recording_msid":"msid","score":1}`, `{"recording_mbid":"mbid","score":0}`} {
		body, _ := json.Marshal(got[i].body)
		if got[i].method != http.MethodPost || got[i].path != "/1/feedback/recording-feedback" || string(body) != want {
			t.Errorf("love %d: %s %s %s", i, got[i].method, got[i].path, body)
		}
	}
}

func TestLookup(t *testing.T) {
	s, c := newServer(t, ok(fixture(t, "lookup.json")))
	ctx := context.Background()
	mbid, err := c.Lookup(ctx, token, Track{Artist: "Ann", Title: "Song & Dance", Album: "Album"})
	if err != nil || mbid != "00000000-0000-4000-8000-0000000000c1" {
		t.Fatalf("mbid %q, %v", mbid, err)
	}
	r := s.requests()[0]
	if r.method != http.MethodGet || r.path != "/1/metadata/lookup/" || r.query != "artist_name=Ann&recording_name=Song+%26+Dance&release_name=Album" || r.token != token {
		t.Errorf("request %+v", r)
	}
	s.next = ok(fixture(t, "lookup-none.json"))
	if mbid, err := c.Lookup(ctx, token, Track{Artist: "Nobody", Title: "Nothing"}); err != nil || mbid != "" {
		t.Errorf("no match: %q, %v", mbid, err)
	}
}
