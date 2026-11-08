package main

import (
	"slices"

	"github.com/egoist/mygo/ui"
)

func init() { addSettings(soundRows, 5, (*shell).outputDeviceRow) }

// outputDeviceRow chooses the device MixMusic's own player plays on, in
// Settings › Playback › Sound: the system's default, or a device by name.
func (s *shell) outputDeviceRow(c *ui.Context) {
	description := outputDescription
	if s.set.OutputDevice.ID != "" && s.svc.Player.OutputFallback() {
		description += " " + s.set.OutputDevice.Name + " isn't connected now: the system's default plays."
	}
	s.row(c, "Output device", description, func() {
		devices := s.outputDevices()
		choices := outputChoices(devices.value, devices.done && devices.err == nil, s.set.OutputDevice)
		titles := make([]string, len(choices))
		chosen := ""
		for i, o := range choices {
			titles[i] = o.title()
			if o.Chosen {
				chosen = titles[i]
			}
		}
		if s.selectOf(c.Key("output-device"), &chosen, titles).Width(260).Label("Output device").Changed() {
			if i := slices.Index(titles, chosen); i >= 0 {
				s.chooseOutput(choices[i].Device)
			}
		}
		if s.iconButton(c, "output-again", "refresh", "Look for devices again", false, 16).Clicked() {
			s.refetch(outputDevicesKey)
		}
	}, "speakers", "headphones", "sound card", "dac", "audio device")
}

// outputDescription tells what the output device does, in Settings ›
// Playback.
const outputDescription = "The speakers or headphones MixMusic plays on. System default follows the device your system plays on; " +
	"a device that's unplugged gives way to it until it's back."
