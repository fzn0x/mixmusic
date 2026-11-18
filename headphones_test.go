package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// The Headphones row of Settings › Playback › Equalizer, the dialog of
// AutoEq's profiles, from a server of made-up ones, and Import file…
// (#616).

// hpProfile is a profile in AutoEq's form, of made-up values.
const hpProfile = "Preamp: -4.2 dB\nFilter 1: ON LSC Fc 105 Hz Gain 3.5 dB Q 0.70\nFilter 2: ON PK Fc 2400 Hz Gain -2.0 dB Q 1.50\n"

// hpServer serves AutoEq's results: an index of a model of two profiles,
// one of which fails, and of a model of one, and counts the requests.
func hpServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	var asked atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Add(1)
		switch r.URL.Path {
		case "/results/INDEX.md":
			w.Write([]byte("# Index\n" +
				"- [Acme One](./Lab%20A/711%20in-ear/Acme%20One) by Lab A on 711\n" +
				"- [Acme One](./Lab%20B/in-ear/Acme%20One) by Lab B\n" +
				"- [Zed Studio](./Lab%20B/over-ear/Zed%20Studio) by Lab B\n"))
		case "/results/Lab A/711 in-ear/Acme One/Acme One ParametricEQ.txt":
			w.Write([]byte(hpProfile))
		case "/results/Lab B/over-ear/Zed Studio/Zed Studio ParametricEQ.txt":
			w.Write([]byte(hpProfile))
		case "/results/Lab B/in-ear/Acme One/Acme One ParametricEQ.txt":
			w.Write([]byte("Filter 1: ON BP Fc 1000 Hz Q 1\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &asked
}

// hpShell is a shell whose AutoEq is the server srv, on the playback
// settings, in a window tall enough for the equalizer.
func hpShell(t *testing.T, srv *httptest.Server) (*shell, *ui.Tester) {
	s := newTestShellOf(t, func(o *app.Options) { o.AutoEqURL = srv.URL + "/results/" })
	tt := testerOf(t, s, 1280, 2000)
	show(s, tt, "/settings/playback")
	return s, tt
}

// hpSaved is the correction settings.json keeps.
func hpSaved(s *shell) settings.Headphones { return s.svc.Preferences.Get().Equalizer.Headphones }

// TestHeadphonesPicker opens AutoEq's list, which loads, finds a model by
// the words typed, groups the profiles of a model under its name, and
// plays the profile picked, closing; one that fails says why, and the
// correction stays. None turns it off.
func TestHeadphonesPicker(t *testing.T) {
	srv, asked := hpServer(t)
	s, tt := hpShell(t, srv)
	if !tt.HasText(locales.EqualizerHeadphonesDesc()) {
		t.Fatalf("no row of Headphones: %q", tt.Texts())
	}
	if n := asked.Load(); n != 0 {
		t.Errorf("%d requests before the list opened", n)
	}
	click(t, s, tt, "Choose…")
	until(t, s, tt, "the list", func() bool { return tt.HasText("3 profiles") })
	for _, want := range []string{"None", "Acme One", "by Lab A on 711", "by Lab B", "Zed Studio", "Search headphones"} {
		if !tt.HasText(want) {
			t.Errorf("the dialog doesn't show %q: %q", want, tt.Texts())
		}
	}
	if !tt.HasText(locales.EqualizerHeadphonesCredit(locales.EqualizerHeadphonesAutoEq())) {
		t.Errorf("no credit to AutoEq: %q", tt.Texts())
	}

	tt.Type("zed")
	tt.Frame()
	if !tt.HasText("1 profile") || tt.HasText("Acme One") || !tt.HasText("Zed Studio") {
		t.Errorf("searching zed: %q", tt.Texts())
	}
	for range 3 {
		tt.Key(0, ui.KeyBackspace)
	}
	tt.Type("711")
	tt.Frame()
	if !tt.HasText("1 profile") || !tt.HasText("Acme One") || tt.HasText("Zed Studio") || tt.HasText("by Lab B") {
		t.Errorf("searching 711: %q", tt.Texts())
	}
	for range 3 {
		tt.Key(0, ui.KeyBackspace)
	}
	tt.Type("nothing like it")
	tt.Frame()
	if !tt.HasText(locales.EqualizerHeadphonesNoMatch()) {
		t.Errorf("searching for nothing: %q", tt.Texts())
	}
	for range len("nothing like it") {
		tt.Key(0, ui.KeyBackspace)
	}
	tt.Frame()

	click(t, s, tt, "by Lab A on 711")
	until(t, s, tt, "the profile picked", func() bool { return hpSaved(s).ID == "Lab A/711 in-ear/Acme One" })
	h := hpSaved(s)
	if h.Name != "Acme One" || h.Source != "Lab A" || h.Rig != "711" || h.Preamp != -4.2 || len(h.Filters) != 2 {
		t.Errorf("saved %+v", h)
	}
	until(t, s, tt, "the dialog closed", func() bool { return !s.headphones.open })
	if want := locales.EqualizerHeadphonesProfileRig("Acme One", "Lab A", "711"); !tt.HasText(want) {
		t.Errorf("the row doesn't say %q: %q", want, tt.Texts())
	}

	// A profile that doesn't read says why, and the one in use stays.
	click(t, s, tt, "Choose…")
	until(t, s, tt, "the list again", func() bool { return tt.HasText("3 profiles") })
	click(t, s, tt, "by Lab B")
	why := locales.EqualizerHeadphonesPickFailed("Acme One", locales.EqualizerHeadphonesLineType(1, "BP"))
	until(t, s, tt, "the failure", func() bool { return tt.HasText(why) })
	if hpSaved(s).ID != "Lab A/711 in-ear/Acme One" || !s.headphones.open {
		t.Errorf("the failure changed the correction to %+v, or closed the dialog", hpSaved(s))
	}

	click(t, s, tt, "None")
	until(t, s, tt, "None", func() bool { return !hpSaved(s).On() })
	if !tt.HasText(locales.EqualizerHeadphonesDesc()) {
		t.Errorf("the row after None: %q", tt.Texts())
	}
}

// TestHeadphonesListFails says why the list didn't load, with Try again;
// None is still there to choose.
func TestHeadphonesListFails(t *testing.T) {
	srv, _ := hpServer(t)
	s, tt := hpShell(t, srv)
	srv.Close()
	click(t, s, tt, "Choose…")
	until(t, s, tt, "the failure", func() bool { return tt.HasText("Try again") })
	if !tt.HasText("None") {
		t.Errorf("no None: %q", tt.Texts())
	}
	click(t, s, tt, "Try again")
	until(t, s, tt, "the failure again", func() bool { return tt.HasText("Try again") && !s.headphones.loading })
}

// TestHeadphonesImport reads a ParametricEQ.txt, which then corrects the
// sound; a file that doesn't read says why, and changes nothing.
func TestHeadphonesImport(t *testing.T) {
	open := openFileDialog
	t.Cleanup(func() { openFileDialog = open })
	dir := t.TempDir()
	good := filepath.Join(dir, "Desk ParametricEQ.txt")
	bad := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(good, []byte(hpProfile), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("Preamp: -3 dB\nDevice: Speakers\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv, asked := hpServer(t)
	s, tt := hpShell(t, srv)
	openFileDialog = func(o mygo.OpenDialogOptions) ([]string, error) {
		if len(o.Filters) != 1 || o.Filters[0].Extensions[0] != "txt" {
			t.Errorf("the dialog asks for %+v", o.Filters)
		}
		return []string{good}, nil
	}
	click(t, s, tt, "Import file…")
	until(t, s, tt, "the file read", func() bool { return hpSaved(s).ID == settings.HeadphonesFile })
	if h := hpSaved(s); h.Name != "Desk" || len(h.Filters) != 2 {
		t.Errorf("saved %+v", h)
	}
	until(t, s, tt, "the row", func() bool { return tt.HasText(locales.EqualizerHeadphonesFile("Desk")) })

	openFileDialog = func(mygo.OpenDialogOptions) ([]string, error) { return []string{bad}, nil }
	click(t, s, tt, "Import file…")
	why := locales.EqualizerHeadphonesImportFailed(locales.EqualizerHeadphonesLineNot(2))
	until(t, s, tt, "the failure", func() bool { return tt.HasText(why) })
	if h := hpSaved(s); h.Name != "Desk" {
		t.Errorf("the failure changed the correction to %+v", h)
	}
	if n := asked.Load(); n != 0 {
		t.Errorf("importing a file made %d requests", n)
	}
}

// TestHeadphonesLongList builds the rows of a long list near its view
// alone, and the others as it scrolls to them.
func TestHeadphonesLongList(t *testing.T) {
	var index strings.Builder
	for i := range 6000 {
		fmt.Fprintf(&index, "- [Model %04d](./Lab/in-ear/Model%%20%04d) by Lab\n", i, i)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(index.String()))
	}))
	t.Cleanup(srv.Close)
	s, tt := hpShell(t, srv)
	click(t, s, tt, "Choose…")
	until(t, s, tt, "the list", func() bool { return tt.HasText("6,000 profiles") || tt.HasText("6000 profiles") })
	// The rows built, each of which shows its model and is labeled so.
	built := func() int {
		rows := map[string]bool{}
		for _, text := range tt.Texts() {
			if strings.HasPrefix(text, "Model ") {
				rows[text] = true
			}
		}
		return len(rows)
	}
	if n := built(); n == 0 || n > 200 {
		t.Errorf("%d rows of 6000 built", n)
	}
	r, ok := tt.Find("Model 0000")
	if !ok {
		t.Fatalf("no first row: %q", tt.Texts())
	}
	for range 400 {
		tt.Scroll(r.X+r.W/2, r.Y+r.H/2, 0, 400)
		if tt.HasText("Model 0300") {
			break
		}
	}
	if !tt.HasText("Model 0300") || tt.HasText("Model 0000") || built() > 200 {
		t.Errorf("scrolled, %d rows built: %q", built(), tt.Texts())
	}
}

// TestPseudoLocaleHeadphones shows the row of Headphones, the dialog of
// AutoEq's profiles and the toast of a profile that fails in en-XA: only
// the names of the models, their sources and rigs, AutoEq's, are as
// AutoEq gives them.
func TestPseudoLocaleHeadphones(t *testing.T) {
	usePseudoLocale(t)
	srv, _ := hpServer(t)
	s := newTestShellOf(t, func(o *app.Options) { o.AutoEqURL = srv.URL + "/results/" })
	s.set.Equalizer.Headphones = settings.Headphones{ID: "Lab A/711 in-ear/Acme One", Name: "Acme One", Source: "Lab A", Rig: "711",
		Filters: []settings.HeadphoneFilter{{Type: settings.FilterPeak, Fc: 100, Gain: 1, Q: 1}}}
	s.save()
	tt := testerOf(t, s, 1280, 2000)
	tt.SetLocale("en-XA")
	xa := locales.In("en-XA")
	show(s, tt, "/settings/playback")
	if want := xa.EqualizerHeadphonesProfileRig("Acme One", "Lab A", "711"); !tt.HasText(want) {
		t.Errorf("no %q: %q", want, tt.Texts())
	}
	click(t, s, tt, xa.EqualizerHeadphonesChoose())
	until(t, s, tt, "the list", func() bool { return tt.HasText(xa.EqualizerHeadphonesFound(3)) })
	click(t, s, tt, xa.EqualizerHeadphonesBy("Lab B"))
	until(t, s, tt, "the failure", func() bool {
		return tt.HasText(xa.EqualizerHeadphonesPickFailed("Acme One", xa.EqualizerHeadphonesLineType(1, "BP")))
	})
	checkPseudo(t, "the dialog of Headphones", tt.Texts(), "Acme One", "Zed Studio", "Lab A", "Lab B", "711", "AutoEq", "Jaakko Pasanen", "MIT", "ParametricEQ.txt", "BP", "PK", "LSC", "HSC", "LP", "HP")
}
