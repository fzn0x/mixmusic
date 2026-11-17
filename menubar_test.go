package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo"
)

// TestMenuBarInTheLanguage labels the items of macOS's menu bar in the
// app's language, as it is made again when the language changes: under
// the pseudo locale, en-XA, each label is a message, in brackets (#775).
func TestMenuBarInTheLanguage(t *testing.T) {
	s := newTestShell(t)
	usePseudoLocale(t)
	var check func(items []*mygo.MenuItem)
	check = func(items []*mygo.MenuItem) {
		for _, item := range items {
			if item.Label != "" && !strings.HasPrefix(item.Label, "[") {
				t.Errorf("the menu bar's %q isn't a message of the catalog", item.Label)
			}
			check(item.Submenu)
		}
	}
	items := s.menuBar()
	check(items)
	if s.zoomIn == nil || !strings.HasPrefix(s.zoomIn.Label, "[") {
		t.Errorf("Zoom In is %+v", s.zoomIn)
	}
}
