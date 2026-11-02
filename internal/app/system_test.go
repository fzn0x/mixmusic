package app

import (
	"strings"
	"testing"
)

// TestNotices checks that the app shows the license of each project whose
// code it includes, as its copies must reproduce them: those of the Opus
// and AAC decoders, of Chromium's Web Audio, and of Material Color
// Utilities.
func TestNotices(t *testing.T) {
	notices := (&System{}).Notices()
	for _, want := range []struct{ name, text string }{
		{"libopus", "Copyright 2001-2023 Xiph.Org"},
		{"SharpJaad", "Copyright (c) 2023 Lukas Volf"},
		{"Chromium", "Copyright (C) 2010 Google Inc."},
		{"Material Color Utilities", "Copyright 2021 Google LLC"},
	} {
		found := false
		for _, n := range notices {
			if strings.HasPrefix(n.Name, want.name) {
				found = strings.Contains(n.Text, want.text)
			}
		}
		if !found {
			t.Errorf("no notice of %s with its license", want.name)
		}
	}
}
