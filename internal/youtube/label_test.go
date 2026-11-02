package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

func TestParseMadeWithAI(t *testing.T) {
	for _, tc := range []struct {
		fixture, videoID string
		want             bool
		err              error
	}{
		{"next-label-made-with-ai", "NWuajLluF_E", true, nil},
		// In German, from the article alone.
		{"next-label-made-with-ai-de", "NWuajLluF_E", true, nil},
		// An older wording, with the same article.
		{"next-label-altered-or-synthetic", "altered0001", true, nil},
		// Another note of how a video was made, with another article.
		{"next-label-auto-dubbed", "autoDubbed1", false, nil},
		{"next-label-none", "dQw4w9WgXcQ", false, nil},
		// The whole answer, should YouTube refuse the mask.
		{"next-label-whole", "wholeAnswer", true, nil},
		// No such video: YouTube answers without one.
		{"next-label-unavailable", "zzzzzzzzzzz", false, ErrNotFound},
		// The answer of another video.
		{"next-label-made-with-ai", "dQw4w9WgXcQ", false, ErrNotFound},
	} {
		got, err := parseMadeWithAI(fixture(t, tc.fixture), tc.videoID)
		if got != tc.want || !errors.Is(err, tc.err) {
			t.Errorf("%s of %s = %v, %v; want %v, %v", tc.fixture, tc.videoID, got, err, tc.want, tc.err)
		}
	}
}

// TestAILabel tells the label by its article's path, its query left out,
// and by its header, but not by an article that only starts as its does.
func TestAILabel(t *testing.T) {
	made := func(header, link string) node {
		return parseNode(map[string]any{
			"bodyHeader": map[string]any{"content": header},
			"bodyText": map[string]any{"commandRuns": []any{map[string]any{
				"onTap": map[string]any{"innertubeCommand": map[string]any{"urlEndpoint": map[string]any{"url": link}}},
			}}},
		})
	}
	for _, tc := range []struct {
		header, link string
		want         bool
	}{
		{"Mit KI erstellt", "https://support.google.com/youtube/answer/15447836", true},
		{"Mit KI erstellt", "//support.google.com/youtube/answer/15447836?hl=de&x=1", true},
		{"Mit KI erstellt", "//support.google.com/youtube/answer/154478360?hl=de", false},
		{"Mit KI erstellt", "//support.google.com/youtube/answer/15569972?hl=de", false},
		{" Made with AI ", "", true},
		{"Altered or synthetic content", "//support.google.com/youtube/answer/1", true},
		{"Auto-dubbed", "//support.google.com/youtube/answer/15569972?hl=en", false},
		{"", "", false},
	} {
		if got := aiLabel(made(tc.header, tc.link)); got != tc.want {
			t.Errorf("aiLabel(%q, %q) = %v", tc.header, tc.link, got)
		}
	}
}

// labelServer is YouTube's website answering the label of every video
// with the fixture name, which also sets a cookie and a visitor ID; with
// refuseMask, it refuses masked requests as YouTube refuses a bad mask.
func labelServer(t *testing.T, name string, refuseMask bool) (*Client, func() []recorded) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var answer map[string]any
	if err := json.Unmarshal(data, &answer); err != nil {
		t.Fatal(err)
	}
	answer["responseContext"] = map[string]any{"visitorData": "ANOTHER-VISITOR"}
	var (
		mu  sync.Mutex
		got []recorded
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(data, &body)
		mu.Lock()
		got = append(got, recorded{r.URL.Path, r.URL.Query(), r.Header.Clone(), body})
		mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "VISITOR_INFO1_LIVE", Value: "visitor"})
		http.SetCookie(w, &http.Cookie{Name: "SAPISID", Value: "replaced"})
		if refuseMask && r.Header.Get("X-Goog-FieldMask") != "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": {"code": 400, "message": "Request contains an invalid argument."}}`))
			return
		}
		json.NewEncoder(w).Encode(answer)
	}))
	t.Cleanup(srv.Close)
	c := New()
	c.BaseURL = srv.URL
	return c, func() []recorded {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(got)
	}
}

// TestMadeWithAIRequest asks the website signed out, with the mask, and
// keeps nothing of its answer for the session: neither its cookies nor its
// visitor ID.
func TestMadeWithAIRequest(t *testing.T) {
	c, requests := labelServer(t, "next-label-made-with-ai", false)
	c.SetCookies(session)
	c.SetChannel("PAGE")
	c.setVisitorID("MUSIC-VISITOR")
	ai, err := c.MadeWithAI(context.Background(), "NWuajLluF_E")
	if err != nil || !ai {
		t.Fatalf("MadeWithAI = %v, %v", ai, err)
	}
	got := requests()
	if len(got) != 1 {
		t.Fatalf("%d requests", len(got))
	}
	r := got[0]
	client := parseNode(r.body).get("context", "client")
	if r.path != "/youtubei/v1/next" || r.body["videoId"] != "NWuajLluF_E" || client.get("clientName").str() != "WEB" {
		t.Errorf("request = %s %v", r.path, r.body)
	}
	if r.header.Get("X-YouTube-Client-Name") != "1" || !strings.HasPrefix(r.header.Get("X-YouTube-Client-Version"), "2.") {
		t.Errorf("client headers = %q, %q", r.header.Get("X-YouTube-Client-Name"), r.header.Get("X-YouTube-Client-Version"))
	}
	if mask := r.header.Get("X-Goog-FieldMask"); !strings.Contains(mask, "currentVideoEndpoint") || !strings.Contains(mask, "howThisWasMadeSectionViewModel") {
		t.Errorf("mask = %q", mask)
	}
	for _, h := range []string{"Cookie", "Authorization", "X-Goog-AuthUser", "X-Goog-PageId", "X-Goog-Visitor-Id"} {
		if v := r.header.Get(h); v != "" {
			t.Errorf("signed out, it sent %s: %q", h, v)
		}
	}
	if user := parseNode(r.body).get("context", "user", "onBehalfOfUser"); user.exists() {
		t.Errorf("signed out, it asked on behalf of %v", user.v)
	}
	if v := c.visitorID(); v != "MUSIC-VISITOR" {
		t.Errorf("the visitor ID became %q", v)
	}
	for _, ck := range c.Cookies() {
		if ck.Name == "VISITOR_INFO1_LIVE" || ck.Value == "replaced" {
			t.Errorf("the session took the cookie %s=%s", ck.Name, ck.Value)
		}
	}
	if !c.SignedIn() || c.Rejected() {
		t.Errorf("signed in %v, rejected %v", c.SignedIn(), c.Rejected())
	}
}

// TestMadeWithAIWithoutMask asks again for the whole answer when YouTube
// refuses the mask.
func TestMadeWithAIWithoutMask(t *testing.T) {
	c, requests := labelServer(t, "next-label-whole", true)
	ai, err := c.MadeWithAI(context.Background(), "wholeAnswer")
	if err != nil || !ai {
		t.Fatalf("MadeWithAI = %v, %v", ai, err)
	}
	got := requests()
	if len(got) != 2 || got[0].header.Get("X-Goog-FieldMask") == "" || got[1].header.Get("X-Goog-FieldMask") != "" {
		t.Errorf("asked %d times, masks %q", len(got), func() (m []string) {
			for _, r := range got {
				m = append(m, r.header.Get("X-Goog-FieldMask"))
			}
			return m
		}())
	}
}

func TestMadeWithAINotFound(t *testing.T) {
	c, _ := labelServer(t, "next-label-unavailable", false)
	if ai, err := c.MadeWithAI(context.Background(), "zzzzzzzzzzz"); ai || !errors.Is(err, ErrNotFound) {
		t.Errorf("MadeWithAI = %v, %v", ai, err)
	}
}
