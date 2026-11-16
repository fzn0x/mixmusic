package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/locales"
)

const (
	// volumeStep is how far a notch of the wheel over a volume control
	// turns the volume, as Volume up and Volume down do: about 3 dB on the
	// Natural curve (internal/volume).
	volumeStep = 0.05
	// volumeScroll is how far a touchpad scrolls over a volume control, in
	// DIPs, for each volumeStep.
	volumeScroll = 50
)

// muteOf is the icon and the label of the mute button: the speaker crossed
// out while nothing sounds, muted or at no volume, and what a click does.
func muteOf(st player.State) (name, label string) {
	name, label = "volume", locales.VolumeMute()
	if st.Muted {
		label = locales.VolumeUnmute()
	}
	if st.Muted || st.Volume == 0 {
		name = "volumeMute"
	}
	return name, label
}

// wheelVolume has the wheel over e turn the volume up and down: each notch
// of a mouse wheel by volumeStep, and a touchpad by volumeStep for each
// volumeScroll it scrolls, not as it glides on after the fingers lift.
// Turning it up unmutes, from the volume before; turning it down leaves a
// muted player muted. With Ctrl or Super held, the wheel does what it does
// elsewhere.
func (s *shell) wheelVolume(e ui.Element) ui.Element {
	rest := ui.Local(e, "wheel", func() float32 { return 0 })
	return e.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputScroll || ev.DY == 0 || ev.Mods&(ui.Ctrl|ui.Super) != 0 {
			return false
		}
		if ev.Momentum {
			return true
		}
		steps := 1
		switch {
		case ev.Precise:
			*rest += ev.DY
			steps = int(*rest / volumeScroll)
			*rest -= float32(steps) * volumeScroll
		case ev.DY < 0:
			steps = -1
		}
		// Down the page is down the volume.
		by := -float64(steps) * volumeStep
		if by == 0 || by < 0 && s.p.State().Muted {
			return true
		}
		s.p.ChangeVolume(by)
		return true
	})
}
