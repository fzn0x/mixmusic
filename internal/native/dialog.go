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

// dialogScroll is where the body of a dialog is scrolled; how tall its
// title and its actions were, as a frame before laid them out; and shown,
// whether the frame showed the dialog.
type dialogScroll struct {
	state       ui.ScrollState
	title, foot float32
	shown       bool
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
// In a window too short for all of it, the actions stay in view and the
// body scrolls, under the title, which stays in view too unless it would
// leave the body less than a third of the room: a long title in a short
// window scrolls with the body. A hairline then parts the body from the
// title or the actions where it hides some of itself, and the body takes
// the focus with Tab, for the arrows, Page Up, Page Down, Home and End to
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
	_, h := c.Size()
	// The head is the title and 48 DIPs around it.
	pinned := ds.title+48+ds.foot <= max(h-2*dialogMargin, 0)*2/3
	// A hairline keeps its width, and shows only while the body hides
	// some of itself on its side: the body's height stays as it scrolls.
	hairline := func(shows bool) ui.Color {
		if shows {
			return k.border
		}
		return ui.Color{}
	}
	// measure keeps how tall e showed in the frame before, if it did, in
	// *at, and asks for another frame when that changed where the title
	// goes.
	measure := func(e ui.Element, at *float32) {
		if v := e.Bounds().H; v > 0 && v != *at {
			*at = v
			c.Invalidate()
		}
	}
	// The title is as tall in the head as in the body, which are as wide.
	title := func() {
		measure(ui.Text(c.Key(o.key+"-title"), o.title).FontSize(k.titleSize()).FontWeight(300).Font(k.display), &ds.title)
	}
	s.dialogFrame(c, open, o, func(panel ui.Element) {
		top := float32(32)
		if pinned {
			ui.Column(c).Shrink(0).Padding(32, 32, 15).BorderWidth(0, 0, 1, 0).BorderColor(hairline(over && sc.Y > 0.5)).Children(title)
			top = 0
		}
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
		b.Children(func() {
			ui.Column(c).Padding(top, 28, bottom).Gap(16).Children(func() {
				if !pinned {
					title()
				}
				body()
			})
		})
		if actions == nil {
			if ds.foot != 0 {
				ds.foot = 0
				c.Invalidate()
			}
			return
		}
		foot := ui.Row(c.Key(o.key+"-foot")).Shrink(0).Padding(15, 32, 32).Gap(8).Justify(ui.End).Wrap().
			BorderWidth(1, 0, 0, 0).BorderColor(hairline(over && sc.Y < sc.MaxY-0.5)).Children(actions)
		measure(foot, &ds.foot)
	})
}

// confirm asks before an action while *open is true, in a dialog that is
// an alert: title, message, cancel, and ok, which has the focus as the
// dialog opens, for Enter to press it. It reports whether the user chose
// ok, in the frame they did. Choosing either, or Escape, closes the
// dialog, setting *open to false; a click outside it does nothing.
func (s *shell) confirm(c *ui.Context, key string, open *bool, title, message, ok string) (chosen bool) {
	s.dialog(c, open, dialogOptions{key: key, title: title, width: 480, alert: true, message: message}, func() {
		ui.Text(c, message).FontSize(s.lk.bodySize())
	}, func() {
		if s.button(c, key+"-cancel", "cancel", false).Clicked() {
			*open = false
		}
		if s.button(c, key+"-ok", ok, true).AutoFocus().Clicked() {
			*open, chosen = false, true
		}
	})
	return chosen
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
