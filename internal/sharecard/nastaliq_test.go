package sharecard

import (
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// TestNastaliqFileWindows finds Urdu Typesetting in Windows' fonts, and
// Noto Nastaliq Urdu before it, in the user's fonts too, and none in
// folders without them.
func TestNastaliqFileWindows(t *testing.T) {
	root, local := t.TempDir(), t.TempDir()
	t.Setenv("SystemRoot", root)
	t.Setenv("LOCALAPPDATA", local)
	if got := nastaliqFile("windows"); got != "" {
		t.Errorf("found %s in folders without a Nastaliq face", got)
	}
	touch(t, filepath.Join(root, "Fonts"), "UrdType.ttf", "UrdTypeb.ttf")
	if got := filepath.Base(nastaliqFile("windows")); got != "UrdType.ttf" {
		t.Errorf("found %s, want UrdType.ttf", got)
	}
	touch(t, filepath.Join(local, "Microsoft", "Windows", "Fonts"), "NotoNastaliqUrdu-Regular.ttf")
	if got := nastaliqFile("windows"); got != filepath.Join(local, "Microsoft", "Windows", "Fonts", "NotoNastaliqUrdu-Regular.ttf") {
		t.Errorf("found %s, want the user's Noto Nastaliq Urdu", got)
	}
}

// TestNastaliqFileLinux finds Noto Nastaliq Urdu under the names Debian,
// Fedora and Arch give it, the user's copy first, and not its bold.
func TestNastaliqFileLinux(t *testing.T) {
	for _, name := range []string{"NotoNastaliqUrdu-Regular.ttf", "NotoNastaliqUrdu[wght].ttf", "NotoNastaliqUrdu-VF.ttf"} {
		t.Run(name, func(t *testing.T) {
			home, share := t.TempDir(), t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
			t.Setenv("XDG_DATA_DIRS", share)
			touch(t, filepath.Join(share, "fonts"), "noto/NotoNastaliqUrdu-Bold.ttf", "noto/"+name)
			if got := nastaliqFile("linux"); got != filepath.Join(share, "fonts", "noto", name) {
				t.Errorf("found %q, want %s", got, name)
			}
			touch(t, filepath.Join(home, ".local", "share", "fonts"), "mine/"+name)
			if got := nastaliqFile("linux"); got != filepath.Join(home, ".local", "share", "fonts", "mine", name) {
				t.Errorf("found %q, want the user's copy", got)
			}
		})
	}
}

// TestNastaliqDrawsUrdu draws a card in Urdu with the system's Nastaliq
// face, on a system that has one, as macOS.
func TestNastaliqDrawsUrdu(t *testing.T) {
	face := Nastaliq()
	if len(face) == 0 {
		t.Skipf("this %s has no Nastaliq face", runtime.GOOS)
	}
	const title = "دل دل پاکستان"
	fam := family(t)
	fam.Fallback = append(slices.Clone(face), fam.Fallback...)
	if !fallbackFor(fam, title) {
		t.Fatalf("the Nastaliq face lacks a letter of %q", title)
	}
	c := Card{Size: Square, Colors: dark, Lines: []Line{{Text: title, Size: 80, Weight: 400}}}
	img, err := c.Render(fam)
	if err != nil {
		t.Fatal(err)
	}
	ground, _ := Card{Size: Square, Colors: dark}.Render(fam)
	if got := ink(img, ground); got < 800 {
		t.Errorf("%q draws %d pixels", title, got)
	}
}
