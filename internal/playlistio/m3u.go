package playlistio

import (
	"net/url"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// ownMark is the line MixMusic's own M3U8 files carry, whose songs come
// as they are, without a search.
const ownMark = "#MIXMUSIC"

// readM3U reads an M3U list: #EXTINF lines, then the song's file or link.
// #PLAYLIST names the list, and a #PLAYLIST after songs starts another,
// as MixMusic's export of the library writes. A song without #EXTINF is
// named by its file. dir is where relative paths start.
func readM3U(name, dir, text string) []List {
	lists := []List{{Name: name}}
	own := false
	var next Entry
	for _, line := range strings.Split(strings.TrimPrefix(text, "\ufeff"), "\n") {
		line = strings.TrimSpace(line)
		upper := strings.ToUpper(line)
		switch {
		case line == "", strings.HasPrefix(upper, "#EXTM3U"):
		case strings.HasPrefix(upper, ownMark):
			own = true
		case strings.HasPrefix(upper, "#PLAYLIST:"):
			title := strings.TrimSpace(line[len("#PLAYLIST:"):])
			if last := &lists[len(lists)-1]; len(last.Entries) == 0 {
				last.Name = or(title, last.Name)
			} else {
				lists = append(lists, List{Name: or(title, name)})
			}
		case strings.HasPrefix(upper, "#EXTINF:"):
			next = extinf(line[len("#EXTINF:"):])
		case strings.HasPrefix(upper, "#EXTALB:"):
			next.Album = strings.TrimSpace(line[len("#EXTALB:"):])
		case strings.HasPrefix(upper, "#EXTART:"):
			if len(next.Artists) == 0 {
				next.Artists = []string{strings.TrimSpace(line[len("#EXTART:"):])}
			}
		case strings.HasPrefix(line, "#"):
		default:
			e := locate(next, dir, line)
			lists[len(lists)-1].Entries = append(lists[len(lists)-1].Entries, e)
			next = Entry{}
		}
	}
	var out []List
	for _, l := range lists {
		if len(l.Entries) == 0 {
			continue
		}
		l.Own = own
		if own {
			// MixMusic joins a track's artists with commas.
			for i, e := range l.Entries {
				if len(e.Artists) == 1 {
					l.Entries[i].Artists = splitArtists(e.Artists[0], true)
				}
			}
		}
		out = append(out, l)
	}
	return out
}

// extinf reads what follows #EXTINF: the length in seconds, -1 when
// unknown, attributes, then a comma and "Artist - Title".
func extinf(s string) Entry {
	head, title, _ := strings.Cut(s, ",")
	var e Entry
	if f := strings.Fields(head); len(f) > 0 {
		if n, err := strconv.ParseFloat(f[0], 64); err == nil && n > 0 {
			e.Duration = time.Duration(n * float64(time.Second))
		}
	}
	e.Title, e.Artists = splitTitle(title)
	return e
}

// locate is e, the song of a list's line, which is its link or its file,
// and named by the file when e has no title.
func locate(e Entry, dir, line string) Entry {
	if isLink(line) {
		e.URL = line
		e.VideoID, e.SpotifyID = link(line, false)
		return e
	}
	if v, s := link(line, false); v != "" || s != "" {
		e.VideoID, e.SpotifyID = v, s
		return e
	}
	e.Path = ResolvePath(dir, line)
	if e.Title == "" {
		e.Title, e.Artists = splitTitle(trackNumber.ReplaceAllString(stem(e.Path), ""))
	}
	return e
}

var (
	windowsPath = regexp.MustCompile(`^(?:[A-Za-z]:[\\/]|\\\\)`)
	// trackNumber is the number of a track that starts the names of
	// files, as "01 - ", "01. " or "1 ".
	trackNumber = regexp.MustCompile(`^\d{1,3}(?:\s*[-.)]\s*|\s+)`)
)

// ResolvePath is the file a list names as path, from the list's folder
// dir: a file:// URL, an absolute path, or one relative to dir. Windows'
// paths read on every system: their backslashes part the folders of a
// relative path, and an absolute one, which names no file here, stays as
// written, for its name to find the song.
func ResolvePath(dir, path string) string {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(strings.ToLower(path), "file:") {
		if u, err := url.Parse(path); err == nil && u.Path != "" {
			path = u.Path
			if windowsPath.MatchString(strings.TrimPrefix(path, "/")) {
				path = strings.TrimPrefix(path, "/")
			}
			if u.Host != "" && u.Host != "localhost" {
				path = `\\` + u.Host + strings.ReplaceAll(path, "/", `\`)
			}
		}
	}
	switch {
	case windowsPath.MatchString(path):
		if runtime.GOOS == "windows" {
			return filepath.Clean(path)
		}
		return path
	case strings.HasPrefix(path, "/"):
		return filepath.Clean(filepath.FromSlash(path))
	case dir == "":
		return filepath.FromSlash(strings.ReplaceAll(path, `\`, "/"))
	}
	return filepath.Join(dir, filepath.FromSlash(strings.ReplaceAll(path, `\`, "/")))
}

// PathName is the name of a file a list names, of a path of any system:
// after its last slash or backslash.
func PathName(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		return path[i+1:]
	}
	return path
}

// stem is a path's name without its extension.
func stem(path string) string {
	name := PathName(path)
	if ext := filepath.Ext(name); len(ext) <= 5 {
		name = strings.TrimSuffix(name, ext)
	}
	return name
}
