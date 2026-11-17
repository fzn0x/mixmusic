package app

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/text/unicode/norm"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/tags"
	"github.com/fzn0x/mixmusic/lyrics"
)

// The lyrics a file of this computer holds (#554), in its tags or in a
// .lrc file of its name beside it, show at once, offline too, and for a
// file that names no artist. They are read at each lookup, so that a .lrc
// put beside a file, or tags written, show the next time.

// maxLRC bounds the bytes of a .lrc file that is read.
const maxLRC = 1 << 20

// Lyrics returns the lyrics of a track. Those a file of this computer
// holds (localLyrics) come first: synced, they are final; plain, they
// give way only to synced lyrics of the sources, and stay when the
// sources have none or can't be reached. Other tracks' come from the
// sources of lyrics that are on (onlineLyrics).
func (c *Catalog) Lyrics(ctx context.Context, track media.Track) (lyrics.Lyrics, error) {
	local, ok := c.localLyrics(track)
	if !ok {
		return c.onlineLyrics(ctx, track)
	}
	if local.Synced {
		return local, nil
	}
	if l, err := c.onlineLyrics(ctx, track); err == nil && l.Synced {
		return l, nil
	}
	return local, nil
}

// localLyrics returns the lyrics the files of a track of this computer
// hold, as they are now: of the file that plays it, then of its song's
// other copies, each's .lrc file beside it, then the synced lyrics of its
// tags, then their plain lyrics. The first that are synced win, else the
// first plain ones. Their source is the name of the file that holds them.
// ok is false for a track of no file here, and files of no lyrics.
func (c *Catalog) localLyrics(track media.Track) (lyrics.Lyrics, bool) {
	if track.Source != media.SourceFile || c.a.localFiles == nil {
		return lyrics.Lyrics{}, false
	}
	var best lyrics.Lyrics
	found := false
	// take keeps the lyrics of text, and reports whether they are synced,
	// which no later lyrics beat.
	take := func(text, source string) bool {
		l, ok := lyrics.FromText(text, source)
		if ok && (!found || l.Synced) {
			best, found = l, true
		}
		return ok && l.Synced
	}
	for _, path := range c.lyricsFiles(track.ID) {
		if text, name := lrcBeside(path); text != "" && take(text, name) {
			break
		}
		t := lyricsTags(path)
		if take(t.SyncedLyrics, filepath.Base(path)) || take(t.Lyrics, filepath.Base(path)) {
			break
		}
	}
	if !found {
		return lyrics.Lyrics{}, false
	}
	if best.Language == "" {
		best.Language = lyrics.GuessLanguage(best.Lines)
	}
	return best, true
}

// lyricsFiles returns the paths of the files of a local track whose
// lyrics are read: the one that plays it first, then its song's other
// copies.
func (c *Catalog) lyricsFiles(id string) []string {
	x := c.a.localFiles
	var paths []string
	if p, ok := x.FileOf(id); ok {
		paths = append(paths, p)
	}
	if s, ok := x.Song(id); ok {
		for _, cp := range s.Copies {
			if !slices.Contains(paths, cp.Path) {
				paths = append(paths, cp.Path)
			}
		}
	}
	return paths
}

// lrcBeside returns the text of the .lrc file beside the file at path, of
// its name but for the extension, in any case or Unicode form, as macOS
// writes names decomposed, and the .lrc's name: "" for none, and for one
// larger than maxLRC. A .lrc of the very name comes before the others.
func lrcBeside(path string) (text, name string) {
	dir, file := filepath.Dir(path), filepath.Base(path)
	exact := strings.TrimSuffix(file, filepath.Ext(file)) + ".lrc"
	want := norm.NFC.String(exact)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", ""
	}
	for _, d := range entries {
		if n := d.Name(); !d.IsDir() && strings.EqualFold(norm.NFC.String(n), want) && (name == "" || n == exact) {
			name = n
		}
	}
	if name == "" {
		return "", ""
	}
	f, err := store.OpenShared(filepath.Join(dir, name))
	if err != nil {
		return "", ""
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxLRC+1))
	if err != nil || len(b) > maxLRC {
		return "", ""
	}
	return lyrics.DecodeText(b), name
}

// lyricsTags reads the tags of the file at path, for its lyrics, as the
// file is now, letting a tag editor replace it meanwhile
// (store.OpenShared): none when it can't be read.
func lyricsTags(path string) (t tags.Tags) {
	defer func() {
		// Tags made to break what reads them hold no lyrics, as the index
		// takes them for a broken file.
		if recover() != nil {
			t = tags.Tags{}
		}
	}()
	f, err := store.OpenShared(path)
	if err != nil {
		return tags.Tags{}
	}
	defer f.Close()
	t, _ = tags.Read(f)
	return t
}
