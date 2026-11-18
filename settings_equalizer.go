package main

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// eqPresets are the equalizer's built-in presets, by id, each written for
// its bands: the names they show are eqPresetLabel's.
var eqPresets = []struct {
	id    string
	gains [10]float64
}{
	{"flat", [10]float64{}},
	{"bass", [10]float64{6, 5, 4, 2, 0, 0, 0, 0, 0, 0}},
	{"bass-cut", [10]float64{-6, -5, -4, -2, -1, 0, 0, 0, 0, 0}},
	{"treble", [10]float64{0, 0, 0, 0, 0, 1, 2, 4, 5, 6}},
	{"treble-cut", [10]float64{0, 0, 0, 0, 0, -1, -2, -4, -5, -6}},
	{"vocal", [10]float64{-2, -1, 0, 2, 4, 4, 3, 1, 0, -1}},
	{"spoken", [10]float64{-6, -4, -1, 1, 3, 4, 4, 2, 0, -3}},
	{"acoustic", [10]float64{3, 2, 1, 1, 2, 1, 2, 2, 2, 1}},
	{"classical", [10]float64{3, 2, 1, 0, 0, 0, -1, -1, 1, 3}},
	{"dance", [10]float64{6, 5, 2, 0, -1, -2, -1, 1, 3, 4}},
	{"electronic", [10]float64{5, 4, 1, 0, -1, 1, 0, 1, 4, 5}},
	{"hip-hop", [10]float64{6, 5, 2, 3, -1, -1, 1, -1, 2, 3}},
	{"jazz", [10]float64{3, 2, 1, 2, -1, -1, 0, 1, 2, 3}},
	{"metal", [10]float64{4, 3, 1, -1, -2, 0, 2, 4, 4, 3}},
	{"piano", [10]float64{1, 1, 0, 2, 3, 1, 3, 4, 2, 2}},
	{"pop", [10]float64{-1, 0, 2, 3, 3, 1, 0, 0, 1, 1}},
	{"rnb", [10]float64{4, 6, 4, 1, -2, -1, 2, 2, 3, 3}},
	{"rock", [10]float64{4, 3, 2, 0, -1, 0, 2, 3, 4, 4}},
	{"live", [10]float64{-2, 0, 2, 3, 3, 3, 2, 1, 1, 1}},
	{"late-night", [10]float64{4, 3, 1, 0, 0, 0, 0, 1, 2, 3}},
	{"lofi", [10]float64{-3, 0, 3, 4, 2, 0, -2, -4, -6, -8}},
	{"small-speakers", [10]float64{-8, -4, 3, 3, 1, 0, 0, 1, 2, 2}},
	{"warm", [10]float64{3, 3, 2, 1, 0, 0, -1, -1, -2, -3}},
}

// eqPresetLabel names the preset of id, in the language shown.
func eqPresetLabel(id string) string {
	switch id {
	case "bass":
		return locales.EqualizerPresetBass()
	case "bass-cut":
		return locales.EqualizerPresetBassCut()
	case "treble":
		return locales.EqualizerPresetTreble()
	case "treble-cut":
		return locales.EqualizerPresetTrebleCut()
	case "vocal":
		return locales.EqualizerPresetVocal()
	case "spoken":
		return locales.EqualizerPresetSpoken()
	case "acoustic":
		return locales.EqualizerPresetAcoustic()
	case "classical":
		return locales.EqualizerPresetClassical()
	case "dance":
		return locales.EqualizerPresetDance()
	case "electronic":
		return locales.EqualizerPresetElectronic()
	case "hip-hop":
		return locales.EqualizerPresetHipHop()
	case "jazz":
		return locales.EqualizerPresetJazz()
	case "metal":
		return locales.EqualizerPresetMetal()
	case "piano":
		return locales.EqualizerPresetPiano()
	case "pop":
		return locales.EqualizerPresetPop()
	case "rnb":
		return locales.EqualizerPresetRnB()
	case "rock":
		return locales.EqualizerPresetRock()
	case "live":
		return locales.EqualizerPresetLive()
	case "late-night":
		return locales.EqualizerPresetLateNight()
	case "lofi":
		return locales.EqualizerPresetLoFi()
	case "small-speakers":
		return locales.EqualizerPresetSmallSpeakers()
	case "warm":
		return locales.EqualizerPresetWarm()
	}
	return locales.EqualizerPresetFlat()
}

func init() { addSettings("playback", 50, (*shell).equalizerSettings) }

// equalizerSettings shape the sound of what MixMusic plays itself: Sound
// effects, which turns the equalizer and the effects after it on and off
// at once, the equalizer's switch, its presets, its simple mode, Auto
// headroom, and its sliders.
func (s *shell) equalizerSettings(c *ui.Context) {
	k := &s.lk
	eq := &s.set.Equalizer
	s.group(c, locales.EqualizerTitle(), func() {
		s.row(c, locales.EqualizerEffects(), locales.EqualizerEffectsDesc(), func() {
			if s.switchOf(c.Key("eq-effects"), &eq.Effects).Label(locales.EqualizerEffects()).Changed() {
				s.save()
			}
		}, "bypass")
		s.row(c, locales.EqualizerTitle(), locales.EqualizerDesc(), func() {
			if s.switchOf(c.Key("eq-on"), &eq.Enabled).Label(locales.EqualizerTitle()).Changed() {
				s.save()
			}
		})
		s.eqPresetRow(c)
		s.row(c, locales.EqualizerSimple(), locales.EqualizerSimpleDesc(), func() {
			if s.switchOf(c.Key("eq-simple"), &eq.Simple).Label(locales.EqualizerSimple()).Changed() {
				// The three show what the bands are, which stay until
				// one of them moves.
				eq.Bass, eq.Mid, eq.Treble = settings.ToneOf(eq.Gains)
				s.save()
			}
		})
		s.row(c, locales.EqualizerAutoHeadroom(), locales.EqualizerAutoHeadroomDesc(), func() {
			auto := eq.AutoHeadroom
			if s.switchOf(c.Key("eq-headroom"), &eq.AutoHeadroom).Label(locales.EqualizerAutoHeadroom()).Changed() {
				if auto {
					// The preamp stays where Auto headroom had it.
					eq.Preamp = app.Headroom(eq.Gains)
				}
				s.save()
			}
		}, "clipping")
		s.found(c, locales.EqualizerBands(), locales.EqualizerBandsDesc(), func() { s.eqSliders(c) })
		// Spotify's audio plays in Spotify's own player, which no app may
		// change: only the system's equalizer reaches it.
		s.found(c, "", locales.EqualizerSpotifyNote(locales.EqualizerSpotifyNotes()), func() {
			ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Padding(12, 0).MaxWidth(720).Children(func() {
				inlineMessage(c, locales.EqualizerSpotifyNote(mark(0, locales.EqualizerSpotifyNotes())), func(text string) {
					s.linkOf(c, text, spotifyEqualizer)
				})
			})
		})
	})
}

// eqSliders are the preamp's slider and the ten bands', or, in simple mode,
// Bass, Mid and Treble's. While Auto headroom is on, the preamp shows its
// cut, and moving it turns Auto headroom off.
func (s *shell) eqSliders(c *ui.Context) {
	k := &s.lk
	eq := &s.set.Equalizer
	// The bands go from the low notes up, left to right, in every
	// language.
	ui.Row(c).Direction(ui.LeftToRight).Gap(12).Padding(16, 0).Children(func() {
		band := func(key, label string, v *float64, lo, hi float64, moved func()) {
			ui.Column(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, eqGainLabel(*v)).FontSize(k.captionSize()).TextColor(k.muted).FontFeatures("tnum")
				if s.bandSlider(c, key, locales.EqualizerBandDB(label), v, lo, hi).Changed() {
					moved()
					s.save()
				}
				ui.Text(c, label).FontSize(k.captionSize()).TextColor(k.muted)
			})
		}
		preamp := eq.Preamp
		if eq.AutoHeadroom {
			preamp = app.Headroom(eq.Gains)
		}
		band("eq-preamp", locales.EqualizerPreamp(), &preamp, settings.MinPreamp, settings.MaxPreamp, func() {
			eq.Preamp, eq.AutoHeadroom = preamp, false
		})
		if eq.Simple {
			tone := func() {
				eq.Gains = settings.ToneGains(eq.Bass, eq.Mid, eq.Treble)
				eq.Preset = "custom"
			}
			band("eq-bass", locales.EqualizerBass(), &eq.Bass, -settings.MaxGain, settings.MaxGain, tone)
			band("eq-mid", locales.EqualizerMid(), &eq.Mid, -settings.MaxGain, settings.MaxGain, tone)
			band("eq-treble", locales.EqualizerTreble(), &eq.Treble, -settings.MaxGain, settings.MaxGain, tone)
			return
		}
		for i := range eq.Gains {
			hz := settings.Bands[i]
			label := fmt.Sprint(hz)
			if hz >= 1000 {
				label = fmt.Sprintf("%dk", hz/1000)
			}
			band("eq-"+count(i), label, &eq.Gains[i], -settings.MaxGain, settings.MaxGain, func() { eq.Preset = "custom" })
		}
	})
}

// spotifyEqualizer is Spotify's page on its equalizer, which says that its
// audio settings can't change while Spotify Connect plays on another device.
const spotifyEqualizer = "https://support.spotify.com/us/article/equalizer/"

// eqGainLabel is the number above a band: its gain in dB with a sign, whole
// as +2 and with one decimal for a half step, as +1.5.
func eqGainLabel(v float64) string {
	if v == 0 {
		return "+0"
	}
	if v == math.Trunc(v) {
		return fmt.Sprintf("%+.0f", v)
	}
	return fmt.Sprintf("%+.1f", v)
}

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
