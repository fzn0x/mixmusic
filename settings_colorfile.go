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
	"github.com/fzn0x/mixmusic/locales"
)

// The colours from a file, in Settings › Appearance › Colour and type.

// colorSource is a choice of Colours from a file, and its label.
type colorSource struct {
	source settings.ColorSource
	label  string
}

// colorSources are the choices of Colours from a file. A function, so that
// their labels show in the language chosen.
func colorSources() []colorSource {
	return []colorSource{
		{settings.ColorsTheme, locales.ColorFileSourceOff()},
		{settings.ColorsPywal, "pywal"},
		{settings.ColorsMatugen, "matugen"},
		{settings.ColorsFile, locales.ColorFileSourceFile()},
	}
}

// colorFileHelp is where docs/themes.md tells how to set up a colour file.
const colorFileHelp = "https://fzn0x.github.io/mixmusic/docs/themes/#colours-from-a-file"

func init() { addSettings(colorTypeRows, 10, (*shell).colorFileSettings) }

// colorFileSettings chooses where the colours come from, and tells what
// MixMusic read of the file.
func (s *shell) colorFileSettings(c *ui.Context) {
	sources := colorSources()
	labels := make([]string, len(sources))
	chosen := labels[0]
	for i, cs := range sources {
		labels[i] = cs.label
		if cs.source == s.set.ColorSource {
			chosen = cs.label
		}
	}
	s.row(c, locales.ColorFileSourceTitle(), locales.ColorFileSourceDescription(), func() {
		if s.selectOf(c.Key("color-source"), &chosen, labels).Width(200).Label(locales.ColorFileSourceTitle()).Changed() {
			if i := slices.Index(labels, chosen); i >= 0 && sources[i].source != s.set.ColorSource {
				s.set.ColorSource = sources[i].source
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
	if !s.findable(locales.ColorFileTitle(), note, words...) {
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
			ui.Text(c, locales.ColorFileTitle()).FontSize(k.bodySize())
			if shown != "" {
				ui.Text(c, shown).FontSize(k.smallSize()).TextColor(k.muted)
			}
			ink := k.muted
			if bad {
				ink = k.danger
			}
			ui.Text(c, note).FontSize(k.smallSize()).TextColor(ink)
			ui.Row(c).Gap(16).Padding(4, 0, 0).Children(func() {
				if s.link(c, "color-file-help", locales.ColorFileHelpLink()) {
					go s.svc.System.OpenExternal(colorFileHelp)
				}
				if source == settings.ColorsMatugen && s.link(c, "copy-template", locales.ColorFileCopyTemplate()) {
					copyText(colorfile.MatugenTemplate)
					s.notes = append(s.notes, notice(locales.ColorFileTemplateCopied()))
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
			in.Placeholder(k.sentence(locales.ColorFilePathPlaceholder())).Width(260).Label(locales.ColorFileTitle())
			if s.button(c, "choose-color-file", locales.ColorFileChoose(), false).Label(locales.ColorFileChooseTitle()).Clicked() {
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
		return locales.ColorFileNoPath(), false
	case st.Source != source || st.Path == "" && st.Err == "":
		return locales.ColorFileReading(), false
	case st.Missing && source == settings.ColorsPywal:
		return locales.ColorFileMissingPywal(), true
	case st.Missing && source == settings.ColorsMatugen:
		return locales.ColorFileMissingMatugen(), true
	case st.Missing:
		return locales.ColorFileMissing(), true
	case st.Err != "" && !st.Colors.Empty():
		return locales.ColorFileUnreadKept(strings.TrimSuffix(st.Err, "."), clockOf(st.ReadAt, now)), true
	case st.Err != "":
		return locales.ColorFileUnread(strings.TrimSuffix(st.Err, ".")), true
	}
	return locales.ColorFileRead(clockOf(st.ReadAt, now)), false
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
		paths, err := openFileDialog(mygo.OpenDialogOptions{Parent: win, Title: locales.ColorFileChooseTitle(), ShowHiddenFiles: true,
			Filters: []mygo.FileFilter{{Name: locales.ColorFileFilterColours(), Extensions: []string{"json"}}, {Name: locales.ColorFileFilterAll(), Extensions: []string{"*"}}}})
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
