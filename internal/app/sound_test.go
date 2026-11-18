package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// TestSoundOf gives the engine the balance and mono of the settings
// always, and their crossfeed and width while Sound effects is on.
func TestSoundOf(t *testing.T) {
	q := settings.Default().Equalizer
	if s := soundOf(q); s != (audio.Sound{Width: 1}) {
		t.Errorf("the defaults shape the sound: %+v", s)
	}
	q.Balance, q.Mono, q.Crossfeed, q.Width = -50, true, 40, 150
	if s := soundOf(q); s != (audio.Sound{Balance: -0.5, Mono: true, Crossfeed: 0.4, Width: 1.5}) {
		t.Errorf("Sound effects on: %+v", s)
	}
	q.Effects = false
	if s := soundOf(q); s != (audio.Sound{Balance: -0.5, Mono: true, Width: 1}) {
		t.Errorf("Sound effects off: %+v", s)
	}
}

// TestHeadroomOfBands is Auto headroom's cut for the bands of the
// settings: none for flat ones, the engine's for a boost.
func TestHeadroomOfBands(t *testing.T) {
	if h := Headroom(make([]float64, 10)); h != 0 {
		t.Errorf("flat bands: %v", h)
	}
	gains := []float64{6, 5, 4, 2, 0, 0, 0, 0, 0, 0}
	if h, want := Headroom(gains), audio.Headroom([10]float64{6, 5, 4, 2}); h != want || h > -6 {
		t.Errorf("Bass boost: %v, want %v", h, want)
	}
	if h := Headroom([]float64{1}); h != Headroom([]float64{1, 0, 0, 0, 0, 0, 0, 0, 0, 0}) {
		t.Errorf("too few gains: %v", h)
	}
}

// TestApplySound gives MixMusic's own engine the equalizer and the stages
// after it as the settings change.
func TestApplySound(t *testing.T) {
	a := newQuietApp(t, nil)
	e := audio.NewEngine(a.openNative, startSilent, engineEvents{a})
	t.Cleanup(e.Close)
	a.nativeOnce.Do(func() { a.native = e })
	a.applyEngine(a.settings.Get().Clean())
	if got := e.Sound(); got != (audio.Sound{Width: 1}) {
		t.Errorf("at first: %+v", got)
	}
	for _, c := range []struct {
		effects bool
		want    audio.Sound
	}{
		{true, audio.Sound{Balance: 0.3, Crossfeed: 0.5, Width: 0.8}},
		{false, audio.Sound{Balance: 0.3, Width: 1}},
	} {
		if _, err := (&Preferences{a}).Update(func(s *settings.Settings) {
			q := &s.Equalizer
			q.Enabled, q.Effects, q.Balance, q.Crossfeed, q.Width = true, c.effects, 30, 50, 80
		}); err != nil {
			t.Fatal(err)
		}
		if got := e.Sound(); got != c.want {
			t.Errorf("Sound effects %v: %+v, want %+v", c.effects, got, c.want)
		}
	}
}

// TestEqualizerPresetFiles exports presets and imports them back, and
// says why a file of no presets doesn't import, in the windows' words.
func TestEqualizerPresetFiles(t *testing.T) {
	p := &Preferences{}
	dir := t.TempDir()
	path := filepath.Join(dir, "presets.json")
	presets := []settings.EqualizerPreset{{Name: "Desk", Gains: []float64{1, 2, 3, 0, 0, 0, 0, 0, 0, 0}, Width: 100, Crossfeed: 30}}
	if err := p.ExportEqualizerPresets(path, presets); err != nil {
		t.Fatal(err)
	}
	got, err := p.ImportEqualizerPresets(path)
	if err != nil || len(got) != 1 || got[0].Name != "Desk" || got[0].Crossfeed != 30 || got[0].Gains[2] != 3 {
		t.Errorf("imported %+v, %v", got, err)
	}

	other := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(other, []byte(`{"volume": 0.5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ImportEqualizerPresets(other); ErrorWords(err) != locales.EqualizerImportNoPresets() {
		t.Errorf("a file of no presets: %v, which the windows say as %q", err, ErrorWords(err))
	}
	if _, err := p.ImportEqualizerPresets(filepath.Join(dir, "gone.json")); err == nil {
		t.Error("a file that isn't there imports")
	}
}
