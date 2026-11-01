package downloads

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/store"
)

// indexVersion is the version of index.json this MixMusic writes.
const indexVersion = 1

// index is what the folder's index.json keeps: the tracks downloaded or
// waiting to be, the files that hold their audio, the albums, playlists
// and mixes downloaded whole, and their covers. Its paths are relative to
// the folder, with forward slashes, so that the folder moves whole.
type index struct {
	Version int  `json:"version"`
	Paused  bool `json:"paused,omitempty"`
	// Seq orders the tracks as they were asked for.
	Seq    int64             `json:"seq"`
	Tracks map[string]*entry `json:"tracks"`
	// Files are the audio of the videos downloaded, by video ID: a track
	// and another of the same recording, as a Spotify track and its match,
	// share one.
	Files map[string]*file `json:"files"`
	// Parts are the downloads under way, by video ID: what the .part file
	// is a part of, to go on with it only from the same stream.
	Parts       map[string]string      `json:"parts,omitempty"`
	Collections map[string]*Collection `json:"collections"`
	// Art maps the addresses of covers to the files that keep them.
	Art map[string]string `json:"art,omitempty"`
	// Pending are the files that couldn't be deleted, as one open on
	// Windows, which go at the next start.
	Pending []string `json:"pending,omitempty"`
}

// entry is a track downloaded, or waiting to be.
type entry struct {
	Track media.Track `json:"track"`
	// Recording is a Spotify track's recording on YouTube Music, once
	// found: what downloads, and plays.
	Recording *media.Track `json:"recording,omitempty"`
	// Quality is the quality the download was asked in: "best" or
	// "saver".
	Quality string `json:"quality"`
	// Pins are what keeps the track: "track" when it was downloaded on its
	// own, and the IDs of the collections downloaded that hold it. A track
	// without pins goes.
	Pins  []string  `json:"pins"`
	Seq   int64     `json:"seq"`
	Added time.Time `json:"added"`
	// Failed is why the download failed for good, in plain words, until
	// it is tried again.
	Failed string `json:"failed,omitempty"`
	// Art is the cover kept, relative to the folder.
	Art string `json:"art,omitempty"`
}

// pinTrack is the pin of a track downloaded on its own.
const pinTrack = "track"

// file is the audio of a video, downloaded.
type file struct {
	// Path is relative to the folder, as "audio/<video id>.webm".
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
	// Format is the container, "webm" or "mp4", and Codec the audio's,
	// "opus" or "mp4a.40.2": what came, which plays.
	Format  string `json:"format"`
	Codec   string `json:"codec"`
	Itag    int    `json:"itag,omitempty"`
	Bitrate int    `json:"bitrate"` // kbit/s, as measured
	Quality string `json:"quality"`
	// Cached is set when the audio came from the offline cache.
	Cached bool      `json:"cached,omitempty"`
	Done   time.Time `json:"done"`
}

// Collection is an album, a playlist or a mix downloaded whole, or the
// liked songs.
type Collection struct {
	// ID is the page's own: "album:<id>", "playlist:<id>" or "liked".
	ID string `json:"id"`
	// Kind is "album", "playlist", "mix" or "liked".
	Kind    string        `json:"kind"`
	Title   string        `json:"title"`
	Source  media.Source  `json:"source"`
	Artwork []media.Image `json:"artwork,omitempty"`
	// Tracks are its tracks' IDs, in its order, as it was downloaded.
	Tracks []string `json:"tracks"`
	// Total is how many tracks its service counts, when it gave fewer:
	// MixMusic reads about a thousand tracks of a playlist at most.
	Total int       `json:"total,omitempty"`
	Added time.Time `json:"added"`
	// Art is the cover kept, relative to the folder.
	Art string `json:"art,omitempty"`
}

func newIndex() *index {
	return &index{Version: indexVersion, Tracks: map[string]*entry{}, Files: map[string]*file{}, Parts: map[string]string{},
		Collections: map[string]*Collection{}, Art: map[string]string{}}
}

// readIndex reads the index of the folder dir: an empty one when it has
// none, and when it doesn't decode, which goes aside to index.json.broken,
// so that a corrupt index never keeps MixMusic from starting. It tells
// whether the folder held one.
func readIndex(dir string) (*index, bool, error) {
	path := filepath.Join(dir, "index.json")
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return newIndex(), false, nil
	case err != nil:
		return nil, false, err
	}
	idx := newIndex()
	if err := json.Unmarshal(data, idx); err != nil || idx.Version > indexVersion {
		if err == nil {
			// A later MixMusic's: kept as it is, and not read.
			return nil, true, errors.New("the downloads were kept by a later version of MixMusic")
		}
		if err := os.Rename(path, path+".broken"); err != nil {
			return nil, true, err
		}
		return newIndex(), false, nil
	}
	idx.fill()
	return idx, true, nil
}

// fill makes the maps an index read without them.
func (x *index) fill() {
	if x.Tracks == nil {
		x.Tracks = map[string]*entry{}
	}
	if x.Files == nil {
		x.Files = map[string]*file{}
	}
	if x.Parts == nil {
		x.Parts = map[string]string{}
	}
	if x.Collections == nil {
		x.Collections = map[string]*Collection{}
	}
	if x.Art == nil {
		x.Art = map[string]string{}
	}
	for id, e := range x.Tracks {
		if e == nil {
			delete(x.Tracks, id)
		}
	}
	for id, f := range x.Files {
		if f == nil || !safePath(f.Path) {
			delete(x.Files, id)
		}
	}
	for id, c := range x.Collections {
		if c == nil {
			delete(x.Collections, id)
		}
	}
	for u, p := range x.Art {
		if !safePath(p) {
			delete(x.Art, u)
		}
	}
	x.Version = indexVersion
}

// write keeps the index in the folder dir, atomically.
func (x *index) write(dir string) error {
	data, err := json.Marshal(x)
	if err != nil {
		return err
	}
	return store.WriteFile(filepath.Join(dir, "index.json"), append(data, '\n'))
}

// safePath reports whether p, a path of the index, names a file of the
// folder's audio or art, and nothing out of it.
func safePath(p string) bool {
	dir, name := filepath.Split(filepath.FromSlash(p))
	dir = filepath.Clean(dir)
	return (dir == "audio" || dir == "art") && name != "" && name != "." && name != ".." && filepath.Base(name) == name
}

// video returns the ID of the video whose audio plays for e, once known.
func (e *entry) video() string {
	t := e.Track
	if e.Recording != nil {
		t = *e.Recording
	}
	if t.Source != media.SourceYouTube {
		return ""
	}
	if _, id, err := media.ParseID(t.ID); err == nil {
		return id
	}
	return ""
}

func (e *entry) pinned(pin string) bool {
	for _, p := range e.Pins {
		if p == pin {
			return true
		}
	}
	return false
}

func (e *entry) unpin(pin string) {
	out := e.Pins[:0]
	for _, p := range e.Pins {
		if p != pin {
			out = append(out, p)
		}
	}
	e.Pins = out
}
