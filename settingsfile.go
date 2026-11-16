package main

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// openSettings opens settings.json, and showSettings shows it in its
// folder. Tests replace them, which would reach the desktop.
var (
	openSettings = func(sys *app.System) error { return sys.OpenSettingsFile() }
	showSettings = func(sys *app.System) error { return sys.ShowSettingsFile() }
)

// openSettingsFile opens settings.json, beside the main thread, as the
// computer opens JSON files.
func (s *shell) openSettingsFile() {
	go func() {
		if err := openSettings(s.svc.System); err != nil {
			s.update(func() { s.notes = append(s.notes, noticeOf(err)) })
		}
	}()
}

// showSettingsFile shows settings.json in its folder, beside the main
// thread.
func (s *shell) showSettingsFile() {
	go func() {
		if err := showSettings(s.svc.System); err != nil {
			s.update(func() { s.notes = append(s.notes, noticeOf(err)) })
		}
	}()
}

// followSettings does what the pages do around a change of the settings,
// once they changed but not from the window's own controls, which do it
// themselves: in settings.json, or by another service. old are the
// settings the window had.
func (s *shell) followSettings(old settings.Settings) {
	if old.Panel != s.set.Panel {
		s.panel = shownPanel(s.set.Panel)
	}
	if old.YouTubeChannel != s.set.YouTubeChannel {
		// The pages are the former channel's, as after a channel chosen in
		// Settings › Accounts.
		clear(s.loads)
	}
	if old.YTDLPPath != s.set.YTDLPPath || old.JSRuntime != s.set.JSRuntime {
		s.refetch("tools")
	}
	if old.RegistryURL != s.set.RegistryURL {
		// Its answer is the former registry's.
		s.forget("registry")
	}
	// The mini player reads out a zoom it chose itself.
	if old.Zoom != s.set.Zoom && s.main && !s.miniChose(s.set.Zoom) {
		s.zoomToast = true
	}
}

// fileToast tells, in the main window, what became of settings.json.
func (s *shell) fileToast(c *ui.Context) {
	n := s.fileNotice
	if n == nil {
		return
	}
	s.fileNotice = nil
	t := ui.Toast{ID: "settings-file"}
	switch n.Kind {
	case app.NoticeProblem:
		t.Type, t.Title, t.Timeout = "error", locales.SettingsFileMistake(), 8*time.Second
		t.Description = locales.SettingsFileKeeps(sentence(n.Problem))
		t.Action, t.OnAction = locales.SettingsFileOpen(), s.openSettingsFile
	case app.NoticeAside:
		t.Type, t.Title, t.Timeout = "error", locales.SettingsFileDidntRead(), -1
		t.Description = locales.SettingsFileStarted(sentence(n.Problem))
		t.Action, t.OnAction = locales.SettingsFileShow(), s.showSettingsFile
	case app.NoticeStart:
		t.Title = locales.SettingsFileNotUTF8Title()
		t.Description = fileNotes(n.Report)
	default:
		t.Type, t.Title = "success", locales.SettingsFileApplied()
		t.Description = fileNotes(n.Report)
	}
	c.AddToast(t)
}

// fileNotes tells what of settings.json MixMusic didn't take as it was.
func fileNotes(r settings.Report) string {
	var notes []string
	if r.NotUTF8 {
		notes = append(notes, locales.SettingsFileNotUTF8())
	}
	if len(r.Unknown) > 0 {
		notes = append(notes, locales.SettingsFileUnknown(and(r.Unknown), len(r.Unknown)))
	}
	if len(r.Adjusted) > 0 {
		notes = append(notes, locales.SettingsFileAdjusted(and(r.Adjusted)))
	}
	return strings.Join(notes, " ")
}

// and lists words, as "a, b and c".
func and(words []string) string {
	if len(words) < 2 {
		return strings.Join(words, "")
	}
	return locales.SettingsFileAnd(strings.Join(words[:len(words)-1], locales.SettingsFileListSep()), words[len(words)-1])
}

// sentence is text as a sentence: its first letter up, and a full stop.
func sentence(text string) string {
	r, n := utf8.DecodeRuneInString(text)
	if n == 0 {
		return ""
	}
	return string(unicode.ToUpper(r)) + text[n:] + "."
}

// settingsFileProblem is the problem panel at the top of the settings,
// while settings.json has a mistake.
func (s *shell) settingsFileProblem(c *ui.Context) {
	p := s.file.Problem
	if p == "" {
		return
	}
	s.alert(c, locales.SettingsFileMistake(), locales.SettingsFileGoesOn(sentence(p)), func() {
		if s.button(c, "settings-file-fix", locales.SettingsFileOpenJSON(), true).Clicked() {
			s.openSettingsFile()
		}
	}).Margin(0, 0, 32)
}
