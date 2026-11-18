package playlistio

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// read reads a file of testdata.
func read(t *testing.T, name string) Source {
	t.Helper()
	src, err := ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return src
}

// only is the one list of src.
func only(t *testing.T, src Source) List {
	t.Helper()
	if len(src.Lists) != 1 {
		t.Fatalf("%d lists, want 1: %+v", len(src.Lists), src.Lists)
	}
	return src.Lists[0]
}

// TestReadExportify reads a CSV of a playlist of Spotify, whose columns
// name its fields: the URIs, the lengths in milliseconds, the ISRCs.
func TestReadExportify(t *testing.T) {
	l := only(t, read(t, "exportify.csv"))
	if l.Name != "exportify" || l.Own || len(l.Entries) != 3 {
		t.Fatalf("list %q own %v of %d", l.Name, l.Own, len(l.Entries))
	}
	want := []Entry{
		{Title: "Harbour Lights", Artists: []string{"Ola Brine"}, Album: "Coastline", Duration: 215 * time.Second,
			ISRC: "GBAYE0601498", SpotifyID: "0harbourLights00000001"},
		{Title: "Gulls at Dawn (Live)", Artists: []string{"Ola Brine, Tam Reed"}, Album: "Live at the Pier",
			Duration: 241500 * time.Millisecond, SpotifyID: "0gullsAtDawn0000000002"},
		{Title: "Tin Roof Rain", Artists: []string{"Tam Reed"}, Album: "Rooftops", Duration: 198 * time.Second},
	}
	for i, w := range want {
		if fmt.Sprint(l.Entries[i]) != fmt.Sprint(w) || !sameEntry(l.Entries[i], w) {
			t.Errorf("entry %d is %+v, want %+v", i, l.Entries[i], w)
		}
	}
}

func sameEntry(a, b Entry) bool {
	return a.Title == b.Title && slices.Equal(a.Artists, b.Artists) && a.Album == b.Album && a.Duration == b.Duration &&
		a.ISRC == b.ISRC && a.VideoID == b.VideoID && a.SpotifyID == b.SpotifyID && a.MixMusicID == b.MixMusicID &&
		a.Path == b.Path && a.Plays == b.Plays
}

// TestReadCodePage reads a spreadsheet's CSV of Windows: semicolons, in
// the code page of Windows the computer uses.
func TestReadCodePage(t *testing.T) {
	was := systemCodePage
	t.Cleanup(func() { systemCodePage = was })
	systemCodePage = func() int { return 1252 }
	l := only(t, read(t, "excel.csv"))
	if len(l.Entries) != 2 {
		t.Fatalf("%d entries", len(l.Entries))
	}
	if e := l.Entries[0]; e.Title != "Café Noir" || !slices.Equal(e.Artists, []string{"Zoë Ström"}) || e.Album != "Été" ||
		e.Duration != 185*time.Second {
		t.Errorf("the first is %+v", e)
	}
	if e := l.Entries[1]; e.Title != "Naïve Song" || e.Duration != 245*time.Second {
		t.Errorf("the second is %+v", e)
	}
	// Windows-1251 reads Cyrillic.
	systemCodePage = func() int { return 1251 }
	if got := Decode([]byte{0xcf, 0xf0, 0xe8, 0xe2, 0xe5, 0xf2}); got != "Привет" {
		t.Errorf("Windows-1251 reads %q", got)
	}
}

// TestReadUTF16 reads a TSV of UTF-16, with its byte order mark and
// without.
func TestReadUTF16(t *testing.T) {
	for _, name := range []string{"utf16.tsv", "utf16-nobom.tsv"} {
		l := only(t, read(t, name))
		if len(l.Entries) != 2 || l.Entries[0].Title != "Lighthouse Keeper" || l.Entries[0].ISRC != "GBAYE0601499" ||
			l.Entries[1].Title != "灯台守" || !slices.Equal(l.Entries[1].Artists, []string{"海野"}) {
			t.Errorf("%s reads %+v", name, l.Entries)
		}
	}
}

// TestReadUnnamedColumns gives a table whose columns' names MixMusic
// doesn't know, for the user to name: its rows read as they say.
func TestReadUnnamedColumns(t *testing.T) {
	src := read(t, "noheader.csv")
	if len(src.Lists) != 0 || len(src.Tables) != 1 {
		t.Fatalf("%d lists, %d tables", len(src.Lists), len(src.Tables))
	}
	tb := src.Tables[0]
	if tb.Header != -1 || tb.Width() != 3 || len(tb.Rows) != 7 {
		t.Fatalf("header %d, width %d, %d rows", tb.Header, tb.Width(), len(tb.Rows))
	}
	lists := tb.Lists([]Field{FieldTitle, FieldArtists, FieldDuration}, 0)
	if len(lists) != 1 || len(lists[0].Entries) != 6 {
		t.Fatalf("lists %+v", lists)
	}
	if e := lists[0].Entries[5]; e.Title != "Low Tide" || e.Duration != 302*time.Second {
		t.Errorf("the last is %+v", e)
	}
	// Without a header, the first row is a song too.
	if lists := tb.Lists([]Field{FieldTitle, FieldArtists, FieldNone}, -1); len(lists[0].Entries) != 7 {
		t.Errorf("%d songs without a header", len(lists[0].Entries))
	}
}

// TestReadM3U reads an M3U8 list: #EXTINF, #EXTALB and #PLAYLIST,
// relative paths of Windows, file:// URLs, links, and a file of no
// #EXTINF, named by its name.
func TestReadM3U(t *testing.T) {
	l := only(t, read(t, "road trip.m3u8"))
	if l.Name != "Road Trip" || len(l.Entries) != 5 {
		t.Fatalf("list %q of %d: %+v", l.Name, len(l.Entries), l.Entries)
	}
	e := l.Entries[0]
	wantPath := filepath.Join("testdata", "music", "Ola Brine", "01 Harbour Lights.mp3")
	if e.Title != "Harbour Lights" || !slices.Equal(e.Artists, []string{"Ola Brine"}) || e.Album != "Coastline" ||
		e.Duration != 215*time.Second || e.Path != wantPath {
		t.Errorf("the first is %+v, want the path %q", e, wantPath)
	}
	wantFile := "C:/Music/Tam Reed/Tin Roof Rain.flac"
	if runtime.GOOS == "windows" {
		wantFile = `C:\Music\Tam Reed\Tin Roof Rain.flac`
	}
	if e := l.Entries[1]; e.Title != "Tin Roof Rain" || e.Duration != 0 || e.Path != wantFile {
		t.Errorf("the second is %+v, want the path %q", e, wantFile)
	}
	if e := l.Entries[2]; e.VideoID != "hArb0urL1te" || e.Title != "Gulls at Dawn" {
		t.Errorf("the third is %+v", e)
	}
	if e := l.Entries[3]; e.Title != "Salt Wind" || len(e.Artists) != 0 {
		t.Errorf("the fourth is %+v", e)
	}
	if e := l.Entries[4]; e.SpotifyID != "0harbourLights00000001" {
		t.Errorf("the fifth is %+v", e)
	}
	if PathName(wantFile) != "Tin Roof Rain.flac" || PathName(`C:\a\b.mp3`) != "b.mp3" {
		t.Errorf("PathName reads %q", PathName(wantFile))
	}
}

// TestReadLines reads text of a song a line: "Artist - Title", links and
// IDs; comments, blank lines and other links are none.
func TestReadLines(t *testing.T) {
	l := only(t, read(t, "songs.txt"))
	want := []Entry{
		{Title: "Harbour Lights", Artists: []string{"Ola Brine"}},
		{Title: "Tin Roof Rain", Artists: []string{"Tam Reed"}},
		{VideoID: "hArb0urL1te"},
		{SpotifyID: "0gullsAtDawn0000000002"},
		{Title: "Low Tide"},
	}
	if len(l.Entries) != len(want) {
		t.Fatalf("%d entries: %+v", len(l.Entries), l.Entries)
	}
	for i, w := range want {
		if !sameEntry(l.Entries[i], w) {
			t.Errorf("entry %d is %+v, want %+v", i, l.Entries[i], w)
		}
	}
	src, err := ReadText("Pasted", "Ola Brine - Harbour Lights\nhttps://www.youtube.com/watch?v=gUll5aTdawn\n")
	if err != nil || len(src.Lists) != 1 || src.Lists[0].Entries[1].VideoID != "gUll5aTdawn" {
		t.Errorf("pasted text reads %+v, %v", src, err)
	}
	if _, err := ReadText("Pasted", " \n# nothing\n"); !errors.Is(err, ErrNoEntries) {
		t.Errorf("text of nothing reads %v", err)
	}
}

// TestReadJSONObjects reads an array of objects of a song each as a table
// whose keys name its columns.
func TestReadJSONObjects(t *testing.T) {
	l := only(t, read(t, "songs.json"))
	if len(l.Entries) != 2 || l.Entries[0].VideoID != "hArb0urL1te" || l.Entries[0].Duration != 215*time.Second ||
		l.Entries[1].Album != "Rooftops" || !slices.Equal(l.Entries[1].Artists, []string{"Tam Reed"}) {
		t.Errorf("entries %+v", l.Entries)
	}
}

// zipOf is an archive of the files under dir.
func zipOf(t *testing.T, dir string) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		w, err := z.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// TestReadSpotifyDataDownload reads the archive of Spotify's data
// download: its playlists with the user's own files, Your Library, and
// the songs its histories played for 30 seconds or more, the most played
// first. An empty playlist, and episodes, are none.
func TestReadSpotifyDataDownload(t *testing.T) {
	src, err := Read("my_spotify_data.zip", zipOf(t, filepath.Join("testdata", "spotify")))
	if err != nil {
		t.Fatal(err)
	}
	if src.Archive != ArchiveSpotify || len(src.Lists) != 3 {
		t.Fatalf("archive %q, lists %+v", src.Archive, src.Lists)
	}
	sea := src.Lists[0]
	if sea.Name != "Sea Songs" || len(sea.Entries) != 2 || sea.Entries[0].SpotifyID != "0harbourLights00000001" ||
		sea.Entries[1].Title != "Tin Roof Rain" || sea.Entries[1].Duration != 198*time.Second {
		t.Errorf("Sea Songs is %+v", sea)
	}
	if lib := src.Lists[1]; lib.Kind != KindLibrary || len(lib.Entries) != 1 || lib.Entries[0].Title != "Gulls at Dawn" {
		t.Errorf("Your Library is %+v", lib)
	}
	hist := src.Lists[2]
	var got []string
	for _, e := range hist.Entries {
		got = append(got, fmt.Sprintf("%s %d", e.Title, e.Plays))
	}
	if want := []string{"Gulls at Dawn 3", "Tin Roof Rain 2", "Harbour Lights 1"}; hist.Kind != KindHistory || !slices.Equal(got, want) {
		t.Errorf("the history is %q of %q, want %q", got, hist.Kind, want)
	}
}

// TestReadGoogleTakeout reads YouTube Music's files in a Google Takeout:
// its playlists, of either format, and the library's songs; the list of
// the playlists is none.
func TestReadGoogleTakeout(t *testing.T) {
	src, err := Read("takeout-20250101.zip", zipOf(t, filepath.Join("testdata", "takeout")))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range src.Lists {
		var ids []string
		for _, e := range l.Entries {
			ids = append(ids, e.VideoID)
		}
		got = append(got, fmt.Sprintf("%s(%s): %s", l.Name, l.Kind, strings.Join(ids, " ")))
	}
	want := []string{"Your Library(library): hArb0urL1te gUll5aTdawn", "Night Drive(): hArb0urL1te gUll5aTdawn",
		"Summer 2019(): t1nR00fRa1n"}
	if src.Archive != ArchiveGoogle || !slices.Equal(got, want) {
		t.Errorf("the takeout reads %q, want %q", got, want)
	}
	lib := src.Lists[0]
	if e := lib.Entries[1]; e.Title != "Gulls at Dawn" || !slices.Equal(e.Artists, []string{"Ola Brine", "Tam Reed"}) {
		t.Errorf("the library's second is %+v", e)
	}
}

// TestArchiveBounds refuses an archive of more than 1,000 files, and one
// that is no data download.
func TestArchiveBounds(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for i := range MaxArchiveFiles + 1 {
		w, _ := z.Create(fmt.Sprintf("f%d.txt", i))
		w.Write([]byte("x"))
	}
	z.Close()
	if _, err := Read("big.zip", b.Bytes()); !errors.Is(err, ErrArchiveFiles) {
		t.Errorf("1,001 files read %v", err)
	}
	b.Reset()
	z = zip.NewWriter(&b)
	w, _ := z.Create("notes.txt")
	w.Write([]byte("Ola Brine - Harbour Lights"))
	z.Close()
	if _, err := Read("notes.zip", b.Bytes()); !errors.Is(err, ErrNoDataDownload) {
		t.Errorf("an archive of notes reads %v", err)
	}
	if _, err := Read("fake.zip", []byte("not an archive")); !errors.Is(err, ErrNoDataDownload) {
		t.Errorf("a file named .zip reads %v", err)
	}
}

// exported are the lists the round trips write.
var exported = []ExportList{
	{Name: "Sea Songs", Rows: []Row{
		{Title: "Harbour Lights", Artists: []string{"Ola Brine"}, Album: "Coastline", Duration: 215 * time.Second,
			ISRC: "GBAYE0601498", Service: "YouTube Music", MixMusicID: "yt:hArb0urL1te",
			URL: "https://music.youtube.com/watch?v=hArb0urL1te"},
		{Title: "=SUM(A1)", Artists: []string{"@Tam", "Reed"}, Duration: 61 * time.Minute, Service: "Spotify",
			MixMusicID: "sp:0gullsAtDawn0000000002", URL: "https://open.spotify.com/track/0gullsAtDawn0000000002"},
	}},
	{Name: "-Liked", Rows: []Row{
		{Title: "Low Tide", Artists: []string{"Ola Brine"}, Service: "File", MixMusicID: "file:L3NvbmdzL2xvdy5tcDM",
			URL: "file:///songs/low.mp3", Path: "/songs/low.mp3"},
	}},
}

// TestRoundTrip writes lists as CSV, M3U8 and JSON, which read back as
// MixMusic's own, every song in its list and its order.
func TestRoundTrip(t *testing.T) {
	for _, f := range []Format{FormatCSV, FormatM3U8, FormatJSON} {
		var b bytes.Buffer
		if err := Write(&b, f, exported); err != nil {
			t.Fatal(err)
		}
		src, err := Read("library."+string(f), b.Bytes())
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if len(src.Lists) != 2 {
			t.Fatalf("%s reads %+v", f, src.Lists)
		}
		for i, l := range src.Lists {
			if !l.Own || l.Name != exported[i].Name || len(l.Entries) != len(exported[i].Rows) {
				t.Errorf("%s: list %d is %q, own %v, %d songs", f, i, l.Name, l.Own, len(l.Entries))
				continue
			}
			for j, e := range l.Entries {
				r := exported[i].Rows[j]
				if e.Title != r.Title || !slices.Equal(e.Artists, r.Artists) || e.Duration.Round(time.Second) != r.Duration {
					t.Errorf("%s: %d/%d reads %+v, wrote %+v", f, i, j, e, r)
				}
				id := e.MixMusicID
				switch {
				case f == FormatM3U8 && e.VideoID != "":
					id = "yt:" + e.VideoID
				case f == FormatM3U8 && e.SpotifyID != "":
					id = "sp:" + e.SpotifyID
				case f == FormatM3U8:
					id = r.MixMusicID
					if e.Path != filepath.Clean(filepath.FromSlash(r.Path)) {
						t.Errorf("%s: the file reads %q", f, e.Path)
					}
				}
				if id != r.MixMusicID {
					t.Errorf("%s: %d/%d reads the ID %q, wrote %q", f, i, j, id, r.MixMusicID)
				}
			}
		}
	}
}

// TestWriteCSV writes UTF-8 with its mark, CRLF, the 9 columns, and an
// apostrophe before what a spreadsheet would take for a formula.
func TestWriteCSV(t *testing.T) {
	var b bytes.Buffer
	if err := WriteCSV(&b, exported); err != nil {
		t.Fatal(err)
	}
	text := b.String()
	want := "\ufeffPlaylist,Title,Artists,Album,Duration,ISRC,Service,MixMusic ID,URL\r\n" +
		"Sea Songs,Harbour Lights,Ola Brine,Coastline,3:35,GBAYE0601498,YouTube Music,yt:hArb0urL1te,https://music.youtube.com/watch?v=hArb0urL1te\r\n" +
		"Sea Songs,'=SUM(A1),\"'@Tam, Reed\",,1:01:00,,Spotify,sp:0gullsAtDawn0000000002,https://open.spotify.com/track/0gullsAtDawn0000000002\r\n" +
		"'-Liked,Low Tide,Ola Brine,,,,File,file:L3NvbmdzL2xvdy5tcDM,file:///songs/low.mp3\r\n"
	if text != want {
		t.Errorf("the CSV is\n%q\nwant\n%q", text, want)
	}
}

// TestLinks reads the links of songs and their IDs, and nothing of other
// links or of words.
func TestLinks(t *testing.T) {
	for _, c := range []struct {
		in             string
		bare           bool
		video, spotify string
	}{
		{"https://music.youtube.com/watch?v=hArb0urL1te", false, "hArb0urL1te", ""},
		{"https://m.youtube.com/watch?feature=share&v=hArb0urL1te", false, "hArb0urL1te", ""},
		{"https://youtu.be/hArb0urL1te?t=10", false, "hArb0urL1te", ""},
		{"https://www.youtube.com/shorts/hArb0urL1te", false, "hArb0urL1te", ""},
		{"hArb0urL1te", false, "hArb0urL1te", ""},
		{"Hallelujahs", false, "", ""},
		{"Hallelujahs", true, "Hallelujahs", ""},
		{"spotify:track:0harbourLights00000001", false, "", "0harbourLights00000001"},
		{"https://open.spotify.com/track/0harbourLights00000001?si=x", false, "", "0harbourLights00000001"},
		{"https://open.spotify.com/album/0harbourLights00000001", false, "", ""},
		{"https://example.com/watch?v=hArb0urL1te", false, "", ""},
	} {
		v, s := link(c.in, c.bare)
		if v != c.video || s != c.spotify {
			t.Errorf("link(%q, %v) = %q, %q, want %q, %q", c.in, c.bare, v, s, c.video, c.spotify)
		}
	}
}

// TestDurations reads lengths as tables write them.
func TestDurations(t *testing.T) {
	for _, c := range []struct {
		in     string
		millis bool
		want   time.Duration
	}{
		{"3:45", false, 225 * time.Second},
		{"1:02:03", false, time.Hour + 2*time.Minute + 3*time.Second},
		{"225", false, 225 * time.Second},
		{"225000", false, 225 * time.Second},
		{"225000", true, 225 * time.Second},
		{"PT3M45S", false, 225 * time.Second},
		{"3,5", false, 3500 * time.Millisecond},
		{"soon", false, 0},
		{"", false, 0},
	} {
		if got := parseDuration(c.in, c.millis); got != c.want {
			t.Errorf("parseDuration(%q, %v) = %v, want %v", c.in, c.millis, got, c.want)
		}
	}
}

// TestBounds refuses a file of more than 50 MB of text, and one that
// holds no songs.
func TestBounds(t *testing.T) {
	if _, err := Read("big.csv", bytes.Repeat([]byte("a"), MaxText+1)); !errors.Is(err, ErrTooBig) {
		t.Errorf("a big file reads %v", err)
	}
	if _, err := Read("empty.m3u8", []byte("#EXTM3U\n")); !errors.Is(err, ErrNoEntries) {
		t.Errorf("an empty list reads %v", err)
	}
	if _, err := Read("broken.json", []byte(`{"x": 1}`)); !errors.Is(err, ErrNotAList) {
		t.Errorf("JSON of nothing reads %v", err)
	}
}
