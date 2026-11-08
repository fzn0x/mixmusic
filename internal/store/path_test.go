package store

import (
	"encoding/json"
	"strings"
	"testing"
)

// The paths of these tests are made up; those that aren't UTF-8 hold a
// byte of Latin-1, as a name on a Linux drive may.
var (
	// textPaths are written as they are: UTF-8, of every system's kind,
	// decomposed as macOS's names are, and one that holds a replacement
	// character of its own.
	textPaths = []string{
		"/home/ada/Music", "/home/ada/Café Nights", "/Users/ada/Cafe\u0301 Nights",
		`C:\Users\Zoë\Music`, `\\nas\music\Kites`, "/home/ada/odd \uFFFD name", "~/colors.json", "",
	}
	// bytePaths aren't UTF-8, or would read back as if they were in
	// base64: they're written in base64.
	bytePaths = []string{
		"/home/ada/caf\xe9.mp3", "/media/ada/\xff\xfe/01.flac", "/home/ada/\xe9\xe8", "\xe9", "base64:L2hvbWU=",
	}
)

// TestPathText writes a path as a JSON file keeps it, and reads it back as
// it was: as it is when it's UTF-8, in base64 when it isn't.
func TestPathText(t *testing.T) {
	for _, path := range textPaths {
		if got := EncodePath(path); got != path {
			t.Errorf("%q is written as %q", path, got)
		}
	}
	for _, path := range append(textPaths, bytePaths...) {
		text := EncodePath(path)
		if text != path && !strings.HasPrefix(text, pathPrefix) {
			t.Errorf("%q is written as %q", path, text)
		}
		data, err := json.Marshal(text)
		if err != nil {
			t.Fatal(err)
		}
		var read string
		if err := json.Unmarshal(data, &read); err != nil {
			t.Fatal(err)
		}
		if got := DecodePath(read); got != path {
			t.Errorf("%q reads back as %q, of %s", path, got, data)
		}
	}
	// What the versions before wrote reads as it is, as does text that
	// only starts as a path in base64.
	for _, text := range append(textPaths, "base64:not base64!", "base64") {
		if got := DecodePath(text); got != text {
			t.Errorf("%q reads as %q", text, got)
		}
	}
}
