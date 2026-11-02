package main

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/addons"
	"github.com/fzn0x/mixmusic/internal/colorfile"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// layoutChoices are the layout designs, in the order of settings.Layouts,
// as Settings › Appearance offers them.
var layoutChoices = []choice{
	{"Blend", "Expressive's rounded shapes and springy motion, on Crate's dense rows, big covers in cards and bold headings. The default.", false},
	{"Expressive", "Playful and colourful: tonal fills, big rounded shapes, pill buttons, a floating player bar and springy motion. The Material themes go with it, their colours made of the cover's.", false},
	{"Crate", "Restrained: dense rows, big covers in cards and bold, tight headings, so that the covers do the work.", false},
	{"Tiles", "Flat tiles, giant light lowercase words and outlined boxes, as MixMusic first was.", false},
}

func (s *shell) appearanceSettings(c *ui.Context) {
	k := &s.lk
	s.group(c, "Layout", func() {
		layout := slices.Index(settings.Layouts, s.set.Layout)
		options := layoutChoices
		if k.headings == headingsLight {
			// Tiles' own words are lowercase, as its other choices.
			options = s.lowLayoutChoices[:0]
			for _, o := range layoutChoices {
				o.title = k.words(o.title)
				options = append(options, o)
			}
			s.lowLayoutChoices = options
		}
		if s.choices(c, "layout", "Layout", &layout, options) {
			s.set.Layout = settings.Layouts[layout]
			s.save()
		}
	})
	s.groupWith(c, "Theme", func() {
		if s.link(c, "more-themes", "get more themes") {
			s.navigate("/marketplace/themes")
		}
		if s.link(c, "themes-folder", "open themes folder") {
			s.openFolder(addons.KindTheme)
		}
	}, func() {
		follow := "Show one theme while the computer is in light mode and another in dark mode. The theme you pick below is the one for the mode it is in now."
		if s.oneMode {
			follow = "Show one theme in light mode and another in dark mode. Your colour file's colours are light or dark, so the theme of their mode shows."
		}
		s.row(c, "Follow the computer", follow, func() {
			if s.switchOf(c.Key("follow"), &s.set.FollowSystem).Label("Follow the computer").Changed() {
				s.save()
			}
		})
		if s.set.FollowSystem {
			s.row(c, "In light mode", "", func() { s.themeSelect(c, "light-theme", "Theme in light mode", &s.set.LightTheme) })
			s.row(c, "In dark mode", "", func() { s.themeSelect(c, "dark-theme", "Theme in dark mode", &s.set.DarkTheme) })
		}
		if !s.wornColors().Empty() {
			s.hint(c, "Your colour file sets the colours. The theme sets the type, corners and effects.")
		}
		// The search finds a theme's card by the theme's name, who made it,
		// and whether it is light or dark.
		cards := s.inv.Themes
		if s.find.sifting() {
			cards = nil
			for _, t := range s.inv.Themes {
				if s.findable(t.Name, themeBy(t)+" "+t.Scheme) {
					cards = append(cards, t)
				}
			}
		}
		if len(cards) > 0 {
			s.show(c, func() {
				ui.Row(c).Wrap().Gap(10).Padding(14, 0).Children(func() {
					active := s.set.ActiveTheme(k.systemDark)
					for _, t := range cards {
						s.themeCard(c, t, active == t.ID)
					}
				})
			})
		}
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
	s.group(c, "Colour and type", func() {
		s.colorFileSettings(c)
		artwork, accent := "Take the accent colour from the artwork of what plays.", "Replaces the theme's accent when the artwork does not set it."
		if s.set.ColorSource != settings.ColorsTheme {
			artwork = "Take the accent colour from the artwork of what plays, over your colour file's."
			accent = "Replaces your colour file's accent when the artwork does not set it."
		}
		s.row(c, "Follow the artwork", artwork, func() {
			if s.switchOf(c.Key("dynamic-accent"), &s.set.DynamicAccent).Label("Follow the artwork").Changed() {
				s.save()
			}
		})
		s.row(c, "Accent colour", accent, func() { s.accentControl(c) })
		s.zoomRow(c)
		s.row(c, "Text size", fmt.Sprintf("%d%%", int(s.set.FontScale*100+0.5)), func() {
			if ui.StepSlider(c.Key("font-scale"), &s.set.FontScale, 0.85, 1.3, 0.05).Width(200).Label("Text size").Changed() {
				s.save()
			}
		}, "font")
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
		}, "comfortable spacing")
		s.row(c, "Icons only in the sidebar", "The sidebar shows its pages as icons. It does in narrow windows too, until you unfold it there.", func() {
			if s.switchOf(c.Key("collapsed"), &s.set.SidebarCollapsed).Label("Icons only in the sidebar").Changed() {
				s.save()
			}
		}, "collapse")
	})
}

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
	if k.cards {
		card.Radius(k.radiusSm)
	}
	if current {
		card.Border(3, k.accent)
	}
	if card.Clicked() {
		s.chooseTheme(id)
	}
	card.Children(func() {
		ui.Text(c, name).FontSize(k.size(1.5)).FontWeight(tk.titleWeight()).Font(tk.display).TextColor(tk.text).SingleLine()
		ui.Row(c).Gap(4).Children(func() {
			for i := range 4 {
				ui.Box(c).Size(22, 22).Background(tk.tile(i))
			}
		})
		ui.Text(c, themeBy(t)).FontSize(k.captionSize()).TextColor(tk.muted).SingleLine()
	})
}

// themeBy tells who made a theme: "built in", or "by" its author.
func themeBy(t addons.Theme) string {
	if t.Builtin {
		return "built in"
	}
	return "by " + t.Author
}

// cardLook is the look of theme id's card, made once for the themes
// installed: a look lists the system's fonts, which takes a while, and the
// cards show every theme in every frame. A card's look is the theme's own,
// in the layout design chosen, whatever the other settings and the
// computer's light or dark.
func (s *shell) cardLook(id string) *look {
	themes := s.inv.Themes
	if s.cardLooks == nil || len(themes) != len(s.cardThemes) || len(themes) > 0 && &themes[0] != &s.cardThemes[0] ||
		s.cardLayout != s.set.Layout {
		s.cardLooks, s.cardThemes, s.cardLayout = map[string]*look{}, themes, s.set.Layout
	}
	tk, ok := s.cardLooks[id]
	if !ok {
		l := newLook(settings.Settings{Theme: id, FontScale: 1, Layout: s.set.Layout}, themes, false, "", colorfile.Colors{})
		tk = &l
		s.cardLooks[id] = tk
	}
	return tk
}

// accentControl chooses the accent colour.
func (s *shell) accentControl(c *ui.Context) {
	s.colorControl(c, "accent", "Accent colour", s.chosenAccent(), func(hex string) { s.set.Accent = hex })
	whose := "use the theme's"
	if !s.wornColors().Empty() {
		whose = "use the file's"
	}
	if s.set.Accent != "" && s.link(c, "accent-theme", whose) {
		s.set.Accent = ""
		s.wells["accent"].edits = 0
		s.save()
	}
}

// colorSaveAfter is how long a colour waits, once the user stops changing
// it, to be saved.
var colorSaveAfter = 500 * time.Millisecond

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

// chosenAccent is the accent the user chose, or the active theme's, or
// the colour file's.
func (s *shell) chosenAccent() ui.Color {
	if a, ok := parseColor(s.set.Accent); ok && s.set.Accent != "" {
		return col(a)
	}
	return s.lk.themeAccent
}

// wornColors are the colour file's colours the look wears, none while the
// theme's show.
func (s *shell) wornColors() colorfile.Colors {
	if s.set.ColorSource == settings.ColorsTheme {
		return colorfile.Colors{}
	}
	return s.colorFile.Colors
}
