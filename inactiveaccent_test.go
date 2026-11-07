package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestAccentWhileBehind keeps the accent of the sliders and switches in a
// window another is in front of, as MixMusic's own parts keep theirs, in
// every layout and around the notch: MyGo 0.8 fades its widgets' to gray
// there unless the theme keeps it.
func TestAccentWhileBehind(t *testing.T) {
	for _, layout := range settings.Layouts {
		if k := lookOf(layout, settings.DensityComfortable); k.theme.InactiveAccent != k.theme.Accent {
			t.Errorf("%s: the accent fades to %v behind, from %v", layout, k.theme.InactiveAccent, k.theme.Accent)
		}
	}
	if notchTheme.InactiveAccent != notchTheme.Accent {
		t.Errorf("the notch's accent fades to %v behind, from %v", notchTheme.InactiveAccent, notchTheme.Accent)
	}
}
