package sharecard

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// The bundled faces have Latin letters, and Selawik, the default, has
// neither Greek nor Cyrillic ones. The letters of every other script come
// from fonts the system has, when it has them: they are not bundled, for
// their size. SystemFallback looks for the fonts the window draws those
// scripts in, so that a card shows what the window shows.

// systemFonts are the files of the fonts to look for, by system, in the
// order a letter is looked up in them. A file is looked for in each of
// fontFolders, and the first one found counts.
var systemFonts = map[string][]string{
	"windows": {
		`segoeui.ttf`, `arial.ttf`,
		// Devanagari, Bengali, Tamil and the other scripts of India; Thai,
		// Lao and Khmer; Ethiopic, Cherokee and more of Africa; Tibetan;
		// Javanese; Myanmar; Georgian and Armenian; Thaana.
		`Nirmala.ttf`, `LeelawUI.ttf`, `ebrima.ttf`, `gadugi.ttf`, `himalaya.ttf`, `javatext.ttf`, `mmrtext.ttf`,
		`sylfaen.ttf`, `mvboli.ttf`,
		`YuGothR.ttc`, `msyh.ttc`, `malgun.ttf`, `meiryo.ttc`, `msjh.ttc`, `simsun.ttc`,
		`seguisym.ttf`,
	},
	"darwin": {
		`Arial Unicode.ttf`, `ArialHB.ttc`, `GeezaPro.ttc`,
		`Kohinoor.ttc`, `KohinoorBangla.ttc`, `KohinoorGujarati.ttc`, `KohinoorTelugu.ttc`,
		`Devanagari Sangam MN.ttc`, `Bangla Sangam MN.ttc`, `Gujarati Sangam MN.ttc`, `Gurmukhi Sangam MN.ttc`,
		`Tamil Sangam MN.ttc`, `Telugu Sangam MN.ttc`, `Kannada Sangam MN.ttc`, `Malayalam Sangam MN.ttc`,
		`Sinhala Sangam MN.ttc`, `NotoSansOriya.ttc`, `NotoSansKannada.ttc`, `NotoSansMyanmar.ttc`,
		`Myanmar Sangam MN.ttc`, `Khmer Sangam MN.ttf`, `Lao Sangam MN.ttf`, `ThonburiUI.ttc`, `Thonburi.ttc`,
		`KefaIII.ttf`, `NotoSansArmenian.ttc`, `SFArmenian.ttf`, `SFGeorgian.ttf`, `SFHebrew.ttf`,
		`PingFang.ttc`, `Hiragino Sans GB.ttc`, `AppleSDGothicNeo.ttc`, `STHeiti Light.ttc`,
		`Arial.ttf`, `Apple Symbols.ttf`,
	},
}

// fontFolders returns the folders of the fonts of goos, in the order they
// count. The user's own come first, so that a font they installed wins.
func fontFolders(goos string) []string {
	home, _ := os.UserHomeDir()
	switch goos {
	case "windows":
		root := os.Getenv("SystemRoot")
		if root == "" {
			root = `C:\Windows`
		}
		dirs := []string{filepath.Join(root, "Fonts")}
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			dirs = append(dirs, filepath.Join(local, "Microsoft", "Windows", "Fonts"))
		}
		return dirs
	case "darwin":
		dirs := []string{"/System/Library/Fonts", "/System/Library/Fonts/Supplemental", "/Library/Fonts"}
		if home != "" {
			dirs = append(dirs, filepath.Join(home, "Library", "Fonts"))
		}
		return dirs
	}
	var dirs []string
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" && home != "" {
		data = filepath.Join(home, ".local", "share")
	}
	if data != "" {
		dirs = append(dirs, filepath.Join(data, "fonts"))
	}
	if home != "" {
		dirs = append(dirs, filepath.Join(home, ".fonts"))
	}
	shared := os.Getenv("XDG_DATA_DIRS")
	if shared == "" {
		shared = "/usr/local/share:/usr/share"
	}
	for _, d := range filepath.SplitList(shared) {
		if d != "" {
			dirs = append(dirs, filepath.Join(d, "fonts"))
		}
	}
	return append(dirs, "/usr/share/fonts", "/usr/local/share/fonts")
}

// linuxScripts are the scripts of Noto Sans, in the order a letter is
// looked up in them, as the pattern of the part of a file's name after
// "NotoSans". Distributions name the files differently: with -Regular,
// with -VF for a variable font, with the axes in brackets, with UI after
// the script, and the CJK fonts as a collection or a font by language.
var linuxScripts = []string{
	"", "Devanagari", "Bengali", "Thai", "Ethiopic", "Tamil", "Telugu", "Kannada", "Malayalam", "Gujarati",
	"Gurmukhi", "Sinhala", "Khmer", "Lao", "Myanmar", "Tibetan", "Armenian", "Georgian", "Hebrew", "Arabic",
	"Oriya", "Thaana", "Javanese", "Cherokee", "CJK(jp|kr|sc|tc|hk)?", "Symbols", "Symbols2",
}

// linuxFonts are the other fonts to find, by name, after the ones of
// linuxScripts.
var linuxFonts = []string{
	"dejavusans.ttf", "wqy-microhei.ttc", "wqy-zenhei.ttc", "droidsansfallbackfull.ttf", "droidsansfallback.ttf",
	"unifont.ttf",
}

// linuxNoto has a pattern for the file names of each of linuxScripts.
var linuxNoto = func() []*regexp.Regexp {
	var out []*regexp.Regexp
	for _, s := range linuxScripts {
		out = append(out, regexp.MustCompile(`^(?i)notosans`+s+`(ui)?(-regular|-vf|\[[^\]]*\])?\.(ttf|otf|ttc)$`))
	}
	return out
}()

// linuxRank is how early name comes in the lookup order, from 0, and false
// when it is not a fallback font.
func linuxRank(name string) (int, bool) {
	for i, re := range linuxNoto {
		if re.MatchString(name) {
			return i, true
		}
	}
	name = strings.ToLower(name)
	for i, f := range linuxFonts {
		if name == f {
			return len(linuxScripts) + i, true
		}
	}
	return 0, false
}

// fontLanguages are the languages fontconfig is asked for a font of, on
// Linux, after the files found by name: the fonts a user set for a script
// count that way, whatever their file is called.
var fontLanguages = []string{
	"hi", "bn", "th", "am", "ta", "te", "kn", "ml", "gu", "pa", "si", "km", "lo", "my", "bo", "hy", "ka", "he",
	"ar", "ja", "zh-cn", "zh-tw", "ko",
}

// fontconfigFile asks fontconfig for the file of the font that has the
// letters of lang, and gives "" when it has none or fontconfig is absent.
// It is a variable for the tests.
var fontconfigFile = func(lang string) string {
	path, err := exec.LookPath("fc-match")
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--format=%{file}", ":lang="+lang).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

var (
	fallbackOnce  sync.Once
	fallbackFonts []*sfnt.Font
)

// SystemFallback returns the fonts of the system that have letters the
// bundled faces lack, which it reads once, and none when there are none.
func SystemFallback() []*sfnt.Font {
	fallbackOnce.Do(func() {
		fallbackFonts = loadFonts(systemFontFiles(runtime.GOOS))
	})
	return fallbackFonts
}

// loadFonts reads the fonts of files, skipping the ones it can't read or
// can't draw, and the same font twice.
func loadFonts(files []string) []*sfnt.Font {
	var out []*sfnt.Font
	seen := map[string]bool{}
	for _, file := range files {
		if seen[file] {
			continue
		}
		seen[file] = true
		data, err := os.ReadFile(file)
		if err != nil || len(data) > 64<<20 {
			continue
		}
		var f *sfnt.Font
		if strings.HasSuffix(strings.ToLower(file), ".ttc") {
			if fonts, err := ParseCollectionFonts(data); err == nil && len(fonts) > 0 {
				f = fonts[0]
			}
		} else if font, err := ParseFont(data); err == nil {
			f = font
		}
		if f != nil && drawable(f) {
			out = append(out, f)
		}
	}
	return out
}

// drawable reports whether the outlines of f can be read. Fonts whose
// outlines are in a CFF2 table, such as a variable CJK font, parse but
// have none that golang.org/x/image can draw, so they would leave letters
// blank that a later fallback could draw.
func drawable(f *sfnt.Font) bool {
	var buf sfnt.Buffer
	for i := 1; i < f.NumGlyphs() && i < 64; i++ {
		segs, err := f.LoadGlyph(&buf, sfnt.GlyphIndex(i), fixed.I(16), nil)
		if err != nil {
			return false
		}
		if len(segs) > 0 {
			return true
		}
	}
	return f.NumGlyphs() > 1
}

// systemFontFiles are the paths of the files of fallback fonts on goos.
func systemFontFiles(goos string) []string {
	dirs := fontFolders(goos)
	var out []string
	switch goos {
	case "windows", "darwin":
		for _, name := range systemFonts[goos] {
			for _, dir := range dirs {
				file := filepath.Join(dir, name)
				if _, err := os.Stat(file); err == nil {
					out = append(out, file)
					break
				}
			}
		}
		return out
	}
	// Each name is taken from the first folder that has it; the order is
	// that of the scripts, then of the folders.
	type found struct {
		path string
		rank int
	}
	var all []found
	have := map[string]bool{}
	for _, dir := range dirs {
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if rank, ok := linuxRank(d.Name()); ok && !have[strings.ToLower(d.Name())] {
				have[strings.ToLower(d.Name())] = true
				all = append(all, found{path, rank})
			}
			return nil
		})
	}
	for rank := 0; rank <= len(linuxScripts)+len(linuxFonts); rank++ {
		for _, f := range all {
			if f.rank == rank {
				out = append(out, f.path)
			}
		}
	}
	for _, lang := range fontLanguages {
		if file := fontconfigFile(lang); file != "" {
			out = append(out, file)
		}
	}
	return out
}
