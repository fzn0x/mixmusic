package settings

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// LocalFiles is the music kept on this computer that the library holds,
// of Settings › Playback › Music on this computer.
type LocalFiles struct {
	// MusicFolders are the folders whose music files the library holds,
	// absolute paths, each once.
	MusicFolders []string `json:"musicFolders"`
}

// defaultLocalFiles is the music of a new install: none.
func defaultLocalFiles() LocalFiles {
	var l LocalFiles
	l.MusicFolders = []string{}
	return l
}

// clean is l with each folder cleaned and once, but relative ones, which
// would follow the working directory.
func (l LocalFiles) clean() LocalFiles {
	folders := []string{}
	for _, f := range l.MusicFolders {
		f = CleanPath(f)
		if f == "" || !filepath.IsAbs(f) {
			continue
		}
		f = filepath.Clean(f)
		if !slices.ContainsFunc(folders, func(g string) bool { return SamePath(f, g) }) {
			folders = append(folders, f)
		}
	}
	l.MusicFolders = folders
	return l
}

// SamePath reports whether two paths name the same file, as their names
// tell: regardless of case on Windows and macOS, whose disks mostly don't
// tell case apart.
func SamePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
