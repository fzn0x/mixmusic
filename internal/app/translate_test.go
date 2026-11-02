package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/lyrics"
)

// translationServer is Unison's translation, answering with code, which
// counts what it was asked and keeps the last request.
type translationServer struct {
	code atomic.Int32
	asks atomic.Int32
	last atomic.Value
}

func newTranslationCatalog(t *testing.T, dir string) (*Catalog, *translationServer) {
	t.Helper()
	ts := &translationServer{}
	ts.code.Store(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.asks.Add(1)
		var req struct {
			Lines   []string `json:"lines"`
			To      string   `json:"to"`
			From    string   `json:"from"`
			VideoID string   `json:"videoId"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		ts.last.Store(req)
		if code := int(ts.code.Load()); code != http.StatusOK {
			w.WriteHeader(code)
			w.Write([]byte(`{"success":false,"error":"Translation upstream rate limited"}`))
			return
		}
		type line struct {
			Translation  *string `json:"translation"`
			Romanization *string `json:"romanization"`
			Needs        bool    `json:"needsTranslation"`
		}
		out := make([]line, len(req.Lines))
		for i, l := range req.Lines {
			tr, ro := req.To+": "+l, "roman "+l
			out[i] = line{&tr, &ro, true}
		}
		json.NewEncoder(w).Encode(map[string]any{"lines": out, "detectedLang": "ja"})
	}))
	t.Cleanup(srv.Close)
	a := &App{unison: lyrics.NewUnison("test"), translations: newTranslations(dir)}
	a.unison.BaseURL = srv.URL
	return &Catalog{a: a}, ts
}

var (
	kites      = media.Track{ID: "yt:abcdefghijk", Source: media.SourceYouTube, Title: "Kites"}
	kiteLyrics = lyrics.Lyrics{Synced: true, Language: "ja", Lines: []lyrics.Line{{TimeMs: 0, Text: "紙の凧"}, {TimeMs: 1000, Text: "空へ"}}}
)

// TestTranslateLyricsKept asks Unison once for a track's lyrics in a
// language, then reads its translation from disk, at the next start too.
func TestTranslateLyricsKept(t *testing.T) {
	dir := t.TempDir()
	c, ts := newTranslationCatalog(t, dir)
	tr, err := c.TranslateLyrics(context.Background(), kites, kiteLyrics, "en")
	if err != nil || tr.From != "ja" || len(tr.Lines) != 2 || tr.Lines[1] != (lyrics.TranslatedLine{Text: "en: 空へ", Romanized: "roman 空へ"}) {
		t.Fatalf("translation = %+v, %v", tr, err)
	}
	req := ts.last.Load().(struct {
		Lines   []string `json:"lines"`
		To      string   `json:"to"`
		From    string   `json:"from"`
		VideoID string   `json:"videoId"`
	})
	if req.To != "en" || req.From != "ja" || req.VideoID != "abcdefghijk" || strings.Join(req.Lines, "/") != "紙の凧/空へ" {
		t.Errorf("request = %+v", req)
	}

	again, ts2 := newTranslationCatalog(t, dir)
	if tr, err := again.TranslateLyrics(context.Background(), kites, kiteLyrics, "en"); err != nil || tr.Lines[0].Text != "en: 紙の凧" {
		t.Errorf("kept: %+v, %v", tr, err)
	}
	if n := ts.asks.Load() + ts2.asks.Load(); n != 1 {
		t.Errorf("Unison asked %d times for one translation", n)
	}
	// Another language, or other lyrics of the track, are other
	// translations.
	c.TranslateLyrics(context.Background(), kites, kiteLyrics, "es")
	other := kiteLyrics
	other.Lines = []lyrics.Line{{Text: "紙の凧"}}
	c.TranslateLyrics(context.Background(), kites, other, "en")
	if ts.asks.Load() != 3 {
		t.Errorf("Unison asked %d times for three translations", ts.asks.Load())
	}
}

// TestTranslateLyricsHoldsOff doesn't ask Unison again for a while after it
// said it was rate limited, a while that doubles, unless the user asks to
// try again.
func TestTranslateLyricsHoldsOff(t *testing.T) {
	c, ts := newTranslationCatalog(t, t.TempDir())
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	c.a.translations.now = func() time.Time { return now }
	ts.code.Store(http.StatusServiceUnavailable)
	ask := func() error {
		_, err := c.TranslateLyrics(context.Background(), kites, kiteLyrics, "en")
		return err
	}
	if err := ask(); err == nil || err.Error() != "Translation isn't available right now." || !errors.Is(err, lyrics.ErrRateLimited) {
		t.Fatalf("rate limited: %v", err)
	}
	now = now.Add(29 * time.Second)
	if err := ask(); err == nil || ts.asks.Load() != 1 {
		t.Errorf("asked %d times within 30 seconds of a rate limit: %v", ts.asks.Load(), err)
	}
	now = now.Add(time.Second)
	ask()
	if ts.asks.Load() != 2 {
		t.Errorf("not asked again after 30 seconds")
	}
	// Rate limited again: held off for a minute.
	now = now.Add(59 * time.Second)
	ask()
	if ts.asks.Load() != 2 {
		t.Errorf("asked again within a minute of a second rate limit")
	}
	c.TranslateAgain()
	ts.code.Store(http.StatusOK)
	if err := ask(); err != nil || ts.asks.Load() != 3 {
		t.Errorf("try again: asked %d times: %v", ts.asks.Load(), err)
	}
	// Answered, Unison is asked at once after the next rate limit's hold.
	if c.a.translations.hold != 0 {
		t.Errorf("hold = %v after an answer", c.a.translations.hold)
	}

	// Another failure doesn't hold translations off.
	ts.code.Store(http.StatusBadGateway)
	other := kiteLyrics
	other.Lines = []lyrics.Line{{Text: "空"}}
	_, err := c.TranslateLyrics(context.Background(), kites, other, "en")
	_, err2 := c.TranslateLyrics(context.Background(), kites, other, "en")
	if err == nil || err2 == nil || errors.Is(err, lyrics.ErrRateLimited) || ts.asks.Load() != 5 {
		t.Errorf("Unison failing: asked %d times: %v", ts.asks.Load(), err)
	}
}

// TestTranslationsKept lets the least used translations go past those
// kept.
func TestTranslationsKept(t *testing.T) {
	tr := newTranslations(t.TempDir())
	tr.keep = 3
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	tr.now = func() time.Time { return now }
	one := lyrics.Translation{To: "en", Lines: []lyrics.TranslatedLine{{Text: "x"}}}
	for _, key := range []string{"a", "b", "c"} {
		if err := tr.put(key, one); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(tr.dir+"/"+key+".json", now, now)
		now = now.Add(time.Minute)
	}
	tr.get("a", 1) // used last now
	tr.put("d", one)
	for key, kept := range map[string]bool{"a": true, "b": false, "c": true, "d": true} {
		if _, ok := tr.get(key, 1); ok != kept {
			t.Errorf("%s kept: %v", key, ok)
		}
	}
	// A translation of other lyrics, of another count of lines, isn't it.
	if _, ok := tr.get("a", 2); ok {
		t.Error("a translation of 1 line read as one of 2")
	}
}

func TestTranslationLanguage(t *testing.T) {
	for tag, want := range map[string]string{
		"en-US": "en", "pt-BR": "pt", "ja": "ja", "zh-Hans-CN": "zh", "zh-CN": "zh", "zh-Hant-TW": "zh-Hant",
		"zh-HK": "zh-Hant", "zh_TW": "zh-Hant", "nb-NO": "no", "iw": "he", "fil-PH": "tl", "": "en", "C": "en", "x-klingon": "en",
	} {
		if got := translationLanguage(tag); got != want {
			t.Errorf("%q: %q, want %q", tag, got, want)
		}
	}
}
