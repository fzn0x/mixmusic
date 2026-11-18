// Package backup keeps MixMusic's data in a file, and brings it back, on
// this computer or another (#592). A backup is a zip of the files of the
// data folder of the parts chosen, with manifest.json, which tells the
// format, the version of MixMusic and the system that made it, when, the
// folders of music and downloads it came from, and each entry's size and
// SHA-256.
//
// Only the files of the parts go in: never the sign-ins, nor the files
// that stand in for the credential store, remote control's pairings, the
// queues of scrobbles still to send, the index of the music, yt-dlp, the
// cache, the logs or any audio.
//
// A restore unpacks the backup, checked, then stages what it replaces in
// the data folder's restore folder; the next start swaps it in before
// anything opens (Apply), and keeps what it replaced, which Undo puts
// back.
package backup

import (
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Format is the format of the backups this MixMusic writes. A backup of an
// older format is brought up to date as it opens; one of a newer format is
// refused.
const Format = 1

// The bounds of a backup MixMusic opens: of its file, and of what it
// unpacks to.
const (
	MaxPacked   = 512 << 20
	MaxUnpacked = 2 << 30
)

// maxPacked and maxUnpacked are the bounds checked: tests lower them.
var maxPacked, maxUnpacked int64 = MaxPacked, MaxUnpacked

// Part is a part of what a backup holds, which the user ticks.
type Part string

const (
	// Library: the liked and disliked songs, the mixes and their covers,
	// the history and the play log of Stats, the searches, the artists
	// and podcasts followed, and the copies of songs linked.
	Library Part = "library"
	// Settings: settings.json.
	Settings Part = "settings"
	// Addons: the themes and plugins, with what plugins keep.
	Addons Part = "addons"
	// Queue: the queue, and where the episodes were left.
	Queue Part = "queue"
	// Downloads: the list of the downloads, without their audio, to
	// download them again.
	Downloads Part = "downloads"
)

// Parts are the parts, in the order the windows list them.
var Parts = []Part{Library, Settings, Addons, Queue, Downloads}

// DownloadsName is the entry of the Downloads part: a DownloadList, which
// isn't a file of the data folder.
const DownloadsName = "downloads.json"

// ManifestName is the entry of the manifest.
const ManifestName = "manifest.json"

// paths are the files and folders of the data folder a part holds, with
// forward slashes.
func (p Part) paths() []string {
	switch p {
	case Library:
		return []string{"library.json", "plays.jsonl", "local-copies.json", "mix-covers"}
	case Settings:
		return []string{"settings.json"}
	case Addons:
		return []string{"addons"}
	case Queue:
		return []string{"queue.json", "positions.json"}
	case Downloads:
		return []string{DownloadsName}
	}
	return nil
}

// holds reports whether name, an entry's path, is of the part p: one of
// its files, or a file in one of its folders.
func (p Part) holds(name string) bool {
	for _, q := range p.paths() {
		if p.folder(q) && strings.HasPrefix(name, q+"/") || !p.folder(q) && name == q {
			return true
		}
	}
	return false
}

// folder reports whether q, of p's paths, is a folder.
func (p Part) folder(q string) bool { return path.Ext(q) == "" }

// known reports whether p is a part this MixMusic knows.
func (p Part) known() bool { return slices.Contains(Parts, p) }

// Manifest is what manifest.json tells of a backup.
type Manifest struct {
	// Format is the backup's format, App the version of MixMusic that
	// made it, and Created when.
	Format  int       `json:"format"`
	App     string    `json:"app"`
	Created time.Time `json:"created"`
	// System and Arch are the system and the processor it was made on, as
	// Go names them: "windows", "darwin", "linux"; "amd64", "arm64".
	System string `json:"system"`
	Arch   string `json:"arch,omitempty"`
	// MusicFolders are the folders of music it came from, and
	// DownloadsFolder the downloads', as that system writes them.
	MusicFolders    []string `json:"musicFolders,omitempty"`
	DownloadsFolder string   `json:"downloadsFolder,omitempty"`
	// Parts are the parts it holds.
	Parts []Part `json:"parts"`
	// Entries are its files, but the manifest.
	Entries []Entry `json:"entries"`
}

// Entry is a file of a backup: its path in the data folder, with forward
// slashes, its part, its size and its SHA-256, in hex.
type Entry struct {
	Path   string `json:"path"`
	Part   Part   `json:"part"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Has reports whether the backup holds part p.
func (m Manifest) Has(p Part) bool { return slices.Contains(m.Parts, p) }

// DownloadList is what the Downloads part holds: the tracks downloaded on
// their own, and the albums, playlists, mixes and liked songs downloaded
// whole, with their tracks, each the first asked for first.
type DownloadList struct {
	Tracks      []media.Track        `json:"tracks"`
	Collections []DownloadCollection `json:"collections"`
}

// DownloadCollection is a collection downloaded whole: "album:<id>",
// "playlist:<id>", a mix's or "liked", of Kind "album", "playlist", "mix"
// or "liked".
type DownloadCollection struct {
	ID      string        `json:"id"`
	Kind    string        `json:"kind"`
	Title   string        `json:"title"`
	Source  media.Source  `json:"source"`
	Artwork []media.Image `json:"artwork,omitempty"`
	// Total is how many tracks its service counts, when it gave fewer.
	Total  int           `json:"total,omitempty"`
	Tracks []media.Track `json:"tracks"`
}

// Songs counts the tracks of l, each once.
func (l DownloadList) Songs() int {
	seen := map[string]bool{}
	for _, t := range l.Tracks {
		seen[t.ID] = true
	}
	for _, c := range l.Collections {
		for _, t := range c.Tracks {
			seen[t.ID] = true
		}
	}
	delete(seen, "")
	return len(seen)
}

// Problem is why a backup is refused.
type Problem string

const (
	// NotBackup: the file isn't a MixMusic backup.
	NotBackup Problem = "not a backup"
	// Damaged: the file, or its Entry, is damaged.
	Damaged Problem = "damaged"
	// Newer: a newer MixMusic made it, of App's version.
	Newer Problem = "newer"
	// TooLarge: its file is past MaxPacked.
	TooLarge Problem = "too large"
	// TooLargeUnpacked: it unpacks to more than MaxUnpacked.
	TooLargeUnpacked Problem = "too large unpacked"
	// Outside: its Entry would land outside the data folder.
	Outside Problem = "outside"
)

// Error is why a backup is refused: its Problem, the entry it is of, if
// any, and the version of MixMusic of a Newer backup.
type Error struct {
	Problem Problem
	Entry   string
	App     string
	Err     error
}

func (e *Error) Error() string {
	var s string
	switch e.Problem {
	case NotBackup:
		s = "it isn't a MixMusic backup"
	case Damaged:
		s = "the backup is damaged"
		if e.Entry != "" {
			s = fmt.Sprintf("%s in the backup is damaged", e.Entry)
		}
	case Newer:
		s = fmt.Sprintf("the backup comes from MixMusic %s, newer than this one", e.App)
	case TooLarge:
		s = fmt.Sprintf("the backup is larger than %d MB", MaxPacked>>20)
	case TooLargeUnpacked:
		s = fmt.Sprintf("the backup unpacks to more than %d GB", MaxUnpacked>>30)
	case Outside:
		s = fmt.Sprintf("%s in the backup would land outside the data folder", e.Entry)
	default:
		s = string(e.Problem)
	}
	if e.Err != nil {
		s += ": " + e.Err.Error()
	}
	return s
}

func (e *Error) Unwrap() error { return e.Err }
