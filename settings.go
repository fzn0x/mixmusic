package main

import (
	"context"
	"fmt"
	"maps"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/addons"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// settingsTabs are the settings' sections, by their paths' last part.
var settingsTabs = []string{"appearance", "playback", "accounts", "plugins", "desktop", "shortcuts", "about"}

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

// colorSaveAfter is how long a colour waits, once the user stops changing
// it, to be saved.
var colorSaveAfter = 500 * time.Millisecond

// settingsPage shows the settings, by section.
func (s *shell) settingsPage(c *ui.Context, tab string) {
	selected := max(slices.Index(settingsTabs, tab), 0)
	s.pageScroll(c, func() {
		before := selected
		s.pageHeader(c, "settings", settingsTabs, &selected)
		if selected != before {
			s.router.Replace("/settings/" + settingsTabs[selected])
		}
		ui.Column(c).MaxWidth(880).Children(func() {
			switch settingsTabs[selected] {
			case "appearance":
				s.appearanceSettings(c)
			case "playback":
				s.playbackSettings(c)
			case "accounts":
				s.accountSettings(c)
			case "plugins":
				s.pluginSettings(c)
			case "desktop":
				s.desktopSettings(c)
			case "shortcuts":
				s.shortcutSettings(c)
			case "about":
				s.aboutSettings(c)
			}
		})
	})
}

// save keeps the settings as the controls changed them.
func (s *shell) save() {
	if _, err := s.svc.Preferences.Set(s.set); err != nil {
		s.notes = append(s.notes, noticeOf(err))
	}
}

// group titles a set of rows.
func (s *shell) group(c *ui.Context, title string, rows func()) { s.groupWith(c, title, nil, rows) }

// groupWith titles a set of rows, with links to what relates to them.
func (s *shell) groupWith(c *ui.Context, title string, links, rows func()) {
	ui.Column(c).Margin(0, 0, 48).Children(func() {
		s.sectionHead(c, title, links)
		ui.Column(c).Dividers(1, s.lk.border).Children(rows)
	})
}

// row is a setting: its title and description, and its control.
func (s *shell) row(c *ui.Context, title, description string, control func()) {
	k := &s.lk
	ui.Row(c).Gap(24).Padding(14, 0).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, title).FontSize(k.bodySize())
			if description != "" {
				ui.Text(c, description).FontSize(k.smallSize()).TextColor(k.muted)
			}
		})
		ui.Row(c).Shrink(0).Gap(8).Children(control)
	})
}

// draft is the text of a setting's input, until the user is done with it.
type draft struct {
	text    string
	focused bool
}

// settingInput is a text input of a setting that holds saved: the user's
// text is a draft until they press Enter or leave the input, when commit
// takes it, trimmed. Until they edit it, it shows the setting.
func (s *shell) settingInput(c *ui.Context, key, saved string, commit func(string)) (ui.Element, *draft) {
	d := s.drafts[key]
	if d == nil {
		d = &draft{text: saved}
		s.drafts[key] = d
	}
	in := ui.TextInput(c.Key(key), &d.text)
	focused := in.Focused()
	switch {
	case in.Submitted() || (d.focused && !focused):
		if v := strings.TrimSpace(d.text); v != saved {
			commit(v)
		}
	case !focused:
		d.text = saved
	}
	d.focused = focused
	return in, d
}

// hint is a muted paragraph among a group's rows.
// chooseTheme wears the theme id: following the computer, as the theme
// of the mode it's in.
func (s *shell) chooseTheme(id string) {
	switch {
	case !s.set.FollowSystem:
		s.set.Theme = id
	case s.lk.systemDark:
		s.set.DarkTheme = id
	default:
		s.set.LightTheme = id
	}
	s.save()
}

// rowWith is a row whose description describe builds, with links say.
func (s *shell) rowWith(c *ui.Context, title string, describe, control func()) {
	k := &s.lk
	ui.Row(c).Gap(24).Padding(14, 0).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, title).FontSize(k.bodySize())
			ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Children(describe)
		})
		ui.Row(c).Shrink(0).Gap(8).Children(control)
	})
}

func (s *shell) hint(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(s.lk.smallSize()).TextColor(s.lk.muted).Padding(12, 0).MaxWidth(720)
}

// problems tells of the add-ons of a kind that didn't load.
func (s *shell) problems(c *ui.Context, kind addons.Kind) {
	what := "theme"
	if kind == addons.KindPlugin {
		what = "plugin"
	}
	for _, p := range s.inv.Problems {
		if p.Kind == kind {
			ui.Text(c, "The "+what+" in “"+p.Folder+"” did not load: "+p.Error).FontSize(s.lk.smallSize()).
				TextColor(s.lk.danger).Padding(8, 0)
		}
	}
}

// openFolder opens the folder of the add-ons of a kind.
func (s *shell) openFolder(kind addons.Kind) {
	if err := s.svc.Addons.OpenFolder(kind); err != nil {
		s.notes = append(s.notes, noticeOf(err))
	}
}

func (s *shell) appearanceSettings(c *ui.Context) {
	k := &s.lk
	s.groupWith(c, "theme", func() {
		if s.link(c, "more-themes", "get more themes") {
			s.navigate("/marketplace/themes")
		}
		if s.link(c, "themes-folder", "open themes folder") {
			s.openFolder(addons.KindTheme)
		}
	}, func() {
		s.row(c, "Follow the computer", "Show one theme while the computer is in light mode and another in dark mode. The theme you pick below is the one for the mode it is in now.", func() {
			if s.switchOf(c.Key("follow"), &s.set.FollowSystem).Label("Follow the computer").Changed() {
				s.save()
			}
		})
		if s.set.FollowSystem {
			s.row(c, "In light mode", "", func() { s.themeSelect(c, "light-theme", "Theme in light mode", &s.set.LightTheme) })
			s.row(c, "In dark mode", "", func() { s.themeSelect(c, "dark-theme", "Theme in dark mode", &s.set.DarkTheme) })
		}
		ui.Row(c).Wrap().Gap(10).Padding(14, 0).Children(func() {
			active := s.set.ActiveTheme(k.systemDark)
			for _, t := range s.inv.Themes {
				s.themeCard(c, t, active == t.ID)
			}
		})
		s.problems(c, addons.KindTheme)
	})
	// The options of the built-in themes, whose effects the native
	// interface draws itself.
	if th, ok := s.activeTheme(); ok && th.Builtin && len(th.Options) > 0 {
		s.group(c, th.Name+" options", func() {
			for _, o := range th.Options {
				s.themeOption(c, th, o)
			}
		})
	}
	s.group(c, "colour and type", func() {
		s.row(c, "Follow the artwork", "Take the accent colour from the artwork of what plays.", func() {
			if s.switchOf(c.Key("dynamic-accent"), &s.set.DynamicAccent).Label("Follow the artwork").Changed() {
				s.save()
			}
		})
		s.row(c, "Accent colour", "Replaces the theme's accent when the artwork does not set it.", func() { s.accentControl(c) })
		s.zoomRow(c)
		s.row(c, "Text size", fmt.Sprintf("%d%%", int(s.set.FontScale*100+0.5)), func() {
			if ui.StepSlider(c.Key("font-scale"), &s.set.FontScale, 0.85, 1.3, 0.05).Width(200).Label("Text size").Changed() {
				s.save()
			}
		})
		s.row(c, "Density", "Compact fits more rows on the screen.", func() {
			compact := 0
			if s.set.Density == settings.DensityCompact {
				compact = 1
			}
			if s.segmented(c, "density", "Density", &compact, "comfortable", "compact") {
				s.set.Density = settings.DensityComfortable
				if compact == 1 {
					s.set.Density = settings.DensityCompact
				}
				s.save()
			}
		})
		s.row(c, "Icons only in the sidebar", "The sidebar shows its pages as icons. It does in narrow windows too, until you unfold it there.", func() {
			if s.switchOf(c.Key("collapsed"), &s.set.SidebarCollapsed).Label("Icons only in the sidebar").Changed() {
				s.save()
			}
		})
	})
}

// themeSelect chooses the theme of a mode of the computer, among the
// themes, the light ones first.
func (s *shell) themeSelect(c *ui.Context, key, label string, id *string) {
	themes := slices.Clone(s.inv.Themes)
	rank := func(t addons.Theme) int {
		if t.Scheme == "light" {
			return 0
		}
		return 1
	}
	slices.SortStableFunc(themes, func(a, b addons.Theme) int {
		if r := rank(a) - rank(b); r != 0 {
			return r
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	labels := make([]string, len(themes))
	chosen := ""
	for i, t := range themes {
		labels[i] = t.Name + " (" + t.Scheme + ")"
		if t.ID == *id {
			chosen = labels[i]
		}
	}
	if s.selectOf(c.Key(key), &chosen, labels).Width(220).Label(label).Changed() {
		if i := slices.Index(labels, chosen); i >= 0 {
			*id = themes[i].ID
			s.save()
		}
	}
}

// themeCard shows a theme in its own ground and ink, with its four tiles;
// the current one wears an accent outline.
func (s *shell) themeCard(c *ui.Context, t addons.Theme, current bool) {
	k := &s.lk
	id, name := t.ID, t.Name
	tk := s.cardLook(id)
	card := s.buttonBase(c.Key("theme-"+id)).Width(200).Height(124).Column().AlignItems(ui.Start).Justify(ui.SpaceBetween).
		Padding(14).Background(tk.bg).Border(1, tk.border).Label(name).Cursor(ui.CursorPointer)
	if current {
		card.Border(3, k.accent)
	}
	if card.Clicked() {
		s.chooseTheme(id)
	}
	card.Children(func() {
		ui.Text(c, name).FontSize(k.size(1.5)).FontWeight(300).Font(tk.display).TextColor(tk.text).SingleLine()
		ui.Row(c).Gap(4).Children(func() {
			for i := range 4 {
				ui.Box(c).Size(22, 22).Background(tk.tile(i))
			}
		})
		by := "built in"
		if !t.Builtin {
			by = "by " + t.Author
		}
		ui.Text(c, by).FontSize(k.captionSize()).TextColor(tk.muted).SingleLine()
	})
}

// cardLook is the look of theme id's card, made once for the themes
// installed: a look lists the system's fonts, which takes a while, and the
// cards show every theme in every frame. A card's look is the theme's own,
// whatever the settings and the computer's light or dark.
func (s *shell) cardLook(id string) *look {
	themes := s.inv.Themes
	if s.cardLooks == nil || len(themes) != len(s.cardThemes) || len(themes) > 0 && &themes[0] != &s.cardThemes[0] {
		s.cardLooks, s.cardThemes = map[string]*look{}, themes
	}
	tk, ok := s.cardLooks[id]
	if !ok {
		l := newLook(settings.Settings{Theme: id, FontScale: 1}, themes, false, "")
		tk = &l
		s.cardLooks[id] = tk
	}
	return tk
}

// accentControl chooses the accent colour.
func (s *shell) accentControl(c *ui.Context) {
	s.colorControl(c, "accent", "Accent colour", s.chosenAccent(), func(hex string) { s.set.Accent = hex })
	if s.set.Accent != "" && s.link(c, "accent-theme", "use the theme's") {
		s.set.Accent = ""
		s.wells["accent"].edits = 0
		s.save()
	}
}

// colorWell is a colour well's colour, and how many of the user's changes
// to it wait to be saved.
type colorWell struct {
	color ui.Color
	edits int
}

// colorControl is a colour well of a setting, which shows current until
// the user changes it, and set puts in the settings. The look follows the
// colour as it changes; the settings keep it once the user stops a moment,
// rather than at every step of a drag.
func (s *shell) colorControl(c *ui.Context, key, label string, current ui.Color, set func(hex string)) {
	w := s.wells[key]
	if w == nil {
		w = &colorWell{}
		s.wells[key] = w
	}
	if w.edits == 0 {
		w.color = current
	}
	if !s.colorWellOf(c.Key(key), &w.color).Label(label).Changed() {
		return
	}
	set(hexColor(w.color))
	s.restyle()
	w.edits++
	n := w.edits
	time.AfterFunc(colorSaveAfter, func() {
		s.update(func() {
			if w.edits == n {
				w.edits = 0
				s.save()
			}
		})
	})
}

// keepColors saves at once the colours whose wells wait for the user to
// stop a moment, as the window closes or MixMusic quits first. The wells'
// own saves, once due, then find nothing to save.
func (s *shell) keepColors() {
	waiting := false
	for _, w := range s.wells {
		if w.edits > 0 {
			w.edits, waiting = 0, true
		}
	}
	if waiting {
		s.save()
	}
}

// activeTheme is the theme the look shows.
func (s *shell) activeTheme() (addons.Theme, bool) {
	id := s.set.ActiveTheme(s.lk.systemDark)
	for _, t := range s.inv.Themes {
		if t.ID == id {
			return t, true
		}
	}
	return addons.Theme{}, false
}

// themeOption is the control of an option of a theme.
func (s *shell) themeOption(c *ui.Context, th addons.Theme, o addons.Option) {
	value := optionValue(th, s.set.ThemeOptions[th.ID], o.ID)
	set := func(v string) {
		s.setThemeOption(th.ID, o.ID, v)
		s.save()
	}
	key := "option-" + th.ID + "-" + o.ID
	desc := ""
	if o.Type == addons.OptionRange {
		desc = value + o.Unit
	}
	s.row(c, o.Label, desc, func() {
		switch o.Type {
		case addons.OptionToggle:
			on := value == "true"
			if s.switchOf(c.Key(key), &on).Label(o.Label).Changed() {
				set(strconv.FormatBool(on))
			}
		case addons.OptionSelect:
			labels := make([]string, len(o.Choices))
			chosen := value
			for i, ch := range o.Choices {
				labels[i] = ch.Label
				if ch.Value == value {
					chosen = ch.Label
				}
			}
			if s.selectOf(c.Key(key), &chosen, labels).Width(200).Label(o.Label).Changed() {
				if i := slices.Index(labels, chosen); i >= 0 {
					set(o.Choices[i].Value)
				}
			}
		case addons.OptionRange:
			v, _ := strconv.ParseFloat(value, 64)
			top, step := o.Max, o.Step
			if top <= o.Min {
				top = 100
			}
			if step <= 0 {
				step = 1
			}
			if ui.StepSlider(c.Key(key), &v, o.Min, top, step).Width(200).Label(o.Label).Changed() {
				set(strconv.FormatFloat(v, 'f', -1, 64))
			}
		case addons.OptionColor:
			current, ok := parseColor(value)
			if !ok {
				current, _ = parseColor(o.Default)
			}
			s.colorControl(c, key, o.Label, col(current), func(hex string) { s.setThemeOption(th.ID, o.ID, hex) })
		}
	})
}

// setThemeOption sets an option of a theme in the settings, whose maps
// are copied, not changed.
func (s *shell) setThemeOption(theme, id, value string) {
	options := maps.Clone(s.set.ThemeOptions)
	if options == nil {
		options = map[string]map[string]string{}
	}
	values := maps.Clone(options[theme])
	if values == nil {
		values = map[string]string{}
	}
	values[id] = value
	options[theme] = values
	s.set.ThemeOptions = options
}

// chosenAccent is the accent the user chose, or the active theme's.
func (s *shell) chosenAccent() ui.Color {
	if a, ok := parseColor(s.set.Accent); ok && s.set.Accent != "" {
		return col(a)
	}
	id := s.set.ActiveTheme(s.lk.systemDark)
	for _, t := range s.inv.Themes {
		if a, ok := parseColor(t.Colors["accent"]); ok && t.ID == id {
			return col(a)
		}
	}
	a, _ := parseColor(fallbackTheme["accent"])
	return col(a)
}

// spotifyPlaysHere tells whether MixMusic plays Spotify itself: where its
// webview, WebView2, decrypts Spotify's audio.
var spotifyPlaysHere = runtime.GOOS == "windows"

func (s *shell) playbackSettings(c *ui.Context) {
	k := &s.lk
	sp := s.accounts().Spotify
	signedIn := sp.SignedIn
	s.group(c, "spotify tracks", func() {
		here := "MixMusic plays them itself, at Spotify's own quality, and shows among your Spotify devices. Needs Spotify Premium, and no Spotify app."
		if !spotifyPlaysHere {
			here = "This computer's webview can't play Spotify's protected audio, so MixMusic can't play Spotify itself."
		}
		modes := []settings.SpotifyPlayback{settings.SpotifyPlaybackHere, settings.SpotifyPlaybackConnect, settings.SpotifyPlaybackMatch}
		mode := slices.Index(modes, s.set.SpotifyPlayback)
		if s.choices(c, "spotify-mode", "How Spotify tracks play", &mode, []choice{
			{"in mixmusic, from spotify", here, !spotifyPlaysHere},
			{"on spotify connect", "Your Spotify app, or another Spotify device, plays them, at Spotify's own quality. Needs Spotify Premium and a device with Spotify open.", false},
			{"in mixmusic, from youtube music", "MixMusic plays their recording on YouTube Music, with the equalizer, crossfade and visualizer. Works without Premium.", false},
		}) {
			s.set.SpotifyPlayback = modes[mode]
			s.save()
		}
		switch {
		case !signedIn:
			ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Padding(12, 0).Children(func() {
				ui.Text(c, "Sign in to Spotify in ")
				s.linkOf(c, "accounts", "/settings/accounts")
				ui.Text(c, " to play your Spotify library.")
			})
		case s.set.SpotifyPlayback == settings.SpotifyPlaybackHere && spotifyPlaysHere && !sp.Streaming:
			busy := s.signingIn == "spotify"
			label := "sign in again"
			if busy {
				label = "waiting for the browser…"
			}
			s.alert(c, "Sign in to Spotify again", "You signed in before MixMusic played Spotify itself, which your sign-in doesn't allow yet. Until you sign in again, Spotify tracks play from YouTube Music.", func() {
				if s.button(c, "spotify-again", label, true).Disabled(busy).Clicked() {
					s.notes = append(s.notes, notice("Finish signing in in your browser"))
					s.account("spotify", "Signed in to Spotify", func(ctx context.Context) error {
						_, err := s.svc.Accounts.SpotifyLogin(ctx)
						return err
					})
				}
			}).Margin(12, 0)
		case s.set.SpotifyPlayback == settings.SpotifyPlaybackConnect:
			s.deviceRow(c)
		}
	})
	s.group(c, "sound", func() {
		s.row(c, "Quality", "Data saver streams smaller files of YouTube Music.", func() {
			saver := 0
			if s.set.Quality == settings.QualitySaver {
				saver = 1
			}
			if s.segmented(c, "quality", "Quality", &saver, "best", "data saver") {
				s.set.Quality = settings.QualityBest
				if saver == 1 {
					s.set.Quality = settings.QualitySaver
				}
				s.save()
			}
		})
		desc := "Off: tracks follow each other."
		if s.set.Crossfade > 0 {
			desc = fmt.Sprintf("Tracks overlap by %d seconds.", s.set.Crossfade)
		}
		s.row(c, "Crossfade", desc, func() {
			v := float64(s.set.Crossfade)
			if ui.StepSlider(c.Key("crossfade"), &v, 0, 12, 1).Width(200).Label("Crossfade").Changed() {
				s.set.Crossfade = int(v + 0.5)
				s.save()
			}
		})
		even := "Turns loud tracks down to one level, from YouTube's measure of each, so none jumps out. Quiet recordings keep their level."
		if s.set.SpotifyPlayback == settings.SpotifyPlaybackConnect || s.set.SpotifyPlayback == settings.SpotifyPlaybackHere && spotifyPlaysHere {
			even += " Spotify plays its own tracks at its own loudness."
		}
		s.row(c, "Even loudness", even, func() {
			if s.switchOf(c.Key("loudness"), &s.set.Loudness).Label("Even loudness").Changed() {
				s.save()
			}
		})
		s.row(c, "Volume levelling", "Squeezes loud and quiet moments closer together as they play, and lifts quiet recordings.", func() {
			if s.switchOf(c.Key("normalize"), &s.set.Normalize).Label("Volume levelling").Changed() {
				s.save()
			}
		})
		s.row(c, "Autoplay", "When the queue runs out, a radio of what you played carries on.", func() {
			if s.switchOf(c.Key("autoplay"), &s.set.Autoplay).Label("Autoplay").Changed() {
				s.save()
			}
		})
	})
	s.equalizerSettings(c)
	s.cacheSettings(c)
	s.toolSettings(c)
}

// deviceRow chooses the Spotify Connect device that plays Spotify tracks.
func (s *shell) deviceRow(c *ui.Context) {
	s.row(c, "Device", "Where Spotify tracks play. Open Spotify on a device for it to show here.", func() {
		devices := fetch(s, "devices", s.svc.Player.Devices)
		labels, ids := []string{"The active device"}, []string{""}
		for _, d := range devices.value {
			labels, ids = append(labels, d.Name), append(ids, d.ID)
		}
		chosen := "a device not found now"
		if i := slices.Index(ids, s.set.SpotifyDevice); i >= 0 {
			chosen = labels[i]
		}
		if s.selectOf(c.Key("device"), &chosen, labels).Width(220).Label("Device").Changed() {
			if i := slices.Index(labels, chosen); i >= 0 {
				s.setDevice(ids[i])
			}
		}
		if s.iconButton(c, "devices-again", "refresh", "Look for devices again", false, 16).Clicked() {
			s.forget("devices")
		}
	})
}

func (s *shell) equalizerSettings(c *ui.Context) {
	k := &s.lk
	eq := &s.set.Equalizer
	s.group(c, "equalizer", func() {
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
		})
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
		// Spotify's audio plays in Spotify's own player, which no app may
		// change: only the system's equalizer reaches it.
		ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Padding(12, 0).MaxWidth(720).Children(func() {
			ui.Text(c, "The equalizer works only on YouTube Music, which MixMusic plays itself, Spotify tracks played from it included. For Spotify, use a system-wide equalizer: ")
			s.linkOf(c, "Spotify notes", spotifyEqualizer)
			ui.Text(c, " that its audio settings can't change while another device plays it.")
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

func (s *shell) aboutSettings(c *ui.Context) {
	k := &s.lk
	info := s.svc.System.Info()
	u := s.svc.System.Updates()
	s.group(c, "mixmusic", func() {
		version := "MixMusic " + info.Version + " · " + info.Platform + " " + info.Arch
		if build, ok := buildNames[u.Kind]; ok {
			version += " · " + build
		}
		s.row(c, "Version", version, func() {})
		s.releaseRow(c, u)
		// Its links go under it, as they are many.
		ui.Column(c).Gap(4).Padding(14, 0).Children(func() {
			ui.Text(c, "MixMusic").FontSize(k.bodySize())
			ui.Text(c, "Spotify and YouTube Music in one player, open source under the MIT license or the GNU GPL, version 3. Your data lives in "+info.DataDir+".").
				FontSize(k.smallSize()).TextColor(k.muted)
			ui.Row(c).Gap(8).Wrap().Margin(12, 0, 0).Children(func() {
				const repo = "https://github.com/fzn0x/mixmusic"
				if s.button(c, "source", "source code", false).Clicked() {
					go s.svc.System.OpenExternal(repo)
				}
				if s.button(c, "contribute", "contribute", false).Clicked() {
					go s.svc.System.OpenExternal(repo + "/blob/main/CONTRIBUTING.md")
				}
				if s.button(c, "report", "report a problem", false).Clicked() {
					go func() { _ = s.svc.System.OpenExternal(s.svc.System.ReportURL(context.Background())) }()
				}
				if s.button(c, "logs", "open logs folder", false).Clicked() {
					if err := s.svc.System.OpenLogsFolder(); err != nil {
						s.notes = append(s.notes, noticeOf(err))
					}
				}
			})
		})
	})
	s.updateSettings(c)
	s.group(c, "licenses", func() {
		for i, n := range append(s.svc.System.Notices(), fontNotices()...) {
			open := s.licenseOpen == i
			s.row(c, n.Name, "", func() {
				label := "show"
				if open {
					label = "hide"
				}
				if s.button(c, "license-"+count(i), label, false).Clicked() {
					if open {
						s.licenseOpen = -1
					} else {
						s.licenseOpen = i
					}
				}
			})
			if open {
				ui.Text(c, n.Text).FontSize(k.captionSize()).Font(monospace()).TextColor(k.muted).Padding(8, 0).Selectable()
			}
		}
	})
}
