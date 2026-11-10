package localfiles

import (
	"errors"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Reread reads the files at paths again at once, as after MixMusic changed
// their tags, whatever their size and time of change tell: those the
// index holds. The watch of the folders, which hears of the change a
// moment later, finds them read already.
func (x *Index) Reread(paths []string) {
	x.work.Lock()
	defer x.work.Unlock()
	read := map[string]Entry{}
	for _, p := range paths {
		p = filepath.Clean(p)
		if _, ok := x.Entry(p); !ok {
			continue
		}
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			read[p] = x.readFile(p, info)
		}
	}
	if len(read) == 0 {
		return
	}
	x.mu.Lock()
	next := maps.Clone(x.entries)
	for path, e := range read {
		next[path] = e
		delete(x.gone, path)
		delete(x.stats, path)
	}
	x.entries = next
	x.version++
	x.mu.Unlock()
	if err := x.save(); err != nil {
		log.Printf("mixmusic: keeping the index of the music on this computer: %v", err)
	}
	x.relink()
	x.changed()
}

// Move is what the tags of a file are to say for it to join an album of
// the index.
type Move struct {
	// Album and AlbumArtists are the album's title and album artists,
	// none for an album grouped by the folder its files are in.
	Album        string
	AlbumArtists []string
	// Others are the files of the album that take AlbumArtists too: those
	// of an album without album artists, grouped by its folder, which a
	// file of another folder can join only once they all have them.
	Others []string
}

// MoveTo tells what the tags of the file at path are to say for it to
// join the album of id, as the index groups albums: by their titles and
// album artists, or, without album artists, by the folder their files are
// in. An album without album artists that the file isn't in the folder of
// gets those it shows, as its one artist or Various artists. The file
// needn't be one of the index's, as one opened.
func (x *Index) MoveTo(path, id string) (Move, error) {
	a, ok := x.Album(id)
	if !ok || len(a.Tracks) == 0 {
		return Move{}, errors.New("the album is no longer on this computer")
	}
	// A file opened, which no folder of the index holds, can join one of
	// its albums too.
	if e, ok := x.Entry(path); ok {
		if t := e.track(); t.Album != nil && t.Album.ID == id {
			return Move{}, errors.New("the song is on that album already")
		}
	}
	var entries []Entry
	for _, t := range a.Tracks {
		if p, ok := PathOf(t.ID); ok {
			if ae, ok := x.Entry(p); ok {
				entries = append(entries, ae)
			}
		}
	}
	if len(entries) == 0 {
		return Move{}, errors.New("the album is no longer on this computer")
	}
	m := Move{Album: entries[0].Album}
	for _, ae := range entries {
		if len(ae.AlbumArtists) > 0 {
			m.AlbumArtists = slices.Clone(ae.AlbumArtists)
			return m, nil
		}
	}
	if filepath.Dir(entries[0].Path) == filepath.Dir(path) {
		return m, nil // grouped by the folder the file is in already
	}
	m.AlbumArtists = refNames(a.Artists)
	for _, ae := range entries {
		m.Others = append(m.Others, ae.Path)
	}
	return m, nil
}

// refNames are the names of rs.
func refNames(rs []media.Ref) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Name
	}
	return out
}

// KeepCover keeps an image in the folder of covers, as one a file holds,
// and returns its address, which the windows show it from, as a cover
// about to be written to a file: false when it can't, or it's of a kind
// the windows don't show.
func (x *Index) KeepCover(data []byte, mime string) (string, bool) {
	name := x.keepArt(data, mime)
	return artScheme + name, name != ""
}
