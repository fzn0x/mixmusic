// Package plugins runs MixMusic's plugins: each in a JavaScript runtime of
// its own (sobek, an ECMAScript engine in Go), on a goroutine of its own,
// with the API of docs/plugins.md. The native interface shows what they
// add: sidebar items, pages, widgets, track actions and commands.
package plugins

import (
	"fmt"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/addons"
	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// UI is what the plugins show through: the native interface's main window.
// Its methods may be called from any goroutine.
type UI interface {
	// Toast tells something, of tone "info", "success" or "error".
	Toast(message, tone string)
	// Navigate shows a page, such as "/search?q=jazz".
	Navigate(path string)
	// Changed tells that what the plugins show changed.
	Changed()
}

// Theme is the look plugins draw in: colours as CSS hex.
type Theme struct {
	Accent, AccentText, Text, Muted, Background, Surface string
	Tiles                                                [4]string
	TileText                                             string
	Dark                                                 bool
}

// Host runs the plugins the user enabled, and keeps what they add to the
// interface. Its methods may be called from any goroutine, and on a nil
// Host, which runs nothing.
type Host struct {
	a *app.App
	p *player.Player

	mu       sync.Mutex
	running  map[string]*plugin
	failures map[string]failure
	ui       UI
	theme    Theme
	items    []*SidebarItem
	pages    []*Page
	widgets  []*Widget
	actions  []*TrackAction
	commands []*Command
}

// failure is why a version of a plugin couldn't start.
type failure struct{ message, version string }

// New returns a host of the plugins of a, which play through p.
func New(a *app.App, p *player.Player) *Host {
	return &Host{a: a, p: p, running: map[string]*plugin{}, failures: map[string]failure{}}
}

// SetUI has the plugins show through ui, nil for nowhere.
func (h *Host) SetUI(ui UI) {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.ui = ui
	h.mu.Unlock()
}

// SetTheme tells the plugins the look they draw in.
func (h *Host) SetTheme(t Theme) {
	if h == nil {
		return
	}
	h.mu.Lock()
	changed := h.theme != t
	h.theme = t
	h.mu.Unlock()
	if changed {
		h.changed()
	}
}

func (h *Host) currentTheme() Theme {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.theme
}

func (h *Host) toast(message, tone string) {
	h.mu.Lock()
	ui := h.ui
	h.mu.Unlock()
	if ui != nil {
		ui.Toast(message, tone)
	}
}

func (h *Host) navigate(path string) {
	h.mu.Lock()
	ui := h.ui
	h.mu.Unlock()
	if ui != nil {
		ui.Navigate(path)
	}
}

func (h *Host) changed() {
	h.mu.Lock()
	ui := h.ui
	h.mu.Unlock()
	if ui != nil {
		ui.Changed()
	}
}

// Sync runs the plugins enabled among those installed: it starts those
// that don't run, stops those no longer enabled or installed, and starts
// again those of another version.
func (h *Host) Sync(enabled []string, installed []addons.Plugin) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	byID := map[string]addons.Plugin{}
	for _, p := range installed {
		byID[p.ID] = p
	}
	for id, pl := range h.running {
		p, ok := byID[id]
		if !ok || !slices.Contains(enabled, id) || p.Version != pl.version {
			h.stopLocked(pl)
		}
	}
	// A plugin that failed starts again once enabled again, or updated.
	for id, f := range h.failures {
		if p, ok := byID[id]; !ok || !slices.Contains(enabled, id) || p.Version != f.version {
			delete(h.failures, id)
		}
	}
	for _, id := range enabled {
		p, ok := byID[id]
		if _, failed := h.failures[id]; !ok || failed || h.running[id] != nil {
			continue
		}
		h.startLocked(p)
	}
}

// Retry forgets why plugins failed, so that the next Sync starts them
// again.
func (h *Host) Retry() {
	if h == nil {
		return
	}
	h.mu.Lock()
	clear(h.failures)
	h.mu.Unlock()
}

// Close stops every plugin.
func (h *Host) Close() {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, pl := range h.running {
		h.stopLocked(pl)
	}
}

// Failure tells why a plugin couldn't start, or "".
func (h *Host) Failure(id string) string {
	if h == nil {
		return ""
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.failures[id].message
}

// Running reports whether a plugin runs.
func (h *Host) Running(id string) bool {
	if h == nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running[id] != nil
}

func (h *Host) startLocked(p addons.Plugin) {
	pl := newPlugin(h, p)
	h.running[p.ID] = pl
	pl.loop.post(func() {
		if err := pl.activate(p); err != nil {
			h.fail(pl, err)
		}
	})
}

// fail stops a plugin that failed, and tells why.
func (h *Host) fail(pl *plugin, err error) {
	h.mu.Lock()
	if h.running[pl.id] == pl {
		h.stopLocked(pl)
	}
	h.failures[pl.id] = failure{err.Error(), pl.version}
	h.mu.Unlock()
	log.Printf("mixmusic: the plugin %s failed: %v", pl.id, err)
	h.toast(fmt.Sprintf("The plugin %s failed to start: %v", pl.name, err), "error")
	h.changed()
}

// stopLocked stops a plugin: what it added goes at once, and its loop runs
// its cleanups, then ends. A plugin that keeps its loop busy is
// interrupted.
func (h *Host) stopLocked(pl *plugin) {
	delete(h.running, pl.id)
	h.removeLocked(pl)
	pl.cancel()
	pl.loop.post(pl.shutdown)
	pl.loop.stop()
	wait := stopWait
	go func() {
		select {
		case <-pl.loop.done:
		case <-time.After(wait):
			if vm := pl.runtime.Load(); vm != nil {
				vm.Interrupt("MixMusic stopped the plugin")
			}
		}
	}()
	go h.changed()
}

// stopWait is how long a plugin's cleanups get before it is interrupted.
var stopWait = 3 * time.Second

// The plugins' additions to the interface.

// SidebarItem is an item a plugin adds to the sidebar, opening one of its
// pages.
type SidebarItem struct {
	Plugin, ID, Label, Path string
	// Icon is the SVG of a 24×24 icon drawn in currentColor, or "".
	Icon string
}

// Page is a page a plugin adds, at /plugin/<plugin id>/<name>.
type Page struct {
	Plugin, Name, Title, Path string
	View                      *View
}

// Widget is a section a plugin adds to the now playing page.
type Widget struct {
	Plugin, ID, Title string
	View              *View
}

// TrackAction is an item a plugin adds to the menu of every track.
type TrackAction struct {
	Plugin, ID, Label string
	pl                *plugin
	run               jsFunc
}

// Run runs the action on a track.
func (a *TrackAction) Run(t media.Track) {
	a.pl.loop.post(func() { a.pl.call(a.run, a.pl.fromGo(t)) })
}

// Command is a command a plugin adds to the command palette, with an
// optional shortcut, as "Mod+Shift+P".
type Command struct {
	// ID is the plugin's ID and the command's, as "sleep-timer.cancel".
	Plugin, ID, Title, Shortcut string
	pl                          *plugin
	run                         jsFunc
}

// Run runs the command.
func (c *Command) Run() { c.pl.loop.post(func() { c.pl.call(c.run) }) }

// SidebarItems returns the items plugins add to the sidebar.
func (h *Host) SidebarItems() []SidebarItem {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]SidebarItem, len(h.items))
	for i, it := range h.items {
		out[i] = *it
	}
	return out
}

// Page returns the page of a plugin at path, or nil.
func (h *Host) Page(path string) *Page {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, p := range h.pages {
		if p.Path == path {
			return p
		}
	}
	return nil
}

// Widgets returns the widgets plugins add to the now playing page.
func (h *Host) Widgets() []*Widget {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.widgets)
}

// TrackActions returns the items plugins add to the menu of every track.
func (h *Host) TrackActions() []*TrackAction {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.actions)
}

// Commands returns the commands plugins add.
func (h *Host) Commands() []*Command {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.commands)
}

// add keeps an addition of a plugin, and returns what removes it.
func add[T any](h *Host, list *[]*T, entry *T) func() {
	h.mu.Lock()
	*list = append(*list, entry)
	h.mu.Unlock()
	h.changed()
	return func() {
		h.mu.Lock()
		*list = slices.DeleteFunc(*list, func(e *T) bool { return e == entry })
		h.mu.Unlock()
		h.changed()
	}
}

// removeLocked removes everything a plugin added.
func (h *Host) removeLocked(pl *plugin) {
	h.items = slices.DeleteFunc(h.items, func(e *SidebarItem) bool { return e.Plugin == pl.id })
	h.pages = slices.DeleteFunc(h.pages, func(e *Page) bool { return e.View.pl == pl })
	h.widgets = slices.DeleteFunc(h.widgets, func(e *Widget) bool { return e.View.pl == pl })
	h.actions = slices.DeleteFunc(h.actions, func(e *TrackAction) bool { return e.pl == pl })
	h.commands = slices.DeleteFunc(h.commands, func(e *Command) bool { return e.pl == pl })
}
