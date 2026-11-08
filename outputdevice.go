package main

// The output device: which of this computer's audio devices MixMusic's own
// player plays on, as Settings › Playback chooses it, and the device
// button beside the volume in the player bar. outputChoices and
// outputMenu are what any place that offers the choice shows, as the mini
// player's panel can.

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// outputDevicesKey is the key the output devices load at.
const outputDevicesKey = "output-devices"

// outputChoice is one of the places MixMusic's own player can play: the
// system's default device, a device of this computer, or the device
// chosen, while it isn't there.
type outputChoice struct {
	// Device is what the settings keep of it: none for the system's
	// default.
	Device settings.OutputDevice
	// Label names it, Detail tells more: the device the system's default is
	// now, or that the device chosen isn't connected.
	Label, Detail string
	// Chosen is set on the choice of the settings.
	Chosen bool
}

// title is the choice's label with its detail, on one line, as a select
// shows it.
func (o outputChoice) title() string {
	if o.Detail == "" {
		return o.Label
	}
	return o.Label + " · " + o.Detail
}

// outputChoices lists where MixMusic's own player can play: the system's
// default, then each output device by name, those of one name told apart
// by a number, and the device chosen, while it isn't among them. Until
// the devices are listed, the device chosen stands for itself.
func outputChoices(devices []app.OutputDevice, listed bool, chosen settings.OutputDevice) []outputChoice {
	def := outputChoice{Label: "System default", Chosen: chosen.ID == ""}
	names := map[string]int{}
	for _, d := range devices {
		names[d.Name]++
		if d.Default {
			def.Detail = d.Name
		}
	}
	out := []outputChoice{def}
	seen := map[string]int{}
	found := false
	for _, d := range devices {
		label := d.Name
		if names[d.Name] > 1 {
			seen[d.Name]++
			label = fmt.Sprintf("%s · %d", d.Name, seen[d.Name])
		}
		on := chosen.ID != "" && d.ID == chosen.ID
		found = found || on
		out = append(out, outputChoice{Device: settings.OutputDevice{ID: d.ID, Name: d.Name}, Label: label, Chosen: on})
	}
	if chosen.ID != "" && !found {
		c := outputChoice{Device: chosen, Label: chosen.Name, Chosen: true}
		if c.Label == "" {
			c.Label = "The device chosen"
		}
		if listed {
			c.Detail = "not connected"
		}
		out = append(out, c)
	}
	return out
}

// outputDevices is the output devices, as they loaded last.
func (s *shell) outputDevices() *loaded[[]app.OutputDevice] {
	return fetch(s, outputDevicesKey, s.svc.Player.OutputDevices)
}

// chooseOutput has MixMusic's own player play on the device d, the system's
// default for none, from where the track is.
func (s *shell) chooseOutput(d settings.OutputDevice) {
	if d.ID == s.set.OutputDevice.ID {
		return
	}
	s.set.OutputDevice = d
	s.save()
}

// outputAside tells why what plays now doesn't play on the output device,
// "" when it does: a cast device, or Spotify's own player, plays it.
func (s *shell) outputAside() string {
	if d, on := s.svc.Player.CastingTo(); on {
		return "Playing on " + d.Name + ": the device is this computer's"
	}
	if src := s.st.Source; src != nil && src.Mode != player.ModeStream {
		if src.Mode == player.ModeConnect {
			return "Spotify Connect plays this track on its own device"
		}
		return "Spotify plays this track on the system's default device"
	}
	return ""
}

// outputMenu lists the choices of outputChoices in a menu, the one chosen
// checked, and why what plays now doesn't play there, if it doesn't.
func (s *shell) outputMenu(m *ui.Menu) {
	devices := s.outputDevices()
	if aside := s.outputAside(); aside != "" {
		m.Item(aside).Disabled(true)
		m.Separator()
	}
	for _, o := range outputChoices(devices.value, devices.done && devices.err == nil, s.set.OutputDevice) {
		item := m.Item(o.Label).Radio(o.Chosen)
		if o.Detail != "" {
			item.Subtitle(o.Detail)
		}
		if item.Chosen() {
			s.chooseOutput(o.Device)
		}
	}
	switch {
	case !devices.done:
		m.Item("Looking for devices…").Disabled(true)
	case devices.err != nil:
		m.Item(devices.err.Error()).Disabled(true)
	}
}

// outputLabel names the output device chosen, for the device button: the
// system's default, or the device, which plays the default in its place
// while it isn't there.
func (s *shell) outputLabel() string {
	d := s.set.OutputDevice
	switch {
	case d.ID == "":
		return "Output device: System default"
	case s.svc.Player.OutputFallback():
		return "Output device: " + d.Name + ", not connected: the system's default plays"
	}
	return "Output device: " + d.Name
}

// roomOutput is the width the device button needs beside the others in
// the player bar, with its gap.
const roomOutput = 36

// outputRoom tells whether the player bar has room for the device button,
// from the frame before: the room its side has left with every other
// button at its full size, which a box that grows into it measures, as the
// first of the side. The button shows once there's room for it and a
// little more, and goes once none is left, so that it doesn't come and go
// as it takes the room.
func (s *shell) outputRoom(c *ui.Context) bool {
	room := ui.Box(c.Key("output-room")).Grow(1).Basis(0).MinWidth(0).Margin(0, -4, 0, 0)
	shown := ui.Local(room, "shown", func() bool { return false })
	free := room.Bounds().W
	switch {
	case *shown && free < 1:
		*shown = false
	case !*shown && free >= roomOutput+4:
		*shown = true
	}
	return *shown
}

// outputButtonShown reports whether the player bar shows the device
// button, given its room: while MixMusic's own player is there, and there
// is a choice to make, as more than one device, or a device chosen.
func (s *shell) outputButtonShown(room bool) bool {
	if !room || !settings.NativeAudio {
		return false
	}
	devices := s.outputDevices()
	return s.set.OutputDevice.ID != "" || len(devices.value) > 1
}

// outputButton is the device button beside the volume, where the player
// bar has room for it: its menu chooses the output device, and a click
// looks for the devices again, for the next time it opens. It's in the
// accent while a device is chosen.
func (s *shell) outputButton(c *ui.Context, room bool) {
	if !s.outputButtonShown(room) {
		return
	}
	b := s.iconButton(c, "output", "speaker", s.outputLabel(), s.set.OutputDevice.ID != "", 18)
	b.Menu(s.outputMenu)
	if b.Clicked() {
		s.refetch(outputDevicesKey)
	}
}

// followOutput tells, in a notice, when the device chosen goes away and
// the system's default plays, and when it plays there again, and looks
// for the devices again then.
func (s *shell) followOutput() func() {
	return s.a.OnOutputDevice(func(note string) {
		s.update(func() {
			s.refetch(outputDevicesKey)
			s.notes = append(s.notes, player.Notice{Message: note})
		})
	})
}
