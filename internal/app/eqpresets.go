package app

import (
	"os"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// The files of equalizer presets that Export… writes and Import… reads
// (#615).

// ExportEqualizerPresets writes presets to a JSON file at path.
func (p *Preferences) ExportEqualizerPresets(path string, presets []settings.EqualizerPreset) error {
	data, err := settings.EncodePresets(presets)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ImportEqualizerPresets reads the presets of the JSON file at path, which
// Export… wrote, or of a list of them, or one alone. A file that holds
// none says so in the windows' words.
func (p *Preferences) ImportEqualizerPresets(path string) ([]settings.EqualizerPreset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	presets, err := settings.DecodePresets(data)
	if err != nil {
		return nil, wordedLater("reading equalizer presets: "+err.Error(), locales.EqualizerImportNoPresets, err)
	}
	return presets, nil
}
