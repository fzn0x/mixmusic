package settings

import (
	"encoding/json"
	"slices"
	"strings"
	"unicode/utf8"
)

// Bands are the center frequencies of the equalizer's bands, in hertz.
var Bands = []int{32, 64, 125, 250, 500, 1000, 2000, 4000, 8000, 16000}

// The ranges of the equalizer's settings.
const (
	// MinPreamp and MaxPreamp are the preamp's, in dB: down to -30, for
	// Auto headroom's cut of bands that boost much.
	MinPreamp = -30
	MaxPreamp = 12
	// MaxGain is a band's, from -MaxGain to MaxGain dB, and Bass, Mid and
	// Treble's.
	MaxGain = 12
	// MaxBalance is the balance's, from -MaxBalance, the left alone, to
	// MaxBalance, the right alone.
	MaxBalance = 100
	// MaxCrossfeed is the crossfeed's, in percent.
	MaxCrossfeed = 100
	// MaxWidth is the stereo width's, in percent: 100 is as recorded.
	MaxWidth = 200
	// MaxPresetName is how long a preset's name may be, in characters.
	MaxPresetName = 48
)

// Equalizer shapes the sound of MixMusic's own player: the equalizer of
// Settings › Playback › Equalizer, and the balance, mono, crossfeed and
// width of Settings › Playback › Sound (#615).
type Equalizer struct {
	// Enabled turns the equalizer on.
	Enabled bool `json:"enabled"`
	// Preset is the ID of the preset the gains are, SavedPresetID of a
	// preset of the user's, or "custom" for gains of the user's own.
	Preset string `json:"preset"`
	// Gains are in decibels, one per band, from -12 to 12.
	Gains []float64 `json:"gains"`
	// Preamp is in decibels, from MinPreamp to MaxPreamp. Auto headroom
	// sets its own while on.
	Preamp float64 `json:"preamp"`
	// AutoHeadroom has the preamp follow the peak of the bands' summed
	// response, plus half a decibel, as a cut, in place of Preamp: the
	// equalizer then makes no frequency louder than it came in. It is on
	// at first, and for the settings of earlier versions.
	AutoHeadroom bool `json:"autoHeadroom"`
	// Simple shows three sliders, Bass, Mid and Treble, in place of the
	// ten bands, which they set (ToneGains).
	Simple bool `json:"simple"`
	// Bass, Mid and Treble are the simple mode's sliders, in decibels, from
	// -12 to 12.
	Bass   float64 `json:"bass"`
	Mid    float64 `json:"mid"`
	Treble float64 `json:"treble"`
	// Effects is Sound effects: off, the equalizer, crossfeed and width
	// stop shaping the sound, and keep their settings. Balance and Mono
	// still apply. It is on at first.
	Effects bool `json:"effects"`
	// Balance weighs the left side against the right, from -100, the left
	// alone, to 100, the right alone.
	Balance int `json:"balance"`
	// Mono plays both sides mixed on each.
	Mono bool `json:"mono"`
	// Crossfeed, for headphones, mixes a softened copy of each side into
	// the other, from 0, off, to 100 percent.
	Crossfeed int `json:"crossfeed"`
	// Width is the stereo width, in percent, from 0, mono, through 100, as
	// recorded, to 200.
	Width int `json:"width"`
	// Presets are the user's own, as they saved them.
	Presets []EqualizerPreset `json:"presets"`
	// Headphones is the correction of the user's headphones, which plays
	// after the bands, whether the equalizer is on or not (#616).
	Headphones Headphones `json:"headphones"`
}

// EqualizerPreset is a preset of the user's: what Save as… keeps of the
// equalizer, under a name of up to MaxPresetName characters, unique
// ignoring case.
type EqualizerPreset struct {
	Name         string    `json:"name"`
	Gains        []float64 `json:"gains"`
	Preamp       float64   `json:"preamp"`
	AutoHeadroom bool      `json:"autoHeadroom"`
	Balance      int       `json:"balance"`
	Mono         bool      `json:"mono"`
	Crossfeed    int       `json:"crossfeed"`
	Width        int       `json:"width"`
}

// newPreset is a preset that changes nothing but the bands: what a preset
// of a file takes for the keys it lacks.
func newPreset() EqualizerPreset {
	return EqualizerPreset{Gains: make([]float64, len(Bands)), AutoHeadroom: true, Width: 100}
}

// UnmarshalJSON reads a preset, taking newPreset's values for the keys it
// lacks: a preset written by hand without its width isn't mono.
func (p *EqualizerPreset) UnmarshalJSON(data []byte) error {
	type plain EqualizerPreset
	v := plain(newPreset())
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*p = EqualizerPreset(v)
	return nil
}

// defaultEqualizer is the equalizer of a new install: off, and flat, with
// Auto headroom and Sound effects on, and nothing else shaping the sound.
func defaultEqualizer() Equalizer {
	var e Equalizer
	e.Preset = "flat"
	e.Gains = make([]float64, len(Bands))
	e.AutoHeadroom = true
	e.Effects = true
	e.Width = 100
	e.Presets = []EqualizerPreset{}
	e.Headphones = noHeadphones()
	return e
}

// cleanGains is gains with one for each band, of their own, each in its
// range.
func cleanGains(gains []float64) []float64 {
	out := make([]float64, len(Bands))
	if len(gains) == len(Bands) {
		for i, g := range gains {
			out[i] = min(max(g, -MaxGain), MaxGain)
		}
	}
	return out
}

// clean is e with a gain for each band, of its own, every value in its
// range, and the user's presets each with a name of its own.
func (e Equalizer) clean() Equalizer {
	e.Gains = cleanGains(e.Gains)
	e.Preamp = min(max(e.Preamp, MinPreamp), MaxPreamp)
	if e.Preset == "" {
		e.Preset = "custom"
	}
	e.Bass = min(max(e.Bass, -MaxGain), MaxGain)
	e.Mid = min(max(e.Mid, -MaxGain), MaxGain)
	e.Treble = min(max(e.Treble, -MaxGain), MaxGain)
	e.Balance = min(max(e.Balance, -MaxBalance), MaxBalance)
	e.Crossfeed = min(max(e.Crossfeed, 0), MaxCrossfeed)
	e.Width = min(max(e.Width, 0), MaxWidth)
	presets := make([]EqualizerPreset, 0, len(e.Presets))
	for _, p := range e.Presets {
		p = p.clean()
		if p.Name != "" && !slices.ContainsFunc(presets, func(q EqualizerPreset) bool { return SameName(q.Name, p.Name) }) {
			presets = append(presets, p)
		}
	}
	e.Presets = presets
	e.Headphones = e.Headphones.clean()
	return e
}

// clean is p with a name of up to MaxPresetName characters, without the
// spaces around it, and every value in its range.
func (p EqualizerPreset) clean() EqualizerPreset {
	p.Name = PresetName(p.Name)
	p.Gains = cleanGains(p.Gains)
	p.Preamp = min(max(p.Preamp, MinPreamp), MaxPreamp)
	p.Balance = min(max(p.Balance, -MaxBalance), MaxBalance)
	p.Crossfeed = min(max(p.Crossfeed, 0), MaxCrossfeed)
	p.Width = min(max(p.Width, 0), MaxWidth)
	return p
}

// PresetName is name as a preset keeps it: without the spaces around it,
// and cut to MaxPresetName characters.
func PresetName(name string) string {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > MaxPresetName {
		name = strings.TrimSpace(string([]rune(name)[:MaxPresetName]))
	}
	return name
}

// SameName reports whether two names of presets are the same, ignoring
// case.
func SameName(a, b string) bool { return strings.EqualFold(PresetName(a), PresetName(b)) }

// savedPrefix starts the Preset of a preset of the user's, which no
// built-in preset's ID does.
const savedPrefix = "saved:"

// SavedPresetID is the Preset of the user's preset of name.
func SavedPresetID(name string) string { return savedPrefix + name }

// Saved returns the index of the user's preset of name, ignoring case, or
// -1.
func (e Equalizer) Saved(name string) int {
	return slices.IndexFunc(e.Presets, func(p EqualizerPreset) bool { return SameName(p.Name, name) })
}

// Chosen returns the index of the user's preset that Preset names, or -1
// when it names a built-in one, or none.
func (e Equalizer) Chosen() int {
	name, ok := strings.CutPrefix(e.Preset, savedPrefix)
	if !ok {
		return -1
	}
	return e.Saved(name)
}

// Save keeps the equalizer as it is as the user's preset of name, in place
// of the one of that name, ignoring case, if any, and chooses it. preamp
// is the preamp the equalizer plays at, Auto headroom's while it's on.
func (e *Equalizer) Save(name string, preamp float64) {
	p := EqualizerPreset{
		Name:  PresetName(name),
		Gains: slices.Clone(e.Gains), Preamp: preamp, AutoHeadroom: e.AutoHeadroom,
		Balance: e.Balance, Mono: e.Mono, Crossfeed: e.Crossfeed, Width: e.Width,
	}
	p = p.clean()
	e.Presets = slices.Clone(e.Presets)
	if i := e.Saved(p.Name); i >= 0 {
		e.Presets[i] = p
	} else {
		e.Presets = append(e.Presets, p)
	}
	e.Preset = SavedPresetID(p.Name)
}

// Apply has the equalizer play the user's preset p, and turns it on.
func (e *Equalizer) Apply(p EqualizerPreset) {
	p = p.clean()
	e.Enabled = true
	e.Preset = SavedPresetID(p.Name)
	e.Gains = p.Gains
	e.Preamp, e.AutoHeadroom = p.Preamp, p.AutoHeadroom
	e.Balance, e.Mono, e.Crossfeed, e.Width = p.Balance, p.Mono, p.Crossfeed, p.Width
	e.Bass, e.Mid, e.Treble = ToneOf(e.Gains)
}

// Rename names the user's preset of name to, a name no other preset of
// the user's has: the equalizer keeps it chosen.
func (e *Equalizer) Rename(name, to string) {
	i := e.Saved(name)
	if i < 0 {
		return
	}
	chosen := e.Chosen() == i
	to = PresetName(to)
	e.Presets = slices.Clone(e.Presets)
	e.Presets[i].Name = to
	if chosen {
		e.Preset = SavedPresetID(to)
	}
}

// Delete drops the user's preset of name. The equalizer plays on as it
// is: one that played it plays Custom.
func (e *Equalizer) Delete(name string) {
	i := e.Saved(name)
	if i < 0 {
		return
	}
	if e.Chosen() == i {
		e.Preset = "custom"
	}
	e.Presets = slices.Delete(slices.Clone(e.Presets), i, i+1)
}

// Taken returns the names of presets, ignoring case, that the user's
// presets have already.
func (e Equalizer) Taken(presets []EqualizerPreset) []string {
	var taken []string
	for _, p := range presets {
		if i := e.Saved(p.Name); i >= 0 {
			taken = append(taken, e.Presets[i].Name)
		}
	}
	return taken
}

// Import adds presets to the user's, each in place of the one of its name,
// ignoring case, if any.
func (e *Equalizer) Import(presets []EqualizerPreset) {
	e.Presets = slices.Clone(e.Presets)
	for _, p := range presets {
		p = p.clean()
		if p.Name == "" {
			continue
		}
		if i := e.Saved(p.Name); i >= 0 {
			e.Presets[i] = p
		} else {
			e.Presets = append(e.Presets, p)
		}
	}
}
