package main

import (
	"fmt"
	"log"
	"math"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/keypad"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// The room the main window needs at 100%, in DIPs of the window: the rail
// for all its pages, and the player bar for all its buttons.
const (
	roomWidth = 920
	// roomWidthDevices is the width with Spotify Connect's devices button
	// in the player bar.
	roomWidthDevices = 1050
	roomHeight       = 480
	// roomPerPage is the height each page a plugin adds to the rail takes.
	roomPerPage = 46
)

// room is the largest zoom the main window has room for, w by h DIPs of
// the window.
func (s *shell) room(w, h float32) float64 {
	width := float32(roomWidth)
	if s.set.SpotifyPlayback == settings.SpotifyPlaybackConnect {
		width = roomWidthDevices
	}
	if s.castShown() {
		width += roomCast
	}
	height := float32(roomHeight + roomPerPage*len(s.plugins.SidebarItems()))
	return float64(min(w/width, h/height))
}

// shownZoom is the zoom a window with room for room shows of the zoom
// chosen: all of it, but above 100% no more than it has room for.
func shownZoom(chosen, room float64) float64 {
	if chosen <= 1 || room >= chosen {
		return chosen
	}
	return max(room, 1)
}

// percent writes a zoom as a percentage, rounded down, as "139%".
func percent(z float64) string { return fmt.Sprintf("%d%%", int(math.Floor(z*100+1e-6))) }

// applyZoom zooms the main window by the zoom chosen, as far as it has
// room for, and shows the toast of a change. It runs first in the frame,
// which then builds at the new size.
func (s *shell) applyZoom(c *ui.Context) {
	w, h := c.Size()
	z := c.Zoom()
	if z <= 0 {
		z = 1
	}
	s.zoomRoom = s.room(w*z, h*z)
	s.zoom = shownZoom(s.set.Zoom, s.zoomRoom)
	c.SetZoom(float32(s.zoom))
	if s.zoomToast {
		s.zoomToast = false
		c.AddToast(ui.Toast{ID: "zoom", Title: "Zoom " + percent(s.set.Zoom), Description: s.zoomLimit(),
			Action: "reset", OnAction: func() { s.setZoom(1) }})
	}
}

// zoomLimit tells, when the window shows less than the zoom chosen, how
// much it has room for, or "".
func (s *shell) zoomLimit() string {
	if s.zoom >= s.set.Zoom {
		return ""
	}
	return "This window has room for " + percent(s.zoom) + "."
}

// zoomBy goes a step of the zooms in, when dir is 1, or out, when -1. Out
// starts from the zoom the window shows, so that the first press of a
// window without room for the zoom chosen makes it smaller.
func (s *shell) zoomBy(dir int) {
	from, steps := s.set.Zoom, settings.ZoomSteps
	if dir < 0 && s.zoom > 0 {
		from = min(from, s.zoom)
	}
	next := from
	if dir > 0 {
		next = steps[len(steps)-1]
		for _, step := range steps {
			if step > from+1e-9 {
				next = step
				break
			}
		}
	} else {
		next = steps[0]
		for i := len(steps) - 1; i >= 0; i-- {
			if steps[i] < from-1e-9 {
				next = steps[i]
				break
			}
		}
	}
	s.setZoom(next)
}

// setZoom chooses the zoom z, keeps it, and has the next frame tell it.
func (s *shell) setZoom(z float64) {
	s.set.Zoom = z
	s.zoomToast = true
	// The one window but the main one that zooms is the mini player.
	if !s.main {
		s.miniChoose(z)
	}
	s.save()
	if s.win != nil {
		s.win.Invalidate()
	}
}

// miniZooms are the zooms the apps' mini players chose, one each, until
// their main windows hear of them: a mini player reads out the zoom it
// chooses, and its main window doesn't toast it too (#259).
var miniZooms struct {
	sync.Mutex
	m map[*app.App]float64
}

// miniChoose keeps the zoom z the mini player chose, before it saves it.
func (s *shell) miniChoose(z float64) {
	miniZooms.Lock()
	defer miniZooms.Unlock()
	if miniZooms.m == nil {
		miniZooms.m = map[*app.App]float64{}
	}
	miniZooms.m[s.a] = z
}

// miniChose tells whether the mini player chose the zoom z, which the main
// window hears the zoom changed to, and forgets the one it chose.
func (s *shell) miniChose(z float64) bool {
	miniZooms.Lock()
	defer miniZooms.Unlock()
	chose, ok := miniZooms.m[s.a]
	delete(miniZooms.m, s.a)
	return ok && chose == z
}

// zoomKeys tells the keys that zoom, as the user has them: "Ctrl+= and
// Ctrl+- change it, Ctrl+0 resets it.", or "" without any. A line never
// breaks inside a key's name, as it did before the + of Ctrl+-.
func (s *shell) zoomKeys() string {
	keys := map[string]string{}
	for _, cmd := range s.commands() {
		if isZoom(cmd.id) {
			keys[cmd.id] = strings.ReplaceAll(shortcutLabel(s.shortcutOf(cmd)), "+", wordJoiner+"+"+wordJoiner)
		}
	}
	in, out, reset := keys["view.zoomIn"], keys["view.zoomOut"], keys["view.zoomReset"]
	var parts []string
	switch {
	case in != "" && out != "":
		parts = append(parts, in+" and "+out+" change it")
	case in != "":
		parts = append(parts, in+" zooms in")
	case out != "":
		parts = append(parts, out+" zooms out")
	}
	if reset != "" {
		parts = append(parts, reset+" resets it")
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ") + "."
}

// wordJoiner joins the characters on either side of it into one word,
// which a line doesn't break inside.
const wordJoiner = "⁠"

// isZoom tells whether a command is one of the zoom's, whose keys work in
// the mini player too.
func isZoom(id string) bool {
	return id == "view.zoomIn" || id == "view.zoomOut" || id == "view.zoomReset"
}

func init() { addSettings(colorTypeRows, 30, (*shell).zoomRow) }

// zoomRow is the setting of the zoom, in Settings › Appearance.
func (s *shell) zoomRow(c *ui.Context) {
	description := "Makes everything larger or smaller: text, buttons and artwork."
	if keys := s.zoomKeys(); keys != "" {
		description += " " + keys
	}
	if limit := s.zoomLimit(); limit != "" {
		description += " " + limit
	}
	s.row(c, "Zoom", description, func() {
		labels := make([]string, len(settings.ZoomSteps))
		chosen := ""
		for i, z := range settings.ZoomSteps {
			labels[i] = percent(z)
			if z == s.set.Zoom {
				chosen = labels[i]
			}
		}
		if s.selectOf(c.Key("zoom"), &chosen, labels).Width(120).Label("Zoom").Changed() {
			for i, l := range labels {
				if l == chosen {
					s.setZoom(settings.ZoomSteps[i])
				}
			}
		}
	}, "scale")
}

// useMenuBar gives macOS's menu bar the system's menus, but a View menu
// whose items refresh the page and zoom MixMusic. It leaves out Reload
// and Force Reload, which do nothing in a window of native interface and
// took ⌘R from Repeat and ⇧⌘R from Refresh this page. The items have no
// keys of their own, so that the keys reach the window, where the user
// may change them, as elsewhere: but Zoom In's ⌘+ (zoomInKey).
func (s *shell) useMenuBar() {
	if runtime.GOOS != "darwin" {
		return
	}
	run := func(fn func()) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) { s.update(fn) }
	}
	s.zoomIn = &mygo.MenuItem{Label: "Zoom In", Click: run(func() { s.zoomBy(1) })}
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Role: mygo.RoleFileMenu},
		{Role: mygo.RoleEditMenu},
		{Label: "View", Submenu: []*mygo.MenuItem{
			{Label: "Refresh This Page", Click: run(s.refreshPage)},
			{Role: mygo.RoleToggleDevTools},
			mygo.Separator(),
			{Label: "Actual Size", Click: run(func() { s.setZoom(1) })},
			s.zoomIn,
			{Label: "Zoom Out", Click: run(func() { s.zoomBy(-1) })},
			mygo.Separator(),
			{Role: mygo.RoleToggleFullScreen},
		}},
		{Role: mygo.RoleWindowMenu},
	}))
	s.zoomInKey()
}

// plusKeys tells whether Zoom in has the keys of +, Mod+Shift+= (alsoKeys):
// while it keeps its own shortcut, and no other command took them. The
// keypad's + is one of them, which MyGo gives the window as no key (#251):
// macOS's menu takes it first, and internal/keypad on Windows and Linux.
func (s *shell) plusKeys() bool {
	for _, cmd := range s.commands() {
		if cmd.id == "view.zoomIn" {
			return slices.Contains(s.shortcutsOf(cmd), "Mod+Shift+=")
		}
	}
	return false
}

// zoomInKey gives View › Zoom In the key ⌘+ while Zoom in has the keys of
// + (plusKeys), as the shortcuts change: macOS gives a menu the keys
// before the window, by the character they type, so ⌘ and the keypad's +
// reach it, and ⌘+ on a layout with a + key of its own.
func (s *shell) zoomInKey() {
	if s.zoomIn == nil {
		return
	}
	key := ""
	if s.plusKeys() {
		key = "CmdOrCtrl+Plus"
	}
	if key != s.zoomInAccelerator {
		s.zoomInAccelerator = key
		s.zoomIn.SetAccelerator(key)
	}
}

// useKeypad has Ctrl and the keypad's + zoom in on Windows and Linux, while
// Zoom in has the keys of + (plusKeys): a hook of the window's thread on
// Windows, and a handler of the GTK window's key presses on Linux, take
// the press before MyGo, which has no key for it (#251).
func (s *shell) useKeypad() {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		return
	}
	var stop func()
	var err error
	mygo.RunOnMain(func() {
		stop, err = keypad.Watch(s.win.NativeHandle(), func() bool {
			if !s.plusKeys() {
				return false
			}
			s.update(func() { s.zoomBy(1) })
			return true
		})
	})
	if err != nil {
		log.Printf("mixmusic: the keypad's +: %v", err)
		return
	}
	s.stop = append(s.stop, stop)
}
