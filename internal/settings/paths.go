package settings

import (
	"encoding/json"

	"github.com/fzn0x/mixmusic/internal/store"
)

// The settings that name files and folders are kept in settings.json as
// store.EncodePath writes them: a path that isn't UTF-8, as a name on a
// Linux drive may be, in base64, so that it names the same file when the
// file is read back, after a restart, and when the store copies the
// settings through JSON. A path that is UTF-8 reads as it is.

// MarshalJSON writes the settings as settings.json keeps them.
func (s Settings) MarshalJSON() ([]byte, error) {
	type plain Settings
	s.paths(store.EncodePath)
	return json.Marshal(plain(s))
}

// readPaths reads the paths UnmarshalJSON read as settings.json keeps
// them. Those the file doesn't set are a path already, of the defaults or
// of the settings read into, which reads as itself: no path starts as
// one in base64 does.
func (s *Settings) readPaths() { s.paths(store.DecodePath) }

// paths changes each setting that names a file or a folder by fn, the
// music folders in a list of their own: the one s held is its caller's
// too.
func (s *Settings) paths(fn func(string) string) {
	s.ColorFile = fn(s.ColorFile)
	s.DownloadsFolder = fn(s.DownloadsFolder)
	if s.MusicFolders != nil {
		folders := make([]string, len(s.MusicFolders))
		for i, f := range s.MusicFolders {
			folders[i] = fn(f)
		}
		s.MusicFolders = folders
	}
	s.YTDLPPath = fn(s.YTDLPPath)
}
