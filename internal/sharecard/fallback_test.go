package sharecard

import (
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// scriptTitles are titles in the scripts the window draws, by the script.
var scriptTitles = []struct{ script, title string }{
	{"Devanagari", "नमस्ते दुनिया"},
	{"Bengali", "বাংলা গান"},
	{"Thai", "สวัสดีชาวโลก"},
	{"Ethiopic", "ሰላም ለዓለም"},
	{"Tamil", "வணக்கம் உலகம்"},
	{"Hebrew", "שלום עולם"},
	{"Arabic", "مرحبا بالعالم"},
	{"Chinese", "你好，世界"},
	{"Japanese", "こんにちは世界"},
	{"Korean", "안녕하세요"},
}

// TestSystemScripts finds a letter of each script among the fonts of the
// system and draws a card of it, on the systems this runs on that ship
// them: Windows 11 and macOS. A card of each is saved in the folder named
// by MIXMUSIC_CARD_DIR, if it is set, to look at.
func TestSystemScripts(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("Linux has the fonts its user installed")
	}
	fam := family(t)
	if len(fam.Fallback) == 0 {
		t.Skip("the system has no fallback fonts")
	}
	for _, s := range scriptTitles {
		t.Run(s.script, func(t *testing.T) {
			if !fallbackFor(fam, s.title) {
				// A runner may ship no font for a script, as Windows
				// Server has no Nirmala UI: the fonts of the machine
				// decide, and the test below proves the lookup.
				t.Skipf("this %s has no font with every letter of %q (looked for %s)",
					runtime.GOOS, s.title, strings.Join(systemFonts[runtime.GOOS], ", "))
			}
			c := Card{Size: Square, Colors: dark, Lines: []Line{{Text: s.title, Size: 80, Weight: 400}}}
			img, err := c.Render(fam)
			if err != nil {
				t.Fatal(err)
			}
			ground, _ := Card{Size: Square, Colors: dark}.Render(fam)
			if got := ink(img, ground); got < 800 {
				t.Errorf("%q draws %d pixels", s.title, got)
			}
			if dir := os.Getenv("MIXMUSIC_CARD_DIR"); dir != "" {
				f, err := os.Create(filepath.Join(dir, s.script+".png"))
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				if err := png.Encode(f, img); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// touch makes empty files under dir.
func touch(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, n := range names {
		p := filepath.Join(dir, filepath.FromSlash(n))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestLinuxFontFolders finds fonts in the user's own folders and under
// the names Fedora, Debian and Arch give them, and in the order of the
// scripts, with the user's copy of a name before the system's.
func TestLinuxFontFolders(t *testing.T) {
	home := t.TempDir()
	share := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_DATA_DIRS", share)
	touch(t, filepath.Join(home, ".local", "share", "fonts"), "mine/NotoSansThai-Regular.ttf")
	touch(t, filepath.Join(home, ".fonts"), "NotoSansBengaliUI-Regular.ttf")
	touch(t, filepath.Join(share, "fonts"),
		"google-noto-sans-cjk-vf-fonts/NotoSansCJK-VF.ttc",
		"truetype/noto/NotoSansThai-Regular.ttf",
		"noto/NotoSansDevanagari[wdth,wght].ttf",
		"noto/NotoSans-Regular.ttf",
		"opentype/noto/NotoSansCJKjp-Regular.otf",
		"truetype/dejavu/DejaVuSans.ttf",
		"noto/NotoSerifThai-Regular.ttf",
		"noto/NotoSansHebrew-Bold.ttf",
		"noto/NotoColorEmoji.ttf",
	)
	old := fontconfigFile
	t.Cleanup(func() { fontconfigFile = old })
	fontconfigFile = func(lang string) string { return "" }

	var got []string
	for _, f := range systemFontFiles("linux") {
		got = append(got, filepath.Base(f))
	}
	want := []string{
		"NotoSans-Regular.ttf", "NotoSansDevanagari[wdth,wght].ttf", "NotoSansBengaliUI-Regular.ttf",
		"NotoSansThai-Regular.ttf", "NotoSansCJK-VF.ttc", "NotoSansCJKjp-Regular.otf", "DejaVuSans.ttf",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("found\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	files := systemFontFiles("linux")
	for _, f := range files {
		if filepath.Base(f) == "NotoSansThai-Regular.ttf" && !strings.Contains(f, "mine") {
			t.Errorf("took the system's Thai font %s over the user's", f)
		}
	}
}

// TestLinuxFontconfig adds the fonts fontconfig names for a language,
// once each, after the ones found by name.
func TestLinuxFontconfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	old := fontconfigFile
	t.Cleanup(func() { fontconfigFile = old })
	var asked []string
	fontconfigFile = func(lang string) string {
		asked = append(asked, lang)
		return map[string]string{"hi": "/x/hindi.ttf", "th": "/x/thai.ttf", "ja": "/x/hindi.ttf"}[lang]
	}
	files := systemFontFiles("linux")
	// Fonts of other folders than the test's own may exist, so only the
	// ones the fake names are checked.
	var got []string
	for _, f := range files {
		if strings.HasPrefix(filepath.ToSlash(f), "/x/") {
			got = append(got, f)
		}
	}
	if len(got) != 3 || got[0] != "/x/hindi.ttf" || got[1] != "/x/thai.ttf" || got[2] != "/x/hindi.ttf" {
		t.Errorf("got %v", got)
	}
	if len(asked) != len(fontLanguages) {
		t.Errorf("asked %d languages, want %d", len(asked), len(fontLanguages))
	}
}

// TestLoadFontsSkipsTheUnreadable skips files that are missing, empty or
// not fonts, and reads the same font once.
func TestLoadFontsSkipsTheUnreadable(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "empty.ttf")
	if err := os.WriteFile(filepath.Join(dir, "junk.ttf"), []byte("not a font"), 0o644); err != nil {
		t.Fatal(err)
	}
	good, err := os.ReadFile(filepath.Join("..", "..", "fonts", "selawk.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "good.ttf"), good, 0o644); err != nil {
		t.Fatal(err)
	}
	files := []string{
		filepath.Join(dir, "missing.ttf"), filepath.Join(dir, "empty.ttf"), filepath.Join(dir, "junk.ttf"),
		filepath.Join(dir, "good.ttf"), filepath.Join(dir, "good.ttf"),
	}
	if got := loadFonts(files); len(got) != 1 {
		t.Errorf("read %d fonts, want 1", len(got))
	}
}

// TestFoundFontDrawsItsLetters puts a font in a user's fonts folder under
// the name of a Noto font, which does not depend on the fonts of the
// machine, and finds it, reads it and sets Greek letters in it that the
// bundled face lacks.
func TestFoundFontDrawsItsLetters(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	old := fontconfigFile
	t.Cleanup(func() { fontconfigFile = old })
	fontconfigFile = func(string) string { return "" }
	data, err := os.ReadFile(filepath.Join("..", "..", "fonts", "robotoflex-regular.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, "data", "fonts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "NotoSansArmenian-Regular.ttf"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	fam := family(t)
	fam.Fallback = nil
	const greek = "Ελλάδα"
	if fallbackFor(fam, greek) {
		t.Fatal("the bundled face has Greek letters, so this proves nothing")
	}
	// The system's own fonts may be there too; the user's come first.
	files := systemFontFiles("linux")
	if len(files) == 0 || !strings.HasPrefix(files[0], dir) {
		t.Fatalf("found %v, want the font put in %s first", files, dir)
	}
	fam.Fallback = loadFonts(files[:1])
	if len(fam.Fallback) != 1 {
		t.Fatalf("read %d fonts, want the one put there", len(fam.Fallback))
	}
	if !fallbackFor(fam, greek) {
		t.Fatal("the found font doesn't draw Greek letters")
	}
	c := Card{Size: Square, Colors: dark, Lines: []Line{{Text: greek, Size: 80, Weight: 400}}}
	img, err := c.Render(fam)
	if err != nil {
		t.Fatal(err)
	}
	ground, _ := Card{Size: Square, Colors: dark}.Render(fam)
	if got := ink(img, ground); got < 800 {
		t.Errorf("the letters draw %d pixels", got)
	}
}
