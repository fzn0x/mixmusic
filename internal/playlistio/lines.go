package playlistio

import (
	"path/filepath"
	"strings"
)

// audioExtensions are those of the music files a line of text may name.
var audioExtensions = map[string]bool{".mp3": true, ".flac": true, ".m4a": true, ".mp4": true, ".aac": true,
	".ogg": true, ".oga": true, ".opus": true, ".wav": true, ".wma": true, ".aif": true, ".aiff": true, ".alac": true,
	".webm": true}

// readLines reads text of a song a line: a link, a file, or "Artist -
// Title". Empty lines, and those that start with # or //, are none.
func readLines(name, dir, text string) []List {
	l := List{Name: name}
	for _, line := range strings.Split(strings.TrimPrefix(text, "\ufeff"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		var e Entry
		switch v, s := link(line, false); {
		case v != "" || s != "":
			e.VideoID, e.SpotifyID = v, s
			if isLink(line) {
				e.URL = line
			}
		case isLink(line):
			continue
		case pathLike(line):
			e = locate(e, dir, line)
		default:
			e.Title, e.Artists = splitTitle(line)
		}
		l.Entries = append(l.Entries, e)
	}
	if len(l.Entries) == 0 {
		return nil
	}
	return []List{l}
}

// pathLike reports whether a line names a file: a file:// URL, an
// absolute path, or a name of a music file.
func pathLike(line string) bool {
	return strings.HasPrefix(strings.ToLower(line), "file:") || strings.HasPrefix(line, "/") ||
		windowsPath.MatchString(line) || audioExtensions[strings.ToLower(filepath.Ext(line))]
}
