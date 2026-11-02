package main

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/colorfile"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// The colours from a file, in Settings › Appearance › Colour and type.

// colorSources are the choices of Colours from a file.
var colorSources = []struct {
	source settings.ColorSource
	label  string
}{
	{settings.ColorsTheme, "Off"},
	{settings.ColorsPywal, "pywal"},
	{settings.ColorsMatugen, "matugen"},
	{settings.ColorsFile, "A file of your own"},
}

// colorFileHelp is where docs/themes.md tells how to set up a colour file.
const colorFileHelp = "https://fzn0x.github.io/mixmusic/docs/themes/#colours-from-a-file"

func init() { addSettings(colorTypeRows, 10, (*shell).colorFileSettings) }

// colorFileSettings chooses where the colours come from, and tells what
// MixMusic read of the file.
func (s *shell) colorFileSettings(c *ui.Context) {
	labels := make([]string, len(colorSources))
	chosen := labels[0]
	for i, cs := range colorSources {
		labels[i] = cs.label
		if cs.source == s.set.ColorSource {
			chosen = cs.label
		}
	}
	s.row(c, "Colours from a file", "Take the colours from pywal or matugen, and follow them as they change. The theme keeps its type, corners and effects.", func() {
		if s.selectOf(c.Key("color-source"), &chosen, labels).Width(200).Label("Colours from a file").Changed() {
			if i := slices.Index(labels, chosen); i >= 0 && colorSources[i].source != s.set.ColorSource {
				s.set.ColorSource = colorSources[i].source
				s.save()
			}
		}
	}, "wallpaper", "a file of your own")
	source := s.set.ColorSource
	if source == settings.ColorsTheme {
		return
	}
	note, bad := colorFileNote(s.colorFile, s.set, time.Now())
	// The search finds the file's row by what it shows: the file, what
	// MixMusic read of it, and its links and button.
	shown := ""
	if s.colorFile.Source == source {
		shown = s.colorFile.Path
	}
	words := []string{string(source), shown, "how to set it up"}
	switch source {
	case settings.ColorsMatugen:
		words = append(words, "copy the template")
	case settings.ColorsFile:
		words = append(words, s.set.ColorFile, "choose")
	}
	if !s.findable("Colour file", note, words...) {
		return
	}
	s.show(c, func() { s.colorFileRow(c, source, shown, note, bad) })
}

// colorFileRow shows the colour file of source, shown where it is, note
// what MixMusic read of it, which bad tells is a problem.
func (s *shell) colorFileRow(c *ui.Context, source settings.ColorSource, shown, note string, bad bool) {
	k := &s.lk
	ui.Row(c).Gap(24).Padding(14, 0).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, "Colour file").FontSize(k.bodySize())
			if shown != "" {
				ui.Text(c, shown).FontSize(k.smallSize()).TextColor(k.muted)
			}
			ink := k.muted
			if bad {
				ink = k.danger
			}
			ui.Text(c, note).FontSize(k.smallSize()).TextColor(ink)
			ui.Row(c).Gap(16).Padding(4, 0, 0).Children(func() {
				if s.link(c, "color-file-help", "how to set it up") {
					go s.svc.System.OpenExternal(colorFileHelp)
				}
				if source == settings.ColorsMatugen && s.link(c, "copy-template", "copy the template") {
					copyText(colorfile.MatugenTemplate)
					s.notes = append(s.notes, notice("Template copied"))
				}
			})
		})
		if source != settings.ColorsFile {
			return
		}
		ui.Row(c).Shrink(0).Gap(8).Children(func() {
			in, _ := s.settingInput(c, "color-file", s.set.ColorFile, func(path string) {
				s.set.ColorFile = settings.CleanPath(path)
				s.save()
			})
			in.Placeholder(k.sentence("path to a colour file")).Width(260).Label("Colour file")
			if s.button(c, "choose-color-file", "choose", false).Label("Choose a colour file").Clicked() {
				s.chooseColorFile()
			}
		})
	})
}

// colorFileNote tells what MixMusic read of the colour file the settings
// name, as of now, and whether that's a problem.
func colorFileNote(st app.ColorFileStatus, set settings.Settings, now time.Time) (string, bool) {
	source := set.ColorSource
	switch {
	case source == settings.ColorsFile && set.ColorFile == "":
		return "Choose a file: pywal's colors.json, what matugen --json prints, or a theme.json.", false
	case st.Source != source || st.Path == "" && st.Err == "":
		return "Reading it…", false
	case st.Missing && source == settings.ColorsPywal:
		return "It isn't there yet: pywal writes it when you run wal.", true
	case st.Missing && source == settings.ColorsMatugen:
		return "It isn't there yet: add MixMusic's template to matugen's config.toml, and run matugen.", true
	case st.Missing:
		return "It isn't there. The theme's colours show until it is.", true
	case st.Err != "" && !st.Colors.Empty():
		return "It can't be read: " + strings.TrimSuffix(st.Err, ".") + ". MixMusic keeps the colours it read at " + clockOf(st.ReadAt, now) + ".", true
	case st.Err != "":
		return "It can't be read: " + strings.TrimSuffix(st.Err, ".") + ". The theme's colours show.", true
	}
	return "Read at " + clockOf(st.ReadAt, now) + ", and followed as it changes.", false
}

// clockOf is the time of t, with its day unless it is now's.
func clockOf(t, now time.Time) string {
	if y, m, d := t.Date(); y == now.Year() && m == now.Month() && d == now.Day() {
		return t.Format("15:04")
	}
	return t.Format("2 Jan 15:04")
}

// openFileDialog asks the user for files, as MyGo's dialog does; tests
// replace it.
var openFileDialog = func(opts mygo.OpenDialogOptions) ([]string, error) { return mygo.Dialog.Open(opts) }

// chooseColorFile asks the user for a colour file, beside the main thread,
// and reads the one they choose.
func (s *shell) chooseColorFile() {
	win := s.win
	go func() {
		paths, err := openFileDialog(mygo.OpenDialogOptions{Parent: win, Title: "Choose a colour file", ShowHiddenFiles: true,
			Filters: []mygo.FileFilter{{Name: "Colour files", Extensions: []string{"json"}}, {Name: "All files", Extensions: []string{"*"}}}})
		s.update(func() {
			switch {
			case err != nil:
				s.notes = append(s.notes, noticeOf(err))
			case len(paths) > 0 && paths[0] != s.set.ColorFile:
				s.set.ColorFile = paths[0]
				s.save()
			}
		})
	}()
}
