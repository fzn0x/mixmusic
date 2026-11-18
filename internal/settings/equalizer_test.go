package settings

import (
	"slices"
	"strings"
	"testing"
)

// TestEqualizerOfEarlierVersions reads the equalizer of a settings.json
// from before #615: Auto headroom and Sound effects are on, the width as
// recorded, and nothing else shapes the sound.
func TestEqualizerOfEarlierVersions(t *testing.T) {
	s, err := Decode([]byte(`{"equalizer": {"enabled": true, "preset": "bass", "gains": [6, 5, 4, 2, 0, 0, 0, 0, 0, 0], "preamp": -3}}`))
	if err != nil {
		t.Fatal(err)
	}
	e := s.Equalizer
	if !e.Enabled || e.Preset != "bass" || e.Preamp != -3 || e.Gains[0] != 6 {
		t.Errorf("the equalizer's own settings changed: %+v", e)
	}
	if !e.AutoHeadroom || !e.Effects || e.Width != 100 || e.Balance != 0 || e.Mono || e.Crossfeed != 0 || e.Simple {
		t.Errorf("the new settings aren't their defaults: %+v", e)
	}
	if e.Presets == nil || len(e.Presets) != 0 {
		t.Errorf("presets %#v, want an empty list", e.Presets)
	}
}

// TestEqualizerClean keeps each value in its range: the preamp down to
// -30 dB, the balance, crossfeed, width and simple sliders in theirs, and
// the user's presets each with a name of its own.
func TestEqualizerClean(t *testing.T) {
	long := strings.Repeat("é", MaxPresetName+5)
	e := Equalizer{
		Preamp: -40, Bass: 20, Mid: -20, Treble: 3, Balance: -300, Crossfeed: 150, Width: 999,
		Presets: []EqualizerPreset{
			{Name: "  Night  ", Gains: []float64{1, 2}, Preamp: 99, Width: -5, Balance: 500},
			{Name: "night", Gains: make([]float64, 10)},
			{Name: "   "},
			{Name: long},
		},
	}.clean()
	if e.Preamp != MinPreamp || e.Bass != 12 || e.Mid != -12 || e.Treble != 3 || e.Balance != -100 || e.Crossfeed != 100 || e.Width != 200 {
		t.Errorf("not clamped: %+v", e)
	}
	if (Equalizer{Preamp: -25}).clean().Preamp != -25 {
		t.Error("a preamp of -25 dB isn't kept")
	}
	if len(e.Presets) != 2 {
		t.Fatalf("presets %+v, want Night and the long one", e.Presets)
	}
	p := e.Presets[0]
	if p.Name != "Night" || len(p.Gains) != len(Bands) || p.Preamp != MaxPreamp || p.Width != 0 || p.Balance != 100 {
		t.Errorf("preset not cleaned: %+v", p)
	}
	if got := []rune(e.Presets[1].Name); len(got) != MaxPresetName {
		t.Errorf("a long name keeps %d characters, want %d", len(got), MaxPresetName)
	}
}

// TestEqualizerPresets saves the equalizer under a name, in place of the
// one of that name ignoring case, plays a saved one with its balance,
// mono, crossfeed and width, renames it, and deletes it, the equalizer
// playing on as it was.
func TestEqualizerPresets(t *testing.T) {
	e := defaultEqualizer()
	e.Gains = []float64{6, 5, 4, 2, 0, 0, 0, 0, 0, 0}
	e.Balance, e.Mono, e.Crossfeed, e.Width = -20, true, 40, 150
	e.Save("Desk", -7.1)
	if e.Preset != SavedPresetID("Desk") || e.Chosen() != 0 || len(e.Presets) != 1 {
		t.Fatalf("saved: %+v", e)
	}
	if p := e.Presets[0]; p.Preamp != -7.1 || !p.AutoHeadroom || p.Balance != -20 || !p.Mono || p.Crossfeed != 40 || p.Width != 150 || p.Gains[0] != 6 {
		t.Errorf("the preset keeps %+v", p)
	}
	e.Gains[0] = 1 // the preset keeps its own
	if e.Presets[0].Gains[0] != 6 {
		t.Error("the preset shares its gains with the equalizer")
	}

	e.Gains = make([]float64, 10)
	e.Save("DESK", 0)
	if len(e.Presets) != 1 || e.Presets[0].Name != "DESK" || e.Presets[0].Gains[0] != 0 {
		t.Errorf("saving under a name taken, ignoring case, didn't replace it: %+v", e.Presets)
	}
	e.Save("Bed", 0)

	var other Equalizer
	other.Apply(e.Presets[0])
	other.Apply(EqualizerPreset{Name: "Walk", Gains: []float64{0, 0, 0, 0, 0, 0, 0, 3, 3, 3}, Preamp: -2, Balance: 30, Mono: true, Crossfeed: 10, Width: 80})
	if !other.Enabled || other.Preset != SavedPresetID("Walk") || other.Preamp != -2 || other.AutoHeadroom ||
		other.Balance != 30 || !other.Mono || other.Crossfeed != 10 || other.Width != 80 || other.Treble == 0 {
		t.Errorf("applied: %+v", other)
	}

	e.Preset = SavedPresetID("desk")
	e.Rename("desk", "Desk speakers")
	if e.Presets[0].Name != "Desk speakers" || e.Preset != SavedPresetID("Desk speakers") {
		t.Errorf("renamed: %+v", e)
	}
	e.Delete("DESK SPEAKERS")
	if len(e.Presets) != 1 || e.Presets[0].Name != "Bed" || e.Preset != "custom" {
		t.Errorf("deleted: %+v", e)
	}

	if taken := e.Taken([]EqualizerPreset{{Name: "bed"}, {Name: "Car"}}); !slices.Equal(taken, []string{"Bed"}) {
		t.Errorf("taken %q", taken)
	}
	e.Import([]EqualizerPreset{{Name: "bed", Gains: []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}}, {Name: "Car"}, {Name: ""}})
	if len(e.Presets) != 2 || e.Presets[0].Name != "bed" || e.Presets[0].Gains[0] != 1 || e.Presets[1].Name != "Car" {
		t.Errorf("imported: %+v", e.Presets)
	}
}

// TestPresetFiles writes the user's presets to a file and reads them back,
// and reads a list of them, or one alone, with its missing keys at what
// changes nothing.
func TestPresetFiles(t *testing.T) {
	presets := []EqualizerPreset{
		{Name: "Desk", Gains: []float64{6, 5, 4, 2, 0, 0, 0, 0, 0, 0}, Preamp: -6.6, AutoHeadroom: true, Width: 100},
		{Name: "One ear", Gains: make([]float64, 10), Mono: true, Balance: -40, Crossfeed: 25, Width: 120},
	}
	data, err := EncodePresets(presets)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"kind": "mixmusic.equalizerPresets"`) {
		t.Errorf("the file doesn't say what it holds:\n%s", data)
	}
	got, err := DecodePresets(data)
	if err != nil || len(got) != 2 || got[0].Name != "Desk" || got[0].Preamp != -6.6 || got[1].Balance != -40 || !got[1].Mono || got[1].Width != 120 {
		t.Errorf("read back %+v, %v", got, err)
	}

	one, err := DecodePresets([]byte("\xEF\xBB\xBF" + `{"name": "Bright", "gains": [0, 0, 0, 0, 0, 0, 1, 2, 3, 4]}`))
	if err != nil || len(one) != 1 || one[0].Name != "Bright" || one[0].Width != 100 || !one[0].AutoHeadroom {
		t.Errorf("one preset alone: %+v, %v", one, err)
	}
	list, err := DecodePresets([]byte(`[{"name": "A", "gains": [1,1,1,1,1,1,1,1,1,1]}, {"name": "a"}, {"name": ""}]`))
	if err != nil || len(list) != 1 || list[0].Name != "a" || list[0].Width != 100 {
		t.Errorf("a list: %+v, %v", list, err)
	}
	for _, bad := range []string{`{"volume": 0.5}`, `[]`, `{"presets": []}`, `[{"name": " "}]`} {
		if _, err := DecodePresets([]byte(bad)); err != ErrNoPresets {
			t.Errorf("%s: %v, want ErrNoPresets", bad, err)
		}
	}
	if _, err := DecodePresets([]byte(`{"presets": [`)); err == nil || err == ErrNoPresets {
		t.Errorf("a broken file: %v", err)
	}
}

// TestPresetsInSettingsFile reads a preset of settings.json written by
// hand, without its width: it plays as recorded, not in mono.
func TestPresetsInSettingsFile(t *testing.T) {
	s, err := Decode([]byte(`{"equalizer": {"presets": [{"name": "Hand", "gains": [1, 0, 0, 0, 0, 0, 0, 0, 0, 0]}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Equalizer.Presets) != 1 || s.Equalizer.Presets[0].Width != 100 || !s.Equalizer.Presets[0].AutoHeadroom {
		t.Errorf("presets %+v", s.Equalizer.Presets)
	}
}

// TestTone sets the bands from Bass, Mid and Treble, flat at one gain when
// the three are at it, and finds the three again from the bands they set.
func TestTone(t *testing.T) {
	if g := ToneGains(3, 3, 3); !slices.Equal(g, []float64{3, 3, 3, 3, 3, 3, 3, 3, 3, 3}) {
		t.Errorf("the three at +3 set %v", g)
	}
	if g := ToneGains(6, 0, 0); g[0] != 6 || g[1] != 6 || g[2] != 4.8 || g[5] != 0 || g[9] != 0 {
		t.Errorf("Bass at +6 sets %v", g)
	}
	for _, c := range [][3]float64{{0, 0, 0}, {6, 0, 0}, {-12, 12, -12}, {2.5, -1.5, 7}, {12, 12, 12}} {
		b, m, tr := ToneOf(ToneGains(c[0], c[1], c[2]))
		if b != c[0] || m != c[1] || tr != c[2] {
			t.Errorf("%v sets bands whose three are %v %v %v", c, b, m, tr)
		}
	}
	if b, m, tr := ToneOf([]float64{1, 2}); b != 0 || m != 0 || tr != 0 {
		t.Error("bands of the wrong count have three")
	}
	if b, _, tr := ToneOf([]float64{12, 12, 12, 12, 0, 0, 0, -12, -12, -12}); b < 9 || tr > -9 {
		t.Errorf("bass up, treble down: %v and %v", b, tr)
	}
}

// TestReverbAndEchoOfEarlierVersions reads the equalizer of a settings.json
// from before #546: the reverb and the echo are off, at their defaults.
func TestReverbAndEchoOfEarlierVersions(t *testing.T) {
	s, err := Decode([]byte(`{"equalizer": {"enabled": true, "effects": true, "width": 100}}`))
	if err != nil {
		t.Fatal(err)
	}
	if r := s.Equalizer.Reverb; r != (Reverb{Room: RoomHall, Mix: 25}) {
		t.Errorf("the reverb is %+v", r)
	}
	if e := s.Equalizer.Echo; e != (Echo{Preset: "slapback", Time: 120, Feedback: 0, Mix: 30}) {
		t.Errorf("the echo is %+v", e)
	}
}

// TestReverbAndEchoClean keeps each value of the reverb and the echo in
// its range: a room it knows, the hall else; the echo's time a whole
// number of 10 ms steps; and the echo's preset one whose values it has,
// Custom else.
func TestReverbAndEchoClean(t *testing.T) {
	for _, c := range []struct{ in, want Reverb }{
		{Reverb{On: true, Room: "cave", Mix: 140}, Reverb{On: true, Room: RoomHall, Mix: 100}},
		{Reverb{Room: RoomPlate, Mix: -3}, Reverb{Room: RoomPlate, Mix: 0}},
		{Reverb{Room: RoomLarge, Mix: 60}, Reverb{Room: RoomLarge, Mix: 60}},
	} {
		if got := c.in.clean(); got != c.want {
			t.Errorf("%+v cleans to %+v, want %+v", c.in, got, c.want)
		}
	}
	for _, c := range []struct{ in, want Echo }{
		{Echo{Preset: "slapback", Time: 120, Feedback: 0, Mix: 30}, Echo{Preset: "slapback", Time: 120, Feedback: 0, Mix: 30}},
		{Echo{Preset: "tail", Time: 750, Feedback: 60, Mix: 30}, Echo{Preset: "tail", Time: 750, Feedback: 60, Mix: 30}},
		{Echo{Preset: "slapback", Time: 130, Feedback: 0, Mix: 30}, Echo{Preset: EchoCustom, Time: 130, Feedback: 0, Mix: 30}},
		{Echo{On: true, Preset: "loud", Time: 5, Feedback: 99, Mix: 101}, Echo{On: true, Preset: EchoCustom, Time: 20, Feedback: 90, Mix: 100}},
		{Echo{Preset: EchoCustom, Time: 9999, Feedback: -1, Mix: -1}, Echo{Preset: EchoCustom, Time: 2000, Feedback: 0, Mix: 0}},
		{Echo{Preset: EchoCustom, Time: 494, Feedback: 30, Mix: 25}, Echo{Preset: EchoCustom, Time: 490, Feedback: 30, Mix: 25}},
		{Echo{Preset: EchoCustom, Time: 495, Feedback: 30, Mix: 25}, Echo{Preset: EchoCustom, Time: 500, Feedback: 30, Mix: 25}},
	} {
		if got := c.in.clean(); got != c.want {
			t.Errorf("%+v cleans to %+v, want %+v", c.in, got, c.want)
		}
	}
	for _, p := range EchoPresets {
		var e Echo
		e.Apply(p)
		if e.clean() != e || e.Preset != p.ID {
			t.Errorf("the preset %s cleans to %+v", p.ID, e.clean())
		}
		if p.Time < MinEchoTime || p.Time > MaxEchoTime || p.Time%EchoTimeStep != 0 || p.Feedback > MaxEchoFeedback || p.Mix > MaxMix {
			t.Errorf("the preset %s is out of range: %+v", p.ID, p)
		}
	}
}
