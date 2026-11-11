package export

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxName bounds the bytes of a name before its number and extension:
// the systems allow 255 for a whole name, of bytes on Linux, of UTF-16
// units on Windows and of characters on macOS.
const maxName = 200

// Name is the name of a track's file, without its extension: "Artist -
// Title", its artists joined by commas, made safe for the names of files
// on Windows, macOS and Linux. Characters no system allows give way to
// ones that read alike, or go; a name Windows keeps for a device, as CON
// or COM1, gets an underscore after it; and a long one is cut at a
// character's end.
func Name(artists []string, title string) string {
	var names []string
	for _, a := range artists {
		if a = strings.TrimSpace(a); a != "" {
			names = append(names, a)
		}
	}
	title = strings.TrimSpace(title)
	name := title
	if len(names) > 0 {
		name = strings.Join(names, ", ") + " - " + title
		if title == "" {
			name = strings.Join(names, ", ")
		}
	}
	return safe(name)
}

// safe makes name one every system takes as a file's: "Untitled" when
// nothing of it stays.
func safe(name string) string {
	var b strings.Builder
	for _, r := range strings.ToValidUTF8(name, "") {
		switch {
		case r == '/' || r == '\\' || r == '|' || r == ':':
			b.WriteRune('-')
		case r == '"':
			b.WriteRune('\'')
		case r == '<' || r == '>' || r == '?' || r == '*':
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		case unicode.IsControl(r) || r == 0xFEFF:
		default:
			b.WriteRune(r)
		}
	}
	name = strings.Join(strings.Fields(b.String()), " ")
	if len(name) > maxName {
		name = name[:maxName]
		for !utf8.ValidString(name) {
			name = name[:len(name)-1]
		}
	}
	// Windows drops dots and spaces at a name's end, and a dot first hides
	// a file on macOS and Linux.
	name = strings.TrimLeft(strings.TrimRight(name, ". "), ". ")
	if name == "" {
		return "Untitled"
	}
	if reserved(name) {
		name += "_"
	}
	return name
}

// reserved reports whether Windows keeps name for a device, as CON or
// COM1, with an extension or without: AUX.txt is AUX.
func reserved(name string) bool {
	base, _, _ := strings.Cut(name, ".")
	base = strings.ToUpper(strings.TrimRight(base, " "))
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return true
	}
	for _, device := range []string{"COM", "LPT"} {
		if n, ok := strings.CutPrefix(base, device); ok && utf8.RuneCountInString(n) == 1 && strings.ContainsAny(n, "0123456789¹²³") {
			return true
		}
	}
	return false
}

// claim makes a new, empty file in dir named name and ext, or, when a file
// of that name is there, "name (2)", "name (3)" and on: two tracks of one
// name keep a file each, and nothing that was there is written over. It
// returns the file's path.
func claim(dir, name, ext string) (string, error) {
	for n := 1; n < 10000; n++ {
		file := name + ext
		if n > 1 {
			file = fmt.Sprintf("%s (%d)%s", name, n, ext)
		}
		path := filepath.Join(dir, file)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		switch {
		case err == nil:
			return path, f.Close()
		case errors.Is(err, fs.ErrExist):
			continue
		}
		return "", err
	}
	return "", fmt.Errorf("the folder holds too many files named %s", name)
}
