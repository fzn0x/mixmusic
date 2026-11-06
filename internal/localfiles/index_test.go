package localfiles

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/tags"
)

// The songs here are made up. A test file holds its tags as text, which
// the fake reader of tags reads: "title|artist|album|album artist|track",
// with "broken" for a file the engine can't play, "panic" for one that
// breaks the reader, and "cover:<bytes>" for a cover.

// testIndex is an index of a folder of its own, whose reads it counts.
type testIndex struct {
	*Index
	dir   string
	reads atomic.Int32
}

func newIndex(t *testing.T) *testIndex {
	t.Helper()
	dir := t.TempDir()
	ti := &testIndex{dir: filepath.Join(dir, "music")}
	if err := os.MkdirAll(ti.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ti.Index = ti.open(t, filepath.Join(dir, "local-files.json"), filepath.Join(dir, "cache", "local-art"))
	return ti
}

func (ti *testIndex) open(t *testing.T, path, art string) *Index {
	t.Helper()
	x, err := Open(path, art, func(r io.ReadSeeker) (time.Duration, error) {
		ti.reads.Add(1)
		b, _ := io.ReadAll(r)
		if bytes.Contains(b, []byte("broken")) {
			return 0, errors.New("the file can't be played: the audio could not be decoded")
		}
		return time.Duration(len(b)) * time.Second, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	x.readTags = fakeTags
	return x
}

func fakeTags(r io.ReadSeeker) (tags.Tags, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return tags.Tags{}, err
	}
	b, _ := io.ReadAll(r)
	s := string(b)
	if strings.Contains(s, "panic") {
		panic("a file made to break the reader")
	}
	var t tags.Tags
	if before, cover, ok := strings.Cut(s, "cover:"); ok {
		s = before
		t.Picture = &tags.Picture{MIME: "image/png", Data: []byte(cover)}
	}
	f := strings.Split(s, "|")
	get := func(i int) string {
		if i < len(f) {
			return strings.TrimSpace(f[i])
		}
		return ""
	}
	t.Title, t.Album = get(0), get(2)
	if a := get(1); a != "" {
		t.Artists = []string{a}
	}
	if a := get(3); a != "" {
		t.AlbumArtists = []string{a}
	}
	if n := get(4); n != "" {
		t.Track = int(n[0] - '0')
	}
	return t, nil
}

// write writes a test file at name, of the music folder, its time of
// change some seconds in the past.
func (ti *testIndex) write(t *testing.T, name, content string, ago time.Duration) string {
	t.Helper()
	path := filepath.Join(ti.dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-ago)
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
	return path
}

// scan scans the music folder, and returns how many files it read.
func (ti *testIndex) scan(t *testing.T, folders ...string) int {
	t.Helper()
	if folders == nil {
		folders = []string{ti.dir}
	}
	before := ti.reads.Load()
	if err := ti.Scan(context.Background(), folders); err != nil {
		t.Fatal(err)
	}
	return int(ti.reads.Load() - before)
}

func titles(tracks []media.Track) []string {
	var out []string
	for _, t := range tracks {
		out = append(out, t.Title)
	}
	return out
}

// TestScan reads the music files of a folder and its folders, but hidden
// ones and other files, and counts those it can't play as skipped.
func TestScan(t *testing.T) {
	ti := newIndex(t)
	ti.write(t, "Harbour/01.mp3", "Slow Tide|The Harbour Lights|Shore||1", time.Hour)
	ti.write(t, "Harbour/02.flac", "Low Water|The Harbour Lights|Shore||2", time.Hour)
	ti.write(t, "loose.wav", "", time.Hour)
	ti.write(t, "bad.ogg", "broken", time.Hour)
	ti.write(t, "evil.m4a", "panic", time.Hour)
	ti.write(t, ".hidden/secret.mp3", "Hidden|Nobody", time.Hour)
	ti.write(t, "notes.txt", "Not music|Anyone", time.Hour)
	if n := ti.scan(t); n != 5 {
		t.Fatalf("read %d files, want 5", n)
	}
	st := ti.Status()
	if st.Scanning || st.Tracks != 3 || len(st.Folders) != 1 || st.Folders[0].Tracks != 3 || st.Folders[0].Skipped != 2 {
		t.Fatalf("status %+v", st)
	}
	if got, want := titles(ti.Tracks()), []string{"loose", "Slow Tide", "Low Water"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tracks %q, want %q", got, want)
	}
	loose := ti.Tracks()[0]
	if loose.Artists[0].Name != UnknownArtist || loose.Album != nil || loose.Source != media.SourceFile {
		t.Errorf("a file without tags: %+v", loose)
	}
	if e, _ := ti.Entry(filepath.Join(ti.dir, "bad.ogg")); e.Skipped != "the audio could not be decoded" {
		t.Errorf("a broken file skipped as %q", e.Skipped)
	}
	if e, _ := ti.Entry(filepath.Join(ti.dir, "evil.m4a")); e.Skipped == "" {
		t.Error("a file that broke the reader isn't skipped")
	}
}

// TestRescan reads only the files that changed, by their size or their
// time of change, and those new, and drops those gone.
func TestRescan(t *testing.T) {
	ti := newIndex(t)
	a := ti.write(t, "a.mp3", "One|Ada Field", time.Hour)
	b := ti.write(t, "b.mp3", "Two|Ada Field", time.Hour)
	ti.write(t, "c.mp3", "Three|Ada Field", time.Hour)
	ti.scan(t)
	if n := ti.scan(t); n != 0 {
		t.Fatalf("a scan of nothing changed read %d files", n)
	}
	ti.write(t, "a.mp3", "One (edit)|Ada Field", time.Hour) // another size
	ti.write(t, "b.mp3", "Owt|Ada Field", 10*time.Minute)   // the same size, of another time
	ti.write(t, "d.mp3", "Four|Ada Field", time.Hour)       // new
	os.Remove(filepath.Join(ti.dir, "c.mp3"))               // gone
	if n := ti.scan(t); n != 3 {
		t.Fatalf("read %d files, want 3", n)
	}
	if got, want := titles(ti.Tracks()), []string{"Four", "One (edit)", "Owt"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tracks %q, want %q", got, want)
	}
	_ = a
	_ = b
	// The index is kept: a new run of the app reads nothing again.
	x := ti.open(t, filepath.Join(filepath.Dir(ti.dir), "local-files.json"), filepath.Join(filepath.Dir(ti.dir), "cache", "local-art"))
	before := ti.reads.Load()
	if err := x.Scan(context.Background(), []string{ti.dir}); err != nil {
		t.Fatal(err)
	}
	if n := ti.reads.Load() - before; n != 0 || len(x.Tracks()) != 3 {
		t.Errorf("the index kept read %d files again, and holds %d", n, len(x.Tracks()))
	}
}

// TestMissing tells a track whose file is gone, as of a mix, and one the
// player found gone, until it is found again.
func TestMissing(t *testing.T) {
	ti := newIndex(t)
	a := ti.write(t, "a.mp3", "One|Ada Field", time.Hour)
	ti.scan(t)
	id := TrackID(a)
	if ti.Missing(id) {
		t.Fatal("a file the index holds is missing")
	}
	ti.Gone(id)
	if !ti.Missing(id) {
		t.Fatal("a file the player found gone isn't missing")
	}
	ti.Found(id)
	if ti.Missing(id) {
		t.Fatal("a file found again is missing")
	}
	elsewhere := filepath.Join(t.TempDir(), "moved.mp3")
	if !ti.Missing(TrackID(elsewhere)) {
		t.Error("a file that isn't there, beyond the index, isn't missing")
	}
	os.WriteFile(elsewhere, []byte("x"), 0o644)
	now := time.Now()
	ti.now = func() time.Time { return now.Add(time.Minute) }
	if ti.Missing(TrackID(elsewhere)) {
		t.Error("a file that came back, looked at again, is missing")
	}
	if ti.Missing("yt:dQw4w9WgXcQ") {
		t.Error("a track of a service is missing")
	}
}

// TestFolders drops the files of a folder no longer named, and keeps
// those of a folder that can't be read, as on a drive unplugged.
func TestFolders(t *testing.T) {
	ti := newIndex(t)
	ti.write(t, "a/1.mp3", "One|Ada Field", time.Hour)
	ti.write(t, "b/2.mp3", "Two|Orin Vale", time.Hour)
	a, b := filepath.Join(ti.dir, "a"), filepath.Join(ti.dir, "b")
	ti.scan(t, a, b)
	if n := len(ti.Tracks()); n != 2 {
		t.Fatalf("%d tracks", n)
	}
	ti.scan(t, a)
	if got := titles(ti.Tracks()); !reflect.DeepEqual(got, []string{"One"}) {
		t.Fatalf("after b went: %q", got)
	}
	ti.scan(t, a, b)
	if err := os.Rename(b, b+"-unplugged"); err != nil {
		t.Fatal(err)
	}
	ti.scan(t, a, b)
	st := ti.Status()
	if got := titles(ti.Tracks()); len(got) != 2 || st.Folders[1].Problem == "" || st.Folders[1].Tracks != 1 {
		t.Errorf("a folder unplugged: %q, %+v", got, st.Folders)
	}
	if !ti.Missing(ti.Tracks()[1].ID) && !ti.Missing(ti.Tracks()[0].ID) {
		t.Error("no track of the unplugged folder is missing")
	}
}

// TestCovers keeps a cover once however many files hold it, in the folder
// of covers, and takes a cover.jpg beside files that hold none.
func TestCovers(t *testing.T) {
	ti := newIndex(t)
	ti.write(t, "A/1.mp3", "One|Ada Field|Kites||1cover:\x89PNG made up", time.Hour)
	ti.write(t, "A/2.mp3", "Two|Ada Field|Kites||2cover:\x89PNG made up", time.Hour)
	ti.write(t, "B/1.mp3", "Three|Orin Vale|Paper", time.Hour)
	ti.write(t, "B/Cover.JPG", "\xff\xd8\xff a cover beside", time.Hour)
	ti.scan(t)
	art := map[string]bool{}
	for _, tr := range ti.Tracks() {
		if len(tr.Artwork) != 1 {
			t.Fatalf("%s has no cover", tr.Title)
		}
		path, ok := ti.ArtFile(tr.Artwork[0].URL)
		if !ok {
			t.Fatalf("no file of %s", tr.Artwork[0].URL)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
		art[path] = true
	}
	if len(art) != 2 {
		t.Errorf("%d covers kept, want 2", len(art))
	}
	files, _ := os.ReadDir(filepath.Join(filepath.Dir(ti.dir), "cache", "local-art"))
	if len(files) != 2 {
		t.Errorf("%d files in the folder of covers", len(files))
	}
	for _, bad := range []string{"localart:../x.jpg", "localart:", "https://example.com/a.jpg", `localart:a\b.jpg`} {
		if _, ok := ti.ArtFile(bad); ok {
			t.Errorf("%q names a file", bad)
		}
	}
	// A cover the cache lost is read again.
	os.RemoveAll(filepath.Join(filepath.Dir(ti.dir), "cache"))
	if n := ti.scan(t); n != 3 {
		t.Errorf("read %d files of lost covers, want 3", n)
	}
}

// TestAlbumsAndArtists groups the tracks into albums, by their album
// artist or the folder they're in, and their artists.
func TestAlbumsAndArtists(t *testing.T) {
	ti := newIndex(t)
	ti.write(t, "Kites/1.mp3", "Paper|Ada Field|Kites||1", time.Hour)
	ti.write(t, "Kites/2.mp3", "String|Ada Field|Kites||2", time.Hour)
	ti.write(t, "Mix/1.mp3", "Night Bus|Juno Reyes|Late Mix||1", time.Hour)
	ti.write(t, "Mix/2.mp3", "Day Bus|Mira Holt|Late Mix||2", time.Hour)
	ti.write(t, "Comp/a/1.mp3", "First|Orin Vale|Together|Friends|1", time.Hour)
	ti.write(t, "Comp/b/2.mp3", "Second|Ada Field|Together|Friends|2", time.Hour)
	ti.scan(t)
	albums := ti.Albums()
	got := map[string]string{}
	for _, a := range albums {
		got[a.Title] = names(a.Artists)
		if a.TrackCount != 2 || a.Tracks != nil {
			t.Errorf("%s: %d tracks, %v", a.Title, a.TrackCount, a.Tracks)
		}
	}
	if want := map[string]string{"Kites": "Ada Field", "Late Mix": VariousArtists, "Together": "Friends"}; !reflect.DeepEqual(got, want) {
		t.Errorf("albums %v, want %v", got, want)
	}
	kites, ok := ti.Album(albums[0].ID)
	if !ok || !reflect.DeepEqual(titles(kites.Tracks), []string{"Paper", "String"}) {
		t.Errorf("Kites: %v", titles(kites.Tracks))
	}
	var artists []string
	for _, a := range ti.Artists() {
		artists = append(artists, a.Name)
	}
	if want := []string{"Ada Field", "Friends", "Juno Reyes", "Mira Holt", "Orin Vale"}; !reflect.DeepEqual(artists, want) {
		t.Errorf("artists %q, want %q", artists, want)
	}
	ada, ok := ti.Artist(artistID("ada field"))
	if !ok || len(ada.TopTracks) != 3 || len(ada.Albums) != 2 {
		t.Errorf("Ada Field: %v, %d albums", titles(ada.TopTracks), len(ada.Albums))
	}
	if !IsAlbum(kites.ID) || IsArtist(kites.ID) || !IsArtist(ada.ID) {
		t.Error("IsAlbum and IsArtist")
	}
	if _, ok := PathOf(kites.ID); ok {
		t.Error("an album's ID names a file")
	}
}

// TestSearch finds tracks by their titles, artists and albums, regardless
// of case and accents, and albums and artists by their names.
func TestSearch(t *testing.T) {
	ti := newIndex(t)
	ti.write(t, "1.mp3", "Café Nights|Ada Field|Kites", time.Hour)
	ti.write(t, "2.mp3", "Paper|Orin Vale|Kites", time.Hour)
	ti.write(t, "3.mp3", "Rain|Orin Vale|Weather", time.Hour)
	ti.scan(t)
	res := ti.Search("cafe ADA", 10)
	if got := titles(res.Tracks); !reflect.DeepEqual(got, []string{"Café Nights"}) {
		t.Errorf("tracks %q", got)
	}
	res = ti.Search("kites", 10)
	if len(res.Tracks) != 2 || len(res.Albums) != 1 || len(res.Artists) != 0 {
		t.Errorf("kites: %d tracks, %d albums, %d artists", len(res.Tracks), len(res.Albums), len(res.Artists))
	}
	if res := ti.Search("orin", 1); len(res.Tracks) != 1 || len(res.Artists) != 1 {
		t.Errorf("orin, of 1: %+v", res)
	}
	if res := ti.Search("  ", 10); len(res.Tracks) != 0 {
		t.Error("an empty search found something")
	}
}

// TestReadFile reads a file the user opened, which no folder holds, and
// refuses one the engine can't play.
func TestReadFile(t *testing.T) {
	ti := newIndex(t)
	path := filepath.Join(t.TempDir(), "opened.mp3")
	os.WriteFile(path, []byte("Opened|Mira Holt|Alone"), 0o644)
	e, err := ti.ReadFile(path)
	if err != nil || TrackOf(e).Title != "Opened" || TrackOf(e).ID != TrackID(path) {
		t.Fatalf("%+v, %v", e, err)
	}
	if got, ok := PathOf(TrackOf(e).ID); !ok || got != path {
		t.Errorf("the ID names %q", got)
	}
	bad := filepath.Join(t.TempDir(), "bad.mp3")
	os.WriteFile(bad, []byte("broken"), 0o644)
	if _, err := ti.ReadFile(bad); err == nil {
		t.Error("a broken file was read")
	}
	if _, err := ti.ReadFile(filepath.Join(t.TempDir(), "nothing.mp3")); err == nil {
		t.Error("no file was read")
	}
}

// TestScanAgain runs a scan asked for while one runs after it, and a
// scan cut short changes nothing.
func TestScanAgain(t *testing.T) {
	ti := newIndex(t)
	ti.write(t, "a.mp3", "One|Ada Field", time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ti.Scan(ctx, []string{ti.dir}); !errors.Is(err, context.Canceled) {
		t.Fatalf("a scan cut short: %v", err)
	}
	if len(ti.Tracks()) != 0 {
		t.Fatal("a scan cut short changed the index")
	}
	var watched atomic.Int32
	stop := ti.Watch(func() { watched.Add(1) })
	defer stop()
	ti.scan(t)
	if watched.Load() == 0 || slices.Index(titles(ti.Tracks()), "One") < 0 {
		t.Errorf("watched %d times, tracks %q", watched.Load(), titles(ti.Tracks()))
	}
}
