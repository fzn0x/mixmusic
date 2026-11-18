package main

import (
	"math"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// The reverb and the echo, in Settings › Playback beside the equalizer
// (#546): each a switch, its rooms or presets, and its sliders.

func init() { addSettings("playback", 51, (*shell).reverbSettings) }

// roomChoices are the reverb's rooms, in the order of settings.Rooms, each
// with how it sounds.
func roomChoices() []choice {
	return []choice{
		{locales.ReverbRoomSmall(), locales.ReverbRoomSmallDesc(), false},
		{locales.ReverbRoomHall(), locales.ReverbRoomHallDesc(), false},
		{locales.ReverbRoomPlate(), locales.ReverbRoomPlateDesc(), false},
		{locales.ReverbRoomLarge(), locales.ReverbRoomLargeDesc(), false},
	}
}

// reverbSettings play the music in a room: the switch, the rooms, of which
// choosing one turns the reverb on, and the mix.
func (s *shell) reverbSettings(c *ui.Context) {
	r := &s.set.Equalizer.Reverb
	s.group(c, locales.ReverbTitle(), func() {
		s.row(c, locales.ReverbTitle(), locales.ReverbDesc(), func() {
			if s.switchOf(c.Key("reverb-on"), &r.On).Label(locales.ReverbTitle()).Changed() {
				s.save()
			}
		}, "room hall ambience")
		room := slices.Index(settings.Rooms, r.Room)
		if s.choices(c, "reverb-room", locales.ReverbRoom(), &room, roomChoices()) {
			r.Room, r.On = settings.Rooms[room], true
			s.save()
		}
		s.row(c, locales.ReverbMix(), locales.ReverbMixAt(r.Mix), func() {
			v := float64(r.Mix)
			if ui.Slider(c.Key("reverb-mix"), &v, 0, settings.MaxMix).Step(1).Width(200).Label(locales.ReverbMixLabel()).Changed() {
				r.Mix = int(math.Round(v))
				s.save()
			}
		}, "wet dry")
		s.effectsWait(c)
	})
}

// effectsWait tells, under the reverb's and the echo's rows, that they
// wait while Sound effects is off.
func (s *shell) effectsWait(c *ui.Context) {
	if !s.set.Equalizer.Effects {
		s.hint(c, locales.ReverbEffectsOff())
	}
}

func init() { addSettings("playback", 52, (*shell).echoSettings) }

// echoPresetText is what the echo's preset of id sounds like, or, for
// none of them, that its settings are the user's own.
func echoPresetText(id string) string {
	i := slices.IndexFunc(settings.EchoPresets, func(p settings.EchoPreset) bool { return p.ID == id })
	if i < 0 {
		return locales.EchoCustom()
	}
	p := settings.EchoPresets[i]
	switch p.ID {
	case "quarter":
		return locales.EchoQuarterDesc(p.Time)
	case "tail":
		return locales.EchoTailDesc(p.Time)
	}
	return locales.EchoSlapbackDesc(p.Time)
}

// echoSettings repeat the music: the switch, the presets, of which
// choosing one turns the echo on, and the time, feedback and mix, which
// make it Custom as they move.
func (s *shell) echoSettings(c *ui.Context) {
	x := &s.set.Equalizer.Echo
	s.group(c, locales.EchoTitle(), func() {
		s.row(c, locales.EchoTitle(), locales.EchoDesc(), func() {
			if s.switchOf(c.Key("echo-on"), &x.On).Label(locales.EchoTitle()).Changed() {
				s.save()
			}
		}, "delay repeat")
		s.row(c, locales.EchoPreset(), echoPresetText(x.Preset), func() {
			chosen := slices.IndexFunc(settings.EchoPresets, func(p settings.EchoPreset) bool { return p.ID == x.Preset })
			if s.segmented(c, "echo-preset", locales.EchoPreset(), &chosen, locales.EchoSlapback(), locales.EchoQuarter(), locales.EchoTail()) {
				x.Apply(settings.EchoPresets[chosen])
				x.On = true
				s.save()
			}
		}, locales.EchoSlapback(), locales.EchoQuarter(), locales.EchoTail())
		slider := func(key, label string, v *int, lo, hi, step int) {
			f := float64(*v)
			if ui.Slider(c.Key(key), &f, float64(lo), float64(hi)).Step(float64(step)).Width(200).Label(label).Changed() {
				*v, x.Preset = int(math.Round(f)), settings.EchoCustom
				s.save()
			}
		}
		s.row(c, locales.EchoTime(), locales.EchoTimeAt(x.Time), func() {
			slider("echo-time", locales.EchoTime(), &x.Time, settings.MinEchoTime, settings.MaxEchoTime, settings.EchoTimeStep)
		}, "delay")
		feedback := locales.EchoFeedbackOff()
		if x.Feedback > 0 {
			feedback = locales.EchoFeedbackAt(x.Feedback)
		}
		s.row(c, locales.EchoFeedback(), feedback, func() {
			slider("echo-feedback", locales.EchoFeedback(), &x.Feedback, 0, settings.MaxEchoFeedback, 1)
		}, "repeats")
		s.row(c, locales.ReverbMix(), locales.EchoMixAt(x.Mix), func() {
			slider("echo-mix", locales.EchoMixLabel(), &x.Mix, 0, settings.MaxMix, 1)
		}, "wet dry")
		s.effectsWait(c)
	})
}
