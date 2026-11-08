package settings

import (
	"slices"

	"github.com/fzn0x/mixmusic/internal/volume"
)

// cleanVolumeCurve is c, or def for a curve MixMusic doesn't know, as in
// settings from before the curve.
func cleanVolumeCurve(c, def volume.Curve) volume.Curve {
	if !slices.Contains(volume.Curves, c) {
		return def
	}
	return c
}
