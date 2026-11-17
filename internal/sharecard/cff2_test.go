package sharecard

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ot "github.com/go-text/typesetting/font/opentype"
)

// TestCFF2Font draws Japanese letters on a system whose only CJK font is
// the variable Noto Sans CJK, whose outlines are in a CFF2 table: the font
// is found, read, and draws at the weight of each line, though its default
// is the thinnest.
func TestCFF2Font(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "fonts", "NotoSansCJKjp-VF-subset.otf"))
	if err != nil {
		t.Fatal(err)
	}
	ld, err := ot.NewLoader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if !ld.HasTable(ot.MustNewTag("CFF2")) || ld.HasTable(ot.MustNewTag("glyf")) || ld.HasTable(ot.MustNewTag("CFF ")) {
		t.Fatalf("the fixture's outlines aren't CFF2 alone: %v", ld.Tables())
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	old := fontconfigFile
	t.Cleanup(func() { fontconfigFile = old })
	fontconfigFile = func(string) string { return "" }
	dir := filepath.Join(home, "data", "fonts", "google-noto-sans-cjk-vf-fonts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "NotoSansCJK-VF.ttc"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	files := systemFontFiles("linux")
	if len(files) == 0 || !strings.HasPrefix(files[0], dir) {
		t.Fatalf("found %v, want the font put in %s first", files, dir)
	}
	fam := family(t)
	fam.Fallback = loadFonts(files[:1])
	if len(fam.Fallback) != 1 {
		t.Fatalf("read %d fonts, want the CFF2 one", len(fam.Fallback))
	}
	const title = "星空の下で歌う"
	if !fallbackFor(fam, title) {
		t.Fatal("the CFF2 font doesn't have the title's letters")
	}
	inkAt := func(weight int) int {
		c := Card{Size: Square, Colors: dark, Lines: []Line{{Text: title, Size: 80, Weight: weight}}}
		img, err := c.Render(fam)
		if err != nil {
			t.Fatal(err)
		}
		ground, _ := Card{Size: Square, Colors: dark}.Render(fam)
		return ink(img, ground)
	}
	regular, bold := inkAt(400), inkAt(700)
	if regular < 2000 {
		t.Errorf("the letters draw %d pixels", regular)
	}
	if bold <= regular*11/10 {
		t.Errorf("bold draws %d pixels, regular %d: the weight isn't set", bold, regular)
	}
}
