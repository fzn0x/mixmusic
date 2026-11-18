package main

import (
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// The equalizer's presets, the built-in ones and the user's own (#615):
// the Preset menu, with Save as… beside it, and, under it, Rename…,
// Delete and Export… while one of the user's plays, Export all… while
// they have any, and Import….

// eqPresetRow chooses the equalizer's preset, and keeps the user's own.
func (s *shell) eqPresetRow(c *ui.Context) {
	k := &s.lk
	eq := &s.set.Equalizer
	// Bands of the user's own, as once a slider moved, show as Custom,
	// which is no preset to choose.
	chosen := locales.EqualizerCustom()
	labels := make([]string, 0, len(eqPresets)+len(eq.Presets))
	for _, p := range eqPresets {
		labels = append(labels, eqPresetLabel(p.id))
		if p.id == eq.Preset {
			chosen = labels[len(labels)-1]
		}
	}
	saved := eq.Chosen()
	for i, p := range eq.Presets {
		labels = append(labels, p.Name)
		if i == saved {
			chosen = p.Name
		}
	}
	// A preset's name finds it, as "bass".
	words := append([]string{locales.EqualizerSaveAs(), locales.EqualizerRename(), locales.EqualizerDelete(),
		locales.EqualizerExport(), locales.EqualizerImport()}, labels...)
	if !s.findable(locales.EqualizerPreset(), locales.EqualizerPresetDesc(), words...) {
		return
	}
	s.show(c, func() {
		ui.Row(c).Gap(24).Padding(14, 0).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				ui.Text(c, locales.EqualizerPreset()).FontSize(k.bodySize())
				ui.Text(c, locales.EqualizerPresetDesc()).FontSize(k.smallSize()).TextColor(k.muted)
				ui.Row(c).Gap(16).Wrap().Padding(4, 0, 0).Children(func() {
					if saved >= 0 {
						p := eq.Presets[saved]
						if s.link(c, "eq-rename", locales.EqualizerRename()) {
							s.renameEqPreset(p.Name)
						}
						if s.link(c, "eq-delete", locales.EqualizerDelete()) {
							s.deleteEqPreset(p.Name)
						}
						if s.link(c, "eq-export", locales.EqualizerExport()) {
							s.exportEqPresets(p.Name, []settings.EqualizerPreset{p})
						}
					}
					if len(eq.Presets) > 0 && s.link(c, "eq-export-all", locales.EqualizerExportAll()) {
						s.exportEqPresets("", eq.Presets)
					}
					if s.link(c, "eq-import", locales.EqualizerImport()) {
						s.importEqPresets()
					}
				})
			})
			ui.Row(c).Shrink(0).Gap(8).Children(func() {
				if s.selectOf(c.Key("eq-preset"), &chosen, labels).Width(200).Label(locales.EqualizerPreset()).Changed() {
					s.chooseEqPreset(slices.Index(labels, chosen))
				}
				if s.button(c, "eq-save-as", locales.EqualizerSaveAs(), false).Clicked() {
					s.saveEqPreset()
				}
			})
		})
	})
}

// chooseEqPreset plays the preset at i of the Preset menu: a built-in one,
// then the user's.
func (s *shell) chooseEqPreset(i int) {
	eq := &s.set.Equalizer
	switch {
	case i < 0:
		return
	case i < len(eqPresets):
		// Choosing how it sounds turns the equalizer on.
		eq.Enabled, eq.Preset = true, eqPresets[i].id
		eq.Gains = slices.Clone(eqPresets[i].gains[:])
		eq.Bass, eq.Mid, eq.Treble = settings.ToneOf(eq.Gains)
	case i-len(eqPresets) < len(eq.Presets):
		eq.Apply(eq.Presets[i-len(eqPresets)])
	default:
		return
	}
	s.save()
}

// builtinPreset returns the name a built-in preset shows that is name,
// ignoring case, or "": Custom too, which the menu shows.
func builtinPreset(name string) string {
	for _, p := range eqPresets {
		if l := eqPresetLabel(p.id); settings.SameName(l, name) {
			return l
		}
	}
	if l := locales.EqualizerCustom(); settings.SameName(l, name) {
		return l
	}
	return ""
}

// nameEqPreset asks for a preset's name, from value, in a dialog of title
// whose button says confirm, until it is one a preset may have, and runs
// done with it: once the user agreed to replace the preset of theirs that
// has it, if any, but for the one named except, which is being renamed.
func (s *shell) nameEqPreset(title, confirm, value, except string, done func(name string)) {
	s.promptName(title, locales.EqualizerPresetName(), value, confirm, func(name string) {
		again := func(why string) {
			s.notes = append(s.notes, player.Notice{Message: why, Error: true})
			s.nameEqPreset(title, confirm, name, except, done)
		}
		if utf8.RuneCountInString(name) > settings.MaxPresetName {
			again(locales.EqualizerNameTooLong(settings.MaxPresetName))
			return
		}
		if l := builtinPreset(name); l != "" {
			again(locales.EqualizerNameBuiltin(l))
			return
		}
		eq := &s.set.Equalizer
		if i := eq.Saved(name); i >= 0 && !settings.SameName(eq.Presets[i].Name, except) {
			s.ask(locales.EqualizerReplaceTitle(eq.Presets[i].Name), locales.EqualizerReplaceMessage(), locales.EqualizerReplace(), func() { done(name) })
			return
		}
		done(name)
	})
}

// saveEqPreset keeps the equalizer as a preset of the user's, under a name
// they give: the preset that plays has its own at first.
func (s *shell) saveEqPreset() {
	name := ""
	if i := s.set.Equalizer.Chosen(); i >= 0 {
		name = s.set.Equalizer.Presets[i].Name
	}
	s.nameEqPreset(locales.EqualizerSaveTitle(), locales.EqualizerSave(), name, "", func(name string) {
		eq := &s.set.Equalizer
		preamp := eq.Preamp
		if eq.AutoHeadroom {
			preamp = app.Headroom(eq.Gains)
		}
		eq.Save(name, preamp)
		s.save()
	})
}

// renameEqPreset renames the user's preset of name, to a name they give.
func (s *shell) renameEqPreset(name string) {
	s.nameEqPreset(locales.EqualizerRenameTitle(), locales.EqualizerRenameButton(), name, name, func(to string) {
		eq := &s.set.Equalizer
		if j := eq.Saved(to); j >= 0 && !settings.SameName(eq.Presets[j].Name, name) {
			eq.Delete(eq.Presets[j].Name) // which the user agreed to replace
		}
		eq.Rename(name, to)
		s.save()
	})
}

// deleteEqPreset deletes the user's preset of name, once they agree.
func (s *shell) deleteEqPreset(name string) {
	s.ask(locales.EqualizerDeleteTitle(name), locales.EqualizerDeleteMessage(), locales.EqualizerDelete(), func() {
		s.set.Equalizer.Delete(name)
		s.save()
	})
}

// presetFileName is a file name for presets called name.
func presetFileName(name string) string {
	return strings.TrimSuffix(cardFileName(name), ".png") + ".json"
}

// exportEqPresets asks where to save presets, beside the main thread, and
// writes them there as a JSON file, at first named name, or after what it
// holds.
func (s *shell) exportEqPresets(name string, presets []settings.EqualizerPreset) {
	presets = slices.Clone(presets)
	if name == "" {
		name = locales.EqualizerPresetsFilter()
	}
	win := s.win
	go func() {
		path, err := saveFileDialog(mygo.SaveDialogOptions{Parent: win, Title: locales.EqualizerExportTitle(), DefaultPath: presetFileName(name),
			Filters: []mygo.FileFilter{{Name: locales.EqualizerPresetsFilter(), Extensions: []string{"json"}}}, CreateDirectories: true})
		if err == nil && path == "" {
			return
		}
		if err == nil {
			if !strings.EqualFold(filepath.Ext(path), ".json") {
				path += ".json"
			}
			err = s.svc.Preferences.ExportEqualizerPresets(path, presets)
		}
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, player.Notice{Message: locales.EqualizerExportFailed(errorText(err)), Error: true})
				return
			}
			s.notes = append(s.notes, notice(locales.EqualizerExported(len(presets), filepath.Base(path))))
		})
	}()
}

// importEqPresets asks for a file of presets, beside the main thread, and
// adds its presets to the user's, once they agree to replace those of the
// same names.
func (s *shell) importEqPresets() {
	win := s.win
	go func() {
		paths, err := openFileDialog(mygo.OpenDialogOptions{Parent: win, Title: locales.EqualizerImportTitle(),
			Filters: []mygo.FileFilter{{Name: locales.EqualizerPresetsFilter(), Extensions: []string{"json"}}}})
		if err == nil && len(paths) == 0 {
			return
		}
		var presets []settings.EqualizerPreset
		if err == nil {
			presets, err = s.svc.Preferences.ImportEqualizerPresets(paths[0])
		}
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, player.Notice{Message: locales.EqualizerImportFailed(errorText(err)), Error: true})
				return
			}
			add := func() {
				s.set.Equalizer.Import(presets)
				s.save()
				s.notes = append(s.notes, notice(locales.EqualizerImported(len(presets))))
			}
			if taken := s.set.Equalizer.Taken(presets); len(taken) > 0 {
				s.ask(locales.EqualizerImportReplaceTitle(len(taken)), locales.EqualizerImportReplaceMessage(len(taken)), locales.EqualizerReplace(), add)
				return
			}
			add()
		})
	}()
}
