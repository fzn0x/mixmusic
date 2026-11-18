package library

// Folders of the sidebar's Your mixes: the user groups their mixes and
// the playlists of the services under names, one level deep. A folder
// keeps each playlist as a mark, by its ID, and a playlist is in one
// folder at most. Folders list by name until the user moves one, and in
// their order from then on; inside each, the playlists list in the order
// they came in, the last last, until the user moves them. A folder that
// holds nothing goes. Names that differ only in case or spacing are one
// folder. The playlists of a service leave their folders as their pins
// do (KeepListed), and folders never reach a service.

import (
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/fzn0x/mixmusic/internal/media"
)

// MaxFolderName is how many characters a folder's name holds at most.
const MaxFolderName = 64

var (
	// ErrNoFolder is returned for folders that do not exist.
	ErrNoFolder = errors.New("library: no such folder")
	// ErrFolderName is returned for a name with nothing to show.
	ErrFolderName = errors.New("library: a folder needs a name")
)

// Folder is a folder of the sidebar's Your mixes: the mixes and playlists
// the user put in it, in their order.
type Folder struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Items []Mark `json:"items"`
}

// FolderName is name as a folder keeps it: in Unicode's NFKC form,
// without spaces around it, each run of spaces in it made one, and cut
// to MaxFolderName characters.
func FolderName(name string) string {
	name = strings.Join(strings.Fields(norm.NFKC.String(name)), " ")
	if utf8.RuneCountInString(name) > MaxFolderName {
		name = strings.TrimSpace(string([]rune(name)[:MaxFolderName]))
	}
	return name
}

// Folders returns the folders, with what each holds, as the sidebar lists
// them: by name, or in the user's order once they moved one.
func (l *Library) Folders() []Folder {
	d := l.file.Get()
	out := make([]Folder, len(d.Folders))
	for i, f := range d.Folders {
		f.Items = slices.Clone(f.Items)
		out[i] = f
	}
	if !d.FoldersArranged {
		byName(out)
	}
	return out
}

// byName sorts folders by name, as readers do.
func byName(folders []Folder) {
	slices.SortStableFunc(folders, func(a, b Folder) int { return media.CompareText(a.Name, b.Name) })
}

// FolderOf returns the ID of the folder the playlist id is in, or "".
func (l *Library) FolderOf(id string) string {
	if ix := l.marks.Load(); ix != nil {
		return ix.filed[id]
	}
	return ""
}

// NewFolder puts the playlist of m in a new folder named name, out of the
// one it was in, and returns the folder. A folder of that name already,
// as FolderName keeps it and regardless of case, takes it instead.
func (l *Library) NewFolder(name string, m Mark) (Folder, error) {
	name = FolderName(name)
	if name == "" {
		return Folder{}, ErrFolderName
	}
	var made Folder
	err := l.file.Update(func(d *Data) error {
		if i := namedLike(d.Folders, name, ""); i >= 0 {
			id := d.Folders[i].ID
			fileIn(d, id, m)
			made = d.Folders[slices.IndexFunc(d.Folders, func(f Folder) bool { return f.ID == id })]
			return nil
		}
		m = filedAs(d.Folders, m)
		d.Folders = unfiled(d.Folders, m.ID)
		made = Folder{ID: newID(), Name: name, Items: []Mark{m}}
		if d.FoldersArranged {
			d.Folders = slices.Insert(d.Folders, 0, made)
		} else {
			d.Folders = append(d.Folders, made)
		}
		return nil
	})
	l.indexMarks()
	made.Items = slices.Clone(made.Items)
	return made, err
}

// FileIn puts the playlist of m in the folder of id, last, out of the one
// it was in. One in that folder already keeps its place.
func (l *Library) FileIn(id string, m Mark) error {
	err := l.file.Update(func(d *Data) error {
		if !slices.ContainsFunc(d.Folders, func(f Folder) bool { return f.ID == id }) {
			return ErrNoFolder
		}
		fileIn(d, id, m)
		return nil
	})
	l.indexMarks()
	return err
}

// fileIn puts the playlist of m in the folder of id, which exists, as
// FileIn tells.
func fileIn(d *Data, id string, m Mark) {
	m = filedAs(d.Folders, m)
	at := func() int { return slices.IndexFunc(d.Folders, func(f Folder) bool { return f.ID == id }) }
	i := at()
	if j := slices.IndexFunc(d.Folders[i].Items, func(x Mark) bool { return x.ID == m.ID }); j >= 0 {
		d.Folders[i].Items[j] = m
		return
	}
	// The folder it leaves may go, but not this one, which holds others.
	d.Folders = unfiled(d.Folders, m.ID)
	i = at()
	d.Folders[i].Items = append(d.Folders[i].Items, m)
}

// filedAs is m as a folder keeps it: with the account and the title of
// its mark in a folder, if it's in one and m knows none.
func filedAs(folders []Folder, m Mark) Mark {
	for _, f := range folders {
		for _, x := range f.Items {
			if x.ID == m.ID {
				return keptMark(x, m)
			}
		}
	}
	return m
}

// Unfile takes the playlist id out of its folder, if it's in one.
func (l *Library) Unfile(id string) error {
	if l.FolderOf(id) == "" {
		return nil
	}
	err := l.file.Update(func(d *Data) error {
		d.Folders = unfiled(d.Folders, id)
		return nil
	})
	l.indexMarks()
	return err
}

// RenameFolder names the folder of id name. Another folder of that name
// already, as NewFolder tells, becomes one with it: it takes the
// playlists of this one after its own, and the name.
func (l *Library) RenameFolder(id, name string) error {
	name = FolderName(name)
	if name == "" {
		return ErrFolderName
	}
	err := l.file.Update(func(d *Data) error {
		i := slices.IndexFunc(d.Folders, func(f Folder) bool { return f.ID == id })
		if i < 0 {
			return ErrNoFolder
		}
		j := namedLike(d.Folders, name, id)
		if j < 0 {
			d.Folders[i].Name = name
			return nil
		}
		for _, m := range d.Folders[i].Items {
			if !slices.ContainsFunc(d.Folders[j].Items, func(x Mark) bool { return x.ID == m.ID }) {
				d.Folders[j].Items = append(d.Folders[j].Items, m)
			}
		}
		d.Folders[j].Name = name
		d.Folders = slices.Delete(d.Folders, i, i+1)
		return nil
	})
	l.indexMarks()
	return err
}

// UngroupFolder takes away the folder of id, whose playlists stay.
func (l *Library) UngroupFolder(id string) error {
	err := l.file.Update(func(d *Data) error {
		n := len(d.Folders)
		d.Folders = slices.DeleteFunc(d.Folders, func(f Folder) bool { return f.ID == id })
		if len(d.Folders) == n {
			return ErrNoFolder
		}
		return nil
	})
	l.indexMarks()
	return err
}

// MoveFolder moves the folder of id onto the folder of onto, as a drag
// drops it, among the folders of among, which a search shows: every
// other folder keeps its place (MovedAmong). The folders list in the
// user's order from then on, starting from their order by name.
func (l *Library) MoveFolder(id, onto string, among []string) error {
	return l.file.Update(func(d *Data) error {
		if !slices.ContainsFunc(d.Folders, func(f Folder) bool { return f.ID == id }) {
			return ErrNoFolder
		}
		if !d.FoldersArranged {
			byName(d.Folders)
			d.FoldersArranged = true
		}
		d.Folders = movedAmong(d.Folders, func(f Folder) string { return f.ID }, among, id, onto)
		return nil
	})
}

// MoveFiled moves the playlist id onto the playlist onto, in the folder
// of folder, among the playlists of among, as MoveFolder moves folders.
func (l *Library) MoveFiled(folder, id, onto string, among []string) error {
	return l.file.Update(func(d *Data) error {
		i := slices.IndexFunc(d.Folders, func(f Folder) bool { return f.ID == folder })
		if i < 0 {
			return ErrNoFolder
		}
		d.Folders[i].Items = movedAmong(d.Folders[i].Items, func(m Mark) string { return m.ID }, among, id, onto)
		return nil
	})
}

// namedLike returns where the folder named name is among folders, as
// FolderName keeps names and regardless of case, but the folder of
// besides: -1 for none.
func namedLike(folders []Folder, name, besides string) int {
	return slices.IndexFunc(folders, func(f Folder) bool {
		return f.ID != besides && strings.EqualFold(FolderName(f.Name), name)
	})
}

// unfiled returns folders without the playlist id, and without those it
// leaves empty.
func unfiled(folders []Folder, id string) []Folder {
	for i := range folders {
		folders[i].Items = slices.DeleteFunc(folders[i].Items, func(m Mark) bool { return m.ID == id })
	}
	return withoutEmpty(folders)
}

// withoutEmpty returns folders without those that hold nothing.
func withoutEmpty(folders []Folder) []Folder {
	return slices.DeleteFunc(folders, func(f Folder) bool { return len(f.Items) == 0 })
}
