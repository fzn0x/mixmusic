package settings

import (
	"bytes"
	"encoding/json"
	"errors"
)

// The file of the user's equalizer presets that Export… writes and
// Import… reads (#615): a JSON object of them, which says what it holds.

// presetFileKind says what a file of presets holds.
const presetFileKind = "mixmusic.equalizerPresets"

// presetFile is a file of presets.
type presetFile struct {
	Kind    string            `json:"kind"`
	Version int               `json:"version"`
	Presets []EqualizerPreset `json:"presets"`
}

// ErrNoPresets is why a file doesn't import: it holds no presets.
var ErrNoPresets = errors.New("the file holds no equalizer presets")

// EncodePresets is presets as a file of them.
func EncodePresets(presets []EqualizerPreset) ([]byte, error) {
	f := presetFile{Kind: presetFileKind, Version: 1, Presets: make([]EqualizerPreset, 0, len(presets))}
	for _, p := range presets {
		f.Presets = append(f.Presets, p.clean())
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// DecodePresets reads the presets of a file: a file of them, as
// EncodePresets writes, a list of them, or one alone. Each takes its name
// cut to MaxPresetName characters, and values in their ranges; one
// without a name is left out, and one of a name one before it had,
// ignoring case, takes its place. A file of none gives ErrNoPresets.
func DecodePresets(data []byte) ([]EqualizerPreset, error) {
	text, _ := Text(data)
	text = bytes.TrimSpace(text)
	var list []EqualizerPreset
	switch {
	case len(text) > 0 && text[0] == '[':
		if err := json.Unmarshal(text, &list); err != nil {
			return nil, err
		}
	default:
		var keys map[string]json.RawMessage
		if err := json.Unmarshal(text, &keys); err != nil {
			return nil, err
		}
		if _, ok := keys["presets"]; ok {
			var f presetFile
			if err := json.Unmarshal(text, &f); err != nil {
				return nil, err
			}
			list = f.Presets
			break
		}
		if _, ok := keys["gains"]; !ok {
			return nil, ErrNoPresets
		}
		var p EqualizerPreset
		if err := json.Unmarshal(text, &p); err != nil {
			return nil, err
		}
		list = []EqualizerPreset{p}
	}
	var e Equalizer
	e.Import(list)
	if len(e.Presets) == 0 {
		return nil, ErrNoPresets
	}
	return e.Presets, nil
}
