package main

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
		{"player.forward", "Forward 10 seconds", "ArrowRight", "playback", func() { s.p.Seek(s.st.Position + 10e9) }},
		{"player.back", "Back 10 seconds", "ArrowLeft", "playback", func() { s.p.Seek(s.st.Position - 10e9) }},
		{"player.volumeUp", "Volume up", "ArrowUp", "playback", func() { s.p.SetVolume(s.st.Volume + 0.05) }},
		{"player.volumeDown", "Volume down", "ArrowDown", "playback", func() { s.p.SetVolume(s.st.Volume - 0.05) }},
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
		{"view.sidebar", "Collapse or expand the sidebar", "Mod+B", "view", s.toggleRail},
		{"view.refresh", "Refresh this page", refreshKeys, "view", s.refreshPage},
		{"view.zoomIn", "Zoom in", "Mod+=", "view", func() { s.zoomBy(1) }},
		{"view.zoomOut", "Zoom out", "Mod+-", "view", func() { s.zoomBy(-1) }},
		{"view.zoomReset", "Reset zoom", "Mod+0", "view", func() { s.setZoom(1) }},
		{"addons.reload", "Reload add-ons", "", "add-ons", func() {
			s.plugins.Retry()
			s.svc.Addons.Reload()
			s.notes = append(s.notes, notice("Add-ons reloaded"))
		}},
	}
	for _, t := range s.inv.Themes {
		id := t.ID
		cmds = append(cmds, command{"theme." + id, "Theme: " + t.Name, "", "themes", func() { s.chooseTheme(id) }})
	}
	for _, pc := range s.plugins.Commands() {
		cmds = append(cmds, command{pc.ID, pc.Title, pc.Shortcut, "plugins", pc.Run})
	}
	return cmds
}

// shortcutOf is a command's shortcut: the user's, or its own, unless
// the user gave those keys to another command, which keeps them.
func (s *shell) shortcutOf(cmd command) string {
	if v, ok := s.set.Shortcuts[cmd.id]; ok {
		return v
	}
	if s.givenAway(cmd.id, cmd.shortcut) {
		return ""
	}
	return cmd.shortcut
}

// givenAway tells whether the user gave the keys of a shortcut to a
// command other than id.
func (s *shell) givenAway(id, keys string) bool {
	if keys == "" {
		return false
	}
	for other, v := range s.set.Shortcuts {
		if other != id && sameShortcut(v, keys) {
			return true
		}
	}
	return false
}

// alsoKeys are more shortcuts of commands, which work while a command
// keeps its own: Mod+Shift+= zooms in too, the keys of + on many
// layouts.
var alsoKeys = map[string][]string{"view.zoomIn": {"Mod+Shift+="}}

// shortcutsOf are all the shortcuts of a command.
func (s *shell) shortcutsOf(cmd command) []string {
	keys := s.shortcutOf(cmd)
	all := []string{keys}
	if _, custom := s.set.Shortcuts[cmd.id]; !custom && keys == cmd.shortcut {
		for _, more := range alsoKeys[cmd.id] {
			if !s.givenAway(cmd.id, more) {
				all = append(all, more)
			}
		}
	}
	return all
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
	for name, k := range keyNames {
		keyNameOf[k] = name
	}
}

// keyNameOf names the keys of shortcuts.
var keyNameOf = map[ui.Key]string{}

// shortcutName writes keys pressed as a shortcut, as "Mod+Shift+L", or ""
// for a key that has no name in shortcuts, as a modifier alone.
func shortcutName(mods ui.Modifiers, key ui.Key) string {
	name, ok := keyNameOf[key]
	if !ok {
		return ""
	}
	var parts []string
	if mods&ui.Cmd != 0 {
		parts = append(parts, "Mod")
	}
	// Control on macOS, where Mod is Cmd, and the Windows key elsewhere.
	if mods&ui.Ctrl != 0 && ui.Cmd != ui.Ctrl {
		parts = append(parts, "Ctrl")
	}
	if mods&ui.Alt != 0 {
		parts = append(parts, "Alt")
	}
	if mods&ui.Shift != 0 {
		parts = append(parts, "Shift")
	}
	if mods&ui.Super != 0 && ui.Cmd != ui.Super {
		parts = append(parts, "Meta")
	}
	return strings.Join(append(parts, name), "+")
}

// sameShortcut tells whether two shortcuts are the same keys, however
// written, as Mod+K and Ctrl+K off macOS.
func sameShortcut(a, b string) bool {
	am, ak, aok := parseShortcut(a)
	bm, bk, bok := parseShortcut(b)
	return aok == bok && am == bm && ak == bk
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

// shortcuts run the commands whose shortcuts were pressed, but while the
// user records one, whose keys are the shortcut's instead.
func (s *shell) runShortcuts(c *ui.Context) {
	s.plainKeys = s.plainKeys[:0]
	if s.recording != "" {
		return
	}
	for _, cmd := range s.commands() {
		for _, keys := range s.shortcutsOf(cmd) {
			mods, key, ok := parseShortcut(keys)
			if !ok {
				continue
			}
			if c.Shortcut(mods, key) {
				cmd.run()
			}
			// The keys a focused control takes, which pointerKeys leaves
			// to the command while the control's focus shows no ring, as
			// after a click.
			if mods == 0 && (key == ui.KeySpace || key == ui.KeyEnter) {
				s.plainKeys = append(s.plainKeys, plainKey{key, cmd.run})
			}
		}
	}
}

// runZoomShortcuts runs the zoom's commands whose shortcuts were pressed,
// the only shortcuts of the mini player.
func (s *shell) runZoomShortcuts(c *ui.Context) {
	for _, cmd := range s.commands() {
		if !isZoom(cmd.id) {
			continue
		}
		for _, keys := range s.shortcutsOf(cmd) {
			if mods, key, ok := parseShortcut(keys); ok && c.Shortcut(mods, key) {
				cmd.run()
			}
		}
	}
}

// plainKey is a key without modifiers that runs a command, as Space plays
// or pauses.
type plainKey struct {
	key ui.Key
	run func()
}

// palette finds and runs commands, and goes to pages.
func (s *shell) palette(c *ui.Context) {
	k := &s.lk
	if !s.paletteOpen {
		s.paletteQuery, s.paletteAt, s.paletteMoved = "", 0, false
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
	s.dialogFrame(c, &s.paletteOpen, dialogOptions{key: "palette", title: "Command palette", width: 640}, func(panel ui.Element) {
		panel.Padding(24).Gap(12)
		in := ui.TextInputBase(c.Key("palette-input"), &s.paletteQuery).Placeholder("type a command or a search").AutoFocus().
			Label("Command palette").FontSize(k.subtitleSize()).FontWeight(300).TextColor(k.accent).Padding(8, 0).Shrink(0)
		if in.Changed() {
			s.paletteAt, s.paletteMoved = 0, true
		}
		in.OnShortcut(0, ui.KeyDown, func() { s.paletteAt, s.paletteMoved = s.paletteAt+1, true })
		in.OnShortcut(0, ui.KeyUp, func() { s.paletteAt, s.paletteMoved = s.paletteAt-1, true })
		run := func(cmd command) {
			s.paletteOpen = false
			cmd.run()
		}
		if in.Submitted() && len(found) > 0 {
			run(found[s.paletteAt])
		}
		// The list gives up room in a window too short for it, and
		// scrolls: a column keeps its content's height, but for this.
		ui.Column(c).MaxHeight(420).MinHeight(0).Children(func() {
			ui.Scroll(c).MaxHeight(420).Children(func() {
				for i, cmd := range found {
					row := ui.ButtonBase(c.Key("palette-" + cmd.id)).Height(36).PaddingX(12).Gap(12).Justify(ui.SpaceBetween).Label(cmd.title).Cursor(ui.CursorPointer)
					ink := k.text
					if i == s.paletteAt {
						row.Background(k.accent)
						ink = k.accentText
						// The row chosen comes into view in the frame the keys
						// chose it: in every frame, the wheel couldn't scroll
						// it away.
						if s.paletteMoved {
							row.ScrollIntoView()
						}
					} else if row.Hovered() {
						row.Background(k.hover)
					}
					if row.Clicked() {
						run(cmd)
					}
					row.Children(func() {
						ui.Text(c, cmd.group).FontSize(k.captionSize()).TextColor(ink.Alpha(0.6)).Width(72).Shrink(0).SingleLine()
						ui.Text(c, cmd.title).FontSize(k.bodySize()).TextColor(ink).SingleLine().Grow(1).MinWidth(0)
						ui.Text(c, shortcutLabel(s.shortcutOf(cmd))).FontSize(k.captionSize()).TextColor(ink.Alpha(0.7))
					})
				}
				s.paletteMoved = false
			})
		})
	})
}
