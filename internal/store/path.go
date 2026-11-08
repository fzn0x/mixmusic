package store

import (
	"encoding/base64"
	"strings"
	"unicode/utf8"
)

// A path is text in a JSON file, which encoding/json writes as UTF-8:
// the bytes of one that aren't, as a file's name on Linux may hold, are
// written as replacement characters, and the path read back names no
// file. Such a path is kept in base64, after pathPrefix, which no
// absolute path starts with; one that is UTF-8 is kept as it is, so that
// the file reads as text.
const pathPrefix = "base64:"

// EncodePath returns path as a JSON file keeps it: itself, unless it
// isn't UTF-8, or starts as one in base64 does.
func EncodePath(path string) string {
	if utf8.ValidString(path) && !strings.HasPrefix(path, pathPrefix) {
		return path
	}
	return pathPrefix + base64.StdEncoding.EncodeToString([]byte(path))
}

// DecodePath returns the path a JSON file keeps as text, as EncodePath
// wrote it, or as the versions before 0.5.11 wrote any path: the text
// itself.
func DecodePath(text string) string {
	rest, ok := strings.CutPrefix(text, pathPrefix)
	if !ok {
		return text
	}
	b, err := base64.StdEncoding.DecodeString(rest)
	if err != nil {
		return text
	}
	return string(b)
}
