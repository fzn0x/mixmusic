package sponsorblock

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// recorded is an answer SponsorBlock gave for the hash prefix 5f6b, cut
// down to a video of each kind.
func recorded(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/skipsegments-5f6b.json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestPrefix sends the first 4 hex characters of the SHA-256 of the
// video's ID, never the ID.
func TestPrefix(t *testing.T) {
	if got := Prefix("1yRWqE5jrkM"); got != "5f6b" {
		t.Errorf("Prefix = %q, want 5f6b", got)
	}
}

// TestSegmentsAsksByPrefix asks for the kinds MixMusic skips and the
// highlight, by the prefix of the video's hash, and keeps the parts of the
// video asked for alone, in order.
func TestSegmentsAsksByPrefix(t *testing.T) {
	body := recorded(t)
	var asked *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()
	c := New("test")
	got, err := c.Segments(context.Background(), srv.URL, "1yRWqE5jrkM")
	if err != nil {
		t.Fatal(err)
	}
	want := []Segment{
		{Kind: Highlight, From: 72781 * time.Millisecond, To: 72781 * time.Millisecond, Of: 191461 * time.Millisecond},
		{Kind: NonMusic, From: 188341 * time.Millisecond, To: 191461 * time.Millisecond, Of: 191461 * time.Millisecond},
	}
	if !slices.Equal(got, want) {
		t.Errorf("segments %+v, want %+v", got, want)
	}
	if asked.URL.Path != "/api/skipSegments/5f6b" {
		t.Errorf("asked %s", asked.URL.Path)
	}
	if strings.Contains(asked.URL.String(), "1yRWqE5jrkM") {
		t.Errorf("the lookup named the video: %s", asked.URL)
	}
	var kinds []Kind
	if err := json.Unmarshal([]byte(asked.URL.Query().Get("categories")), &kinds); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(kinds, append(slices.Clone(Kinds), Highlight)) {
		t.Errorf("asked for %v", kinds)
	}
	if got := asked.URL.Query().Get("actionTypes"); got != `["skip","poi"]` {
		t.Errorf("action types %s", got)
	}
	if ua := asked.Header.Get("User-Agent"); ua != "test" {
		t.Errorf("user agent %q", ua)
	}
}

// TestSegmentsOfEachKind reads a part of each kind of the recorded answer.
func TestSegmentsOfEachKind(t *testing.T) {
	body := recorded(t)
	for id, want := range map[string]Kind{
		"eXjGWlJOhWg": Intro, "3KQzuNkJih8": Preview, "aQp-iM0bCs4": Sponsor, "ADbuFoWx9Nk": Outro,
		"WEaQkJwGQRE": Interaction, "vV4P9dcQQtU": SelfPromo, "fjNRPNF_FHs": Filler,
	} {
		got, err := parse(body, id)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Kind != want {
			t.Errorf("%s: %+v, want a part of %s", id, got, want)
		}
	}
}

// TestSegmentsNone takes SponsorBlock's 404, for a prefix of no video
// with parts, as none, and a video the answer doesn't hold as none too.
func TestSegmentsNone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	}))
	defer srv.Close()
	got, err := New("test").Segments(context.Background(), srv.URL, "dQw4w9WgXcQ")
	if err != nil || got != nil {
		t.Errorf("Segments = %v, %v", got, err)
	}
	if got, err := parse(recorded(t), "dQw4w9WgXcQ"); err != nil || got != nil {
		t.Errorf("parse = %v, %v", got, err)
	}
}

// TestSegmentsFails fails a lookup SponsorBlock refuses, a broken answer
// and one longer than maxAnswer.
func TestSegmentsFails(t *testing.T) {
	defer func(n int64) { maxAnswer = n }(maxAnswer)
	maxAnswer = 100
	for name, serve := range map[string]http.HandlerFunc{
		"refused":  func(w http.ResponseWriter, r *http.Request) { http.Error(w, "slow down", http.StatusTooManyRequests) },
		"broken":   func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`[{"videoID":`)) },
		"too long": func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("[" + strings.Repeat(" ", 200) + "]")) },
	} {
		srv := httptest.NewServer(serve)
		if _, err := New("test").Segments(context.Background(), srv.URL, "1yRWqE5jrkM"); err == nil {
			t.Errorf("%s: no error", name)
		}
		srv.Close()
	}
}

// TestSegmentsStopInTime gives up on a server that doesn't answer once
// timeout passed.
func TestSegmentsStopInTime(t *testing.T) {
	defer func(d time.Duration) { timeout = d }(timeout)
	timeout = 50 * time.Millisecond
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-done:
		}
	}))
	defer srv.Close()
	defer close(done)
	start := time.Now()
	if _, err := New("test").Segments(context.Background(), srv.URL, "1yRWqE5jrkM"); err == nil {
		t.Error("no error")
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("took %v", took)
	}
}

// TestSegmentsLeaveOutNonsense leaves out parts of other kinds, of other
// actions, and those that end before they start.
func TestSegmentsLeaveOutNonsense(t *testing.T) {
	body := `[{"videoID":"v","segments":[
		{"category":"sponsor","actionType":"skip","segment":[10,20],"videoDuration":0},
		{"category":"sponsor","actionType":"mute","segment":[30,40],"videoDuration":0},
		{"category":"chapter","actionType":"chapter","segment":[0,50],"videoDuration":0},
		{"category":"intro","actionType":"skip","segment":[5,5],"videoDuration":0},
		{"category":"outro","actionType":"skip","segment":[-1,3],"videoDuration":0},
		{"category":"filler","actionType":"skip","segment":[1],"videoDuration":0}]}]`
	got, err := parse([]byte(body), "v")
	if err != nil {
		t.Fatal(err)
	}
	if want := []Segment{{Kind: Sponsor, From: 10 * time.Second, To: 20 * time.Second}}; !slices.Equal(got, want) {
		t.Errorf("segments %+v, want %+v", got, want)
	}
}

// TestServer takes an https address of a mirror, and nothing else.
func TestServer(t *testing.T) {
	for in, want := range map[string]string{
		"":                               "",
		"https://sponsor.example":        "https://sponsor.example",
		" https://sponsor.example/ ":     "https://sponsor.example",
		"https://sponsor.example/mirror": "https://sponsor.example/mirror",
		"http://sponsor.example":         "",
		"sponsor.example":                "",
		"https://":                       "",
		"https://a@sponsor.example":      "",
		"https://sponsor.example/?x=1":   "",
	} {
		if got := Server(in); got != want {
			t.Errorf("Server(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestKindWords names every kind, in the settings and in a notice.
func TestKindWords(t *testing.T) {
	for _, k := range append(slices.Clone(Kinds), Highlight) {
		if k.Name() == string(k) || k.Part() == "a part" {
			t.Errorf("%s has no words", k)
		}
	}
	if slices.Contains(DefaultKinds, Filler) {
		t.Error("Filler is skipped at first")
	}
}
