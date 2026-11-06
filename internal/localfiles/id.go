package localfiles

import (
	"encoding/base64"
	"path/filepath"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
)

// A local track's ID is its file's path, in base64, so that it names the
// file wherever the track is kept, a mix or the queue of a run before,
// and plays as long as the file is there: "file:<path>". Its album's and
// its artists' name what the index groups them by: "file:album:<key>"
// and "file:artist:<name>". base64url has no colon, so that the three
// never meet.
const (
	albumPrefix  = "album:"
	artistPrefix = "artist:"
)

var b64 = base64.RawURLEncoding

// TrackID returns the ID of the track of the file at path.
func TrackID(path string) string {
	return media.ID(media.SourceFile, b64.EncodeToString([]byte(filepath.Clean(path))))
}

// PathOf returns the path of the file of a local track's ID, and false
// for an ID that names no file, as an album's.
func PathOf(id string) (string, bool) {
	source, native, err := media.ParseID(id)
	if err != nil || source != media.SourceFile || strings.Contains(native, ":") {
		return "", false
	}
	b, err := b64.DecodeString(native)
	if err != nil || len(b) == 0 || !filepath.IsAbs(string(b)) {
		return "", false
	}
	return string(b), true
}

// albumID returns the ID of the album the index groups by key.
func albumID(key string) string {
	return media.ID(media.SourceFile, albumPrefix+b64.EncodeToString([]byte(key)))
}

// artistID returns the ID of the artist of a name.
func artistID(name string) string {
	return media.ID(media.SourceFile, artistPrefix+b64.EncodeToString([]byte(fold(name))))
}

// keyOf returns what an album's or an artist's ID names, after prefix, or
// false for another ID.
func keyOf(id, prefix string) (string, bool) {
	source, native, err := media.ParseID(id)
	if err != nil || source != media.SourceFile {
		return "", false
	}
	rest, ok := strings.CutPrefix(native, prefix)
	if !ok {
		return "", false
	}
	b, err := b64.DecodeString(rest)
	return string(b), err == nil
}

// IsAlbum and IsArtist report whether an ID is a local album's or artist's.
func IsAlbum(id string) bool  { _, ok := keyOf(id, albumPrefix); return ok }
func IsArtist(id string) bool { _, ok := keyOf(id, artistPrefix); return ok }
