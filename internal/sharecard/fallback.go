package sharecard

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/image/font/sfnt"
)

// The bundled faces have Latin, Greek and Cyrillic letters only. The
// letters of other scripts, Hebrew, Arabic and Chinese, Japanese and
// Korean ones, come from fonts the system has, when it has them: they are
// not bundled, for their size.

// systemFonts are the files of the fonts to look for, by system, in the
// order a letter is looked up in them.
var systemFonts = map[string][]string{
	"windows": {
		`segoeui.ttf`, `arial.ttf`, `YuGothR.ttc`, `msyh.ttc`, `malgun.ttf`, `meiryo.ttc`, `msjh.ttc`, `simsun.ttc`,
	},
	"darwin": {
		`/System/Library/Fonts/Supplemental/Arial Unicode.ttf`, `/System/Library/Fonts/ArialHB.ttc`,
		`/System/Library/Fonts/GeezaPro.ttc`, `/System/Library/Fonts/PingFang.ttc`,
		`/System/Library/Fonts/Hiragino Sans GB.ttc`, `/System/Library/Fonts/AppleSDGothicNeo.ttc`,
		`/System/Library/Fonts/Supplemental/Arial.ttf`,
	},
}

// linuxFonts are the names of files to find under the fonts folders of
// Linux, which differ by distribution.
var linuxFonts = []string{"DejaVuSans.ttf", "NotoSansHebrew-Regular.ttf", "NotoSansArabic-Regular.ttf", "NotoSansCJK-Regular.ttc",
	"NotoSansCJKjp-Regular.otf", "wqy-microhei.ttc", "wqy-zenhei.ttc"}

var (
	fallbackOnce  sync.Once
	fallbackFonts []*sfnt.Font
)

// SystemFallback returns the fonts of the system that have letters the
// bundled faces lack, which it reads once, and none when there are none.
func SystemFallback() []*sfnt.Font {
	fallbackOnce.Do(func() {
		for _, file := range systemFontFiles(runtime.GOOS) {
			data, err := os.ReadFile(file)
			if err != nil || len(data) > 64<<20 {
				continue
			}
			if strings.HasSuffix(strings.ToLower(file), ".ttc") {
				if fonts, err := ParseCollectionFonts(data); err == nil && len(fonts) > 0 {
					fallbackFonts = append(fallbackFonts, fonts[0])
				}
				continue
			}
			if f, err := ParseFont(data); err == nil {
				fallbackFonts = append(fallbackFonts, f)
			}
		}
	})
	return fallbackFonts
}

// systemFontFiles are the paths of the files of fallback fonts on goos.
func systemFontFiles(goos string) []string {
	switch goos {
	case "windows":
		dir := filepath.Join(os.Getenv("SystemRoot"), "Fonts")
		if os.Getenv("SystemRoot") == "" {
			dir = `C:\Windows\Fonts`
		}
		var out []string
		for _, f := range systemFonts[goos] {
			out = append(out, filepath.Join(dir, f))
		}
		return out
	case "darwin":
		return systemFonts[goos]
	}
	var out []string
	for _, dir := range []string{"/usr/share/fonts", "/usr/local/share/fonts"} {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			for _, name := range linuxFonts {
				if d.Name() == name {
					out = append(out, path)
				}
			}
			return nil
		})
	}
	return out
}
