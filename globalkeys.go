package main

// The global shortcuts of Settings › Shortcuts: keys that run a few
// commands whichever window is in front (#80).

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/player"
)

// globalRecording starts recording, before a command's ID, while the user
// records the keys of the command's global shortcut: the window's
// shortcuts wait meanwhile, as while the user records one of theirs.
const globalRecording = "global:"

// globalHint is the first line of the group of global shortcuts on goos.
func globalHint(goos string) string {
	hint := "These keys control MixMusic whichever window is in front, and while it runs they do nothing else."
	switch goos {
	case "linux":
		hint += " On Wayland your desktop asks you to confirm them, and its keyboard settings can change them."
	case "darwin":
		hint += " A letter is the key in its place on a US keyboard."
	}
	return hint
}

// globalShortcutSettings list the commands that global shortcuts run, and
// their keys, which the user records as the window's shortcuts.
func (s *shell) globalShortcutSettings(c *ui.Context) {
	s.group(c, "from any app", func() {
		s.hint(c, globalHint(runtime.GOOS))
		for _, gc := range app.GlobalCommands {
			s.globalShortcutRow(c, gc)
		}
	})
}

// globalShortcutRow is a command's global shortcut: its keys, and why
// they didn't register, with try again.
func (s *shell) globalShortcutRow(c *ui.Context, gc app.GlobalCommand) {
	k := &s.lk
	keys := s.set.GlobalShortcuts[gc.ID]
	recording := s.recording == globalRecording+gc.ID
	problem := ""
	if keys != "" && !recording {
		problem = s.svc.Preferences.GlobalShortcutProblem(gc.ID)
	}
	ui.Row(c).Gap(24).Padding(14, 0).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, gc.Title).FontSize(k.bodySize())
			switch {
			case recording:
				// Keys in use elsewhere never reach the window.
				ui.Text(c, "If nothing happens, these keys are in use elsewhere.").FontSize(k.smallSize()).TextColor(k.muted)
			case problem != "":
				ui.Row(c).Wrap().Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Text(c, problem).FontSize(k.smallSize()).TextColor(k.danger).MinWidth(0)
					if s.link(c, "global-retry-"+gc.ID, "try again") {
						s.svc.Preferences.RetryGlobalShortcuts()
					}
				})
			}
		})
		ui.Row(c).Shrink(0).Gap(8).Children(func() {
			label, ink, action := shortcutLabel(keys), k.muted, "change"
			switch {
			case recording:
				label, ink, action = "press the keys · esc cancels · backspace removes", k.accent, "cancel"
			case label == "":
				label, action = "none", "set"
			}
			ui.Text(c, label).FontSize(k.smallSize()).TextColor(ink).MinWidth(120).TextAlign(ui.End).AlignSelf(ui.Center)
			b := s.button(c, "global-edit-"+gc.ID, action, false).Label(action + " the global shortcut of " + gc.Title)
			if recording {
				// The keys pressed name the shortcut, until the button
				// loses the focus, as the window does.
				b.HandleInput(func(ev ui.InputEvent) bool { return s.recordGlobalKey(gc, ev) })
				if !b.Focused() && s.recordingFocused {
					s.recording = ""
				}
				s.recordingFocused = b.Focused()
			}
			if b.Clicked() {
				s.recording, s.recordingFocused = "", false
				if !recording {
					s.recording = globalRecording + gc.ID
					c.Announce("Press the keys of the global shortcut of " + gc.Title + ". Escape cancels, Backspace removes it.")
				}
			}
			if keys != "" && !recording &&
				s.button(c, "global-remove-"+gc.ID, "remove", false).Label("remove the global shortcut of "+gc.Title).Clicked() {
				s.setGlobalShortcut(gc, "")
			}
		})
	})
}

// recordGlobalKey takes a key pressed as the user records the keys of a
// command's global shortcut; Tab goes on, as the focus leaves.
func (s *shell) recordGlobalKey(gc app.GlobalCommand, ev ui.InputEvent) bool {
	if ev.Kind != ui.InputKeyDown || ev.Key == ui.KeyTab {
		return false
	}
	switch {
	case ev.Repeat:
	case ev.Key == ui.KeyEscape:
		s.recording = ""
	case ev.Key == ui.KeyBackspace || ev.Key == ui.KeyDelete:
		s.setGlobalShortcut(gc, "")
	default:
		// A modifier alone waits for its key.
		if keys := shortcutName(ev.Mods, ev.Key); keys != "" {
			s.takeGlobalKeys(gc, keys)
		}
	}
	return true
}

// takeGlobalKeys gives a command the keys pressed as its global shortcut,
// unless they can't be one: a toast tells why, and the recording goes on.
func (s *shell) takeGlobalKeys(gc app.GlobalCommand, keys string) {
	if problem := app.GlobalKeysProblem(keys, runtime.GOOS); problem != "" {
		s.notes = append(s.notes, player.Notice{Message: problem, Error: true})
		return
	}
	s.setGlobalShortcut(gc, keys)
}

// setGlobalShortcut gives a command the keys of a global shortcut, "" for
// none. Another command that had them gives them up, a command of the
// window too: the global shortcut takes them in MixMusic's window as well.
func (s *shell) setGlobalShortcut(gc app.GlobalCommand, keys string) {
	s.recording = ""
	if s.set.GlobalShortcuts == nil {
		s.set.GlobalShortcuts = map[string]string{}
	}
	if keys == "" {
		delete(s.set.GlobalShortcuts, gc.ID)
		s.save()
		return
	}
	s.giveUpGlobal(gc.ID, keys)
	if s.set.Shortcuts == nil {
		s.set.Shortcuts = map[string]string{}
	}
	for _, other := range s.commands() {
		if other.id != gc.ID && sameShortcut(s.shortcutOf(other), keys) {
			s.set.Shortcuts[other.id] = ""
			s.notes = append(s.notes, notice(other.title+" has no shortcut now"))
		}
	}
	s.set.GlobalShortcuts[gc.ID] = keys
	s.save()
	// The system may refuse them: the row says so too.
	if problem := s.svc.Preferences.GlobalShortcutProblem(gc.ID); problem != "" {
		s.notes = append(s.notes, player.Notice{Message: problem, Error: true})
	}
}

// giveUpGlobal takes keys, which the command id gets, from the global
// shortcut of any other command.
func (s *shell) giveUpGlobal(id, keys string) {
	for _, gc := range app.GlobalCommands {
		if gc.ID != id && sameShortcut(s.set.GlobalShortcuts[gc.ID], keys) {
			delete(s.set.GlobalShortcuts, gc.ID)
			s.notes = append(s.notes, notice(gc.Title+" has no global shortcut now"))
		}
	}
}

// globalPress takes the press of one of MixMusic's own global shortcuts,
// whose keys never reach the window, as the user records the keys of a
// shortcut in Settings › Shortcuts: on the main thread, while the window
// has the focus. The command of the keys runs otherwise.
func (s *shell) globalPress(_, keys string) bool {
	if s.recording == "" || s.win != nil && !s.win.IsFocused() {
		return false
	}
	if s.router.Path() != "/settings/shortcuts" {
		// The page went without a frame that ended the recording.
		s.recording = ""
		return false
	}
	took := false
	if id, ok := strings.CutPrefix(s.recording, globalRecording); ok {
		for _, gc := range app.GlobalCommands {
			if gc.ID == id {
				s.takeGlobalKeys(gc, keys)
				took = true
			}
		}
	} else {
		for _, cmd := range s.commands() {
			if cmd.id == s.recording {
				s.setShortcut(cmd, keys)
				took = true
				break
			}
		}
	}
	if !took {
		// A plugin's command that went meanwhile.
		s.recording = ""
	}
	if s.win != nil {
		s.win.Invalidate()
	}
	return took
}
