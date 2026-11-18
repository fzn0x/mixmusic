package app

import (
	"errors"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/locales"
)

// The folders of the sidebar's Your mixes (library/folders.go): they hold
// the user's mixes and the playlists of their services' libraries, and
// stay in MixMusic, never reaching a service.

// Folders returns the folders of the sidebar's Your mixes, with what each
// holds, by name or in the order the user put them.
func (c *Collection) Folders() []library.Folder { return c.a.library.Folders() }

// FolderOf returns the ID of the folder the playlist id is in, or "".
func (c *Collection) FolderOf(id string) string { return c.a.library.FolderOf(id) }

// NewFolder puts the playlist of m, a mix or a playlist of a service, in
// a new folder named name, or in the folder of that name, regardless of
// case and spacing, and returns the folder.
func (c *Collection) NewFolder(name string, m library.Mark) (library.Folder, error) {
	f, err := c.a.library.NewFolder(name, c.a.filedMark(m))
	return f, folderError(err)
}

// FileInFolder puts the playlist of m in the folder of id, out of the one
// it was in.
func (c *Collection) FileInFolder(id string, m library.Mark) error {
	return folderError(c.a.library.FileIn(id, c.a.filedMark(m)))
}

// RemoveFromFolder takes the playlist id out of its folder: a folder
// left without a playlist goes.
func (c *Collection) RemoveFromFolder(id string) error {
	return folderError(c.a.library.Unfile(id))
}

// RenameFolder names the folder of id name: one with the name of another
// becomes one with it.
func (c *Collection) RenameFolder(id, name string) error {
	return folderError(c.a.library.RenameFolder(id, name))
}

// UngroupFolder takes away the folder of id, whose playlists stay.
func (c *Collection) UngroupFolder(id string) error {
	return folderError(c.a.library.UngroupFolder(id))
}

// MoveFolder moves the folder of id onto the folder of onto, among the
// folders of among, which a search shows: every other folder keeps its
// place.
func (c *Collection) MoveFolder(id, onto string, among []string) error {
	return folderError(c.a.library.MoveFolder(id, onto, among))
}

// MoveInFolder moves the playlist id onto the playlist onto, in the
// folder of folder, among the playlists of among, which a search shows.
func (c *Collection) MoveInFolder(folder, id, onto string, among []string) error {
	return folderError(c.a.library.MoveFiled(folder, id, onto, among))
}

// filedMark is m as a folder keeps it: a playlist, of the account of its
// service the user is signed in to, as a pin is.
func (a *App) filedMark(m library.Mark) library.Mark {
	m.Kind = library.MarkPlaylist
	m.Account = a.markAccount(m.ID)
	return m
}

// folderError is err, an error of the library's folders, as the windows
// show it, in the interface's language.
func folderError(err error) error {
	switch {
	case errors.Is(err, library.ErrFolderName):
		return wordedLater("a folder needs a name", locales.FoldersNeedName, err)
	case errors.Is(err, library.ErrNoFolder):
		return wordedLater("the folder is gone", locales.FoldersGone, err)
	}
	return err
}
