package main

// The music on this computer: Library › On this computer, its folders in
// Settings › Playback, the files the user opens or drops on the window,
// and its tracks as the lists show them, greyed once their file is gone.

import (
	"errors"
	"path/filepath"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// computerViews are the sections of Library › On this computer.
var computerViews = []string{"songs", "albums", "artists"}

// computerPage is what Library › On this computer keeps between frames:
// the section it shows, and how many tiles of albums and artists; and
// what the search of q found of it, of the index's version.
type computerPage struct {
	view  int
	tiles int

	searched bool
	q        string
	version  uint64
	found    media.SearchResults
}

// computerTiles is how many tiles of albums or artists a section shows at
// first, and more shows more of: a library of thousands builds those only.
const computerTiles = 120

// computerLibrary is Library › On this computer: the songs, albums and
// artists of the folders of the settings, which play from their files.
func (s *shell) computerLibrary(c *ui.Context) {
	k := &s.lk
	lf := s.svc.LocalFiles
	st := lf.Status()
	tracks := lf.Tracks()
	if len(tracks) == 0 {
		title, text := locales.ComputerEmptyTitle(), locales.ComputerEmptyText()
		switch {
		case st.Scanning:
			title, text = locales.ComputerScanningTitle(), scanLine(st)
		case len(s.set.MusicFolders) > 0:
			title, text = locales.ComputerNoMusicTitle(), locales.ComputerNoMusicText()
		}
		ui.Text(c, k.words(title)).FontSize(k.bigSize()).FontWeight(k.displayWeight()).Font(k.display).TextColor(k.accent)
		ui.Text(c, text).FontSize(k.subtitleSize()).FontWeight(k.leadWeight()).MaxWidth(640).Margin(0, 0, 24)
		ui.Row(c).Gap(8).Wrap().Children(func() {
			if s.button(c, "computer-add", locales.ComputerSettingsAddFolder(), true).Clicked() {
				s.addMusicFolder()
			}
			if s.button(c, "computer-open", locales.ComputerOpenFiles(), false).Clicked() {
				s.openMusicFiles()
			}
		})
		return
	}
	albums, artists := lf.Albums(), lf.Artists()
	page := &s.computer
	ui.Row(c).Gap(16).Wrap().AlignItems(ui.Center).Margin(0, 0, 24).Children(func() {
		s.chips(c, computerViews, []int{len(tracks), len(albums), len(artists)}, &page.view)
		ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Children(func() {
			if st.Scanning {
				ui.Text(c, scanLine(st)+" · ")
			} else {
				ui.Text(c, locales.ComputerFromFolders(len(st.Folders)))
			}
			s.linkOf(c, k.sentence(locales.ComputerManage()), "/settings/playback")
		})
	})
	switch computerViews[page.view] {
	case "songs":
		ui.Row(c).Gap(8).Margin(0, 0, 16).Children(func() {
			if s.button(c, "computer-play", locales.LibraryPlay(), true).Clicked() {
				s.p.PlayAll(tracks, false)
			}
			if s.button(c, "computer-shuffle", locales.LibraryShuffle(), false).Clicked() {
				s.shuffled(tracks)
			}
			if s.button(c, "computer-open-files", locales.ComputerOpenFiles(), false).Clicked() {
				s.openMusicFiles()
			}
		})
		view := s.trackTools(c, "computer", "list", tracks)
		s.trackList(c, "computer", view.tracks, trackListOptions{art: true, album: true,
			empty: locales.ComputerNoMatch(s.searching("computer"))})
	case "albums":
		page.tiles = max(page.tiles, computerTiles)
		s.tileWrap(c, func() {
			for i, a := range albums[:min(len(albums), page.tiles)] {
				s.albumTile(c, "computer-album-"+count(i), a)
			}
		})
		s.moreTiles(c, len(albums))
	case "artists":
		page.tiles = max(page.tiles, computerTiles)
		s.tileWrap(c, func() {
			for i, a := range artists[:min(len(artists), page.tiles)] {
				s.artistTile(c, "computer-artist-"+count(i), a)
			}
		})
		s.moreTiles(c, len(artists))
	}
}

// moreTiles shows more of n tiles, past those shown.
func (s *shell) moreTiles(c *ui.Context, n int) {
	page := &s.computer
	if n <= page.tiles {
		return
	}
	ui.Row(c).Margin(24, 0, 0).Children(func() {
		if s.button(c, "computer-more", locales.ComputerShowMore(page.tiles, n), false).Clicked() {
			page.tiles += computerTiles
		}
	})
}

// computerResults shows what the music on this computer holds of a
// search, of the pivot of the search page, in a section of its own above
// the services' results: its songs, of all its pivot the first few, its
// albums or its artists.
func (s *shell) computerResults(c *ui.Context, q, pivot string) {
	found := s.computerSearch(q)
	switch pivot {
	case "all", "songs":
		tracks := found.Tracks
		if len(tracks) == 0 {
			return
		}
		if pivot == "all" {
			tracks = tracks[:min(len(tracks), 5)]
		}
		s.sectionTitle(c, locales.LibraryOnThisComputer())
		ui.Column(c).Margin(0, 0, 48).Children(func() {
			s.trackList(c, "computer-results", tracks, trackListOptions{art: true, album: true})
		})
	case "albums":
		if len(found.Albums) > 0 {
			s.shelf(c, "computer-albums", locales.LibraryOnThisComputer(), func() {
				for i, a := range found.Albums {
					s.albumTile(c, "computer-album-"+count(i), a)
				}
			})
		}
	case "artists":
		if len(found.Artists) > 0 {
			s.shelf(c, "computer-artists", locales.LibraryOnThisComputer(), func() {
				for i, a := range found.Artists {
					s.artistTile(c, "computer-artist-"+count(i), a)
				}
			})
		}
	}
}

// computerSearch is what the music on this computer holds of q: searched
// again only once q or the index changed, not in each frame.
func (s *shell) computerSearch(q string) media.SearchResults {
	p := &s.computer
	v := s.svc.LocalFiles.Version()
	if !p.searched || p.q != q || p.version != v {
		p.found, p.q, p.version, p.searched = s.svc.LocalFiles.Search(q, 50), q, v, true
	}
	return p.found
}

// scanLine tells how far a scan of the folders came.
func scanLine(st localfiles.Status) string {
	if st.Found == 0 {
		return locales.ComputerScanStarting()
	}
	return locales.ComputerScanReading(st.Read, st.Found)
}

func init() { addSettings("playback", 85, (*shell).computerSettings) }

// computerSettings are Settings › Playback › Music on this computer: the
// folders of music the library holds, with how many tracks each holds, how
// many files MixMusic skipped and how many of the folders in it can't be
// read, a way to add one, to remove one, and to read them again.
func (s *shell) computerSettings(c *ui.Context) {
	k := &s.lk
	st := s.svc.LocalFiles.Status()
	s.group(c, locales.ComputerSettingsGroup(), func() {
		about := locales.ComputerSettingsAbout()
		if st.Scanning {
			about = scanLine(st)
		}
		s.row(c, locales.ComputerSettingsFolders(), about, func() {
			if s.button(c, "computer-folder-add", locales.ComputerSettingsAddFolder(), false).Label(locales.ComputerSettingsAddFolderLabel()).Clicked() {
				s.addMusicFolder()
			}
			if len(s.set.MusicFolders) > 0 && s.button(c, "computer-rescan", locales.ComputerSettingsRescan(), false).Disabled(st.Scanning).
				Label(locales.ComputerSettingsRescanLabel()).Clicked() {
				s.svc.LocalFiles.Rescan()
			}
		}, "local", "files", "library", "music", "mp3", "flac")
		for i, folder := range s.set.MusicFolders {
			f := folderStatus(st, folder)
			s.rowWith(c, folder, func() {
				switch {
				case f.Problem != "":
					ui.Text(c, locales.ComputerSettingsProblem(f.Problem)).TextColor(k.danger)
				case st.Scanning && f.Tracks == 0:
					ui.Text(c, locales.ComputerSettingsReading())
				default:
					line := locales.ComputerSettingsTracks(f.Tracks)
					if f.Skipped > 0 {
						line = locales.ComputerSettingsTracksSkipped(f.Tracks, count(f.Skipped))
					}
					ui.Text(c, line)
					if len(f.Unread) > 0 {
						ui.Text(c, " · "+unreadLine(len(f.Unread))).TextColor(k.danger)
					}
				}
			}, func() {
				if s.button(c, "computer-folder-remove-"+count(i), locales.ComputerSettingsRemove(), false).Label(locales.ComputerSettingsRemoveLabel(folder)).Clicked() {
					s.removeMusicFolder(folder)
				}
			}, folder)
		}
		s.unwatchedFolders(c)
	})
}

// unreadLine tells of the n folders in a folder of music that the last
// scan couldn't read, whose tracks the library keeps as they were.
func unreadLine(n int) string { return locales.ComputerSettingsUnread(n) }

// folderStatus is what the last scan found of folder.
func folderStatus(st localfiles.Status, folder string) localfiles.Folder {
	for _, f := range st.Folders {
		if settings.SamePath(f.Path, folder) {
			return f
		}
	}
	return localfiles.Folder{Path: folder}
}

// addMusicFolder asks for a folder of music, and adds it to the library,
// which reads it.
func (s *shell) addMusicFolder() {
	go func() {
		dir, err := s.svc.LocalFiles.ChooseFolder(s.win)
		s.update(func() {
			switch {
			case err != nil:
				s.notes = append(s.notes, noticeOf(err))
			case dir != "":
				for _, f := range s.set.MusicFolders {
					if settings.SamePath(f, dir) {
						s.notes = append(s.notes, notice(locales.ComputerInLibrary(filepath.Base(dir))))
						return
					}
				}
				s.set.MusicFolders = append(append([]string{}, s.set.MusicFolders...), dir)
				s.save()
				s.notes = append(s.notes, notice(locales.ComputerReadingFolder(filepath.Base(dir))))
			}
		})
	}()
}

// removeMusicFolder takes a folder out of the library, after asking: its
// files stay where they are.
func (s *shell) removeMusicFolder(folder string) {
	s.ask(locales.ComputerRemoveFolderTitle(), locales.ComputerRemoveFolderQuestion(folder),
		locales.LibraryRemove(), func() {
			var keep []string
			for _, f := range s.set.MusicFolders {
				if !settings.SamePath(f, folder) {
					keep = append(keep, f)
				}
			}
			s.set.MusicFolders = append([]string{}, keep...)
			s.save()
		})
}

// openMusicFiles asks for music files, and plays them.
func (s *shell) openMusicFiles() {
	go func() {
		paths, err := s.svc.LocalFiles.ChooseFiles(s.win)
		if err != nil {
			s.update(func() { s.notes = append(s.notes, noticeOf(err)) })
			return
		}
		s.playFiles(paths)
	}()
}

// dropFiles plays the music files dropped on the window, and those of the
// folders dropped, from its goroutine.
func (s *shell) dropFiles(e *mygo.FileDropEvent) {
	paths := append([]string{}, e.Paths...)
	go s.playFiles(paths)
}

// playFiles reads files, beside the main thread, and plays those that
// play, as a queue, telling of those that don't.
func (s *shell) playFiles(paths []string) {
	if len(paths) == 0 {
		return
	}
	opened := s.svc.LocalFiles.Open(paths)
	s.update(func() { s.playOpened(opened) })
}

// playOpened plays what opening files found.
func (s *shell) playOpened(o app.Opened) {
	switch {
	case len(o.Tracks) == 0 && o.Skipped == 1:
		s.notes = append(s.notes, noticeOf(errors.New(locales.ComputerCantPlayFile())))
		return
	case len(o.Tracks) == 0 && o.Skipped > 1:
		s.notes = append(s.notes, noticeOf(errors.New(locales.ComputerCantPlayFiles())))
		return
	case len(o.Tracks) == 0:
		return
	}
	s.p.PlayAll(o.Tracks, false)
	var msg string
	switch {
	case len(o.Tracks) > 1 && o.Skipped > 0:
		msg = locales.ComputerPlayingFilesSkipped(len(o.Tracks), o.Skipped)
	case len(o.Tracks) > 1:
		msg = locales.ComputerPlayingFiles(len(o.Tracks))
	case o.Skipped > 0:
		msg = locales.ComputerPlayingSkipped(o.Tracks[0].Title, o.Skipped)
	default:
		msg = locales.ComputerPlaying(o.Tracks[0].Title)
	}
	s.notes = append(s.notes, notice(msg))
}

// missingFile reports whether t is a track on this computer whose file is
// gone, which its row shows greyed, as "File not found".
func (s *shell) missingFile(t media.Track) bool {
	return t.Source == media.SourceFile && s.svc.LocalFiles.Missing(t.ID)
}

// localItems are the items of a track's menu of a track on this computer:
// a way to its file.
func (s *shell) localItems(m *ui.Menu, t media.Track) {
	path, ok := s.svc.LocalFiles.FileOf(t.ID)
	if !ok || s.missingFile(t) {
		return
	}
	label := locales.ComputerShowInExplorer()
	switch runtime.GOOS {
	case "darwin":
		label = locales.ComputerShowInFinder()
	case "linux":
		label = locales.ComputerShowInFolder()
	}
	if m.Item(label).Chosen() {
		mygo.Shell.ShowItemInFolder(path)
	}
	s.tagItems(m, t)
}

// localArt is the covers of the music on this computer, which show from
// their files, as the downloads' do.
func (s *shell) localArt(url string) (string, bool) {
	if p, ok := s.mixCoverFile(url); ok {
		return p, true
	}
	if localfiles.IsArt(url) {
		return s.svc.LocalFiles.ArtworkFile(url)
	}
	return s.svc.Downloads.ArtworkFile(url)
}
