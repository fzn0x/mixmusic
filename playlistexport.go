package main

// Exporting lists (#613): Export the list… in the More menu of a mix, a
// playlist or an album, and on Liked songs, and Export the library… in
// the command palette, which puts every mix and Liked songs in one file:
// CSV, M3U8 or JSON, as the user chooses in the dialog that saves it.
// internal/app's Imports writes them, and a toast tells where, with a
// way to the file.

import (
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/export"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// exportListItem is Export the list… in a menu of a list of tracks.
func (s *shell) exportListItem(m *ui.Menu, title string, tracks []media.Track) {
	if m.Item(locales.PlaylistExportList()).Chosen() {
		s.exportLists(title, []app.ExportList{{Name: title, Tracks: tracks}})
	}
}

// exportLikedButton is Export the list… on Liked songs.
func (s *shell) exportLikedButton(c *ui.Context, liked []media.Track) {
	if s.button(c, "liked-export", locales.PlaylistExportList(), false).Clicked() {
		name := locales.RailLikedSongs()
		s.exportLists(name, []app.ExportList{{Name: name, Tracks: liked}})
	}
}

// exportLibrary is Export the library…: every mix and Liked songs.
func (s *shell) exportLibrary() {
	s.exportLists(locales.PlaylistExportLibraryName(), s.importsOf().Library())
}

// exportLists asks where to save lists, named name, beside the main
// thread, and saves them there in the format of the file's extension:
// CSV, unless the user chose M3U8 or JSON.
func (s *shell) exportLists(name string, lists []app.ExportList) {
	win := s.win
	go func() {
		path, err := saveFileDialog(mygo.SaveDialogOptions{Parent: win, Title: locales.PlaylistExportDialog(name),
			DefaultPath: export.Name(nil, name) + ".csv", CreateDirectories: true, Filters: []mygo.FileFilter{
				{Name: locales.PlaylistExportCsv(), Extensions: []string{"csv"}},
				{Name: locales.PlaylistExportM3u(), Extensions: []string{"m3u8"}},
				{Name: locales.PlaylistExportJson(), Extensions: []string{"json"}},
			}})
		if err == nil && path == "" {
			return
		}
		if err == nil {
			switch strings.ToLower(filepath.Ext(path)) {
			case ".csv", ".m3u8", ".m3u", ".json":
			default:
				path += ".csv"
			}
			err = s.importsOf().Export(path, lists)
		}
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.actionToasts = append(s.actionToasts, actionToast{locales.ExportedTo(name, path), locales.ExportShow(),
				func() { showInFolder(path) }})
		})
	}()
}
