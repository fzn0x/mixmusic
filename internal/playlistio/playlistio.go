// Package playlistio reads lists of songs from files people bring from
// elsewhere, and writes MixMusic's lists to files other players read
// (#613): CSV and TSV, M3U and M3U8, plain text, JSON, and the archives of
// Spotify's and Google's data downloads. It knows nothing of the
// services: what it reads are entries, which internal/app finds the
// tracks of.
package playlistio

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The bounds of what one import reads.
const (
	// MaxFiles is how many files an import reads at once.
	MaxFiles = 100
	// MaxEntries is how many songs an import reads, of all its files.
	MaxEntries = 20000
	// MaxText is how many bytes of text an import reads, of all its
	// files.
	MaxText = 50 << 20
	// MaxArchive is how big an archive of a data download may be, and
	// how much it may hold once unpacked.
	MaxArchive = 100 << 20
	// MaxArchiveFiles is how many files an archive may hold.
	MaxArchiveFiles = 1000
)

// The kinds of a list, besides a playlist's.
const (
	// KindLibrary is the songs a service's library keeps: Spotify's Your
	// Library, YouTube Music's library songs.
	KindLibrary = "library"
	// KindHistory is the songs a streaming history heard for 30 seconds
	// or more, the most played first.
	KindHistory = "history"
)

// The data downloads an archive may be.
const (
	ArchiveSpotify = "spotify"
	ArchiveGoogle  = "google"
)

// The errors of what can't be read, which internal/app words for the
// windows.
var (
	ErrTooBig         = errors.New("it's bigger than 50 MB")
	ErrArchiveTooBig  = errors.New("the archive is bigger than 100 MB")
	ErrArchiveFiles   = errors.New("the archive holds more than 1,000 files")
	ErrNoEntries      = errors.New("it holds no songs")
	ErrNotAList       = errors.New("it isn't a list of songs MixMusic reads")
	ErrNoDataDownload = errors.New("the archive is no data download of Spotify or Google")
)

// Entry is a song of a list as its file tells it: what names it, and the
// IDs and the file that may say which it is.
type Entry struct {
	Title   string
	Artists []string
	Album   string
	// Duration is the song's length, 0 when the file doesn't tell.
	Duration time.Duration
	ISRC     string
	// VideoID is a YouTube video's ID, SpotifyID a Spotify track's.
	VideoID   string
	SpotifyID string
	// MixMusicID is a track's ID in MixMusic, as "yt:…", which MixMusic's
	// own exports carry.
	MixMusicID string
	// URL is the link the file gave, if any.
	URL string
	// Path is a file the list names, as it names it: a path of this
	// computer, or of another.
	Path string
	// Service names the track's service, as MixMusic's exports do.
	Service string
	// Plays is how often a streaming history played the song for 30
	// seconds or more.
	Plays int
}

// List is a list of songs: a playlist, a library or a history.
type List struct {
	// Name is the list's own name, or its file's.
	Name string
	// Kind is "" for a playlist, else KindLibrary or KindHistory.
	Kind    string
	Entries []Entry
	// Own is set for a list MixMusic wrote: its tracks come as they
	// are, without a search.
	Own bool
}

// Source is what a file, or a text pasted, holds: lists, or a table whose
// columns the user names (Table), or both of an archive.
type Source struct {
	// Name is the file's name without its extension.
	Name  string
	Lists []List
	// Tables are tables whose columns weren't found by their names.
	Tables []*Table
	// Archive is the data download an archive is, if it is one.
	Archive string
}

// Count is how many songs the lists hold.
func (s Source) Count() int {
	n := 0
	for _, l := range s.Lists {
		n += len(l.Entries)
	}
	return n
}

// ReadFile reads the lists of the file at path: by its extension, else by
// what it holds.
func ReadFile(path string) (Source, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Source{}, err
	}
	limit := int64(MaxText)
	if isArchiveName(path) {
		limit = MaxArchive
	}
	if info.Size() > limit {
		if limit == MaxArchive {
			return Source{}, ErrArchiveTooBig
		}
		return Source{}, ErrTooBig
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Source{}, err
	}
	return Read(path, data)
}

// isArchiveName reports whether a file's name is an archive's.
func isArchiveName(path string) bool { return strings.EqualFold(filepath.Ext(path), ".zip") }

// Read reads the lists of data, the content of the file at path: an
// archive, JSON, M3U, a table or lines of text. The path's folder is where
// the relative paths of an M3U list start.
func Read(path string, data []byte) (Source, error) {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	ext := strings.ToLower(filepath.Ext(path))
	if bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		if len(data) > MaxArchive {
			return Source{}, ErrArchiveTooBig
		}
		src, err := readArchive(data)
		src.Name = name
		return src, err
	}
	if ext == ".zip" {
		return Source{}, ErrNoDataDownload
	}
	if len(data) > MaxText {
		return Source{}, ErrTooBig
	}
	text := Decode(data)
	src := Source{Name: name}
	var err error
	switch ext {
	case ".json":
		src.Lists, src.Tables, err = readJSON(name, text)
	case ".m3u", ".m3u8":
		src.Lists = readM3U(name, filepath.Dir(path), text)
	case ".csv", ".tsv":
		var t *Table
		if t, err = readTable(name, text); err == nil {
			src.Lists, src.Tables = t.listsOrSelf()
		}
	default:
		src.Lists, src.Tables, err = readAny(name, filepath.Dir(path), text)
	}
	if err != nil {
		return src, err
	}
	if src.Count() == 0 && len(src.Tables) == 0 {
		return src, ErrNoEntries
	}
	return src, nil
}

// ReadText reads the lists of text the user pasted, named name: what it
// holds tells how to read it.
func ReadText(name, text string) (Source, error) {
	if len(text) > MaxText {
		return Source{}, ErrTooBig
	}
	src := Source{Name: name}
	var err error
	src.Lists, src.Tables, err = readAny(name, "", strings.TrimPrefix(text, "\ufeff"))
	if err == nil && src.Count() == 0 && len(src.Tables) == 0 {
		err = ErrNoEntries
	}
	return src, err
}

// readAny reads text that no extension tells: M3U, JSON, a table whose
// columns have names MixMusic knows, else lines of links and "Artist -
// Title".
func readAny(name, dir, text string) ([]List, []*Table, error) {
	trimmed := strings.TrimSpace(text)
	switch {
	case strings.HasPrefix(trimmed, "#EXTM3U") || strings.Contains(trimmed, "#EXTINF"):
		return readM3U(name, dir, text), nil, nil
	case strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "["):
		lists, tables, err := readJSON(name, text)
		if err == nil {
			return lists, tables, nil
		}
	}
	if t, err := readTable(name, text); err == nil && t.found() && len(t.Rows) > 1 {
		lists, _ := t.listsOrSelf()
		return lists, nil, nil
	}
	return readLines(name, dir, text), nil, nil
}

// splitTitle reads "Artist - Title", as M3U lists and text files name
// songs, and the names of files: no dash is a title alone.
func splitTitle(s string) (title string, artists []string) {
	s = strings.TrimSpace(s)
	for _, dash := range []string{" - ", " \u2013 ", " \u2014 "} {
		if a, t, ok := strings.Cut(s, dash); ok && strings.TrimSpace(a) != "" && strings.TrimSpace(t) != "" {
			return strings.TrimSpace(t), []string{strings.TrimSpace(a)}
		}
	}
	return s, nil
}

// String is how a song is named in the logs.
func (e Entry) String() string {
	if len(e.Artists) > 0 {
		return fmt.Sprintf("%s - %s", strings.Join(e.Artists, ", "), e.Title)
	}
	return e.Title
}
