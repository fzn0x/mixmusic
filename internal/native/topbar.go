package native

import (
	"context"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// topBar holds the history arrows and the search box. Under a hidden
// title bar, it keeps clear of the window's controls and drags the window.
func (s *shell) topBar(c *ui.Context) {
	k := &s.lk
	bar := c.TitleBar()
	row := ui.Row(c).Height(max(bar.Height, 48)).Shrink(0).Padding(0, bar.Right+16, 0, k.gutter).Gap(16)
	if bar.Height > 0 {
		row.DragWindow()
	}
	row.Children(func() {
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
		palette := s.buttonBase(c.Key("palette")).Height(32).PaddingX(8).Radius(k.radiusSm).Label("Open the command palette").Cursor(ui.CursorPointer)
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

// suggestPause is how long the typing pauses before the search box asks
// for suggestions.
const suggestPause = 180 * time.Millisecond

// searchField is the 36px outlined box that searches both services. It
// suggests searches below it: what was typed, which Enter searches, then
// YouTube Music's suggestions for it, or the recent searches while it is
// empty.
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
	search := func(q string) {
		if q = strings.TrimSpace(q); q != "" {
			s.query = q
			s.forget("suggest:")
			s.navigate(searchPath(q))
		}
	}
	box.Children(func() {
		ui.Icon(c, icon("search")).FontSize(18).TextColor(ink)
		cb := ui.ComboboxBase(c.Key("search"), &s.query)
		in := cb.Input.Placeholder("search spotify and youtube music").Grow(1).MinWidth(0).Bind(&s.searchBox).
			Label("Search").FontSize(k.bodySize())
		if in.Changed() {
			s.queryAt = c.Now()
		}
		var options []string
		if cb.Open() {
			options = s.suggestions(c)
		}
		cb.Popup(func(panel ui.Element) {
			// A menu, below the box and as wide, rather than below the text.
			panel.AttachTo(box, ui.AnchorBottomStart, ui.AnchorTopStart).MarginY(6).MinWidth(box.Bounds().W).Padding(4).
				Radius(k.radiusSm).Background(k.raised).Border(1, k.border).Shadow(0, 12, 32, -8, ui.RGBA(0, 0, 0, 0.55))
			for _, q := range options {
				item := cb.Item(q).Height(36).PaddingX(12).Gap(12).AlignItems(ui.Center).Radius(k.radiusSm).Cursor(ui.CursorPointer)
				ink, mark := k.text, k.muted
				if item.Highlighted() {
					item.Background(k.accent)
					ink, mark = k.accentText, k.accentText
				}
				item.Children(func() {
					ui.Icon(c, icon("search")).FontSize(14).TextColor(mark)
					ui.Text(c, q).FontSize(k.bodySize()).TextColor(ink).SingleLine().MinWidth(0)
				})
			}
		})
		if q, ok := cb.Chosen(); ok {
			search(q)
		} else if in.Submitted() {
			search(s.query)
		}
	})
}

// suggestions are the searches the box suggests: what was typed, then up
// to seven of YouTube Music's suggestions for it, asked once the typing
// pauses, or the recent searches while nothing is typed.
func (s *shell) suggestions(c *ui.Context) []string {
	q := strings.TrimSpace(s.query)
	if wait := suggestPause - c.Now().Sub(s.queryAt); wait > 0 {
		c.After(wait)
	} else if l := fetch(s, "suggest:"+q, func(ctx context.Context) ([]string, error) { return s.svc.Catalog.Suggestions(ctx, q) }); l.done {
		s.suggestedFor, s.suggested = q, l.value
	}
	var options []string
	if q != "" {
		options = append(options, q)
	}
	// Those of what was typed before show until those of the text come.
	if s.suggestedFor == q || s.suggestedFor != "" && strings.HasPrefix(q, s.suggestedFor) {
		for _, v := range s.suggested {
			if len(options) == 8 {
				break
			}
			if v = strings.TrimSpace(v); v != "" && !strings.EqualFold(v, q) {
				options = append(options, v)
			}
		}
	}
	return options
}
