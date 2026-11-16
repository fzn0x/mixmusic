package main

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings(soundRows, 5, (*shell).outputDeviceRow) }

// outputDeviceRow chooses the device MixMusic's own player plays on, in
// Settings › Playback › Sound: the system's default, or a device by name.
func (s *shell) outputDeviceRow(c *ui.Context) {
	description := outputDescription()
	if s.set.OutputDevice.ID != "" && s.svc.Player.OutputFallback() {
		description = locales.OutputDescriptionGone(s.set.OutputDevice.Name)
	}
	s.row(c, locales.OutputDevice(), description, func() {
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
		if s.selectOf(c.Key("output-device"), &chosen, titles).Width(260).Label(locales.OutputDevice()).Changed() {
			if i := slices.Index(titles, chosen); i >= 0 {
				s.chooseOutput(choices[i].Device)
			}
		}
		if s.iconButton(c, "output-again", "refresh", locales.OutputLookAgain(), false, 16).Clicked() {
			s.refetch(outputDevicesKey)
		}
	}, "speakers", "headphones", "sound card", "dac", "audio device")
}

// outputDescription tells what the output device does, in Settings ›
// Playback.
func outputDescription() string { return locales.OutputDescription() }
