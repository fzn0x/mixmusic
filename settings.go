package main

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/addons"
)

// settingsTabs are the settings' sections, by their paths' last part.
var settingsTabs = []string{"appearance", "playback", "accounts", "plugins", "desktop", "shortcuts", "about"}

// settingsPage shows the settings, by section, or, searched for q, what
// every section holds of it.
func (s *shell) settingsPage(c *ui.Context, tab, q string) {
	f := &s.find
	// The page the router shows takes the search its location holds, as a
	// link, Back or the palette set it. The one it leaves, which shows
	// beside it a moment, shows its own.
	current := s.pagePath == s.router.Path() && q == s.router.Query("q")
	if current && q != f.route {
		f.query, f.route, f.typedAt = q, q, c.Now()
	}
	selected := max(slices.Index(settingsTabs, tab), 0)
	s.pageScroll(c, func() {
		f.begin(q, current)
		defer f.end()
		// While the user searches, no section is chosen; choosing one
		// leaves the search.
		pick := selected
		if f.on() {
			pick = -1
		}
		before := pick
		s.pageHeader(c, "Settings", settingsTabs, &pick)
		if pick != before {
			s.router.Replace(settingsPath(settingsTabs[pick], ""))
		}
		s.findBox(c, tab, q, current)
		ui.Column(c).MaxWidth(880).Children(func() {
			s.settingsFileProblem(c)
			if !f.on() {
				s.settingsSection(c, settingsTabs[selected])
				if current {
					f.reveal = "" // a group the section doesn't show
				}
				return
			}
			for _, t := range settingsTabs {
				f.section = t
				s.settingsSection(c, t)
			}
			f.section = ""
			if f.found == 0 {
				s.emptyLine(c, "No setting matches “"+strings.TrimSpace(q)+"”.")
			}
		})
		if current {
			s.findTell(c)
		}
	})
}

// settingsSection builds the section of the settings tab names.
func (s *shell) settingsSection(c *ui.Context, tab string) {
	switch tab {
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
}

// save keeps what the controls changed of the settings: only that, so
// that an edit of settings.json that applied since refresh read them, which
// the window doesn't show until its refresh runs, stays.
func (s *shell) save() {
	next := s.set.Clean()
	if _, err := s.svc.Preferences.Change(s.base, next); err != nil {
		s.notes = append(s.notes, noticeOf(err))
		return
	}
	s.base = next
}

// group titles a set of rows.
func (s *shell) group(c *ui.Context, title string, rows func()) { s.groupWith(c, title, nil, rows) }

// groupWith titles a set of rows, with links to what relates to them.
// While the user searches the settings, it shows what the search finds of
// them (findGroup).
func (s *shell) groupWith(c *ui.Context, title string, links, rows func()) {
	k := &s.lk
	f := &s.find
	if f.on() {
		s.findGroup(c, title, links, rows)
		return
	}
	g := ui.Column(c).Margin(0, 0, k.sections())
	// The group a search's link showed in its section comes into view.
	if f.reveal != "" && f.reveal == title && f.current {
		f.reveal = ""
		g.ScrollIntoView()
	}
	g.Children(func() {
		s.sectionHead(c, title, links)
		group := ui.Column(c).Dividers(1, k.border)
		if k.cards {
			group.Background(k.surface).Radius(k.radiusSm+k.cardPad()).Padding(4, 2*k.cardPad())
		}
		group.Children(rows)
	})
}

// row is a setting: its title and description, and its control. The
// search of the settings finds it by them, and by words.
func (s *shell) row(c *ui.Context, title, description string, control func(), words ...string) {
	if !s.find.on() {
		s.rowOf(c, title, description, control)
		return
	}
	if s.findable(title, description, words...) {
		s.show(c, func() { s.rowOf(c, title, description, control) })
	}
}

// rowOf builds a setting's row.
func (s *shell) rowOf(c *ui.Context, title, description string, control func()) {
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

// rowWith is a row whose description describe builds, with links say.
// The search of the settings finds it by its title, and by words, which
// say what describe does.
func (s *shell) rowWith(c *ui.Context, title string, describe, control func(), words ...string) {
	if !s.findable(title, "", words...) {
		return
	}
	k := &s.lk
	s.show(c, func() {
		ui.Row(c).Gap(24).Padding(14, 0).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				ui.Text(c, title).FontSize(k.bodySize())
				ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Children(describe)
			})
			ui.Row(c).Shrink(0).Gap(8).Children(control)
		})
	})
}

// hint is a muted paragraph among a group's rows.
func (s *shell) hint(c *ui.Context, text string) {
	s.found(c, "", text, func() {
		ui.Text(c, text).FontSize(s.lk.smallSize()).TextColor(s.lk.muted).Padding(12, 0).MaxWidth(720)
	})
}

// problems tells of the add-ons of a kind that didn't load.
func (s *shell) problems(c *ui.Context, kind addons.Kind) {
	what := "theme"
	if kind == addons.KindPlugin {
		what = "plugin"
	}
	for _, p := range s.inv.Problems {
		if p.Kind == kind {
			text := "The " + what + " in “" + p.Folder + "” did not load: " + p.Error
			s.found(c, "", text, func() {
				ui.Text(c, text).FontSize(s.lk.smallSize()).TextColor(s.lk.danger).Padding(8, 0)
			})
		}
	}
}

// openFolder opens the folder of the add-ons of a kind.
func (s *shell) openFolder(kind addons.Kind) {
	if err := s.svc.Addons.OpenFolder(kind); err != nil {
		s.notes = append(s.notes, noticeOf(err))
	}
}
