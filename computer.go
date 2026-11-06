package main

// The music on this computer: Library › On this computer, its folders in
// Settings › Playback, the files the user opens or drops on the window,
// and its tracks as the lists show them, greyed once their file is gone.

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
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
		title, text := "Nothing on this computer yet",
			"Add a folder of music, or open files: they play from your computer, beside Spotify and YouTube Music, in your queue and your mixes. Nothing of them is uploaded."
		switch {
		case st.Scanning:
			title, text = "Looking through your music", scanLine(st)
		case len(s.set.MusicFolders) > 0:
			title, text = "No music found", "The folders hold no files MixMusic plays: MP3, FLAC, WAV, M4A, Opus or Vorbis."
		}
		ui.Text(c, k.words(title)).FontSize(k.bigSize()).FontWeight(k.displayWeight()).Font(k.display).TextColor(k.accent)
		ui.Text(c, text).FontSize(k.subtitleSize()).FontWeight(k.leadWeight()).MaxWidth(640).Margin(0, 0, 24)
		ui.Row(c).Gap(8).Wrap().Children(func() {
			if s.button(c, "computer-add", "add a folder", true).Clicked() {
				s.addMusicFolder()
			}
			if s.button(c, "computer-open", "open files", false).Clicked() {
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
				ui.Text(c, "From "+plural(len(st.Folders), "folder")+" · ")
			}
			s.linkOf(c, k.sentence("manage"), "/settings/playback")
		})
	})
	switch computerViews[page.view] {
	case "songs":
		ui.Row(c).Gap(8).Margin(0, 0, 16).Children(func() {
			if s.button(c, "computer-play", "play", true).Clicked() {
				s.p.PlayAll(tracks, false)
			}
			if s.button(c, "computer-shuffle", "shuffle", false).Clicked() {
				s.shuffled(tracks)
			}
			if s.button(c, "computer-open-files", "open files", false).Clicked() {
				s.openMusicFiles()
			}
		})
		view := s.trackTools(c, "computer", "list", tracks)
		s.trackList(c, "computer", view.tracks, trackListOptions{art: true, album: true,
			empty: "No song matches “" + s.searching("computer") + "”."})
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
		if s.button(c, "computer-more", fmt.Sprintf("show more · %d of %d", page.tiles, n), false).Clicked() {
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
		s.sectionTitle(c, "On this computer")
		ui.Column(c).Margin(0, 0, 48).Children(func() {
			s.trackList(c, "computer-results", tracks, trackListOptions{art: true, album: true})
		})
	case "albums":
		if len(found.Albums) > 0 {
			s.shelf(c, "computer-albums", "On this computer", func() {
				for i, a := range found.Albums {
					s.albumTile(c, "computer-album-"+count(i), a)
				}
			})
		}
	case "artists":
		if len(found.Artists) > 0 {
			s.shelf(c, "computer-artists", "On this computer", func() {
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
		return "Looking through your folders…"
	}
	return fmt.Sprintf("Reading %d of %s…", st.Read, plural(st.Found, "file"))
}

func init() { addSettings("playback", 85, (*shell).computerSettings) }

// computerSettings are Settings › Playback › Music on this computer: the
// folders of music the library holds, with how many tracks each holds and
// how many files MixMusic skipped, a way to add one, to remove one, and to
// read them again.
func (s *shell) computerSettings(c *ui.Context) {
	k := &s.lk
	st := s.svc.LocalFiles.Status()
	s.group(c, "Music on this computer", func() {
		about := "Folders of music files MixMusic plays from your computer: MP3, FLAC, WAV, M4A, Opus and Vorbis. Nothing of them is uploaded."
		if st.Scanning {
			about = scanLine(st)
		}
		s.row(c, "Folders", about, func() {
			if s.button(c, "computer-folder-add", "add a folder", false).Label("Add a folder of music").Clicked() {
				s.addMusicFolder()
			}
			if len(s.set.MusicFolders) > 0 && s.button(c, "computer-rescan", "rescan", false).Disabled(st.Scanning).
				Label("Read the folders again").Clicked() {
				s.svc.LocalFiles.Rescan()
			}
		}, "local", "files", "library", "music", "mp3", "flac")
		for i, folder := range s.set.MusicFolders {
			f := folderStatus(st, folder)
			s.rowWith(c, folder, func() {
				switch {
				case f.Problem != "":
					ui.Text(c, f.Problem+". Its tracks show greyed until it's back.").TextColor(k.danger)
				case st.Scanning && f.Tracks == 0:
					ui.Text(c, "Reading…")
				default:
					line := plural(f.Tracks, "track")
					if f.Skipped > 0 {
						line += fmt.Sprintf(" · %d skipped, of kinds MixMusic can't play or broken", f.Skipped)
					}
					ui.Text(c, line)
				}
			}, func() {
				if s.button(c, "computer-folder-remove-"+count(i), "remove", false).Label("Remove " + folder).Clicked() {
					s.removeMusicFolder(folder)
				}
			}, folder)
		}
	})
}

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
						s.notes = append(s.notes, notice(filepath.Base(dir)+" is in the library already"))
						return
					}
				}
				s.set.MusicFolders = append(append([]string{}, s.set.MusicFolders...), dir)
				s.save()
				s.notes = append(s.notes, notice("Reading the music of "+filepath.Base(dir)))
			}
		})
	}()
}

// removeMusicFolder takes a folder out of the library, after asking: its
// files stay where they are.
func (s *shell) removeMusicFolder(folder string) {
	s.ask("remove folder", "Take "+folder+" out of the library? Its files stay where they are, and its tracks stay in your mixes, greyed.",
		"remove", func() {
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
		s.notes = append(s.notes, noticeOf(errors.New("MixMusic can't play this file")))
		return
	case len(o.Tracks) == 0 && o.Skipped > 1:
		s.notes = append(s.notes, noticeOf(errors.New("MixMusic can't play these files")))
		return
	case len(o.Tracks) == 0:
		return
	}
	s.p.PlayAll(o.Tracks, false)
	msg := "Playing " + o.Tracks[0].Title
	if len(o.Tracks) > 1 {
		msg = fmt.Sprintf("Playing %s", plural(len(o.Tracks), "file"))
	}
	if o.Skipped > 0 {
		msg += fmt.Sprintf(" · %d skipped, which MixMusic can't play", o.Skipped)
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
	path, ok := localfiles.PathOf(t.ID)
	if !ok || s.missingFile(t) {
		return
	}
	label := "Show in Explorer"
	switch runtime.GOOS {
	case "darwin":
		label = "Show in Finder"
	case "linux":
		label = "Show in its folder"
	}
	if m.Item(label).Chosen() {
		mygo.Shell.ShowItemInFolder(path)
	}
}

// localArt is the covers of the music on this computer, which show from
// their files, as the downloads' do.
func (s *shell) localArt(url string) (string, bool) {
	if localfiles.IsArt(url) {
		return s.svc.LocalFiles.ArtworkFile(url)
	}
	return s.svc.Downloads.ArtworkFile(url)
}
