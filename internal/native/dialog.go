package native

import "github.com/egoist/mygo/ui"

// dialogMargin is the least room a dialog leaves to each edge of the
// window.
const dialogMargin = 24

// dialogOptions tell what a dialog is. key keeps its elements, and where
// its body is scrolled, apart from another dialog's; title heads it, and
// names it to assistive technology; width is its panel's, or 0 for as
// wide as what it holds. An alert, as a question asked before an action,
// is one to assistive technology too, which reads message after the
// title; a click outside it does nothing.
type dialogOptions struct {
	key, title string
	width      float32
	alert      bool
	message    string
}

// dialogScroll is where the body of a dialog is scrolled, and shown
// whether the frame showed the dialog.
type dialogScroll struct {
	state ui.ScrollState
	shown bool
}

// dialogFrame shows a dialog's panel over the dimmed window while *open is
// true, fn building what it holds. The panel is never wider or taller than
// the window less dialogMargin at each edge, and cuts what it holds there.
// Escape closes it, setting *open to false, and so does a click outside
// it, but for an alert's.
func (s *shell) dialogFrame(c *ui.Context, open *bool, o dialogOptions, fn func(panel ui.Element)) {
	k := &s.lk
	w, h := c.Size()
	ui.DialogBase(c, open, func(back, panel ui.Element) {
		back.Background(ui.RGBA(0, 0, 0, 0.4))
		if o.alert && back.Clicked() {
			// DialogBase closed it for the click outside, which an alert
			// doesn't heed.
			*open = true
		}
		if o.width > 0 {
			panel.Width(o.width)
		}
		s.frosted(panel).MaxWidth(max(w-2*dialogMargin, 0)).MaxHeight(max(h-2*dialogMargin, 0)).Radius(10).
			Border(1, k.border).Shadow(0, 10, 30, 0, ui.RGBA(0, 0, 0, 0.3)).Clip().Label(o.title)
		if o.alert {
			panel.Role(ui.RoleAlertDialog).Description(o.message)
		}
		fn(panel)
	})
}

// dialog shows a dialog over the window while *open is true: its title at
// the top, its actions, if any, at its foot, and its body between them.
// In a window too short for all of it, the title and the actions stay in
// view and the body scrolls. A hairline then parts the body from the title
// or the actions where it hides some of itself, and the body takes the
// focus with Tab, for the arrows, Page Up, Page Down, Home and End to
// scroll it. What should have the focus as the dialog opens asks for it,
// with AutoFocus: else the body could take it.
func (s *shell) dialog(c *ui.Context, open *bool, o dialogOptions, body, actions func()) {
	if !*open {
		return
	}
	k := &s.lk
	ds := s.dialogScrolls[o.key]
	if ds == nil {
		// The body's height shows once the frame is laid out: the next
		// one tells whether it scrolls.
		ds = &dialogScroll{}
		s.dialogScrolls[o.key] = ds
		c.Invalidate()
	}
	ds.shown = true
	sc := &ds.state
	over := sc.MaxY > 0.5
	// A hairline keeps its width, and shows only while the body hides
	// some of itself on its side: the body's height stays as it scrolls.
	hairline := func(shows bool) ui.Color {
		if shows {
			return k.border
		}
		return ui.Color{}
	}
	s.dialogFrame(c, open, o, func(panel ui.Element) {
		ui.Column(c).Shrink(0).Padding(32, 32, 15).BorderWidth(0, 0, 1, 0).BorderColor(hairline(over && sc.Y > 0.5)).Children(func() {
			ui.Text(c, o.title).FontSize(k.titleSize()).FontWeight(300).Font(k.display)
		})
		// The body keeps room for the focus ring, drawn outside it, which
		// the panel would cut.
		b := ui.Scroll(c.Key(o.key + "-body")).MarginX(4).TrackScroll(sc)
		if over {
			b.Focusable().Label("more of " + o.title)
		}
		bottom := float32(32)
		if actions != nil {
			bottom = 0
		}
		b.Children(func() { ui.Column(c).Padding(0, 28, bottom).Gap(16).Children(body) })
		if actions != nil {
			ui.Row(c).Shrink(0).Padding(15, 32, 32).Gap(8).Justify(ui.End).Wrap().
				BorderWidth(1, 0, 0, 0).BorderColor(hairline(over && sc.Y < sc.MaxY-0.5)).Children(actions)
		}
	})
}

// sweepDialogs forgets where the bodies of the dialogs the frame didn't
// show were scrolled: a dialog opens again at the top of its body.
func (s *shell) sweepDialogs() {
	for key, ds := range s.dialogScrolls {
		if !ds.shown {
			delete(s.dialogScrolls, key)
		}
		ds.shown = false
	}
}
