package main

// The review of an import (#613): how many songs were found, are to
// check, are in the library already and weren't found; the songs, each
// ticked to import or not, with what was found of it and a way to
// replace it by a search; the songs not found to save as a CSV file; and
// where the songs go.

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/export"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// importRowHeight is the height of a song's row in the review.
const importRowHeight = 60

// The songs the review shows, as its segments choose them.
const (
	importShowAll = iota
	importShowFound
	importShowCheck
	importShowMissing
)

// The places the songs of an import go, as the review's choices list
// them.
const (
	importToNew = iota
	importToMix
	importToLiked
)

// importReplace is the dialog that replaces what an import found of a
// song: the song, the search, and what it found.
type importReplace struct {
	open      bool
	list, row int
	title     string
	query     string
	searched  string
	results   []media.Track
	err       error
	searching bool
	seq       int
}

// importReview reviews what an import found, and imports the songs
// ticked.
func (s *shell) importReview(c *ui.Context, st app.ImportStatus) {
	k := &s.lk
	iv := &s.imports
	imp := s.importsOf()
	if !iv.known || iv.version != st.Version {
		iv.lists, iv.version, iv.known = imp.Lists(), st.Version, true
		iv.shown = nil
	}
	if iv.shown == nil {
		iv.shown = importShown(iv.lists, iv.show)
	}
	var picked []app.ImportList
	pending := 0
	for _, l := range iv.lists {
		if l.Picked {
			picked = append(picked, l)
			for _, r := range l.Rows {
				if r.State == app.ImportPending {
					pending++
				}
			}
		}
	}
	if st.Cut {
		ui.Text(c, locales.PlaylistImportCut(st.Total)).FontSize(k.bodySize()).TextColor(k.muted)
	}
	ui.Text(c, locales.PlaylistImportCounts(st.Counts.Found, st.Counts.Check, st.Counts.InLibrary, st.Counts.Missing)).
		FontSize(k.subtitleSize()).FontWeight(k.leadWeight())
	if pending > 0 {
		ui.Row(c).Gap(12).Wrap().AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.PlaylistImportPending(pending)).FontSize(k.bodySize())
			if s.button(c, "import-look", locales.PlaylistImportLookAgain(), false).Clicked() {
				imp.TryAgain()
			}
		})
	}
	s.importWhere(c, picked)
	ui.Row(c).Gap(8).Wrap().Children(func() {
		target, ok := s.importTarget(picked)
		go_ := s.button(c, "import-go", locales.PlaylistImportDo(st.Counts.Ticked), true).Disabled(st.Counts.Ticked == 0 || iv.busy || !ok)
		if go_.Clicked() && st.Counts.Ticked > 0 && !iv.busy && ok {
			s.finishImport(target)
		}
		if s.button(c, "import-tick-all", locales.PlaylistImportTickAll(), false).Clicked() {
			imp.TickAll(true)
		}
		if s.button(c, "import-untick-all", locales.PlaylistImportUntickAll(), false).Clicked() {
			imp.TickAll(false)
		}
		if st.Counts.Missing > 0 && s.button(c, "import-misses", locales.PlaylistImportSaveMisses(), false).Clicked() && len(picked) > 0 {
			s.saveImportMisses(picked[0].Name)
		}
		s.importCancel(c)
	})
	before := iv.show
	s.segmented(c, "import-show", locales.PlaylistImportShowLabel(), &iv.show, locales.PlaylistImportShowAll(),
		locales.PlaylistImportSure(), locales.PlaylistImportToCheck(), locales.PlaylistImportNotFound())
	if iv.show != before {
		iv.shown = importShown(iv.lists, iv.show)
	}
	several := len(picked) > 1
	list := ui.Column(c.Key("import-rows"))
	from, to := s.nearRows(c, "import-rows", list, len(iv.shown), importRowHeight)
	list.Children(func() {
		if from > 0 {
			ui.Box(c).Height(float32(from) * importRowHeight)
		}
		for i := from; i < to; i++ {
			s.importRow(c, iv.shown[i], several)
		}
		if to < len(iv.shown) {
			ui.Box(c).Height(float32(len(iv.shown)-to) * importRowHeight)
		}
	})
}

// importShown are the songs of the lists picked the segment show shows.
func importShown(lists []app.ImportList, show int) []importItem {
	out := []importItem{}
	for li, l := range lists {
		if !l.Picked {
			continue
		}
		for ri, r := range l.Rows {
			ok := true
			switch show {
			case importShowFound:
				ok = r.State == app.ImportSure
			case importShowCheck:
				ok = r.State == app.ImportCheck
			case importShowMissing:
				ok = r.State == app.ImportMissing || r.State == app.ImportPending
			}
			if ok {
				out = append(out, importItem{li, ri})
			}
		}
	}
	return out
}

// importWhere chooses where the songs go: a new mix, named as its list,
// or one of each list; a mix of the user's; or Liked songs.
func (s *shell) importWhere(c *ui.Context, picked []app.ImportList) {
	iv := &s.imports
	mixes := s.svc.Collection.Playlists()
	newTitle := locales.PlaylistImportToNewMix()
	if len(picked) > 1 {
		newTitle = locales.PlaylistImportToNewMixes()
	}
	mixAbout := locales.PlaylistImportToMixAbout()
	if len(mixes) == 0 {
		mixAbout = locales.PlaylistImportNoMixes()
	}
	s.choicesOf(c, "import-where", locales.PlaylistImportWhere(), &iv.where, []choice{
		{title: newTitle, description: locales.PlaylistImportToNewMixAbout()},
		{title: locales.PlaylistImportToMix(), description: mixAbout, disabled: len(mixes) == 0},
		{title: locales.PlaylistImportToLiked(), description: locales.PlaylistImportToLikedAbout()},
	})
	switch {
	case iv.where == importToNew && len(picked) == 1:
		if iv.named != picked[0].Source {
			iv.named, iv.name = picked[0].Source, picked[0].Name
		}
		ui.TextInput(c.Key("import-name"), &iv.name).Label(locales.RailName()).Width(360)
	case iv.where == importToMix && len(mixes) > 0:
		titles := importMixTitles(mixes)
		if !containsString(titles, iv.mix) {
			iv.mix = titles[0]
		}
		s.selectOf(c.Key("import-mix"), &iv.mix, titles).Width(360).Label(locales.PlaylistImportMix())
	}
}

// importMixTitles are the titles of the mixes, each told from the others
// of its title by its place among them.
func importMixTitles(mixes []media.Playlist) []string {
	out := make([]string, len(mixes))
	seen := map[string]int{}
	for i, p := range mixes {
		seen[p.Title]++
		out[i] = p.Title
		if n := seen[p.Title]; n > 1 {
			out[i] = p.Title + " (" + count(n) + ")"
		}
	}
	return out
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// importTarget is where the user chose the songs go, and whether that's
// a place they can go.
func (s *shell) importTarget(picked []app.ImportList) (app.ImportTarget, bool) {
	iv := &s.imports
	switch iv.where {
	case importToLiked:
		return app.ImportTarget{Liked: true}, true
	case importToMix:
		mixes := s.svc.Collection.Playlists()
		for i, t := range importMixTitles(mixes) {
			if t == iv.mix {
				return app.ImportTarget{Mix: mixes[i].ID}, true
			}
		}
		return app.ImportTarget{}, false
	}
	t := app.ImportTarget{}
	if len(picked) == 1 {
		t.Name = strings.TrimSpace(iv.name)
	}
	return t, true
}

// importRow is a song of the review: its tick, what its file names, what
// was found of it, and Replace….
func (s *shell) importRow(c *ui.Context, it importItem, several bool) {
	k := &s.lk
	iv := &s.imports
	l := iv.lists[it.list]
	r := l.Rows[it.row]
	key := "import-row-" + count(it.list) + "-" + count(it.row)
	named := importEntryLine(r)
	row := ui.Row(c.Key(key)).Height(importRowHeight).Gap(12).Padding(0, 8).AlignItems(ui.Center).Radius(k.radiusSm)
	if row.Hovered() {
		row.Background(k.hover)
	}
	row.Children(func() {
		on := r.Ticked
		box := s.pointerKeys(ui.Checkbox(c.Key(key+"-tick"), &on, "")).Label(locales.PlaylistImportTick(named)).Disabled(r.Track == nil)
		if box.Changed() {
			s.importsOf().SetTicked(it.list, it.row, on)
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, named).FontSize(k.bodySize()).SingleLine()
			ui.Row(c).Gap(6).MinWidth(0).AlignItems(ui.Center).Children(func() {
				verdict, ink := importVerdict(r), k.muted
				if r.State == app.ImportSure {
					ink = k.accent
				}
				ui.Text(c, verdict).FontSize(k.smallSize()).TextColor(ink).Shrink(0)
				if r.Track != nil {
					s.sourceMark(c, r.Track.Source, false)
					ui.Text(c, importTrackLine(*r.Track)).FontSize(k.smallSize()).TextColor(k.muted).SingleLine().MinWidth(0)
				}
				if several {
					ui.Text(c, l.Name).FontSize(k.smallSize()).TextColor(k.muted).SingleLine().Shrink(0)
				}
			})
		})
		if s.link(c, key+"-replace", locales.PlaylistImportReplace()) {
			s.openImportReplace(it.list, it.row, named, r)
		}
	})
}

// importEntryLine names a song as its file does: "Artist - Title", or its
// link or ID when it has no title.
func importEntryLine(r app.ImportRow) string {
	e := r.Entry
	switch {
	case e.Title != "":
		return e.String()
	case e.URL != "":
		return e.URL
	case e.VideoID != "":
		return e.VideoID
	case e.SpotifyID != "":
		return "spotify:track:" + e.SpotifyID
	}
	return e.Path
}

// importVerdict is what became of a song of an import, in words.
func importVerdict(r app.ImportRow) string {
	switch {
	case r.State == app.ImportSure && r.InLibrary:
		return locales.PlaylistImportInLibrary()
	case r.State == app.ImportSure:
		return locales.PlaylistImportSure()
	case r.State == app.ImportCheck:
		return locales.PlaylistImportToCheck()
	case r.State == app.ImportMissing:
		return locales.PlaylistImportNotFound()
	}
	return locales.PlaylistImportNotLookedFor()
}

// openImportReplace opens the dialog that replaces what was found of a
// song, searching for it at once by its title and artists.
func (s *shell) openImportReplace(list, row int, named string, r app.ImportRow) {
	q := strings.TrimSpace(r.Entry.Title + " " + strings.Join(r.Entry.Artists, " "))
	if q == "" {
		q = named
	}
	s.imports.replace = importReplace{open: true, list: list, row: row, title: named, query: q, seq: s.imports.replace.seq}
	s.searchImportReplace()
}

// searchImportReplace searches what the dialog's box holds, beside the
// main thread.
func (s *shell) searchImportReplace() {
	rp := &s.imports.replace
	rp.seq++
	rp.searching = true
	seq, q := rp.seq, strings.TrimSpace(rp.query)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res, err := s.importsOf().Search(ctx, q)
		s.update(func() {
			rp := &s.imports.replace
			if rp.seq != seq {
				return
			}
			rp.results, rp.err, rp.searching, rp.searched = res, err, false, q
		})
	}()
}

// importReplaceDialog is the dialog that replaces what was found of a
// song: a search, and the tracks it found, one of which replaces it.
func (s *shell) importReplaceDialog(c *ui.Context) {
	k := &s.lk
	rp := &s.imports.replace
	if !rp.open {
		return
	}
	s.dialog(c, &rp.open, dialogOptions{key: "import-replace", title: locales.PlaylistImportReplaceTitle(rp.title), width: 560}, func() {
		in := ui.TextInput(c.Key("import-replace-query"), &rp.query).AutoFocus().Label(locales.RailSearch()).Width(480)
		if in.Submitted() && strings.TrimSpace(rp.query) != "" {
			s.searchImportReplace()
		}
		switch {
		case rp.searching:
			s.loading(c)
		case rp.err != nil:
			ui.Text(c, errorText(rp.err)).FontSize(k.bodySize()).TextColor(k.danger)
		case len(rp.results) == 0 && rp.searched != "":
			s.emptyLine(c, locales.PlaylistImportReplaceNone(rp.searched))
		}
		ui.Column(c).Gap(2).Children(func() {
			for i, t := range rp.results {
				b := s.buttonBase(c.Key("import-replace-"+count(i))).Padding(8, 10).Radius(k.radiusSm).Label(importTrackLine(t)).
					Cursor(ui.CursorPointer)
				if b.Hovered() {
					b.Background(k.hover)
				}
				if b.Clicked() {
					s.importsOf().Replace(rp.list, rp.row, t)
					rp.open = false
				}
				b.Children(func() {
					ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
						s.cover(c, "import-replace-cover-"+count(i), t.Artwork, 40, false)
						s.sourceMark(c, t.Source, false)
						ui.Text(c, importTrackLine(t)).FontSize(k.bodySize()).SingleLine().MinWidth(0)
					})
				})
			}
		})
	}, func() {
		if s.dialogButton(c, "import-replace-cancel", locales.LibraryCancel(), false).Clicked() {
			rp.open = false
		}
	})
}

// saveImportMisses asks where to save the songs an import didn't find,
// beside the main thread, and saves them there as CSV.
func (s *shell) saveImportMisses(name string) {
	win := s.win
	go func() {
		path, err := saveFileDialog(mygo.SaveDialogOptions{Parent: win, Title: locales.PlaylistImportMissesDialog(),
			DefaultPath: export.Name(nil, locales.PlaylistImportMissesFile(name)) + ".csv", CreateDirectories: true,
			Filters: []mygo.FileFilter{{Name: locales.PlaylistExportCsv(), Extensions: []string{"csv"}}}})
		if err == nil && path == "" {
			return
		}
		if err == nil {
			if !strings.EqualFold(filepath.Ext(path), ".csv") {
				path += ".csv"
			}
			err = s.importsOf().SaveMisses(path)
		}
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.actionToasts = append(s.actionToasts, actionToast{locales.PlaylistImportMissesSaved(filepath.Base(path)),
				locales.ExportShow(), func() { showInFolder(path) }})
		})
	}()
}
