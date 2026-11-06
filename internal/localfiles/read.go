package localfiles

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/tags"
)

// coverNames are the images beside a file that are its album's cover,
// when the file holds none, in the order they're looked for.
var coverNames = []string{"cover", "folder", "front", "album"}

// maxCover bounds the bytes of a cover beside the files that is kept.
const maxCover = 16 << 20

// readFile reads a file's tags, whether the engine plays it, and how long
// it plays, and keeps its cover: a broken file, or one the engine can't
// play, is an entry of why.
func (x *Index) readFile(path string, info fs.FileInfo) (e Entry) {
	e = Entry{Path: path, Size: info.Size(), Modified: info.ModTime().UnixNano()}
	defer func() {
		// Tags made to break what reads them are a broken file.
		if p := recover(); p != nil {
			log.Printf("mixmusic: reading %s: %v", path, p)
			e = Entry{Path: path, Size: info.Size(), Modified: info.ModTime().UnixNano(), Skipped: "it's broken"}
		}
	}()
	f, err := os.Open(path)
	if err != nil {
		e.Skipped = "it can't be read"
		return e
	}
	defer f.Close()
	d, err := x.probe(f)
	if err != nil {
		e.Skipped = skipReason(err)
		return e
	}
	e.DurationMs = int(d / time.Millisecond)
	t, err := x.readTags(f)
	if err != nil && !errors.Is(err, tags.ErrUnknown) {
		return e
	}
	e.Title, e.Artists, e.Album, e.AlbumArtists = t.Title, t.Artists, t.Album, t.AlbumArtists
	e.Track, e.Disc, e.Year = t.Track, t.Disc, t.Year
	if t.Picture != nil {
		e.Art = x.keepArt(t.Picture.Data, t.Picture.MIME)
	}
	if e.Art == "" {
		e.Art = x.coverBeside(path)
	}
	return e
}

// skipReason words why the engine can't play a file.
func skipReason(err error) string {
	msg := err.Error()
	if _, why, ok := strings.Cut(msg, ": "); ok {
		msg = why
	}
	return msg
}

// keepArt keeps a cover in the folder of covers, under its hash, and
// returns its file's name: "" when it can't.
func (x *Index) keepArt(data []byte, mime string) string {
	ext := ""
	switch mime {
	case "image/jpeg":
		ext = ".jpg"
	case "image/png":
		ext = ".png"
	case "image/gif":
		ext = ".gif"
	case "image/webp":
		ext = ".webp"
	default:
		return ""
	}
	sum := sha256.Sum256(data)
	name := hex.EncodeToString(sum[:16]) + ext
	path := filepath.Join(x.art, name)
	if _, err := os.Stat(path); err == nil {
		return name
	}
	if err := os.MkdirAll(x.art, 0o755); err != nil {
		return ""
	}
	tmp, err := os.CreateTemp(x.art, "cover-*")
	if err != nil {
		return ""
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), path) != nil {
		os.Remove(tmp.Name())
		// Another file of the same cover, read beside, may have kept it.
		if _, err := os.Stat(path); err == nil && werr == nil && cerr == nil {
			return name
		}
		return ""
	}
	return name
}

// coverBeside keeps the image beside a file that is its album's cover, as
// cover.jpg or folder.png: "" when there is none.
func (x *Index) coverBeside(path string) string {
	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, want := range coverNames {
		for _, d := range entries {
			name := strings.ToLower(d.Name())
			ext := filepath.Ext(name)
			if d.IsDir() || strings.TrimSuffix(name, ext) != want {
				continue
			}
			mime := map[string]string{".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp"}[ext]
			if mime == "" {
				continue
			}
			f, err := os.Open(filepath.Join(dir, d.Name()))
			if err != nil {
				continue
			}
			data, err := io.ReadAll(io.LimitReader(f, maxCover+1))
			f.Close()
			if err != nil || len(data) > maxCover {
				continue
			}
			return x.keepArt(data, mime)
		}
	}
	return ""
}

// hasArt reports whether the cover of a file is in the folder of covers,
// or the file has none.
func (x *Index) hasArt(name string) bool {
	if name == "" {
		return true
	}
	_, err := os.Stat(filepath.Join(x.art, name))
	return err == nil
}

// ArtFile returns the file of a cover the index keeps, by its address.
func (x *Index) ArtFile(url string) (string, bool) {
	name, ok := strings.CutPrefix(url, artScheme)
	if !ok || name == "" || strings.ContainsAny(name, `/\:`) || strings.Contains(name, "..") {
		return "", false
	}
	return filepath.Join(x.art, name), true
}

// artScheme starts the addresses of the covers the index keeps, which
// the windows show from their files, and nothing fetches.
const artScheme = "localart:"

// ReadFile reads a file the user opened, which no folder of the index
// holds, as a scan reads one: its entry, or why it can't play.
func (x *Index) ReadFile(path string) (Entry, error) {
	path = filepath.Clean(path)
	if e, ok := x.Entry(path); ok {
		if info, err := os.Stat(path); err == nil && e.Size == info.Size() && e.Modified == info.ModTime().UnixNano() {
			return e, nil
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return Entry{}, err
	}
	if info.IsDir() {
		return Entry{}, errors.New("it's a folder")
	}
	e := x.readFile(path, info)
	if e.Skipped != "" {
		return e, errors.New(e.Skipped)
	}
	return e, nil
}
