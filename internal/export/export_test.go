package export

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/fzn0x/mixmusic/internal/tags"
	"github.com/fzn0x/mixmusic/internal/tagwrite"
)

func TestName(t *testing.T) {
	for _, c := range []struct {
		artists []string
		title   string
		want    string
	}{
		{[]string{"Ola Brine"}, "Paper Lanterns", "Ola Brine - Paper Lanterns"},
		{[]string{"Ola Brine", " Tam Reed "}, "Slow Ferry", "Ola Brine, Tam Reed - Slow Ferry"},
		{nil, "Tin Roof Rain", "Tin Roof Rain"},
		{[]string{"Ola Brine"}, "", "Ola Brine"},
		{nil, "", "Untitled"},
		// What Windows refuses in a name, or macOS's Finder shows as a
		// slash.
		{[]string{"AC/DC Tribute"}, `Why? <Live> "Take 2": a|b*c\d`, "AC-DC Tribute - Why Live 'Take 2'- a-bc-d"},
		{[]string{"Ola"}, "Tabs\tand\nlines\x00", "Ola - Tabs and lines"},
		// A dot first hides a file; Windows drops dots and spaces at the
		// end.
		{nil, "...Hidden...", "Hidden"},
		{nil, " . ", "Untitled"},
		// The names Windows keeps for its devices.
		{nil, "CON", "CON_"},
		{nil, "nul", "nul_"},
		{nil, "com1", "com1_"},
		{nil, "LPT¹", "LPT¹_"},
		{nil, "Aux.Live", "Aux.Live_"},
		{nil, "COM10", "COM10"},
		{nil, "Console", "Console"},
		{[]string{"Con"}, "Air", "Con - Air"},
	} {
		if got := Name(c.artists, c.title); got != c.want {
			t.Errorf("Name(%q, %q) = %q, want %q", c.artists, c.title, got, c.want)
		}
	}
	long := Name([]string{"Ola Brine"}, strings.Repeat("灯", 300))
	if len(long) > maxName || !utf8.ValidString(long) || !strings.HasPrefix(long, "Ola Brine - 灯") {
		t.Errorf("a long name is cut to %d bytes: %q", len(long), long)
	}
}

// TestClaim gives two tracks of one name a file each, and keeps the files
// that were there.
func TestClaim(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Ola Brine - Slow Ferry.opus"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	var got []string
	for range 3 {
		p, err := claim(dir, "Ola Brine - Slow Ferry", ".opus")
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, filepath.Base(p))
	}
	if want := []string{"Ola Brine - Slow Ferry (2).opus", "Ola Brine - Slow Ferry (3).opus", "Ola Brine - Slow Ferry (4).opus"}; !slices.Equal(got, want) {
		t.Errorf("claimed %q, want %q", got, want)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "Ola Brine - Slow Ferry.opus")); string(b) != "mine" {
		t.Errorf("the file that was there holds %q", b)
	}
}

// pngImage is a PNG image of w by h pixels of one colour.
func pngImage(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0xC0
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// testdata reads a file of the repository's testdata.
func testdata(t *testing.T, path string) string {
	t.Helper()
	p := filepath.Join("..", "..", filepath.FromSlash(path))
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
	return p
}

// readBack reads the tags of the file at path.
func readBack(t *testing.T, path string) tags.Tags {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tg, err := tags.Read(f)
	if err != nil {
		t.Fatalf("reading %s: %v", filepath.Base(path), err)
	}
	return tg
}

// TestWrite copies a download of Opus in WebM as Ogg Opus with its tags
// and cover, and leaves the download as it was, and the folder without
// anything but the copies.
func TestWrite(t *testing.T) {
	dir := t.TempDir()
	src := testdata(t, "webm/testdata/stereo-cues-front.webm")
	before, _ := os.ReadFile(src)
	cover, err := Cover(pngImage(t, 32, 32))
	if err != nil {
		t.Fatal(err)
	}
	tr := Track{Path: src, Artists: []string{"Ola Brine"}, Title: "Paper Lanterns", Tags: tagwrite.Tags{Title: "Paper Lanterns",
		Artists: []string{"Ola Brine"}, Album: "Harbour Lights", AlbumArtists: []string{"Ola Brine"}, Track: 4, Year: 2019,
		ISRC: "QZES81900004", Cover: cover}}
	var paths []string
	for range 2 {
		p, err := Write(dir, tr)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, filepath.Base(p))
	}
	if want := []string{"Ola Brine - Paper Lanterns.opus", "Ola Brine - Paper Lanterns (2).opus"}; !slices.Equal(paths, want) {
		t.Errorf("wrote %q, want %q", paths, want)
	}
	got := readBack(t, filepath.Join(dir, paths[0]))
	if got.Title != "Paper Lanterns" || !slices.Equal(got.Artists, []string{"Ola Brine"}) || got.Album != "Harbour Lights" ||
		got.Track != 4 || got.Year != 2019 || got.ISRC != "QZES81900004" || !slices.Equal(got.AlbumArtists, []string{"Ola Brine"}) {
		t.Errorf("the copy reads back as %+v", got)
	}
	if got.Picture == nil || !bytes.Equal(got.Picture.Data, cover.Data) {
		t.Error("the copy has no cover, or another")
	}
	if after, _ := os.ReadFile(src); !bytes.Equal(before, after) {
		t.Error("the download changed")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Errorf("the folder holds %d files", len(entries))
	}
}

// TestWriteKinds copies an M4A file with its tags, a file on this
// computer as it is, and a file of a kind whose tags aren't written as it
// is too.
func TestWriteKinds(t *testing.T) {
	dir := t.TempDir()
	tg := tagwrite.Tags{Title: "Gulls at Dawn", Artists: []string{"Ola Brine"}}
	m4a := filepath.Join(t.TempDir(), "x.m4a")
	data, err := os.ReadFile(testdata(t, "mp4/testdata/dash.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m4a, data, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Write(dir, Track{Path: m4a, Artists: tg.Artists, Title: tg.Title, Tags: tg})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "Ola Brine - Gulls at Dawn.m4a" {
		t.Errorf("wrote %s", filepath.Base(p))
	}
	if got := readBack(t, p); got.Title != tg.Title || got.Album != "" {
		t.Errorf("the M4A copy reads back as %+v", got)
	}

	local := testdata(t, "ogg/testdata/tone.opus")
	p, err = Write(dir, Track{Path: local, Artists: []string{"Tam Reed"}, Title: "Tone", Tags: tg, AsIs: true})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(local)
	b, _ := os.ReadFile(p)
	if filepath.Base(p) != "Tam Reed - Tone.opus" || !bytes.Equal(a, b) {
		t.Errorf("a file on this computer went to %s, changed: %v", filepath.Base(p), !bytes.Equal(a, b))
	}

	wav := filepath.Join(t.TempDir(), "x.WAV")
	if err := os.WriteFile(wav, []byte("RIFF\x24\x00\x00\x00WAVEfmt "), 0o644); err != nil {
		t.Fatal(err)
	}
	if p, err = Write(dir, Track{Path: wav, Title: "Room Tone", Tags: tg}); err != nil || filepath.Base(p) != "Room Tone.wav" {
		t.Errorf("a WAV file went to %q: %v", p, err)
	}

	if _, err := Write(dir, Track{Path: filepath.Join(dir, "gone.webm"), Title: "Gone"}); err == nil || !strings.Contains(err.Error(), "isn't there") {
		t.Errorf("a file that's gone: %v", err)
	}
}

// TestWriteFails leaves nothing in the folder when the copy can't be
// written.
func TestWriteFails(t *testing.T) {
	dir := t.TempDir()
	broken := filepath.Join(t.TempDir(), "x.webm")
	file, err := os.ReadFile(testdata(t, "webm/testdata/stereo-cues-front.webm"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(broken, file[:len(file)/2], 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(dir, Track{Path: broken, Title: "Half", Tags: tagwrite.Tags{Title: "Half"}}); !errors.Is(err, tagwrite.ErrFormat) {
		t.Errorf("a file cut short: %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("the failure left %d files", len(entries))
	}
}

// TestCover takes a JPEG or PNG image as it is, and makes a JPEG image of
// another kind.
func TestCover(t *testing.T) {
	p, err := Cover(pngImage(t, 8, 8))
	if err != nil || p.MIME != "image/png" {
		t.Errorf("a PNG image: %+v, %v", p, err)
	}
	var b bytes.Buffer
	pal := image.NewPaletted(image.Rect(0, 0, 8, 8), []color.Color{color.Black, color.White})
	if err := gif.Encode(&b, pal, nil); err != nil {
		t.Fatal(err)
	}
	p, err = Cover(b.Bytes())
	if err != nil || p.MIME != "image/jpeg" || !bytes.HasPrefix(p.Data, []byte("\xFF\xD8\xFF")) {
		t.Errorf("a GIF image: %v", err)
	}
	if _, err := Cover([]byte("not an image")); err == nil {
		t.Error("text made a cover")
	}
}
