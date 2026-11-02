package main

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// eqPresets are the equalizer's presets, the web interface's.
var eqPresets = []struct {
	id, label string
	gains     [10]float64
}{
	{"flat", "Flat", [10]float64{}},
	{"bass", "Bass boost", [10]float64{6, 5, 4, 2, 0, 0, 0, 0, 0, 0}},
	{"treble", "Treble boost", [10]float64{0, 0, 0, 0, 0, 1, 2, 4, 5, 6}},
	{"vocal", "Vocal", [10]float64{-2, -1, 0, 2, 4, 4, 3, 1, 0, -1}},
	{"electronic", "Electronic", [10]float64{5, 4, 1, 0, -1, 1, 0, 1, 4, 5}},
	{"rock", "Rock", [10]float64{4, 3, 2, 0, -1, 0, 2, 3, 4, 4}},
	{"acoustic", "Acoustic", [10]float64{3, 2, 1, 1, 2, 1, 2, 2, 2, 1}},
	{"late-night", "Late night", [10]float64{4, 3, 1, 0, 0, 0, 0, 1, 2, 3}},
	{"lofi", "Lo-fi", [10]float64{-3, 0, 3, 4, 2, 0, -2, -4, -6, -8}},
}

func init() { addSettings("playback", 50, (*shell).equalizerSettings) }

// equalizerSettings shape the sound of what MixMusic plays itself.
func (s *shell) equalizerSettings(c *ui.Context) {
	k := &s.lk
	eq := &s.set.Equalizer
	s.group(c, "Equalizer", func() {
		s.row(c, "Equalizer", "Shape the sound of YouTube Music with ten bands and a preamp.", func() {
			if s.switchOf(c.Key("eq-on"), &eq.Enabled).Label("Equalizer").Changed() {
				s.save()
			}
		})
		labels := make([]string, len(eqPresets))
		// Bands of the user's own, as once a slider moved, show as Custom,
		// which is no preset to choose.
		chosen := "Custom"
		for i, p := range eqPresets {
			labels[i] = p.label
			if p.id == eq.Preset {
				chosen = p.label
			}
		}
		// A preset's name finds it, as "bass".
		s.row(c, "Preset", "", func() {
			if s.selectOf(c.Key("eq-preset"), &chosen, labels).Width(200).Label("Preset").Changed() {
				i := slices.Index(labels, chosen)
				if i >= 0 {
					// Choosing how it sounds turns the equalizer on.
					eq.Enabled, eq.Preset = true, eqPresets[i].id
					eq.Gains = slices.Clone(eqPresets[i].gains[:])
					s.save()
				}
			}
		}, labels...)
		s.found(c, "Bands", "The ten bands of the equalizer and its preamp, in dB: pre 32 64 125 250 500 1k 2k 4k 8k 16k Hz.", func() {
			ui.Row(c).Gap(12).Padding(16, 0).Children(func() {
				band := func(key, label string, v *float64) {
					ui.Column(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fmt.Sprintf("%+.0f", *v)).FontSize(k.captionSize()).TextColor(k.muted).FontFeatures("tnum")
						if s.bandSlider(c, key, label+" dB", v, -12, 12).Changed() {
							if key != "eq-preamp" {
								eq.Preset = "custom"
							}
							s.save()
						}
						ui.Text(c, label).FontSize(k.captionSize()).TextColor(k.muted)
					})
				}
				band("eq-preamp", "pre", &eq.Preamp)
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
		s.found(c, "", spotifyEqualizerNote, func() {
			ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Padding(12, 0).MaxWidth(720).Children(func() {
				ui.Text(c, "The equalizer works only on YouTube Music, which MixMusic plays itself, Spotify tracks played from it included. For Spotify, use a system-wide equalizer: ")
				s.linkOf(c, "Spotify notes", spotifyEqualizer)
				ui.Text(c, " that its audio settings can't change while another device plays it.")
			})
		})
	})
}

// spotifyEqualizerNote is what the equalizer's note on Spotify says.
const spotifyEqualizerNote = "The equalizer works only on YouTube Music, which MixMusic plays itself, Spotify tracks played from it included. For Spotify, use a system-wide equalizer: Spotify notes that its audio settings can't change while another device plays it."

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
