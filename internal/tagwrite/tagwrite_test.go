package tagwrite

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/tags"
)

// The songs here are made up, as are their tags. The audio of the files
// built here is made-up bytes; that of the files of the repository's
// testdata is tones.

// pngCover is a PNG image of w by h pixels, of a shade.
func pngCover(t testing.TB, w, h int, shade byte) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = shade + byte(i)
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// jpegCover is a JPEG image of w by h pixels.
func jpegCover(t testing.TB, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{byte(x * 9), byte(y * 5), 120, 255})
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// save writes data to a file of name in a folder of its own, and returns
// its path.
func save(t testing.TB, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// write writes the fields of tg to the file at path, which must work, and
// returns what it holds then.
func write(t testing.TB, path string, tg Tags, fields Fields) []byte {
	t.Helper()
	if err := Write(path, tg, fields, Options{}); err != nil {
		t.Fatalf("writing %s: %v", filepath.Base(path), err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if left := leftovers(t, filepath.Dir(path)); len(left) > 0 {
		t.Errorf("the write left %q beside the file", left)
	}
	return b
}

// leftovers are the files of dir but its music files and the index's.
func leftovers(t testing.TB, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			out = append(out, e.Name())
		}
	}
	return out
}

// readTags reads the tags of the file at path.
func readTags(t testing.TB, path string) tags.Tags {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := tags.Read(f)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// entry reads the file at path as the library does: through an index of
// the music on this computer of its folder, which reads its tags and
// keeps its cover.
func entry(t testing.TB, path string) localfiles.Entry {
	t.Helper()
	dir := t.TempDir()
	x, err := localfiles.Open(filepath.Join(dir, "local-files.json"), filepath.Join(dir, "art"),
		func(io.ReadSeeker) (time.Duration, error) { return time.Second, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := x.Scan(context.Background(), []string{filepath.Dir(path)}); err != nil {
		t.Fatal(err)
	}
	e, ok := x.Entry(path)
	if !ok || e.Skipped != "" {
		t.Fatalf("the index holds no %s: %+v", path, e)
	}
	if e.Art != "" {
		art, ok := x.ArtFile("localart:" + e.Art)
		if !ok {
			t.Fatalf("no file of the cover %s", e.Art)
		}
		if _, err := os.Stat(art); err != nil {
			t.Errorf("the cover isn't kept: %v", err)
		}
	}
	return e
}

// song is what the tests write: unicode, two artists, a cover.
func song(t testing.TB) Tags {
	return Tags{
		Title: "Lanterns über the Bay 灯", Artists: []string{"Ada Field", "Orin Vale 🎐"}, Album: "Café Nights",
		AlbumArtists: []string{"Various Artists"}, Track: 3, Disc: 2, Year: 2021, Genre: "Dream Pop", ISRC: "QZ-ES8-21-00001",
		Cover: &Picture{Data: pngCover(t, 16, 16, 40)},
	}
}

// matches checks that the file at path reads back as tg, through the
// index of the library and through internal/tags.
func matches(t testing.TB, path string, tg Tags) {
	t.Helper()
	tg = tg.clean()
	e := entry(t, path)
	got := localfiles.Entry{Title: e.Title, Artists: e.Artists, Album: e.Album, AlbumArtists: e.AlbumArtists,
		Track: e.Track, Disc: e.Disc, Year: e.Year}
	want := localfiles.Entry{Title: tg.Title, Artists: tg.Artists, Album: tg.Album, AlbumArtists: tg.AlbumArtists,
		Track: tg.Track, Disc: tg.Disc, Year: tg.Year}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the library reads\n%+v\nwant\n%+v", got, want)
	}
	if (e.Art != "") != (tg.Cover != nil) {
		t.Errorf("the library's cover is %q, written %v", e.Art, tg.Cover != nil)
	}
	r := readTags(t, path)
	if r.Genre != tg.Genre || r.ISRC != tg.ISRC {
		t.Errorf("genre %q and ISRC %q, want %q and %q", r.Genre, r.ISRC, tg.Genre, tg.ISRC)
	}
	switch {
	case tg.Cover == nil && r.Picture != nil:
		t.Errorf("a cover of %d bytes, after it was taken away", len(r.Picture.Data))
	case tg.Cover != nil && (r.Picture == nil || !bytes.Equal(r.Picture.Data, tg.Cover.Data)):
		t.Errorf("the cover isn't the one written")
	}
}

func sum(b []byte) [32]byte { return sha256.Sum256(b) }

// TestKinds tells the kinds of files whose tags are written, and refuses
// WAV and what isn't music.
func TestKinds(t *testing.T) {
	for _, c := range []struct {
		name string
		data []byte
		want Kind
		err  error
	}{
		{"mp3", mp3File(nil, 3, false), MP3, nil},
		{"mp3 with ID3", mp3File(id3Tag4(textFrame4("TIT2", "x")), 3, false), MP3, nil},
		{"flac", flacFile(t, nil, 0), FLAC, nil},
		{"flac after ID3", append(id3Tag4(textFrame4("TIT2", "x")), flacFile(t, nil, 0)...), FLAC, nil},
		{"opus", fixture(t, "../../ogg/testdata/tone.opus"), Opus, nil},
		{"vorbis", fixture(t, "../../ogg/testdata/tone.ogg"), Vorbis, nil},
		{"m4a", fixture(t, "../../mp4/testdata/faststart.m4a"), MP4, nil},
		{"wav", []byte("RIFF\x24\x00\x00\x00WAVEfmt \x10\x00\x00\x00"), "", ErrUnsupported},
		{"text", []byte("not a song at all"), "", ErrUnsupported},
	} {
		got, err := KindOf(bytes.NewReader(c.data))
		if got != c.want || !errors.Is(err, c.err) {
			t.Errorf("%s: %q, %v; want %q, %v", c.name, got, err, c.want, c.err)
		}
	}
}

// TestWAVStaysAsItIs refuses to write the tags of a WAV file, which stays
// as it was.
func TestWAVStaysAsItIs(t *testing.T) {
	data := []byte("RIFF\x24\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x40\x1f\x00\x00\x80\x3e\x00\x00\x02\x00\x10\x00data\x00\x00\x00\x00")
	path := save(t, "memo.wav", data)
	if err := Writable(path); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Writable: %v", err)
	}
	if err := Write(path, Tags{Title: "Memo"}, Title, Options{}); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Write: %v", err)
	}
	unchanged(t, path, data)
}

// unchanged checks that the file at path still holds data, and that
// nothing was left beside it.
func unchanged(t testing.TB, path string, data []byte) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, data) {
		t.Errorf("the file changed: %d bytes, was %d", len(b), len(data))
	}
	if left := leftovers(t, filepath.Dir(path)); len(left) > 0 {
		t.Errorf("left %q beside the file", left)
	}
}

// TestFailedWriteLeavesTheFile keeps the file as it was when the new one
// fails its check, when the cover isn't an image, when the values can't
// be read back as written, and when the file is damaged.
func TestFailedWriteLeavesTheFile(t *testing.T) {
	data := flacFile(t, [][2]string{{"TITLE", "Paper Moons"}}, 1024)
	path := save(t, "moons.flac", data)
	refused := errors.New("the engine can't play it")
	err := Write(path, Tags{Title: "Changed"}, Title, Options{Check: func(r io.ReadSeeker) error {
		got, err := tags.Read(r)
		if err != nil || got.Title != "Changed" {
			t.Errorf("the check read %+v, %v", got, err)
		}
		return refused
	}})
	if !errors.Is(err, ErrFormat) || !strings.Contains(err.Error(), refused.Error()) {
		t.Errorf("a failed check: %v", err)
	}
	unchanged(t, path, data)

	if err := Write(path, Tags{Cover: &Picture{Data: []byte("not an image")}}, Cover, Options{}); !errors.Is(err, ErrCover) {
		t.Errorf("a cover that isn't an image: %v", err)
	}
	if err := Write(path, Tags{Year: 21}, Year, Options{}); err == nil {
		t.Error("a year of two digits was written")
	}
	if err := Write(path, Tags{Track: 10000}, Track, Options{}); err == nil {
		t.Error("track 10000 was written")
	}
	unchanged(t, path, data)

	broken := bytes.Clone(data)
	broken[4] = 0x7F // a block of type 127, which is none
	path = save(t, "broken.flac", broken)
	if err := Write(path, Tags{Title: "x"}, Title, Options{}); !errors.Is(err, ErrFormat) {
		t.Errorf("a damaged file: %v", err)
	}
	unchanged(t, path, broken)
}

// TestReadOnlyFile refuses a read-only file before writing anything.
func TestReadOnlyFile(t *testing.T) {
	data := mp3File(nil, 4, false)
	path := save(t, "song.mp3", data)
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o644) })
	if err := Writable(path); !errors.Is(err, ErrReadOnly) {
		if os.Getuid() == 0 {
			t.Skip("permissions don't stop root")
		}
		t.Errorf("Writable: %v", err)
	}
	if err := Write(path, Tags{Title: "x"}, Title, Options{}); !errors.Is(err, ErrReadOnly) {
		t.Errorf("Write: %v", err)
	}
	unchanged(t, path, data)
}

// TestReadOnlyFolder refuses a file in a folder MixMusic can't write in,
// where the new file would go.
func TestReadOnlyFolder(t *testing.T) {
	data := mp3File(nil, 4, false)
	path := save(t, "song.mp3", data)
	denyWrite(t, filepath.Dir(path))
	if err := Writable(path); !errors.Is(err, ErrFolderReadOnly) {
		t.Errorf("Writable: %v", err)
	}
	if err := Write(path, Tags{Title: "x"}, Title, Options{}); !errors.Is(err, ErrFolderReadOnly) {
		t.Errorf("Write: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(b, data) {
		t.Errorf("the file changed: %v", err)
	}
}

// TestRewriteToAnotherFile writes a file anew to another, as an export
// would, leaving the first as it was.
func TestRewriteToAnotherFile(t *testing.T) {
	src := fixture(t, "../../ogg/testdata/tone.opus")
	var out bytes.Buffer
	tg := song(t)
	if err := Rewrite(&out, bytes.NewReader(src), tg, All); err != nil {
		t.Fatal(err)
	}
	path := save(t, "export.opus", out.Bytes())
	matches(t, path, tg)
	samePackets(t, src, out.Bytes())
}

// fixture reads a file of the repository's testdata.
func fixture(t testing.TB, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
