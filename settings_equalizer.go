package main

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// eqPresets are the equalizer's presets, the web interface's, by id: the
// names they show are eqPresetLabel's.
var eqPresets = []struct {
	id    string
	gains [10]float64
}{
	{"flat", [10]float64{}},
	{"bass", [10]float64{6, 5, 4, 2, 0, 0, 0, 0, 0, 0}},
	{"treble", [10]float64{0, 0, 0, 0, 0, 1, 2, 4, 5, 6}},
	{"vocal", [10]float64{-2, -1, 0, 2, 4, 4, 3, 1, 0, -1}},
	{"electronic", [10]float64{5, 4, 1, 0, -1, 1, 0, 1, 4, 5}},
	{"rock", [10]float64{4, 3, 2, 0, -1, 0, 2, 3, 4, 4}},
	{"acoustic", [10]float64{3, 2, 1, 1, 2, 1, 2, 2, 2, 1}},
	{"late-night", [10]float64{4, 3, 1, 0, 0, 0, 0, 1, 2, 3}},
	{"lofi", [10]float64{-3, 0, 3, 4, 2, 0, -2, -4, -6, -8}},
}

// eqPresetLabel names the preset of id, in the language shown.
func eqPresetLabel(id string) string {
	switch id {
	case "bass":
		return locales.EqualizerPresetBass()
	case "treble":
		return locales.EqualizerPresetTreble()
	case "vocal":
		return locales.EqualizerPresetVocal()
	case "electronic":
		return locales.EqualizerPresetElectronic()
	case "rock":
		return locales.EqualizerPresetRock()
	case "acoustic":
		return locales.EqualizerPresetAcoustic()
	case "late-night":
		return locales.EqualizerPresetLateNight()
	case "lofi":
		return locales.EqualizerPresetLoFi()
	}
	return locales.EqualizerPresetFlat()
}

func init() { addSettings("playback", 50, (*shell).equalizerSettings) }

// equalizerSettings shape the sound of what MixMusic plays itself.
func (s *shell) equalizerSettings(c *ui.Context) {
	k := &s.lk
	eq := &s.set.Equalizer
	s.group(c, locales.EqualizerTitle(), func() {
		s.row(c, locales.EqualizerTitle(), locales.EqualizerDesc(), func() {
			if s.switchOf(c.Key("eq-on"), &eq.Enabled).Label(locales.EqualizerTitle()).Changed() {
				s.save()
			}
		})
		labels := make([]string, len(eqPresets))
		// Bands of the user's own, as once a slider moved, show as Custom,
		// which is no preset to choose.
		chosen := locales.EqualizerCustom()
		for i, p := range eqPresets {
			labels[i] = eqPresetLabel(p.id)
			if p.id == eq.Preset {
				chosen = labels[i]
			}
		}
		// A preset's name finds it, as "bass".
		s.row(c, locales.EqualizerPreset(), "", func() {
			if s.selectOf(c.Key("eq-preset"), &chosen, labels).Width(200).Label(locales.EqualizerPreset()).Changed() {
				i := slices.Index(labels, chosen)
				if i >= 0 {
					// Choosing how it sounds turns the equalizer on.
					eq.Enabled, eq.Preset = true, eqPresets[i].id
					eq.Gains = slices.Clone(eqPresets[i].gains[:])
					s.save()
				}
			}
		}, labels...)
		s.found(c, locales.EqualizerBands(), locales.EqualizerBandsDesc(), func() {
			ui.Row(c).Gap(12).Padding(16, 0).Children(func() {
				band := func(key, label string, v *float64) {
					ui.Column(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fmt.Sprintf("%+.0f", *v)).FontSize(k.captionSize()).TextColor(k.muted).FontFeatures("tnum")
						if s.bandSlider(c, key, locales.EqualizerBandDB(label), v, -12, 12).Changed() {
							if key != "eq-preamp" {
								eq.Preset = "custom"
							}
							s.save()
						}
						ui.Text(c, label).FontSize(k.captionSize()).TextColor(k.muted)
					})
				}
				band("eq-preamp", locales.EqualizerPreamp(), &eq.Preamp)
				for i := range eq.Gains {
					hz := settings.Bands[i]
					label := fmt.Sprint(hz)
					if hz >= 1000 {
						label = fmt.Sprintf("%dk", hz/1000)
					}
					band("eq-"+count(i), label, &eq.Gains[i])
				}
			})
		})
		// Spotify's audio plays in Spotify's own player, which no app may
		// change: only the system's equalizer reaches it.
		s.found(c, "", locales.EqualizerSpotifyNote(), func() {
			ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Padding(12, 0).MaxWidth(720).Children(func() {
				ui.Text(c, locales.EqualizerSpotifyLead())
				s.linkOf(c, locales.EqualizerSpotifyNotes(), spotifyEqualizer)
				ui.Text(c, locales.EqualizerSpotifyTail())
			})
		})
	})
}

// spotifyEqualizer is Spotify's page on its equalizer, which says that its
// audio settings can't change while Spotify Connect plays on another device.
const spotifyEqualizer = "https://support.spotify.com/us/article/equalizer/"

// bandSlider is the slider of an equalizer's band, which goes up from lo at
// its foot to hi at its head, by half a decibel. SliderBase draws nothing
// of itself: it is drawn as the theme draws its sliders across, standing
// up, a track of the border's colour, the accent filling it from 0 dB to
// the value, either way, and the theme's white knob.
func (s *shell) bandSlider(c *ui.Context, key, label string, v *float64, lo, hi float64) ui.Element {
	t := &s.lk.theme
	kw, kh := t.Space(5), t.Space(4)
	// Half the knob's height pads it, to keep the knob on the track.
	sl := ui.SliderBase(c.Key(key), v, lo, hi).Step(0.5).Vertical().Height(140).Width(28).PaddingY(kh / 2).Label(label)
	focused := sl.FocusVisible()
	sl.Draw(func(p *ui.Painter, r ui.Rect) {
		at := func(x float64) float32 {
			f := float32(max(0, min(1, (x-lo)/(hi-lo))))
			return r.Y + kh/2 + (r.H-kh)*(1-f)
		}
		w, cx := t.Space(1.5), r.X+r.W/2
		y, zero := at(*v), at(0)
		p.Fill(ui.Rect{X: cx - w/2, Y: r.Y, W: w, H: r.H}, t.Border, w/2)
		p.Fill(ui.Rect{X: cx - w/2, Y: min(y, zero), W: w, H: max(y, zero) - min(y, zero)}, t.Accent, w/2)
		knob, rad := ui.Rect{X: cx - kw/2, Y: y - kh/2, W: kw, H: kh}, kh/2
		face := ui.RGB(255, 255, 255)
		if t.Dark {
			face = ui.RGB(224, 225, 225)
		}
		// A tight shadow edges the knob, a soft one lifts it, as the
		// theme's do.
		p.Shadow(knob, rad, 0, 0.5, 1, 0, ui.RGBA(0, 0, 0, 0.08))
		p.Shadow(knob, rad, 0, 1.5, 7, 0, ui.RGBA(0, 0, 0, 0.1))
		p.Fill(knob, face, rad)
		if focused {
			p.FocusRing(knob, [4]float32{rad, rad, rad, rad})
		}
	})
	return sl
}
