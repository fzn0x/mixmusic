package lyrics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// translateRequest is what MixMusic sends Unison to translate.
type translateRequest struct {
	Lines   []string `json:"lines"`
	To      string   `json:"to"`
	From    string   `json:"from"`
	VideoID string   `json:"videoId"`
}

// translator answers translations as Unison does, through answer, and
// keeps the requests it got.
func translator(t *testing.T, answer func(w http.ResponseWriter, req translateRequest)) (*Unison, *[]translateRequest) {
	t.Helper()
	var got []translateRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req translateRequest
		if r.Method != http.MethodPost || r.URL.Path != "/translate" || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("User-Agent") != "test-agent" {
			t.Errorf("request = %s %s, %v", r.Method, r.URL.Path, r.Header)
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		got = append(got, req)
		answer(w, req)
	}))
	t.Cleanup(srv.Close)
	u := NewUnison("test-agent")
	u.BaseURL = srv.URL
	return u, &got
}

// echo translates each line to "translated <line>", and romanizes it as
// "roman <line>", as a made-up language might.
func echo(w http.ResponseWriter, req translateRequest) {
	type line struct {
		Translation      *string `json:"translation"`
		Romanization     *string `json:"romanization"`
		NeedsTranslation bool    `json:"needsTranslation"`
	}
	out := make([]line, len(req.Lines))
	for i, l := range req.Lines {
		if l == "" || l == "♪" {
			continue
		}
		tr, ro := "translated "+l, "roman "+l
		out[i] = line{&tr, &ro, true}
	}
	json.NewEncoder(w).Encode(map[string]any{"lines": out, "detectedLang": "xx", "provider": "test", "cached": false})
}

func TestTranslate(t *testing.T) {
	u, got := translator(t, func(w http.ResponseWriter, req translateRequest) {
		// Line by line: translated and romanized; blank; in the language
		// already; romanized only; and only repeated.
		w.Write([]byte(`{"lines": [
			{"translation": "Paper kites", "romanization": "kami no tako", "needsTranslation": true},
			{"translation": null, "romanization": null, "needsTranslation": false},
			{"translation": "Already English", "romanization": null, "needsTranslation": false},
			{"translation": null, "romanization": "sora", "needsTranslation": true},
			{"translation": "OK", "romanization": "ok", "needsTranslation": true}
		], "detectedLang": "ja", "provider": "google-lyrics-translate", "cached": true}`))
	})
	tr, err := u.Translate(context.Background(), []string{"紙の凧", "", "Already English", "空", "OK"}, "en", "", "abcdefghijk")
	if err != nil {
		t.Fatal(err)
	}
	want := []TranslatedLine{{"Paper kites", "kami no tako"}, {}, {}, {"", "sora"}, {}}
	if tr.From != "ja" || tr.To != "en" || len(tr.Lines) != len(want) {
		t.Fatalf("translation = %+v", tr)
	}
	for i := range want {
		if tr.Lines[i] != want[i] {
			t.Errorf("line %d = %+v, want %+v", i, tr.Lines[i], want[i])
		}
	}
	if req := (*got)[0]; req.To != "en" || req.From != "" || req.VideoID != "abcdefghijk" || len(req.Lines) != 5 {
		t.Errorf("request = %+v", req)
	}
}

// TestTranslateLong asks for long lyrics 200 lines at a time, and leaves a
// line too long for Unison untranslated.
func TestTranslateLong(t *testing.T) {
	u, got := translator(t, echo)
	lines := make([]string, 450)
	for i := range lines {
		lines[i] = "line " + strings.Repeat("a", i%3)
	}
	lines[7] = strings.Repeat("長", 501)
	tr, err := u.Translate(context.Background(), lines, "en", "ja", "")
	if err != nil || len(tr.Lines) != 450 || tr.From != "xx" {
		t.Fatalf("translation of %d lines, from %q: %v", len(tr.Lines), tr.From, err)
	}
	if len(*got) != 3 || len((*got)[0].Lines) != 200 || len((*got)[2].Lines) != 50 || (*got)[1].From != "ja" {
		t.Errorf("%d requests", len(*got))
	}
	if tr.Lines[449] != (TranslatedLine{"translated " + lines[449], "roman " + lines[449]}) {
		t.Errorf("the last line = %+v", tr.Lines[449])
	}
	if (*got)[0].Lines[7] != "" || tr.Lines[7] != (TranslatedLine{}) {
		t.Errorf("a line of 501 characters went as %q", (*got)[0].Lines[7][:10])
	}
	// Characters past the Basic Multilingual Plane count twice.
	if _, err := u.Translate(context.Background(), []string{strings.Repeat("🎵", 251)}, "en", "", ""); err != nil || (*got)[3].Lines[0] != "" {
		t.Errorf("a line of 502 UTF-16 characters went")
	}
}

func TestTranslateFails(t *testing.T) {
	status := func(code int, body string) *Unison {
		u, _ := translator(t, func(w http.ResponseWriter, req translateRequest) {
			w.WriteHeader(code)
			w.Write([]byte(body))
		})
		return u
	}
	lines := []string{"one", "two"}
	// Nothing Unison could translate: nothing to show, which is no error.
	tr, err := status(http.StatusBadRequest, `{"success":false,"error":"Could not detect source language"}`).Translate(context.Background(), lines, "en", "", "")
	if err != nil || len(tr.Lines) != 2 || tr.Lines[0] != (TranslatedLine{}) {
		t.Errorf("400: %+v, %v", tr, err)
	}
	for _, code := range []int{http.StatusServiceUnavailable, http.StatusTooManyRequests} {
		_, err := status(code, `{"success":false,"error":"Translation upstream rate limited"}`).Translate(context.Background(), lines, "en", "", "")
		if !errors.Is(err, ErrRateLimited) || !errors.Is(err, ErrUnavailable) {
			t.Errorf("HTTP %d: err = %v", code, err)
		}
	}
	for _, c := range []struct {
		code int
		body string
	}{
		{http.StatusBadGateway, `{"success":false,"error":"Translation upstream failed"}`},
		{http.StatusOK, `{"lines": [{"translation": "ONE"}]}`},
		{http.StatusOK, `{"lines": [`},
	} {
		_, err := status(c.code, c.body).Translate(context.Background(), lines, "en", "", "")
		if !errors.Is(err, ErrUnavailable) || errors.Is(err, ErrRateLimited) {
			t.Errorf("HTTP %d %s: err = %v", c.code, c.body, err)
		}
	}
}
