package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/romaji"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/lyrics"
)

// TestLyricsRomaji shows Japanese lyrics in romaji, under each line or in
// its place, with no translation asked for, and, with their translation,
// each line, then its romaji, then its translation.
func TestLyricsRomaji(t *testing.T) {
	s, texts := lyricsIn(t, "ja", &loaded[translated]{done: true, value: kiteTranslation})
	s.set.LyricsRomanize = map[string]string{"ja": settings.RomanizeUnder}
	got := texts()
	if i := slices.Index(got, "紙の凧"); i < 0 || i+1 >= len(got) || got[i+1] != "kami no tako" || slices.Contains(got, "Paper kite") {
		t.Errorf("romaji under: %q", got)
	}
	if s.loads["lyrics-translation:"+testTracks[0].ID+":en"].(*loaded[translated]).asks != 0 {
		t.Error("romaji asked for a translation")
	}
	s.set.LyricsRomanize = map[string]string{"ja": settings.RomanizeOnly}
	if got := texts(); !slices.Contains(got, "kami no tako") || !slices.Contains(got, "sora e") || slices.Contains(got, "紙の凧") {
		t.Errorf("romaji only: %q", got)
	}
	s.set.LyricsRomanize = map[string]string{"ja": settings.RomanizeUnder}
	s.set.LyricsTranslate = []string{"ja"}
	got = texts()
	if i := slices.Index(got, "空へ"); i < 0 || i+2 >= len(got) || got[i+1] != "sora e" || got[i+2] != "To the sky" {
		t.Errorf("romaji with translation: %q", got)
	}
}

// TestLyricsRomajiRemembered keeps the romaji of lyrics whose language
// MixMusic couldn't tell, but that hold kana, as that of Japanese.
func TestLyricsRomajiRemembered(t *testing.T) {
	s, texts := lyricsIn(t, "", nil)
	texts()
	tt := testerOf(t, s, 1280, 900)
	tt.Frame()
	click(t, s, tt, "Show lyrics in Latin letters")
	if got := s.svc.Preferences.Get().LyricsRomanize; got["ja"] != settings.RomanizeUnder || len(got) != 1 {
		t.Fatalf("saved %v", got)
	}
	if got := texts(); !slices.Contains(got, "kami no tako") {
		t.Errorf("turned on: %q", got)
	}
}

// TestLyricsRomajiNotJapanese offers no romaji for lyrics in Japanese
// already in Latin letters, nor for kanji alone in a language MixMusic
// couldn't tell, which may be Chinese.
func TestLyricsRomajiNotJapanese(t *testing.T) {
	for _, c := range []struct {
		lang  string
		lines []lyrics.Line
	}{
		{"ja", []lyrics.Line{{Text: "kami no tako"}, {TimeMs: 60000, Text: "sora e"}}},
		{"", []lyrics.Line{{Text: "紙鳶"}, {TimeMs: 60000, Text: "天空"}}},
	} {
		s, texts := lyricsIn(t, c.lang, nil)
		s.loads["lyrics:"+testTracks[0].ID].(*loaded[lyrics.Lyrics]).value.Lines = c.lines
		if got := texts(); slices.Contains(got, "romaji") || !slices.Contains(got, c.lines[0].Text) {
			t.Errorf("%q lyrics %q: %q", c.lang, c.lines[0].Text, got)
		}
	}
}

// gate serves the dictionary of the readings of kanji as answer says,
// once the test opens it, counting the requests.
type gate struct {
	open     chan struct{}
	opened   sync.Once
	answer   http.HandlerFunc
	requests atomic.Int32
}

func (g *gate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.requests.Add(1)
	select {
	case <-g.open:
	case <-r.Context().Done():
		return
	}
	g.answer(w, r)
}

// let opens the gate.
func (g *gate) let() { g.opened.Do(func() { close(g.open) }) }

// romajiFrom shows the made-up Japanese lyrics in romaji under them, the
// dictionary of the readings of kanji downloading from g.
func romajiFrom(t *testing.T, g *gate) (*shell, *ui.Tester) {
	t.Helper()
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	// The server closes once its answers are out.
	t.Cleanup(g.let)
	s := newTestShellOf(t, func(o *app.Options) { o.RomajiURL = srv.URL + "/ipa.dict" })
	s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "Unison", Language: "ja", Lines: kiteLines}}
	st := s.svc.Preferences.Get()
	st.LyricsRomanize = map[string]string{"ja": settings.RomanizeUnder}
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := testerOf(t, s, 1280, 900)
	s.open("/lyrics")
	return s, tt
}

// TestLyricsRomajiDownloading shows the kana of Japanese lyrics in romaji
// and their kanji as they are while the dictionary of the readings of
// kanji downloads, says so, and, when it couldn't, says that, and tries
// again when asked.
func TestLyricsRomajiDownloading(t *testing.T) {
	g := &gate{open: make(chan struct{}), answer: func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }}
	s, tt := romajiFrom(t, g)
	until(t, s, tt, "the note of the download", func() bool {
		return tt.HasText("The readings of kanji are downloading: kanji show as they are until then.")
	})
	got := tt.Texts()
	if i := slices.Index(got, "紙の凧"); i < 0 || got[i+1] != "紙 no 凧" || !slices.Contains(got, "空 he") {
		t.Errorf("while the dictionary downloads: %q", got)
	}
	g.let()
	until(t, s, tt, "the note of the failure", func() bool {
		return tt.HasText("The readings of kanji couldn't be downloaded: kanji show as they are.")
	})
	if !tt.HasText("紙 no 凧") {
		t.Errorf("the lyrics after the failure: %q", tt.Texts())
	}
	click(t, s, tt, "try again")
	until(t, s, tt, "the second download", func() bool { return g.requests.Load() == 2 })
}

// moduleDictionary is the IPA dictionary of kagome's module, in the Go
// module cache, which the tests serve in place of GitHub.
func moduleDictionary(t *testing.T) []byte {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		t.Skipf("no Go module cache: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(out)), "github.com", "ikawaha", "kagome-dict", "ipa@v1.2.6", "ipa.dict"))
	if err != nil {
		t.Skip("the IPA dictionary isn't in the module cache: go mod download github.com/ikawaha/kagome-dict/ipa@v1.2.6 puts it there")
	}
	return data
}

// untilRomaji waits for the window to show romaji, as the dictionary loads
// in a fraction of a second, and in some seconds with the race detector.
func untilRomaji(t *testing.T, s *shell, tt *ui.Tester, romaji string) {
	t.Helper()
	for deadline := time.Now().Add(time.Minute); !tt.HasText(romaji); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no %q: %q", romaji, tt.Texts())
		}
		s.drain()
		tt.Frame()
	}
}

// TestLyricsRomajiReadsKanji reads the kanji of Japanese lyrics once the
// dictionary of their readings downloaded.
func TestLyricsRomajiReadsKanji(t *testing.T) {
	data := moduleDictionary(t)
	g := &gate{open: make(chan struct{}), answer: func(w http.ResponseWriter, r *http.Request) { w.Write(data) }}
	g.let()
	s, tt := romajiFrom(t, g)
	untilRomaji(t, s, tt, "kami no tako")
	got := tt.Texts()
	if i := slices.Index(got, "空へ"); i < 0 || got[i+1] != "sora e" || slices.ContainsFunc(got, func(s string) bool { return strings.HasPrefix(s, "The readings of kanji") }) {
		t.Errorf("with the dictionary: %q", got)
	}
	if l, ok := s.loads[romajiKey(testTracks[0])].(*loaded[romaji.Lines]); !ok || l.value.Readings != romaji.Read {
		t.Errorf("romaji %+v", l)
	}
}

// TestLyricsRomajiOnceReady makes again the romaji of lyrics made while
// the dictionary couldn't be downloaded, once it downloaded for others.
func TestLyricsRomajiOnceReady(t *testing.T) {
	data := moduleDictionary(t)
	g := &gate{open: make(chan struct{}), answer: func(w http.ResponseWriter, r *http.Request) { w.Write(data) }}
	g.let()
	s, tt := romajiFrom(t, g)
	s.loads[romajiKey(testTracks[0])] = &loaded[romaji.Lines]{done: true, value: romaji.Lines{Romaji: []string{"紙 no 凧", "空 he"}, Readings: romaji.Unavailable}}
	s.drain()
	tt.Frame()
	if !tt.HasText("The readings of kanji couldn't be downloaded: kanji show as they are.") {
		t.Fatalf("no failure: %q", tt.Texts())
	}
	// The lyrics of another track download the dictionary.
	other := lyrics.Lyrics{Language: "ja", Lines: []lyrics.Line{{Text: "今日"}}}
	if got, _ := s.svc.Catalog.LyricsRomaji(context.Background(), other); got.Readings != romaji.Downloading {
		t.Fatalf("another track's lyrics: %+v", got)
	}
	if err := s.svc.Catalog.WaitRomaji(context.Background()); err != nil {
		t.Fatal(err)
	}
	untilRomaji(t, s, tt, "kami no tako")
	if tt.HasText("The readings of kanji couldn't be downloaded: kanji show as they are.") {
		t.Errorf("the failure stays: %q", tt.Texts())
	}
}
