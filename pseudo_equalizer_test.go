package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// TestPseudoLocaleEqualizer shows the equalizer and the rows of Sound
// after it in en-XA (#615): in simple mode, a preset of the user's
// playing, the balance to the left, crossfeed on, and the notes of Sound
// effects off and of 5.1; then the questions and toasts of the presets.
// Only the preset's name, the user's, is as they wrote it.
func TestPseudoLocaleEqualizer(t *testing.T) {
	usePseudoLocale(t)
	s := newTestShell(t)
	q := &s.set.Equalizer
	q.Enabled, q.Simple, q.Effects = true, true, false
	q.Balance, q.Crossfeed = -40, 30
	q.Save("Desk", -3)
	s.set.Surround = true
	s.save()
	tt := testerOf(t, s, 1280, 1000)
	tt.SetLocale("en-XA")
	xa := locales.In("en-XA")
	show(s, tt, "/settings/playback")
	for _, want := range []string{xa.EqualizerBalanceLeft(40), xa.EqualizerCrossfeedAt(30), xa.EqualizerWidthAt(100),
		xa.EqualizerEffectsOff(), xa.EqualizerSurroundNote(), xa.EqualizerEffects(), xa.EqualizerAutoHeadroom(),
		xa.EqualizerRename(), xa.EqualizerDelete(), xa.EqualizerExport(), xa.EqualizerExportAll(), xa.EqualizerImport(),
		xa.EqualizerBass(), xa.EqualizerMid(), xa.EqualizerTreble()} {
		if !tt.HasText(want) {
			t.Errorf("Settings › Playback doesn't show %q: %q", want, tt.Texts())
		}
	}
	checkPseudo(t, "Settings › Playback, the equalizer in simple mode", tt.Texts(), "Desk")

	// Saved again under its own name, it asks first.
	click(t, s, tt, xa.EqualizerSaveAs())
	if !tt.HasText(xa.EqualizerSave()) && !hasTextStarting(tt.Texts(), strings.ToLower(xa.EqualizerSaveTitle())) {
		t.Errorf("no dialog: %q", tt.Texts())
	}
	checkPseudo(t, "The dialog of Save as…", tt.Texts(), "Desk")
	clickButton(t, s, tt, xa.EqualizerSave())
	if !tt.HasText(xa.EqualizerReplaceMessage()) {
		t.Errorf("no question: %q", tt.Texts())
	}
	checkPseudo(t, "The question before a preset is replaced", tt.Texts(), "Desk")
	tt.Key(0, ui.KeyEscape)
	s.drain()
	tt.Frame()

	// A name too long says so.
	click(t, s, tt, xa.EqualizerSaveAs())
	typeName(s, tt, strings.Repeat("x", settings.MaxPresetName+1))
	clickButton(t, s, tt, xa.EqualizerSave())
	if !tt.HasText(xa.EqualizerNameTooLong(settings.MaxPresetName)) {
		t.Errorf("no toast: %q", tt.Texts())
	}
	checkPseudo(t, "A preset's name too long", tt.Texts(), "Desk")
	tt.Key(0, ui.KeyEscape)
	s.drain()
	tt.Frame()

	click(t, s, tt, xa.EqualizerDelete())
	if !tt.HasText(xa.EqualizerDeleteMessage()) {
		t.Errorf("no question: %q", tt.Texts())
	}
	checkPseudo(t, "The question before a preset is deleted", tt.Texts(), "Desk")
}
