package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/autoeq"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// The correction of headphones (#616): AutoEq's profiles from a server of
// made-up ones, never GitHub, and the files Import file… reads.

// headphonesProfile is a profile in AutoEq's form, of made-up values.
const headphonesProfile = "Preamp: -4.2 dB\nFilter 1: ON LSC Fc 105 Hz Gain 3.5 dB Q 0.70\nFilter 2: ON PK Fc 2400 Hz Gain -2.0 dB Q 1.50\n"

// headphonesServer serves AutoEq's results: an index of two profiles, one
// of which is there.
func headphonesServer(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/results/INDEX.md":
			w.Write([]byte("- [Acme One](./Lab%20A/711%20in-ear/Acme%20One) by Lab A on 711\n- [Acme Two](./Lab%20B/over-ear/Acme%20Two) by Lab B\n"))
		case "/results/Lab A/711 in-ear/Acme One/Acme One ParametricEQ.txt":
			w.Write([]byte(headphonesProfile))
		case "/results/Lab B/over-ear/Acme Two/Acme Two ParametricEQ.txt":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestHeadphonesFromAutoEq lists AutoEq's profiles, and has the engine
// play the one picked once the settings keep it, after the bands, whether
// the equalizer and Sound effects are on or not; None stops it.
func TestHeadphonesFromAutoEq(t *testing.T) {
	srv := headphonesServer(t)
	a := newQuietApp(t, func(o *Options) { o.AutoEqURL = srv.URL + "/results/" })
	e := audio.NewEngine(a.openNative, startSilent, engineEvents{a})
	t.Cleanup(e.Close)
	a.nativeOnce.Do(func() { a.native = e })
	a.applyEngine(a.settings.Get().Clean())
	if c := e.Correction(); len(c.Filters) != 0 || c.Preamp != 0 {
		t.Errorf("at first: %+v", c)
	}

	p := &Preferences{a}
	list, err := p.HeadphonesIndex(context.Background())
	if err != nil || len(list.Entries) != 2 || list.Offline != nil {
		t.Fatalf("the list: %+v, %v", list, err)
	}
	if p.HeadphonesKept(list.Entries[0]) {
		t.Error("a profile kept before it was picked")
	}
	h, err := p.HeadphonesProfile(context.Background(), list.Entries[0])
	if err != nil || h.ID != "Lab A/711 in-ear/Acme One" || h.Name != "Acme One" || h.Source != "Lab A" || h.Rig != "711" {
		t.Fatalf("the profile: %+v, %v", h, err)
	}
	if !p.HeadphonesKept(list.Entries[0]) {
		t.Error("the profile picked isn't kept")
	}
	if _, err := os.Stat(filepath.Join(a.opts.DataDir, "autoeq", "INDEX.md")); err != nil {
		t.Errorf("the list isn't kept in the data folder: %v", err)
	}
	want := audio.Correction{Preamp: -4.2, Filters: []audio.CorrectionFilter{
		{Type: audio.CorrectionLowShelf, Hz: 105, Gain: 3.5, Q: 0.7},
		{Type: audio.CorrectionPeak, Hz: 2400, Gain: -2, Q: 1.5},
	}}
	for _, on := range []bool{true, false} {
		if _, err := p.Update(func(s *settings.Settings) {
			s.Equalizer.Headphones = h
			s.Equalizer.Enabled, s.Equalizer.Effects = on, on
		}); err != nil {
			t.Fatal(err)
		}
		if c := e.Correction(); c.Preamp != want.Preamp || !slices.Equal(c.Filters, want.Filters) {
			t.Errorf("the equalizer and Sound effects on %v: %+v", on, c)
		}
	}
	if _, err := p.Update(func(s *settings.Settings) { s.Equalizer.Headphones = settings.Headphones{} }); err != nil {
		t.Fatal(err)
	}
	if c := e.Correction(); len(c.Filters) != 0 || c.Preamp != 0 {
		t.Errorf("None: %+v", c)
	}

	// A profile that fails says why, in the windows' words.
	if _, err := p.HeadphonesProfile(context.Background(), list.Entries[1]); ErrorWords(err) != locales.EqualizerHeadphonesServer("503 Service Unavailable") {
		t.Errorf("a 503: %v, said as %q", err, ErrorWords(err))
	}
	gone := autoeq.Entry{Name: "Gone", Path: "Lab A/in-ear/Gone"}
	if _, err := p.HeadphonesProfile(context.Background(), gone); ErrorWords(err) != locales.EqualizerHeadphonesGone() {
		t.Errorf("a 404: %v, said as %q", err, ErrorWords(err))
	}
}

// TestHeadphonesListOffline says why the list can't load with none kept,
// and why it couldn't be checked with one kept, which it shows.
func TestHeadphonesListOffline(t *testing.T) {
	srv := headphonesServer(t)
	url := srv.URL + "/results/"
	srv.Close()
	a := newQuietApp(t, func(o *Options) { o.AutoEqURL = url })
	p := &Preferences{a}
	if _, err := p.HeadphonesIndex(context.Background()); err == nil || ErrorWords(err) == "" {
		t.Errorf("offline, none kept: %v", err)
	}
	dir := filepath.Join(a.opts.DataDir, "autoeq")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte("- [Acme One](./Lab%20A/in-ear/Acme%20One) by Lab A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	list, err := p.HeadphonesIndex(context.Background())
	if err != nil || len(list.Entries) != 1 || list.Offline == nil || ErrorWords(list.Offline) == "" {
		t.Errorf("offline, one kept: %+v, %v", list, err)
	}
}

// TestImportHeadphones reads a ParametricEQ.txt, named after its file,
// and says why one doesn't read, in the windows' words.
func TestImportHeadphones(t *testing.T) {
	p := &Preferences{}
	dir := t.TempDir()
	write := func(name, text string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	for file, name := range map[string]string{"Acme One ParametricEQ.txt": "Acme One", "desk.txt": "desk", "ParametricEQ.txt": "ParametricEQ"} {
		h, err := p.ImportHeadphones(write(file, headphonesProfile))
		if err != nil || h.ID != settings.HeadphonesFile || h.Name != name || h.Preamp != -4.2 || len(h.Filters) != 2 {
			t.Errorf("%s: %+v, %v", file, h, err)
		}
	}
	many := "Preamp: -3 dB\n" + strings.Repeat("Filter: ON PK Fc 1000 Hz Gain 1 dB Q 1\n", 21)
	for _, c := range []struct{ text, words string }{
		{"Channel: L\n", locales.EqualizerHeadphonesLineNot(1)},
		{"Filter 1: ON BP Fc 1000 Hz Q 1\n", locales.EqualizerHeadphonesLineType(1, "BP")},
		{"\nFilter 2: ON PK Fc 100 Hz Gain 1 dB\n", locales.EqualizerHeadphonesLineMissing(2, "Q")},
		{"Filter: ON PK Fc 100 Hz Fc 100 Hz Gain 1 dB Q 1\n", locales.EqualizerHeadphonesLineTwice(1, "Fc")},
		{"Filter: ON PK Fc 100000 Hz Gain 1 dB Q 1\n", locales.EqualizerHeadphonesLineRange(1, "Fc", "1 Hz", "22000 Hz")},
		{"Preamp: -40 dB\n", locales.EqualizerHeadphonesLineRange(1, "Preamp", "-30 dB", "30 dB")},
		{"Filter: ON PK Fc 100 Hz Gain 1 dB BW Oct 1\n", locales.EqualizerHeadphonesLineBandwidth(1)},
		{"Filter: ON PK Fc 100 Hz Gain 1 dB Q 1 more\n", locales.EqualizerHeadphonesLineLeft(1, "more")},
		{"# nothing\n", locales.EqualizerHeadphonesNoFilters()},
		{many, locales.EqualizerHeadphonesTooMany(21, 20)},
		{strings.Repeat("#\n", 40000), locales.EqualizerHeadphonesTooBig()},
	} {
		_, err := p.ImportHeadphones(write("bad.txt", c.text))
		if got := ErrorWords(err); got != c.words {
			t.Errorf("%.40q: %v, said as %q, want %q", c.text, err, got, c.words)
		}
		if err != nil && !strings.Contains(err.Error(), "bad.txt") {
			t.Errorf("%.40q: the logs' words don't name the file: %v", c.text, err)
		}
	}
	if _, err := p.ImportHeadphones(filepath.Join(dir, "gone.txt")); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Errorf("no file: %v", err)
	}
}

// TestCorrectionOf gives the engine the filters of the correction the
// settings keep, and none for None.
func TestCorrectionOf(t *testing.T) {
	if c := correctionOf(settings.Default().Equalizer.Headphones); len(c.Filters) != 0 || c.Preamp != 0 || c.Filters == nil {
		t.Errorf("None: %+v", c)
	}
	h := settings.Headphones{ID: settings.HeadphonesFile, Preamp: -2, Filters: []settings.HeadphoneFilter{{Type: settings.FilterHighPass, Fc: 20, Q: 0.7}}}
	if c := correctionOf(h); c.Preamp != -2 || !slices.Equal(c.Filters, []audio.CorrectionFilter{{Type: audio.CorrectionHighPass, Hz: 20, Q: 0.7}}) {
		t.Errorf("a file's: %+v", c)
	}
	h.ID = ""
	if c := correctionOf(h); len(c.Filters) != 0 || c.Preamp != 0 {
		t.Errorf("filters of none: %+v", c)
	}
}
