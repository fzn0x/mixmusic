package main

import (
	"context"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/autoeq"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// The correction of headphones (#616), in Settings › Playback › Equalizer:
// the Headphones row, the dialog of AutoEq's profiles it opens, and Import
// file…, which reads a ParametricEQ.txt.

// autoEqPage is AutoEq's page on GitHub, which the dialog credits.
const autoEqPage = "https://github.com/jaakkopasanen/AutoEq"

// headphonesRowHeight is the height of a row of the dialog's list.
const headphonesRowHeight = 40

// headphonesPicker is the dialog of AutoEq's profiles.
type headphonesPicker struct {
	open  bool
	query string
	// loading is set while the list loads. list is it, once it did, with
	// hay, the words each of its profiles is found by; offline is why it
	// couldn't be checked again, and failed why it didn't load at all.
	loading         bool
	list            []autoeq.Entry
	hay             []string
	offline, failed error
	// getting is the folder of the profile that downloads, if any.
	getting string
	// rows are the rows of the list that the query found, which was
	// foundFor of the list as it was.
	rows     []headphonesItem
	foundFor string
	built    bool
	count    int
}

// headphonesItem is a row of the dialog's list: None, a model whose
// profiles follow it, or a profile, which names its model when it is its
// only one.
type headphonesItem struct {
	none   bool
	model  string
	entry  *autoeq.Entry
	single bool
}

// headphonesDesc is what the row of Headphones says of h.
func headphonesDesc(h settings.Headphones) string {
	switch {
	case !h.On():
		return locales.EqualizerHeadphonesDesc()
	case h.ID == settings.HeadphonesFile:
		return locales.EqualizerHeadphonesFile(h.Name)
	case h.Rig != "":
		return locales.EqualizerHeadphonesProfileRig(h.Name, h.Source, h.Rig)
	}
	return locales.EqualizerHeadphonesProfile(h.Name, h.Source)
}

// headphonesRow tells what corrects the sound of the user's headphones,
// with Choose…, which opens AutoEq's list, and Import file….
func (s *shell) headphonesRow(c *ui.Context) {
	k := &s.lk
	h := s.set.Equalizer.Headphones
	desc := headphonesDesc(h)
	if !s.findable(locales.EqualizerHeadphones(), desc, locales.EqualizerHeadphonesImport(), "AutoEq ParametricEQ.txt") {
		return
	}
	s.show(c, func() {
		ui.Row(c).Gap(24).Padding(14, 0).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				ui.Text(c, locales.EqualizerHeadphones()).FontSize(k.bodySize())
				ui.Text(c, desc).FontSize(k.smallSize()).TextColor(k.muted)
				ui.Row(c).Gap(16).Wrap().Padding(4, 0, 0).Children(func() {
					if s.link(c, "eq-headphones-import", locales.EqualizerHeadphonesImport()) {
						s.importHeadphones()
					}
				})
			})
			ui.Row(c).Shrink(0).Gap(8).Children(func() {
				if s.button(c, "eq-headphones", locales.EqualizerHeadphonesChoose(), false).Clicked() {
					s.openHeadphones()
				}
			})
		})
	})
}

// openHeadphones opens the dialog of AutoEq's profiles, and loads their
// list: the one kept, which is checked again once a week, or GitHub's.
func (s *shell) openHeadphones() {
	hp := &s.headphones
	hp.open, hp.query = true, ""
	if !hp.loading {
		s.loadHeadphones()
	}
}

// loadHeadphones loads AutoEq's list, beside the main thread.
func (s *shell) loadHeadphones() {
	hp := &s.headphones
	hp.loading, hp.failed = true, nil
	go func() {
		list, err := s.svc.Preferences.HeadphonesIndex(context.Background())
		s.update(func() {
			hp.loading = false
			if err != nil {
				hp.failed = err
				return
			}
			hp.offline = list.Offline
			if len(list.Entries) != len(hp.list) || len(hp.list) > 0 && &list.Entries[0] != &hp.list[0] {
				hp.list, hp.built = list.Entries, false
				hp.hay = make([]string, len(hp.list))
				for i, e := range hp.list {
					hp.hay[i] = strings.ToLower(e.Name + " " + e.Source + " " + e.Rig + " " + e.Form)
				}
			}
		})
	}()
}

// headphonesFound makes the rows of the list the query finds: None, then
// each model with a profile that holds every word typed, by its name,
// its source, its rig or its form, and those profiles.
func (hp *headphonesPicker) headphonesFound() {
	if hp.built && hp.foundFor == hp.query {
		return
	}
	hp.built, hp.foundFor = true, hp.query
	words := strings.Fields(strings.ToLower(hp.query))
	hp.rows = append(hp.rows[:0], headphonesItem{none: true})
	hp.count = 0
	for i := 0; i < len(hp.list); {
		j := i
		for j < len(hp.list) && hp.list[j].Name == hp.list[i].Name {
			j++
		}
		var found []int
		for n := i; n < j; n++ {
			all := true
			for _, w := range words {
				all = all && strings.Contains(hp.hay[n], w)
			}
			if all {
				found = append(found, n)
			}
		}
		switch {
		case len(found) == 1:
			hp.rows = append(hp.rows, headphonesItem{entry: &hp.list[found[0]], single: true})
		case len(found) > 1:
			hp.rows = append(hp.rows, headphonesItem{model: hp.list[i].Name})
			for _, n := range found {
				hp.rows = append(hp.rows, headphonesItem{entry: &hp.list[n]})
			}
		}
		hp.count += len(found)
		i = j
	}
}

// headphonesByOf tells who measured e, and on what rig.
func headphonesByOf(e autoeq.Entry) string {
	if e.Rig != "" {
		return locales.EqualizerHeadphonesByOn(e.Source, e.Rig)
	}
	return locales.EqualizerHeadphonesBy(e.Source)
}

// headphonesDialog shows AutoEq's profiles while it is open: a search, the
// list, None first, then each model and its profiles, by who measured
// them and on what rig, and AutoEq's credit. Picking a profile downloads
// it, once, and has it correct the sound; one that fails says why, and
// the correction stays as it was.
func (s *shell) headphonesDialog(c *ui.Context) {
	hp := &s.headphones
	if !hp.open {
		return
	}
	k := &s.lk
	s.dialogFrame(c, &hp.open, dialogOptions{key: "headphones", title: locales.EqualizerHeadphones(), width: 640}, func(panel ui.Element) {
		panel.Padding(24).Gap(12)
		ui.Text(c, locales.EqualizerHeadphones()).FontSize(k.subtitleSize()).FontWeight(k.leadWeight()).Shrink(0)
		ui.TextInput(c.Key("headphones-search"), &hp.query).Placeholder(k.sentence(locales.EqualizerHeadphonesSearch())).
			AutoFocus().Label(locales.EqualizerHeadphonesSearch()).Shrink(0)
		status := func(text string) {
			ui.Text(c, text).FontSize(k.smallSize()).TextColor(k.muted).Shrink(0)
		}
		hp.headphonesFound()
		switch {
		case hp.loading && hp.list == nil:
			status(locales.EqualizerHeadphonesLoading())
		case hp.failed != nil && hp.list == nil:
			ui.Row(c).Gap(12).AlignItems(ui.Center).Shrink(0).Children(func() {
				ui.Text(c, locales.EqualizerHeadphonesListFailed(errorText(hp.failed))).FontSize(k.smallSize()).TextColor(k.danger).Grow(1).MinWidth(0)
				if s.dialogButton(c, "headphones-again", locales.EqualizerHeadphonesTryAgain(), false).Clicked() {
					s.loadHeadphones()
				}
			})
		default:
			if hp.offline != nil {
				status(locales.EqualizerHeadphonesOffline(errorText(hp.offline)))
			}
			switch {
			case hp.list == nil:
			case hp.count == 0:
				status(locales.EqualizerHeadphonesNoMatch())
			default:
				status(locales.EqualizerHeadphonesFound(hp.count))
			}
		}
		// The list gives up room in a window too short for it, and
		// scrolls; it builds the rows near the view alone.
		ui.Column(c).MaxHeight(420).MinHeight(0).Children(func() {
			ui.Scroll(c.Key("headphones-scroll")).MaxHeight(420).Children(func() {
				list := ui.Column(c.Key("headphones-list"))
				from, to := s.nearRows(c, "headphones-list", list, len(hp.rows), headphonesRowHeight)
				list.Children(func() {
					if from > 0 {
						ui.Box(c).Height(float32(from) * headphonesRowHeight)
					}
					for i := from; i < to; i++ {
						s.headphonesItemRow(c, hp.rows[i])
					}
					if to < len(hp.rows) {
						ui.Box(c).Height(float32(len(hp.rows)-to) * headphonesRowHeight)
					}
				})
			})
		})
		ui.RichText(c).FontSize(k.captionSize()).TextColor(k.muted).Shrink(0).Children(func() {
			inlineMessage(c, locales.EqualizerHeadphonesCredit(mark(0, locales.EqualizerHeadphonesAutoEq())), func(text string) {
				s.linkOf(c, text, autoEqPage)
			})
		})
		ui.Row(c).Justify(ui.End).Shrink(0).Children(func() {
			if s.dialogButton(c, "headphones-close", locales.EqualizerHeadphonesClose(), false).Clicked() {
				hp.open = false
			}
		})
	})
}

// headphonesItemRow is a row of the dialog's list: a model's name, which
// heads its profiles, or a button that plays None or a profile, flooded
// while it plays.
func (s *shell) headphonesItemRow(c *ui.Context, it headphonesItem) {
	k := &s.lk
	hp := &s.headphones
	h := s.set.Equalizer.Headphones
	if !it.none && it.entry == nil {
		ui.Row(c.Key("headphones-model-" + it.model)).Height(headphonesRowHeight).PaddingX(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, it.model).FontSize(k.bodySize()).FontWeight(k.leadWeight()).SingleLine().Grow(1).MinWidth(0)
		})
		return
	}
	key, name, by, chosen := "headphones-none", locales.EqualizerHeadphonesNone(), "", !h.On()
	if e := it.entry; e != nil {
		key, name, by, chosen = "headphones-"+e.Path, e.Name, headphonesByOf(*e), h.ID == e.Path
		if hp.getting == e.Path {
			by = locales.EqualizerHeadphonesGetting()
		}
		if !it.single {
			// A profile under its model's name says who measured it.
			name, by = by, ""
		}
	}
	row := s.buttonBase(c.Key(key)).Height(headphonesRowHeight).PaddingX(12).Gap(12).Label(name).
		Cursor(ui.CursorPointer).AlignItems(ui.Center).Radius(k.radiusSm)
	if by != "" {
		row.Description(by)
	}
	ink, sub := k.text, k.muted
	switch {
	case chosen:
		row.Background(k.selected)
		ink, sub = k.selectedText, k.selectedSub
	case row.Hovered():
		row.Background(k.hover)
	}
	if row.Clicked() {
		if it.entry == nil {
			s.set.Equalizer.Headphones = settings.Headphones{}
			s.save()
			hp.open = false
		} else {
			s.pickHeadphones(*it.entry)
		}
	}
	row.Children(func() {
		if !it.none && !it.single {
			ui.Box(c).Width(16).Shrink(0)
		}
		ui.Text(c, name).FontSize(k.bodySize()).TextColor(ink).SingleLine().Shrink(1).MinWidth(0)
		if by != "" {
			ui.Text(c, by).FontSize(k.smallSize()).TextColor(sub).SingleLine().Shrink(1).MinWidth(0)
		}
	})
}

// pickHeadphones downloads the profile e, beside the main thread, unless
// it is kept, and has it correct the sound, closing the dialog; one that
// fails says why, and the correction stays as it was.
func (s *shell) pickHeadphones(e autoeq.Entry) {
	hp := &s.headphones
	if hp.getting != "" {
		return
	}
	hp.getting = e.Path
	go func() {
		h, err := s.svc.Preferences.HeadphonesProfile(context.Background(), e)
		s.update(func() {
			hp.getting = ""
			if err != nil {
				s.notes = append(s.notes, player.Notice{Message: locales.EqualizerHeadphonesPickFailed(e.Name, errorText(err)), Error: true})
				return
			}
			s.set.Equalizer.Headphones = h
			s.save()
			hp.open = false
		})
	}()
}

// importHeadphones asks for a ParametricEQ.txt, beside the main thread,
// and has it correct the sound; one that doesn't read says why, and the
// correction stays as it was.
func (s *shell) importHeadphones() {
	win := s.win
	go func() {
		paths, err := openFileDialog(mygo.OpenDialogOptions{Parent: win, Title: locales.EqualizerHeadphonesImportTitle(),
			Filters: []mygo.FileFilter{{Name: locales.EqualizerHeadphonesFilter(), Extensions: []string{"txt"}}}})
		if err == nil && len(paths) == 0 {
			return
		}
		var h settings.Headphones
		if err == nil {
			h, err = s.svc.Preferences.ImportHeadphones(paths[0])
		}
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, player.Notice{Message: locales.EqualizerHeadphonesImportFailed(errorText(err)), Error: true})
				return
			}
			s.set.Equalizer.Headphones = h
			s.save()
		})
	}()
}
