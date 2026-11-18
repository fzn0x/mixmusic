package main

// Importing playlists (#613): Import a playlist… opens its page, from its
// tile beside New mix in Library › Mixes, from the command palette, and
// as playlist files are dropped on the window, which today play only
// music. The page follows the import's steps as internal/app's Imports
// keeps them: a table's columns to name, the lists to pick, the songs
// looked for, then what was found to review (playlistimport_review.go),
// and where the songs go.

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/playlistio"
	"github.com/fzn0x/mixmusic/locales"
)

// importPagePath is the page that imports playlists.
const importPagePath = "/import"

// importHelp is where the docs tell how to download the data of Spotify
// and Google.
const importHelp = "https://fzn0x.github.io/mixmusic/docs/import-export/#data-downloads"

// importExtensions are those of the files a drop imports, rather than
// plays.
var importExtensions = map[string]bool{".csv": true, ".tsv": true, ".m3u": true, ".m3u8": true, ".json": true,
	".txt": true, ".zip": true}

// importView is what the page that imports playlists keeps between
// frames.
type importView struct {
	watching bool
	pasted   string
	// table is the table whose columns the user names, fields what they
	// chose each holds, and header whether its first row names them.
	table  *playlistio.Table
	fields []string
	header bool
	// picks are the lists the user picks.
	picks []bool
	// The review: the lists as of version, the rows shown, what shows,
	// where the songs go, the mix chosen, the new mix's name.
	lists   []app.ImportList
	version uint64
	known   bool
	shown   []importItem
	show    int
	where   int
	mix     string
	name    string
	named   string
	busy    bool
	replace importReplace
}

// importItem is a song of the review: its list and its row.
type importItem struct{ list, row int }

// imports is the imports of playlists.
func (s *shell) importsOf() *app.Imports { return s.svc.Collection.Imports() }

// openImport shows the page that imports playlists, unless it shows.
func (s *shell) openImport() {
	if s.router.Path() != importPagePath {
		s.navigate(importPagePath)
	}
}

// importTile opens the page that imports playlists, as New mix's tile,
// beside it, makes a mix.
func (s *shell) importTile(c *ui.Context) {
	k := &s.lk
	about := k.sentence(locales.PlaylistImportTileAbout())
	t := s.buttonBase(c.Key("import-tile")).Width(k.tileWidth()).Shrink(0).Column().AlignItems(ui.Start).
		Label(locales.PlaylistImportTitle()).Cursor(ui.CursorPointer)
	if t.Clicked() {
		s.openImport()
	}
	if !k.cards {
		t.Height(k.tileWidth()).Justify(ui.SpaceBetween).Padding(16).Background(k.tile(3)).Radius(k.radius).Children(func() {
			ui.Text(c, k.words(locales.PlaylistImportTitle())).FontSize(k.size(1.5)).FontWeight(k.titleWeight()).Font(k.display).TextColor(k.tileText)
			ui.Text(c, about).FontSize(k.smallSize()).TextColor(k.tileText)
		})
		return
	}
	fill := k.surface
	if t.Hovered() {
		fill = k.surface2
	}
	pad := k.cardPad()
	t.Gap(8).Padding(pad, pad, pad+4).Radius(k.radiusSm + pad).Background(fill).Children(func() {
		ui.Box(c).Size(k.cover, k.cover).Radius(k.radiusSm).Background(k.tile(3)).Center().Children(func() {
			ui.Icon(c, icon("download")).FontSize(k.cover / 4).TextColor(k.tileText)
		})
		ui.Column(c).Width(k.cover).Gap(2).Children(func() {
			k.labelWeight(ui.Text(c, locales.PlaylistImportTitle()).FontSize(k.bodySize()).SingleLine())
			ui.Text(c, about).FontSize(k.smallSize()).TextColor(k.muted).SingleLine()
		})
	})
}

// dropPlaylists imports the playlist files among paths, dropped on the
// window, and returns the others, which play.
func (s *shell) dropPlaylists(paths []string) []string {
	var lists, rest []string
	for _, p := range paths {
		if importExtensions[strings.ToLower(filepath.Ext(p))] {
			lists = append(lists, p)
		} else {
			rest = append(rest, p)
		}
	}
	if len(lists) > 0 {
		s.update(func() { s.importFiles(lists) })
	}
	return rest
}

// chooseImportFiles asks for the files to import, beside the main thread,
// and imports them.
func (s *shell) chooseImportFiles() {
	win := s.win
	go func() {
		paths, err := openFileDialog(mygo.OpenDialogOptions{Parent: win, Title: locales.PlaylistImportFilesDialog(), Multiple: true,
			Filters: []mygo.FileFilter{
				{Name: locales.PlaylistImportFilesPlaylists(), Extensions: []string{"csv", "tsv", "m3u", "m3u8", "json", "txt", "zip"}},
				{Name: locales.PlaylistImportFilesAll(), Extensions: []string{"*"}},
			}})
		s.update(func() {
			switch {
			case err != nil:
				s.notes = append(s.notes, noticeOf(err))
			case len(paths) > 0:
				s.importFiles(paths)
			}
		})
	}()
}

// askNewImport runs start, after asking when an import is under way,
// which a new one replaces.
func (s *shell) askNewImport(start func()) {
	if s.importsOf().Status().Step == app.ImportNone {
		start()
		return
	}
	s.ask(locales.PlaylistImportAskTitle(), locales.PlaylistImportAskMessage(), locales.PlaylistImportAskOk(), start)
}

// importFiles imports files, beside the main thread, and shows the import.
func (s *shell) importFiles(paths []string) {
	s.askNewImport(func() {
		s.imports = importView{watching: s.imports.watching}
		s.openImport()
		go func() {
			opened, err := s.importsOf().Open(paths)
			s.update(func() { s.importOpened(opened, err) })
		}()
	})
}

// importText imports the text the user pasted.
func (s *shell) importText(text string) {
	s.askNewImport(func() {
		go func() {
			_, err := s.importsOf().Paste(text)
			s.update(func() {
				if err != nil {
					s.notes = append(s.notes, player.Notice{Message: locales.PlaylistImportTextFailed(errorText(err)), Error: true})
					return
				}
				s.imports = importView{watching: s.imports.watching}
			})
		}()
	})
}

// importOpened tells of the files an import couldn't read.
func (s *shell) importOpened(o app.ImportOpened, err error) {
	switch n := len(o.Failed); {
	case n == 1:
		f := o.Failed[0]
		s.notes = append(s.notes, player.Notice{Message: locales.PlaylistImportFailed(f.Name, errorText(f.Err)), Error: true})
	case n > 1:
		f := o.Failed[0]
		s.notes = append(s.notes, player.Notice{Message: locales.PlaylistImportFailedSeveral(n, f.Name, errorText(f.Err)), Error: true})
	case err != nil:
		s.notes = append(s.notes, noticeOf(err))
	}
}

// importPage is the page that imports playlists, at the import's step.
func (s *shell) importPage(c *ui.Context) {
	iv := &s.imports
	imp := s.importsOf()
	if !iv.watching {
		iv.watching = true
		s.stop = append(s.stop, imp.Watch(func() { s.update(func() {}) }))
	}
	st := imp.Status()
	s.pageScroll(c, func() {
		s.pageHeader(c, locales.PlaylistImportTitle(), nil, nil)
		ui.Column(c).Gap(16).AlignSelf(ui.Stretch).Children(func() {
			switch st.Step {
			case app.ImportColumns:
				s.importColumns(c, st)
			case app.ImportPick:
				s.importPick(c, st)
			case app.ImportMatching:
				s.importMatching(c, st)
			case app.ImportHalted:
				s.importHalted(c, st)
			case app.ImportReview:
				s.importReview(c, st)
			default:
				s.importStart(c)
			}
		})
	})
	s.importReplaceDialog(c)
}

// importStart is the page before the user chose what to import: files,
// or text to paste.
func (s *shell) importStart(c *ui.Context) {
	k := &s.lk
	iv := &s.imports
	ui.Text(c, locales.PlaylistImportIntro()).FontSize(k.bodySize()).MaxWidth(720)
	if s.button(c, "import-choose", locales.PlaylistImportChoose(), true).Clicked() {
		s.chooseImportFiles()
	}
	ui.TextArea(c.Key("import-paste"), &iv.pasted).Placeholder(locales.PlaylistImportPastePlaceholder()).Height(160).MaxWidth(720).
		Label(locales.PlaylistImportPasteLabel())
	if s.button(c, "import-paste-go", locales.PlaylistImportPasteButton(), false).Disabled(strings.TrimSpace(iv.pasted) == "").Clicked() {
		s.importText(iv.pasted)
	}
	s.linkOf(c, locales.PlaylistImportDataHelp(), importHelp).FontSize(k.smallSize())
}

// fieldLabels name the fields of a table's columns, as playlistio.Fields
// lists them.
func fieldLabels() []string {
	return []string{locales.PlaylistImportFieldNone(), locales.PlaylistImportFieldTitle(), locales.PlaylistImportFieldArtists(),
		locales.PlaylistImportFieldAlbum(), locales.PlaylistImportFieldDuration(), locales.PlaylistImportFieldIsrc(),
		locales.PlaylistImportFieldVideoId(), locales.PlaylistImportFieldUrl(), locales.PlaylistImportFieldSpotifyUri(),
		locales.PlaylistImportFieldPlaylist()}
}

// importColumns has the user name the columns of a table whose names
// MixMusic doesn't know: what each holds, by the first 5 rows, however
// many columns there are.
func (s *shell) importColumns(c *ui.Context, st app.ImportStatus) {
	k := &s.lk
	iv := &s.imports
	t := st.Table
	labels := fieldLabels()
	if iv.table != t {
		iv.table, iv.header = t, len(t.Rows) > 1
		iv.fields = make([]string, t.Width())
		for i := range iv.fields {
			iv.fields[i] = labels[0]
		}
	}
	ui.Text(c, locales.PlaylistImportColumnsHint(t.Name)).FontSize(k.bodySize())
	s.pointerKeys(ui.Checkbox(c.Key("import-header"), &iv.header, locales.PlaylistImportHeaderRow()))
	ui.Row(c).Gap(16).Wrap().Children(func() {
		for col := range iv.fields {
			ui.Column(c).Gap(6).Width(180).Children(func() {
				s.selectOf(c.Key("import-column-"+count(col)), &iv.fields[col], labels).Width(180).
					Label(locales.PlaylistImportColumn(col + 1))
				for r := range min(5, len(t.Rows)) {
					cell := ""
					if col < len(t.Rows[r]) {
						cell = t.Rows[r][col]
					}
					ink := k.text
					if r == 0 && iv.header {
						ink = k.muted
					}
					ui.Text(c, cell).FontSize(k.smallSize()).TextColor(ink).SingleLine().MaxWidth(180)
				}
			})
		}
	})
	fields := make([]playlistio.Field, len(iv.fields))
	names := false
	for i, label := range iv.fields {
		for j, l := range labels {
			if l == label {
				fields[i] = playlistio.Fields[j]
			}
		}
		switch fields[i] {
		case playlistio.FieldTitle, playlistio.FieldVideoID, playlistio.FieldURL, playlistio.FieldSpotifyURI:
			names = true
		}
	}
	if !names {
		ui.Text(c, locales.PlaylistImportColumnsNeed()).FontSize(k.smallSize()).TextColor(k.muted)
	}
	imp := s.importsOf()
	ui.Row(c).Gap(8).Wrap().Children(func() {
		if s.button(c, "import-columns-go", locales.PlaylistImportContinue(), true).Disabled(!names).Clicked() && names {
			imp.SetColumns(fields, iv.header)
		}
		if s.button(c, "import-skip", locales.PlaylistImportSkipFile(), false).Clicked() {
			imp.SkipTable()
		}
		s.importCancel(c)
	})
}

// importCancel drops the import.
func (s *shell) importCancel(c *ui.Context) {
	if s.button(c, "import-cancel", locales.LibraryCancel(), false).Clicked() {
		go s.importsOf().Cancel()
		s.imports = importView{watching: s.imports.watching}
	}
}

// importPick has the user pick the lists to import, of several: the
// playlists of a data download, or of several files.
func (s *shell) importPick(c *ui.Context, st app.ImportStatus) {
	k := &s.lk
	iv := &s.imports
	if len(iv.picks) != len(st.Lists) {
		iv.picks = make([]bool, len(st.Lists))
		for i, l := range st.Lists {
			iv.picks[i] = l.Picked
		}
	}
	ui.Text(c, locales.PlaylistImportPickHint()).FontSize(k.bodySize())
	any := false
	ui.Column(c).Gap(8).Children(func() {
		for i, l := range st.Lists {
			s.pointerKeys(ui.Checkbox(c.Key("import-pick-"+count(i)), &iv.picks[i], locales.PlaylistImportPickList(l.Name, l.Songs)))
			any = any || iv.picks[i]
		}
	})
	ui.Row(c).Gap(8).Wrap().Children(func() {
		if s.button(c, "import-pick-go", locales.PlaylistImportContinue(), true).Disabled(!any).Clicked() && any {
			s.importsOf().Pick(iv.picks)
		}
		s.importCancel(c)
	})
}

// importMatching shows how far the looking for the songs is, which the
// user may stop, to review what was found, or cancel.
func (s *shell) importMatching(c *ui.Context, st app.ImportStatus) {
	k := &s.lk
	label := locales.PlaylistImportMatching(st.Done, st.Total)
	ui.Text(c, label).FontSize(k.subtitleSize()).FontWeight(k.leadWeight())
	ui.Progress(c, float64(st.Done)/float64(max(st.Total, 1))).Width(320).Label(label)
	ui.Text(c, locales.PlaylistImportMatchingNote()).FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(640)
	ui.Row(c).Gap(8).Wrap().Children(func() {
		if s.button(c, "import-stop", locales.PlaylistImportStop(), false).Clicked() {
			go s.importsOf().Stop()
		}
		s.importCancel(c)
	})
}

// importHalted tells why the looking stopped on its own, with a way to
// try again, or to review what was found.
func (s *shell) importHalted(c *ui.Context, st app.ImportStatus) {
	imp := s.importsOf()
	s.alert(c, locales.PlaylistImportHaltTitle(), errorText(st.Halt), func() {
		if s.button(c, "import-again", locales.WidgetsTryAgain(), true).Clicked() {
			imp.TryAgain()
		}
		if s.button(c, "import-review", locales.PlaylistImportReview(), false).Clicked() {
			go imp.Stop()
		}
		s.importCancel(c)
	})
}

// importDone tells what an import came to, and opens where its songs
// went.
func (s *shell) importDone(res app.ImportResult, err error) {
	switch {
	case err != nil:
		s.notes = append(s.notes, noticeOf(err))
		return
	case res.Liked:
		s.notes = append(s.notes, notice(locales.PlaylistImportDoneLiked(res.Added)))
		s.navigate("/library/liked")
	case len(res.Mixes) == 1:
		mix := res.Mixes[0]
		s.actionToasts = append(s.actionToasts, actionToast{locales.PlaylistImportDoneMix(res.Added, mix.Title),
			locales.PlaylistImportOpen(), func() { s.navigate(playlistPath(mix.ID)) }})
		s.navigate(playlistPath(mix.ID))
	default:
		s.notes = append(s.notes, notice(locales.PlaylistImportDoneMixes(res.Added, len(res.Mixes))))
		s.navigate("/library/mixes")
	}
	s.imports = importView{watching: s.imports.watching}
}

// finishImport adds the songs ticked where the user chose, beside the
// main thread.
func (s *shell) finishImport(target app.ImportTarget) {
	iv := &s.imports
	iv.busy = true
	go func() {
		res, err := s.importsOf().Finish(context.Background(), target)
		s.update(func() {
			s.imports.busy = false
			s.importDone(res, err)
		})
	}()
}

// importTrackLine names a track found: its title and its artists.
func importTrackLine(t media.Track) string {
	if n := t.ArtistNames(); n != "" {
		return t.Title + " · " + n
	}
	return t.Title
}
