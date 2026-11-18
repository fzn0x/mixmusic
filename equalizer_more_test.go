package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// The equalizer's presets of the user's own, its simple mode, Auto
// headroom, and the rows of Sound after it: balance, mono, crossfeed and
// width (#615).

// eqSaved is the equalizer settings.json keeps.
func eqSaved(s *shell) settings.Equalizer { return s.svc.Preferences.Get().Equalizer }

// eqButton clicks the button named name, which a link of the same name
// may sit beside.
func eqButton(t *testing.T, s *shell, tt *ui.Tester, name string) {
	t.Helper()
	a, ok := tt.AccessByRole(ui.RoleButton, name)
	if !ok {
		t.Fatalf("no button %q: %q", name, tt.Texts())
	}
	tt.ClickAt(a.Bounds.X+a.Bounds.W/2, a.Bounds.Y+a.Bounds.H/2)
	s.drain()
	tt.Frame()
}

// eqTypeName empties the field of the dialog that asks for a name, and
// types name there.
func eqTypeName(s *shell, tt *ui.Tester, name string) {
	for range len([]rune(s.naming.value)) {
		tt.Key(0, ui.KeyBackspace)
	}
	tt.Type(name)
	tt.Frame()
}

// TestEqualizerPresetsOfYourOwn saves the equalizer as a preset, which the
// menu lists after the built-in ones and plays with its balance; asks
// before a name taken replaces a preset, ignoring case; turns down a name
// too long or a built-in preset's; renames and deletes it.
func TestEqualizerPresetsOfYourOwn(t *testing.T) {
	s := newTestShell(t)
	// Tall enough for the menu to show all its presets.
	tt := testerOf(t, s, 1280, 2000)
	show(s, tt, "/settings/playback")
	click(t, s, tt, "Flat")
	click(t, s, tt, "Rock")
	until(t, s, tt, "Rock chosen", func() bool { return eqSaved(s).Preset == "rock" })
	s.set.Equalizer.Balance = -30
	s.save()
	tt.Frame()

	click(t, s, tt, "Save as…")
	eqTypeName(s, tt, "Desk")
	eqButton(t, s, tt, "Save")
	until(t, s, tt, "the preset saved", func() bool { return eqSaved(s).Chosen() == 0 })
	p := eqSaved(s).Presets[0]
	if p.Name != "Desk" || p.Balance != -30 || !slices.Equal(p.Gains, []float64{4, 3, 2, 0, -1, 0, 2, 3, 4, 4}) {
		t.Errorf("saved %+v", p)
	}
	if sel, ok := tt.AccessByRole(ui.RolePopUpButton, "Preset"); !ok || sel.Value != "Desk" {
		t.Errorf("the menu shows %+v", sel)
	}
	for _, want := range []string{"Rename…", "Delete", "Export…", "Export all…", "Import…"} {
		if !tt.HasText(want) {
			t.Errorf("no %q under the menu: %q", want, tt.Texts())
		}
	}

	// Chosen again from the menu, after Flat, it plays with its balance.
	click(t, s, tt, "Desk")
	click(t, s, tt, "Flat")
	s.set.Equalizer.Balance = 0
	s.save()
	tt.Frame()
	click(t, s, tt, "Flat")
	click(t, s, tt, "Desk")
	until(t, s, tt, "Desk played", func() bool { q := eqSaved(s); return q.Chosen() == 0 && q.Balance == -30 })

	// A name taken, ignoring case, asks first.
	s.set.Equalizer.Gains[0] = 6
	s.save()
	tt.Frame()
	click(t, s, tt, "Save as…")
	if s.naming == nil || s.naming.value != "Desk" {
		t.Fatalf("the dialog doesn't start from the preset's name: %+v", s.naming)
	}
	eqTypeName(s, tt, "DESK")
	eqButton(t, s, tt, "Save")
	if !tt.HasText("Replace Desk?") {
		t.Fatalf("no question: %q", tt.Texts())
	}
	eqButton(t, s, tt, "Replace")
	until(t, s, tt, "the preset replaced", func() bool {
		q := eqSaved(s)
		return len(q.Presets) == 1 && q.Presets[0].Name == "DESK" && q.Presets[0].Gains[0] == 6
	})

	// Too long, or a built-in preset's: the dialog shows again.
	for _, c := range []struct{ name, why string }{
		{strings.Repeat("x", settings.MaxPresetName+1), "A preset's name can have up to 48 characters."},
		{"bass BOOST", "A built-in preset is called Bass boost. Choose another name."},
	} {
		click(t, s, tt, "Save as…")
		eqTypeName(s, tt, c.name)
		eqButton(t, s, tt, "Save")
		if !tt.HasText(c.why) || s.naming == nil || s.naming.value != c.name {
			t.Errorf("%q: the window shows %q", c.name, tt.Texts())
		}
		tt.Key(0, ui.KeyEscape)
		s.drain()
		tt.Frame()
	}
	if n := len(eqSaved(s).Presets); n != 1 {
		t.Errorf("%d presets", n)
	}

	click(t, s, tt, "Rename…")
	eqTypeName(s, tt, "Night")
	eqButton(t, s, tt, "Rename")
	until(t, s, tt, "the preset renamed", func() bool { q := eqSaved(s); return q.Presets[0].Name == "Night" && q.Chosen() == 0 })

	click(t, s, tt, "Delete")
	if !tt.HasText("Delete Night?") {
		t.Fatalf("no question: %q", tt.Texts())
	}
	eqButton(t, s, tt, "Delete")
	until(t, s, tt, "the preset deleted", func() bool { q := eqSaved(s); return len(q.Presets) == 0 && q.Preset == "custom" })
	if q := eqSaved(s); q.Gains[0] != 6 || q.Balance != -30 {
		t.Errorf("deleting the preset changed the sound: %+v", q)
	}
}

// TestEqualizerExportImport saves the user's presets as a JSON file and
// adds those of a file to theirs, asking before it replaces one of the
// same name; a file of no presets says so.
func TestEqualizerExportImport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mine")
	saveDialog, openDialog := saveFileDialog, openFileDialog
	t.Cleanup(func() { saveFileDialog, openFileDialog = saveDialog, openDialog })
	saveFileDialog = func(opts mygo.SaveDialogOptions) (string, error) {
		if opts.DefaultPath != "Equalizer presets.json" {
			t.Errorf("the file is called %q at first", opts.DefaultPath)
		}
		return path, nil
	}
	s := newTestShell(t)
	s.set.Equalizer.Save("Desk", -2)
	s.set.Equalizer.Mono = true
	s.set.Equalizer.Save("One ear", 0)
	s.save()
	tt := testerOf(t, s, 1280, 1000)
	show(s, tt, "/settings/playback")
	click(t, s, tt, "Export all…")
	until(t, s, tt, "the presets exported", func() bool { return tt.HasText("Exported 2 presets as mine.json") })
	data, err := os.ReadFile(path + ".json")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := settings.DecodePresets(data); err != nil || len(got) != 2 || got[1].Name != "One ear" || !got[1].Mono {
		t.Errorf("the file holds %+v, %v", got, err)
	}

	s.set.Equalizer.Delete("One ear")
	s.save()
	tt.Frame()
	openFileDialog = func(mygo.OpenDialogOptions) ([]string, error) { return []string{path + ".json"}, nil }
	click(t, s, tt, "Import…")
	until(t, s, tt, "the question", func() bool { return tt.HasText("Replace 1 preset?") })
	eqButton(t, s, tt, "Replace")
	until(t, s, tt, "the presets imported", func() bool { return len(eqSaved(s).Presets) == 2 && tt.HasText("Imported 2 presets") })

	bad := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(bad, []byte(`{"volume": 0.5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	openFileDialog = func(mygo.OpenDialogOptions) ([]string, error) { return []string{bad}, nil }
	click(t, s, tt, "Import…")
	until(t, s, tt, "the failure", func() bool {
		return tt.HasText("Couldn't import the presets: the file holds no equalizer presets")
	})
}

// TestEqualizerSimpleMode shows Bass, Mid and Treble in place of the ten
// bands, at what the bands are, and moving one sets the bands.
func TestEqualizerSimpleMode(t *testing.T) {
	s := newTestShell(t)
	s.set.Equalizer.Enabled = true
	s.set.Equalizer.Gains = settings.ToneGains(6, 0, -3)
	s.save()
	tt := testerOf(t, s, 1280, 1000)
	show(s, tt, "/settings/playback")
	reveal(tt, "Simple mode")
	clickSwitch(t, s, tt, "Simple mode")
	until(t, s, tt, "simple mode", func() bool { return eqSaved(s).Simple })
	if tt.HasText("64 dB") || !tt.HasText("Bass dB") || !tt.HasText("Mid dB") || !tt.HasText("Treble dB") {
		t.Fatalf("the sliders: %q", tt.Texts())
	}
	if q := eqSaved(s); q.Bass != 6 || q.Mid != 0 || q.Treble != -3 {
		t.Errorf("the three show %v %v %v", q.Bass, q.Mid, q.Treble)
	}
	reveal(tt, "Bass dB")
	a, _ := tt.AccessByRole(ui.RoleSlider, "Bass dB")
	tt.ClickAt(a.Bounds.X+a.Bounds.W/2, a.Bounds.Y+a.Bounds.H/2)
	tt.Key(0, ui.KeyEnd)
	until(t, s, tt, "the bass up", func() bool { return eqSaved(s).Bass == 12 })
	if q := eqSaved(s); !slices.Equal(q.Gains, settings.ToneGains(12, 0, -3)) || q.Preset != "custom" {
		t.Errorf("the bands are %v, the preset %q", q.Gains, q.Preset)
	}
}

// TestEqualizerAutoHeadroom shows Auto headroom's cut on the preamp, keeps
// it as the preamp's own when it's turned off, and turns off as the
// preamp moves.
func TestEqualizerAutoHeadroom(t *testing.T) {
	s := newTestShell(t)
	s.set.Equalizer.Enabled = true
	s.set.Equalizer.Gains = []float64{6, 5, 4, 2, 0, 0, 0, 0, 0, 0}
	s.save()
	h := app.Headroom(s.set.Equalizer.Gains)
	if h >= -6 {
		t.Fatalf("a cut of %v for Bass boost", h)
	}
	tt := testerOf(t, s, 1280, 1000)
	show(s, tt, "/settings/playback")
	if !tt.HasText(eqGainLabel(h)) {
		t.Errorf("the preamp doesn't show %s: %q", eqGainLabel(h), tt.Texts())
	}
	reveal(tt, "Auto headroom")
	clickSwitch(t, s, tt, "Auto headroom")
	until(t, s, tt, "Auto headroom off", func() bool { return !eqSaved(s).AutoHeadroom })
	if q := eqSaved(s); q.Preamp != h {
		t.Errorf("the preamp went to %v, from %v", q.Preamp, h)
	}
	clickSwitch(t, s, tt, "Auto headroom")
	until(t, s, tt, "Auto headroom on", func() bool { return eqSaved(s).AutoHeadroom })
	reveal(tt, "pre dB")
	a, _ := tt.AccessByRole(ui.RoleSlider, "pre dB")
	tt.ClickAt(a.Bounds.X+a.Bounds.W/2, a.Bounds.Y+a.Bounds.H/2)
	tt.Key(0, ui.KeyHome)
	until(t, s, tt, "the preamp moved", func() bool { q := eqSaved(s); return !q.AutoHeadroom && q.Preamp == settings.MinPreamp })
}

// TestBalanceDetent rests the balance at the centre as it comes near it,
// and lets it move on from there.
func TestBalanceDetent(t *testing.T) {
	for _, c := range []struct {
		was  int
		v    float64
		want int
	}{{20, 3, 0}, {-30, -4.2, 0}, {5, 4, 0}, {0, 3, 3}, {0, -1, -1}, {3, 4, 4}, {40, 5, 5}, {-60, -100, -100}} {
		if got := balanceAt(c.was, c.v); got != c.want {
			t.Errorf("from %d to %v: %d, want %d", c.was, c.v, got, c.want)
		}
	}
}

// TestSoundRows shows balance, mono, crossfeed and width in Sound, each
// telling where it is, and the notes of the stages that wait: while Sound
// effects is off, and while 5.1 surround plays.
func TestSoundRows(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 1000)
	show(s, tt, "/settings/playback")
	for _, want := range []string{"Balance", "Even on both sides.", "Mono", "Crossfeed", "Stereo width",
		"100%: at 0% the sound is mono, at 100% as recorded, and wider above."} {
		if !tt.HasText(want) {
			t.Errorf("no %q: %q", want, tt.Texts())
		}
	}
	slide := func(name string, key ui.Key) {
		t.Helper()
		reveal(tt, name)
		a, ok := tt.AccessByRole(ui.RoleSlider, name)
		if !ok {
			t.Fatalf("no slider %q", name)
		}
		tt.ClickAt(a.Bounds.X+a.Bounds.W/2, a.Bounds.Y+a.Bounds.H/2)
		tt.Key(0, key)
		s.drain()
		tt.Frame()
	}
	slide("Balance", ui.KeyHome)
	until(t, s, tt, "the balance left", func() bool { return eqSaved(s).Balance == -100 })
	if !tt.HasText("100% to the left: the right side plays quieter.") {
		t.Errorf("the balance tells %q", tt.Texts())
	}
	slide("Crossfeed", ui.KeyEnd)
	until(t, s, tt, "the crossfeed up", func() bool { return eqSaved(s).Crossfeed == 100 })
	if !tt.HasText("100%: each ear hears a little of the other side, softened, as from speakers.") {
		t.Errorf("the crossfeed tells %q", tt.Texts())
	}
	clickSwitch(t, s, tt, "Mono")
	until(t, s, tt, "mono", func() bool { return eqSaved(s).Mono })

	for _, note := range []string{"Sound effects are off", "While 5.1 surround plays"} {
		if hasTextStarting(tt.Texts(), note) {
			t.Errorf("%q shows", note)
		}
	}
	s.set.Equalizer.Effects, s.set.Surround = false, true
	s.save()
	tt.Frame()
	for _, note := range []string{"Sound effects are off", "While 5.1 surround plays"} {
		if !hasTextStarting(tt.Texts(), note) {
			t.Errorf("no note %q: %q", note, tt.Texts())
		}
	}
}

// TestBalanceLeftToRight keeps the balance's left the left side in a
// language written right to left: a click a quarter along from the left
// leans it left.
func TestBalanceLeftToRight(t *testing.T) {
	s := rtlShell(t, settings.LayoutTiles)
	tt := rtlTester(s, 1280, 1000, ui.RightToLeft)
	show(s, tt, "/settings/playback")
	reveal(tt, locales.EqualizerBalance())
	a, ok := tt.AccessByRole(ui.RoleSlider, locales.EqualizerBalance())
	if !ok {
		t.Fatalf("no slider: %q", tt.Texts())
	}
	tt.ClickAt(a.Bounds.X+a.Bounds.W/4, a.Bounds.Y+a.Bounds.H/2)
	s.drain()
	tt.Frame()
	until(t, s, tt, "the balance to the left", func() bool { b := eqSaved(s).Balance; return b < -30 && b > -70 })
}
