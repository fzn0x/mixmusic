package sharecard

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
)

// Urdu is read in Nastaliq, the sloping hand its books and papers are set
// in; the upright Naskh of Arabic reads as another language's. A card lays
// out its own lines, as tall as the face they are set in, so a card in
// Urdu can set Urdu's letters in a Nastaliq face the system has, before
// the other fallbacks, whose Arabic letters are Naskh (#564).

// nastaliqFonts are the files of the Nastaliq faces to look for, by
// system, the first found being the one a card takes: Noto Nastaliq Urdu,
// which macOS carries and a user may install, then Windows' own Urdu
// Typesetting, which comes with Windows' Urdu supplemental fonts. A file is
// looked for in each of fontFolders.
var nastaliqFonts = map[string][]string{
	"windows": {`NotoNastaliqUrdu-Regular.ttf`, `NotoNastaliqUrdu[wght].ttf`, `UrdType.ttf`},
	"darwin":  {`NotoNastaliq.ttc`, `NotoNastaliqUrdu-Regular.ttf`, `NotoNastaliqUrdu[wght].ttf`},
}

// linuxNastaliq is the pattern of the names distributions give Noto
// Nastaliq Urdu's file: with -Regular, with -VF for a variable font, or
// with the axes in brackets.
var linuxNastaliq = regexp.MustCompile(`^(?i)notonastaliqurdu(-regular|-vf|\[[^\]]*\])?\.(ttf|otf|ttc)$`)

var (
	nastaliqOnce sync.Once
	nastaliq     []*Font
)

// Nastaliq returns the Nastaliq face of the system, for the letters of a
// card in Urdu, or none when the system has none. It looks once.
func Nastaliq() []*Font {
	nastaliqOnce.Do(func() {
		if file := nastaliqFile(runtime.GOOS); file != "" {
			nastaliq = loadFonts([]string{file})
		}
	})
	return nastaliq
}

// nastaliqFile is the path of the file of the Nastaliq face on goos, or ""
// when it has none.
func nastaliqFile(goos string) string {
	dirs := fontFolders(goos)
	if names, ok := nastaliqFonts[goos]; ok {
		for _, name := range names {
			for _, dir := range dirs {
				file := filepath.Join(dir, name)
				if _, err := os.Stat(file); err == nil {
					return file
				}
			}
		}
		return ""
	}
	// The user's folders come first, as in systemFontFiles.
	for _, dir := range dirs {
		var found string
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if linuxNastaliq.MatchString(d.Name()) {
				found = path
				return fs.SkipAll
			}
			return nil
		})
		if found != "" {
			return found
		}
	}
	return ""
}
