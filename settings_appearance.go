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
	"github.com/fzn0x/mixmusic/locales"
)

// The groups of Settings › Appearance.

// layoutChoices are the layout designs, in the order of settings.Layouts,
// as Settings › Appearance offers them. A function, so that they show in the
// language chosen.
func layoutChoices() []choice {
	return []choice{
		{"Blend", locales.AppearanceLayoutBlendDescription(), false},
		{"Expressive", locales.AppearanceLayoutExpressiveDescription(), false},
		{"Crate", locales.AppearanceLayoutCrateDescription(), false},
		{"Tiles", locales.AppearanceLayoutTilesDescription(), false},
	}
}

func init() { addSettings("appearance", 10, (*shell).layoutSettings) }

// layoutSettings choose the layout design.
func (s *shell) layoutSettings(c *ui.Context) {
	k := &s.lk
	s.group(c, locales.AppearanceLayoutTitle(), func() {
		layout := slices.Index(settings.Layouts, s.set.Layout)
		options := layoutChoices()
		if k.headings == headingsLight {
			// Tiles' own words are lowercase, as its other choices.
			for i := range options {
				options[i].title = k.words(options[i].title)
			}
		}
		if s.choices(c, "layout", locales.AppearanceLayoutTitle(), &layout, options) {
			s.set.Layout = settings.Layouts[layout]
			s.save()
		}
	})
}

func init() { addSettings("appearance", 20, (*shell).themeSettings) }

// themeSettings choose the theme, or one for each mode of the computer.
func (s *shell) themeSettings(c *ui.Context) {
	k := &s.lk
	s.groupWith(c, locales.AppearanceThemeTitle(), func() {
		if s.link(c, "more-themes", locales.AppearanceGetMoreThemes()) {
			s.navigate("/marketplace/themes")
		}
		if s.link(c, "themes-folder", locales.AppearanceOpenThemesFolder()) {
			s.openFolder(addons.KindTheme)
		}
	}, func() {
		follow := locales.AppearanceFollowComputerDescription()
		if s.oneMode {
			follow = locales.AppearanceFollowComputerOneModeDescription()
		}
		s.row(c, locales.AppearanceFollowComputer(), follow, func() {
			if s.switchOf(c.Key("follow"), &s.set.FollowSystem).Label(locales.AppearanceFollowComputer()).Changed() {
				s.save()
			}
		})
		if s.set.FollowSystem {
			s.row(c, locales.AppearanceInLightMode(), "", func() {
				s.themeSelect(c, "light-theme", locales.AppearanceThemeInLightMode(), &s.set.LightTheme)
			})
			s.row(c, locales.AppearanceInDarkMode(), "", func() {
				s.themeSelect(c, "dark-theme", locales.AppearanceThemeInDarkMode(), &s.set.DarkTheme)
			})
		}
		if !s.wornColors().Empty() {
			s.hint(c, locales.AppearanceColourFileSetsColours())
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
}

func init() { addSettings("appearance", 30, (*shell).themeOptionSettings) }

// themeOptionSettings show the options of the built-in themes, whose
// effects the native interface draws itself.
func (s *shell) themeOptionSettings(c *ui.Context) {
	if th, ok := s.activeTheme(); ok && th.Builtin && len(th.Options) > 0 {
		s.group(c, locales.AppearanceThemeOptions(th.Name), func() {
			for _, o := range th.Options {
				s.themeOption(c, th, o)
			}
		})
	}
}

// colorTypeRows are the rows of Settings › Appearance › Colour and type,
// which register from their features' files.
const colorTypeRows = "appearance › Colour and type"

func init() { addSettings("appearance", 40, (*shell).colorTypeSettings) }

// colorTypeSettings show the rows of colour and type.
func (s *shell) colorTypeSettings(c *ui.Context) {
	s.group(c, locales.AppearanceColourAndType(), func() { s.settingsOf(c, colorTypeRows) })
}

func init() { addSettings(colorTypeRows, 20, (*shell).accentRows) }

// accentRows choose where the accent colour comes from.
func (s *shell) accentRows(c *ui.Context) {
	artwork, accent := locales.AppearanceArtworkAccent(), locales.AppearanceAccentFallback()
	if s.set.ColorSource != settings.ColorsTheme {
		artwork = locales.AppearanceArtworkAccentOverFile()
		accent = locales.AppearanceAccentFallbackOverFile()
	}
	s.row(c, locales.AppearanceFollowArtwork(), artwork, func() {
		if s.switchOf(c.Key("dynamic-accent"), &s.set.DynamicAccent).Label(locales.AppearanceFollowArtwork()).Changed() {
			s.save()
		}
	})
	s.row(c, locales.AppearanceAccentColour(), accent, func() { s.accentControl(c) })
}

func init() { addSettings(colorTypeRows, 40, (*shell).textSizeRow) }

// textSizeRow scales the text.
func (s *shell) textSizeRow(c *ui.Context) {
	s.row(c, locales.AppearanceTextSize(), fmt.Sprintf("%d%%", int(s.set.FontScale*100+0.5)), func() {
		if ui.StepSlider(c.Key("font-scale"), &s.set.FontScale, 0.85, 1.3, 0.05).Width(200).Label(locales.AppearanceTextSize()).Changed() {
			s.save()
		}
	}, "font")
}

func init() { addSettings(colorTypeRows, 50, (*shell).densityRow) }

// densityRow chooses how much room the interface gives each element.
func (s *shell) densityRow(c *ui.Context) {
	s.row(c, locales.AppearanceDensity(), locales.AppearanceDensityDescription(), func() {
		compact := 0
		if s.set.Density == settings.DensityCompact {
			compact = 1
		}
		if s.segmented(c, "density", locales.AppearanceDensity(), &compact, locales.AppearanceDensityComfortable(), locales.AppearanceDensityCompact()) {
			s.set.Density = settings.DensityComfortable
			if compact == 1 {
				s.set.Density = settings.DensityCompact
			}
			s.save()
		}
	}, "comfortable spacing")
}

func init() { addSettings(colorTypeRows, 60, (*shell).sidebarRow) }

// sidebarRow shows the sidebar as icons only.
func (s *shell) sidebarRow(c *ui.Context) {
	s.row(c, locales.AppearanceIconsOnlySidebar(), locales.AppearanceIconsOnlySidebarDescription(), func() {
		if s.switchOf(c.Key("collapsed"), &s.set.SidebarCollapsed).Label(locales.AppearanceIconsOnlySidebar()).Changed() {
			s.save()
		}
	}, "collapse")
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
		labels[i] = locales.AppearanceThemeChoice(t.Name, t.Scheme)
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
		return locales.AppearanceThemeBuiltIn()
	}
	return locales.AppearanceThemeBy(t.Author)
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
		l := newLook(settings.Settings{Appearance: settings.Appearance{Theme: id, FontScale: 1, Layout: s.set.Layout}}, themes, false, "", colorfile.Colors{})
		tk = &l
		s.cardLooks[id] = tk
	}
	return tk
}

// accentControl chooses the accent colour.
func (s *shell) accentControl(c *ui.Context) {
	s.colorControl(c, "accent", locales.AppearanceAccentColour(), s.chosenAccent(), func(hex string) { s.set.Accent = hex })
	whose := locales.AppearanceUseTheme()
	if !s.wornColors().Empty() {
		whose = locales.AppearanceUseFile()
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
