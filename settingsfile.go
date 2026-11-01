package main

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/settings"
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
		s.forget("tools")
	}
	if old.RegistryURL != s.set.RegistryURL {
		// Its answer is the former registry's.
		s.forget("registry")
	}
	if old.Zoom != s.set.Zoom && s.main {
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
		t.Type, t.Title, t.Timeout = "error", "settings.json has a mistake", 8*time.Second
		t.Description = sentence(n.Problem) + " MixMusic keeps the settings it had until you fix it."
		t.Action, t.OnAction = "open", s.openSettingsFile
	case app.NoticeAside:
		t.Type, t.Title, t.Timeout = "error", "settings.json didn't read", -1
		t.Description = sentence(n.Problem) + " MixMusic started with the settings of a new install, and kept your file as settings.json.broken."
		t.Action, t.OnAction = "show", s.showSettingsFile
	case app.NoticeStart:
		t.Title = "settings.json isn't UTF-8"
		t.Description = fileNotes(n.Report)
	default:
		t.Type, t.Title = "success", "Applied settings.json"
		t.Description = fileNotes(n.Report)
	}
	c.AddToast(t)
}

// fileNotes tells what of settings.json MixMusic didn't take as it was.
func fileNotes(r settings.Report) string {
	var notes []string
	if r.NotUTF8 {
		notes = append(notes, "It isn't UTF-8, so letters outside ASCII read as � in it: save it as UTF-8.")
	}
	if len(r.Unknown) > 0 {
		them := "it"
		if len(r.Unknown) > 1 {
			them = "them"
		}
		notes = append(notes, "MixMusic doesn't know "+and(r.Unknown)+": its next save drops "+them+".")
	}
	if len(r.Adjusted) > 0 {
		notes = append(notes, "Put "+and(r.Adjusted)+" back in range.")
	}
	return strings.Join(notes, " ")
}

// and lists words, as "a, b and c".
func and(words []string) string {
	if len(words) < 2 {
		return strings.Join(words, "")
	}
	return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
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
	s.alert(c, "settings.json has a mistake", sentence(p)+" MixMusic goes on with the settings it had, and saves what you change here once the file reads again.", func() {
		if s.button(c, "settings-file-fix", "open settings.json", true).Clicked() {
			s.openSettingsFile()
		}
	}).Margin(0, 0, 32)
}
