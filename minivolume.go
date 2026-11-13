package main

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

// miniVolumeLinger is how long the mini player's volume slider stays once
// nothing holds it: long enough for the pointer to cross from the button
// to it.
const miniVolumeLinger = 400 * time.Millisecond

// miniVolumeState is what the mini player's volume keeps from frame to
// frame: whether its slider shows, and when it goes once nothing holds it;
// hush is set when Escape or a press elsewhere closed it, which keeps it
// closed until the pointer and the focus leave the button; inSlider is set
// when the pointer is over the slider, or it is dragged or has the
// keyboard's focus.
type miniVolumeState struct {
	open, hush, inSlider bool
	until                time.Time
}

// miniVolume is the mini player's volume, which has no room for the player
// bar's slider: a button that mutes and unmutes as the player bar's does,
// over which the wheel turns the volume, and a small slider above it. The
// slider shows while the pointer is over the button or it, while it is
// dragged, and while the keyboard's focus is on either, which Tab moves
// from the button to it; it goes a moment after.
func (s *shell) miniVolume(c *ui.Context) {
	k := &s.lk
	name, label := muteOf(s.st)
	b := s.wheelVolume(s.untippedIconButton(c, "mini-mute", name, label, false, 16))
	if b.Clicked() {
		s.p.ToggleMute()
	}
	st := ui.Local(b, "slider", func() miniVolumeState { return miniVolumeState{} })
	hold := b.Hovered() || b.FocusVisible() || st.inSlider
	switch {
	case !hold:
		st.hush = false
		if !st.open {
			break
		}
		now := c.Now()
		if st.until.IsZero() {
			st.until = now.Add(miniVolumeLinger)
		}
		if left := st.until.Sub(now); left > 0 {
			c.After(left)
		} else {
			st.open = false
		}
	case !st.hush:
		st.open, st.until = true, time.Time{}
	}
	open, focused := st.open, false
	st.inSlider = false
	ui.PopoverBase(c, b, &st.open, func(panel ui.Element) {
		panel.AttachTo(b, ui.AnchorTopEnd, ui.AnchorBottomEnd).MarginY(4).Padding(4, 10).Radius(k.radiusSm).
			Background(k.raised).Border(1, k.border).Shadow(0, 8, 24, -8, ui.RGBA(0, 0, 0, 0.45))
		s.wheelVolume(panel)
		panel.Children(func() {
			v := s.st.Volume
			if s.st.Muted {
				v = 0
			}
			// A click leaves the focus where it was, as the player bar's
			// does: the slider closes once the pointer leaves it.
			slider := ui.Slider(c.Key("mini-volume"), &v, 0, 1).Width(112).KeepFocus().Label(locales.VolumeCurveGroup())
			if slider.Changed() {
				s.p.SetVolume(v)
			}
			_, _, held := slider.Dragged()
			focused = slider.Focused()
			st.inSlider = panel.Hovered() || held || slider.FocusVisible()
		})
	})
	if open && !st.open {
		st.hush = true
		// Escape gives the focus back to the button. MyGo would give it to
		// what had it as the overlay opened, which it may take from a tip
		// that showed in the overlay's place before, as Tab passed by.
		if focused {
			b.Focus()
		}
	}
}
