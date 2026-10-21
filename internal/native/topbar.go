package native

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// topBar holds the history arrows and the search box, under the window's
// own controls, and drags the window.
func (s *shell) topBar(c *ui.Context) {
	k := &s.lk
	bar := c.TitleBar()
	ui.Row(c).Height(max(bar.Height, 48)).Shrink(0).Padding(0, bar.Right+16, 0, k.gutter).Gap(16).DragWindow().Children(func() {
		ui.Row(c).Gap(4).Children(func() {
			if s.iconButton(c, "back", "back", "Back", false, 18).Disabled(!s.router.CanGoBack()).Clicked() {
				s.router.Back()
			}
			if s.iconButton(c, "forward", "forward", "Forward", false, 18).Disabled(!s.router.CanGoForward()).Clicked() {
				s.router.Forward()
			}
		})
		s.searchField(c)
		ui.Spacer(c)
		palette := ui.ButtonBase(c.Key("palette")).Height(32).PaddingX(8).Radius(k.radiusSm).Label("Open the command palette").Cursor(ui.CursorPointer)
		ink := k.muted
		if palette.Hovered() {
			palette.Background(k.hover)
			ink = k.text
		}
		if palette.Clicked() {
			s.paletteOpen = true
		}
		palette.Children(func() { ui.Text(c, shortcutLabel("Mod+K")).FontSize(k.captionSize()).TextColor(ink) })
	})
}

// searchField is the 36px outlined box that searches both services.
func (s *shell) searchField(c *ui.Context) {
	k := &s.lk
	box := ui.Row(c).Height(36).MaxWidth(520).Grow(1).Shrink(1).MinWidth(160).Gap(8).PaddingX(12).
		Radius(k.radiusSm).Background(k.bg).Cursor(ui.CursorText)
	if box.Clicked() {
		s.searchBox.Focus()
	}
	focused := s.searchBox.Focused(c)
	ink := k.muted
	if focused {
		box.Border(2, k.accent)
		ink = k.text
	} else {
		box.Border(2, k.outline)
	}
	box.Children(func() {
		ui.Icon(c, icon("search")).FontSize(18).TextColor(ink)
		in := ui.TextInputBase(c.Key("search"), &s.query).Placeholder("search spotify and youtube music").
			Grow(1).MinWidth(0).Bind(&s.searchBox).Label("Search").FontSize(k.bodySize())
		if in.Submitted() {
			if q := strings.TrimSpace(s.query); q != "" {
				s.navigate(searchPath(q))
			}
		}
	})
}
