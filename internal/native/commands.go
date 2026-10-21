package native

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
)

// command is something the user can do from anywhere: by its shortcut, or
// from the command palette. Its id and shortcut are the web interface's,
// so the shortcuts the user set apply to both.
type command struct {
	id, title, shortcut, group string
	run                        func()
}

// commands are MixMusic's own commands, and those of plugins.
func (s *shell) commands() []command {
	cur, playing := s.st.Current()
	cmds := []command{
		{"player.toggle", "Play or pause", "Space", "playback", s.p.Toggle},
		{"player.next", "Next track", "Mod+ArrowRight", "playback", s.p.Next},
		{"player.previous", "Previous track", "Mod+ArrowLeft", "playback", s.p.Previous},
		{"player.forward", "Forward 10 seconds", "Shift+ArrowRight", "playback", func() { s.p.Seek(s.st.Position + 10e9) }},
		{"player.back", "Back 10 seconds", "Shift+ArrowLeft", "playback", func() { s.p.Seek(s.st.Position - 10e9) }},
		{"player.volumeUp", "Volume up", "Mod+ArrowUp", "playback", func() { s.p.SetVolume(s.st.Volume + 0.05) }},
		{"player.volumeDown", "Volume down", "Mod+ArrowDown", "playback", func() { s.p.SetVolume(s.st.Volume - 0.05) }},
		{"player.mute", "Mute", "M", "playback", s.p.ToggleMute},
		{"player.shuffle", "Shuffle", "Mod+S", "playback", s.p.ToggleShuffle},
		{"player.repeat", "Repeat", "Mod+R", "playback", s.p.CycleRepeat},
		{"player.like", "Like what plays", "Mod+L", "playback", func() {
			if playing {
				s.setLiked(cur.Track, !s.a.IsLiked(cur.Track.ID))
			}
		}},
		{"player.radio", "Start a radio of what plays", "", "playback", func() {
			if playing {
				s.startRadio(cur.Track)
			} else {
				s.notes = append(s.notes, notice("Play something first"))
			}
		}},
		{"go.home", "Go home", "Alt+1", "go to", func() { s.navigate("/") }},
		{"go.search", "Search", "Mod+F", "go to", func() { s.searchBox.Focus() }},
		{"go.explore", "Explore", "Alt+5", "go to", func() { s.navigate("/explore") }},
		{"go.library", "Library", "Alt+2", "go to", func() { s.navigate("/library") }},
		{"go.liked", "Liked songs", "Alt+3", "go to", func() { s.navigate("/library/liked") }},
		{"go.nowPlaying", "Now playing", "Alt+4", "go to", func() { s.navigate("/now-playing") }},
		{"go.stats", "Stats", "", "go to", func() { s.navigate("/stats") }},
		{"go.marketplace", "Marketplace", "", "go to", func() { s.navigate("/marketplace") }},
		{"go.settings", "Settings", "Mod+,", "go to", func() { s.navigate("/settings") }},
		{"view.queue", "Show or hide the queue", "Mod+J", "view", func() { s.togglePanel("queue") }},
		{"view.lyrics", "Show or hide the lyrics", "Mod+Shift+L", "view", func() { s.togglePanel("lyrics") }},
		{"view.palette", "Command palette", "Mod+K", "view", func() { s.paletteOpen = !s.paletteOpen }},
		{"view.mini", "Mini player", "Mod+M", "view", s.showMini},
		{"view.sidebar", "Collapse or expand the sidebar", "Mod+B", "view", func() {
			st := s.svc.Preferences.Get()
			st.SidebarCollapsed = !st.SidebarCollapsed
			if _, err := s.svc.Preferences.Set(st); err != nil {
				s.notes = append(s.notes, noticeOf(err))
			}
		}},
		{"addons.reload", "Reload add-ons", "", "add-ons", func() {
			s.plugins.Retry()
			s.svc.Addons.Reload()
			s.notes = append(s.notes, notice("Add-ons reloaded"))
		}},
	}
	for _, pc := range s.plugins.Commands() {
		cmds = append(cmds, command{pc.ID, pc.Title, pc.Shortcut, "plugins", pc.Run})
	}
	return cmds
}

// shortcutOf is a command's shortcut: the user's, or its own.
func (s *shell) shortcutOf(cmd command) string {
	if v, ok := s.set.Shortcuts[cmd.id]; ok {
		return v
	}
	return cmd.shortcut
}

// keyNames are the keys of shortcuts, as the web interface names them.
var keyNames = map[string]ui.Key{
	"Space": ui.KeySpace, "Enter": ui.KeyEnter, "Escape": ui.KeyEscape, "Tab": ui.KeyTab,
	"Backspace": ui.KeyBackspace, "Delete": ui.KeyDelete, "Home": ui.KeyHome, "End": ui.KeyEnd,
	"PageUp": ui.KeyPageUp, "PageDown": ui.KeyPageDown,
	"ArrowLeft": ui.KeyLeft, "ArrowRight": ui.KeyRight, "ArrowUp": ui.KeyUp, "ArrowDown": ui.KeyDown,
	",": ui.KeyComma, ".": ui.KeyPeriod, "/": ui.KeySlash, ";": ui.KeySemicolon, "-": ui.KeyMinus, "=": ui.KeyEqual,
	"[": ui.KeyBracketLeft, "]": ui.KeyBracketRight, "\\": ui.KeyBackslash, "`": ui.KeyBackquote, "'": ui.KeyQuote,
	"F1": ui.KeyF1, "F2": ui.KeyF2, "F3": ui.KeyF3, "F4": ui.KeyF4, "F5": ui.KeyF5, "F6": ui.KeyF6,
	"F7": ui.KeyF7, "F8": ui.KeyF8, "F9": ui.KeyF9, "F10": ui.KeyF10, "F11": ui.KeyF11, "F12": ui.KeyF12,
}

func init() {
	for i, k := range []ui.Key{ui.Key0, ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5, ui.Key6, ui.Key7, ui.Key8, ui.Key9} {
		keyNames[string(rune('0'+i))] = k
	}
	letters := []ui.Key{ui.KeyA, ui.KeyB, ui.KeyC, ui.KeyD, ui.KeyE, ui.KeyF, ui.KeyG, ui.KeyH, ui.KeyI, ui.KeyJ, ui.KeyK, ui.KeyL, ui.KeyM,
		ui.KeyN, ui.KeyO, ui.KeyP, ui.KeyQ, ui.KeyR, ui.KeyS, ui.KeyT, ui.KeyU, ui.KeyV, ui.KeyW, ui.KeyX, ui.KeyY, ui.KeyZ}
	for i, k := range letters {
		keyNames[string(rune('A'+i))] = k
	}
}

// parseShortcut reads a shortcut as "Mod+Shift+L": Mod is Ctrl, or Cmd on
// macOS.
func parseShortcut(s string) (ui.Modifiers, ui.Key, bool) {
	if s == "" {
		return 0, 0, false
	}
	parts := strings.Split(s, "+")
	// "Mod++" ends in a plus key, which has no name of its own here.
	key, ok := keyNames[parts[len(parts)-1]]
	if !ok {
		return 0, 0, false
	}
	var mods ui.Modifiers
	for _, p := range parts[:len(parts)-1] {
		switch p {
		case "Mod":
			mods |= ui.Cmd
		case "Ctrl":
			mods |= ui.Ctrl
		case "Shift":
			mods |= ui.Shift
		case "Alt":
			mods |= ui.Alt
		case "Meta", "Cmd":
			mods |= ui.Super
		default:
			return 0, 0, false
		}
	}
	return mods, key, true
}

// shortcutLabel writes a shortcut as the system does: ⌘K on macOS, Ctrl+K
// elsewhere.
func shortcutLabel(s string) string {
	if s == "" {
		return ""
	}
	r := strings.NewReplacer("ArrowLeft", "←", "ArrowRight", "→", "ArrowUp", "↑", "ArrowDown", "↓")
	if runtime.GOOS == "darwin" {
		r = strings.NewReplacer("Mod+", "⌘", "Shift+", "⇧", "Alt+", "⌥", "Ctrl+", "⌃",
			"ArrowLeft", "←", "ArrowRight", "→", "ArrowUp", "↑", "ArrowDown", "↓")
		return r.Replace(s)
	}
	return r.Replace(strings.ReplaceAll(s, "Mod+", "Ctrl+"))
}

// shortcuts run the commands whose shortcuts were pressed.
func (s *shell) runShortcuts(c *ui.Context) {
	for _, cmd := range s.commands() {
		if mods, key, ok := parseShortcut(s.shortcutOf(cmd)); ok && c.Shortcut(mods, key) {
			cmd.run()
		}
	}
}

// palette finds and runs commands, and goes to pages.
func (s *shell) palette(c *ui.Context) {
	k := &s.lk
	if !s.paletteOpen {
		s.paletteQuery, s.paletteAt = "", 0
		return
	}
	var found []command
	q := strings.ToLower(strings.TrimSpace(s.paletteQuery))
	for _, cmd := range s.commands() {
		if cmd.id == "view.palette" {
			continue
		}
		if q == "" || strings.Contains(strings.ToLower(cmd.title), q) || strings.Contains(cmd.group, q) {
			found = append(found, cmd)
		}
	}
	if query := strings.TrimSpace(s.paletteQuery); query != "" {
		// Enter searches what was typed; the arrows reach the commands.
		search := command{"search", "Search for “" + query + "”", "", "search", func() { s.navigate(searchPath(query)) }}
		found = append([]command{search}, found...)
	}
	s.paletteAt = min(max(s.paletteAt, 0), max(len(found)-1, 0))
	panel := ui.Modal(c, &s.paletteOpen, func() {
		in := ui.TextInputBase(c.Key("palette-input"), &s.paletteQuery).Placeholder("type a command or a search").AutoFocus().
			Label("Command palette").FontSize(k.subtitleSize()).FontWeight(300).TextColor(k.accent).Padding(8, 0)
		if in.Changed() {
			s.paletteAt = 0
		}
		in.OnShortcut(0, ui.KeyDown, func() { s.paletteAt++ })
		in.OnShortcut(0, ui.KeyUp, func() { s.paletteAt-- })
		run := func(cmd command) {
			s.paletteOpen = false
			cmd.run()
		}
		if in.Submitted() && len(found) > 0 {
			run(found[s.paletteAt])
		}
		ui.Column(c).MaxHeight(420).Children(func() {
			ui.Scroll(c).MaxHeight(420).Children(func() {
				for i, cmd := range found {
					row := ui.ButtonBase(c.Key("palette-" + cmd.id)).Height(36).PaddingX(12).Justify(ui.SpaceBetween).Label(cmd.title).Cursor(ui.CursorPointer)
					ink := k.text
					if i == s.paletteAt {
						row.Background(k.accent)
						ink = k.accentText
					} else if row.Hovered() {
						row.Background(k.hover)
					}
					if row.Clicked() {
						run(cmd)
					}
					row.Children(func() {
						ui.Text(c, cmd.title).FontSize(k.bodySize()).TextColor(ink).SingleLine()
						ui.Text(c, shortcutLabel(s.shortcutOf(cmd))).FontSize(k.captionSize()).TextColor(ink.Alpha(0.7))
					})
				}
			})
		})
	})
	s.frosted(panel).Width(640).Padding(24).Gap(12).Border(1, k.border)
}
