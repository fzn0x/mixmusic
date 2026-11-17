package app

// The lyrics of the music on this computer (#554): those of a .lrc file
// beside a file, and of its tags, show offline too and without an artist.
// Only httptest servers on 127.0.0.1 answer; no music service is asked.
// The songs and their lyrics are made up.

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf16"

	"golang.org/x/text/unicode/norm"

	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/lyrics"
)

// flacTagged is the flac package's test file with a VORBIS_COMMENT block
// of pairs in place of any it had.
func flacTagged(t *testing.T, pairs ...string) []byte {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "flac", "testdata", "stereo-16.flac"))
	if err != nil {
		t.Fatal(err)
	}
	if string(src[:4]) != "fLaC" {
		t.Fatal("not FLAC")
	}
	vc := binary.LittleEndian.AppendUint32(nil, 4)
	vc = append(vc, "test"...)
	vc = binary.LittleEndian.AppendUint32(vc, uint32(len(pairs)))
	for _, p := range pairs {
		vc = binary.LittleEndian.AppendUint32(vc, uint32(len(p)))
		vc = append(vc, p...)
	}
	var blocks [][]byte
	at := 4
	for {
		h := src[at]
		n := int(src[at+1])<<16 | int(src[at+2])<<8 | int(src[at+3])
		kind := h & 0x7F
		if kind != 4 { // drop an old VORBIS_COMMENT
			blocks = append(blocks, append([]byte{kind, src[at+1], src[at+2], src[at+3]}, src[at+4:at+4+n]...))
		}
		at += 4 + n
		if h&0x80 != 0 {
			break
		}
		if kind == 0 { // right after STREAMINFO
			blocks = append(blocks, append([]byte{4, byte(len(vc) >> 16), byte(len(vc) >> 8), byte(len(vc))}, vc...))
		}
	}
	blocks[len(blocks)-1][0] |= 0x80
	return append(append([]byte("fLaC"), bytes.Join(blocks, nil)...), src[at:]...)
}

// writeMusicFile writes b to the file name of dir.
func writeMusicFile(t *testing.T, dir, name string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// lyricsIndex is the index of the folders of the music of dir, read as the
// app reads it (tags.Read), with a probe in place of the engine.
func lyricsIndex(t *testing.T, dir string, folders ...string) *localfiles.Index {
	t.Helper()
	x, err := localfiles.Open(filepath.Join(dir, "local-files.json"), filepath.Join(dir, "art"),
		func(io.ReadSeeker) (time.Duration, error) { return 200 * time.Second, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := x.Scan(context.Background(), folders); err != nil {
		t.Fatal(err)
	}
	return x
}

// trackTitled returns the track of x titled title.
func trackTitled(t *testing.T, x *localfiles.Index, title string) media.Track {
	t.Helper()
	for _, tr := range x.Tracks() {
		if tr.Title == title {
			return tr
		}
	}
	t.Fatalf("no track %q in %+v", title, x.Tracks())
	return media.Track{}
}

// localLyricsSources are lyricsSources of a catalog of the music of x,
// Better Lyrics having none; asks counts Better Lyrics' lookups.
func localLyricsSources(t *testing.T, x *localfiles.Index) (ls *lyricsSources, asks *atomic.Int32) {
	ls = newLyricsSources(t)
	asks = withBetterLyrics(t, ls, status(http.StatusNotFound, `{}`))
	ls.c.a.localFiles = x
	return ls, asks
}

// lyricsOffline has every source of ls refuse the connection.
func lyricsOffline(ls *lyricsSources) {
	gone := httptest.NewServer(http.NotFoundHandler())
	gone.Close()
	ls.c.a.betterLyrics.BaseURL = gone.URL
	ls.c.a.unison.BaseURL = gone.URL
	ls.c.a.lyrics.BaseURL = gone.URL
	ls.c.a.youtube.BaseURL = gone.URL
}

// checkLocalLyrics checks that l are lyrics synced or not, from source, of
// first as their first line.
func checkLocalLyrics(t *testing.T, l lyrics.Lyrics, err error, synced bool, source, first string) {
	t.Helper()
	if err != nil || l.Synced != synced || l.Source != source || len(l.Lines) == 0 || l.Lines[0].Text != first {
		t.Errorf("lyrics %+v, %v; want synced %v, from %q, first %q", l, err, synced, source, first)
	}
}

// TestLocalLyricsFromFile shows the lyrics of the .lrc beside a file, of
// a file that names its artist and one that doesn't, online without asking
// the sources, and offline. It is the issue's own test, made to check the
// lyrics.
func TestLocalLyricsFromFile(t *testing.T) {
	dir := t.TempDir()
	music := filepath.Join(dir, "music")
	// A tagged song: its lyrics in its LYRICS comment, and a .lrc beside it.
	writeMusicFile(t, music, "Quiet Field.flac", flacTagged(t, "TITLE=Quiet Field", "ARTIST=Ada Field", "ALBUM=Home Tapes",
		"LYRICS=[00:01.00]A line stored in the file\n[00:04.00]Its second line"))
	writeMusicFile(t, music, "Quiet Field.lrc", []byte("[ar:Ada Field]\n[ti:Quiet Field]\n[00:01.00]A line from the .lrc beside the file\n[00:04.00]Its second line\n"))
	// A song of no artist, as a home recording: a .lrc beside it too.
	writeMusicFile(t, music, "Voice Memo.flac", flacTagged(t, "TITLE=Voice Memo", "LYRICS=Words stored in the file"))
	writeMusicFile(t, music, "Voice Memo.lrc", []byte("[00:01.00]Words beside\n[00:03.00]More words\n"))
	x := lyricsIndex(t, dir, music)
	if !localfiles.NoArtist(trackTitled(t, x, "Voice Memo")) {
		t.Fatal("the voice memo names an artist")
	}
	want := map[string][2]string{
		"Quiet Field": {"Quiet Field.lrc", "A line from the .lrc beside the file"},
		"Voice Memo":  {"Voice Memo.lrc", "Words beside"},
	}
	for title, w := range want {
		tr := trackTitled(t, x, title)
		ls, asks := localLyricsSources(t, x)
		l, err := ls.c.Lyrics(context.Background(), tr)
		checkLocalLyrics(t, l, err, true, w[0], w[1])
		if n := asks.Load() + ls.unisonAsks.Load() + ls.lrcAsks.Load(); n != 0 {
			t.Errorf("%s: the sources were asked %d times for a file of synced lyrics", title, n)
		}
		ls, _ = localLyricsSources(t, x)
		lyricsOffline(ls)
		l, err = ls.c.Lyrics(context.Background(), tr)
		checkLocalLyrics(t, l, err, true, w[0], w[1])
	}
}

// TestLocalLyricsPlain keeps a file's plain lyrics unless a source has
// synced lyrics: over plain lyrics of the sources, when they have none,
// and when they can't be reached.
func TestLocalLyricsPlain(t *testing.T) {
	dir := t.TempDir()
	music := filepath.Join(dir, "music")
	writeMusicFile(t, music, "Glass Rivers.flac", flacTagged(t, "TITLE=Glass Rivers", "ARTIST=Ada Field", "ALBUM=Field Notes",
		"UNSYNCEDLYRICS=Glass rivers\r\nRun clear"))
	writeMusicFile(t, music, "Small Hours.flac", flacTagged(t, "TITLE=Small Hours", "LYRICS=Small hours\nNo artist named"))
	x := lyricsIndex(t, dir, music)
	song := trackTitled(t, x, "Glass Rivers")
	for _, c := range []struct {
		name   string
		unison http.HandlerFunc
		off    bool
		synced bool
		source string
		first  string
	}{
		{"the sources have synced lyrics", status(http.StatusOK, unisonSynced), false, true, "Unison", "From Unison"},
		{"the sources have plain lyrics, longer", status(http.StatusOK, unisonPlain), false, false, "Glass Rivers.flac", "Glass rivers"},
		{"the sources have none", status(http.StatusNotFound, unisonMiss), false, false, "Glass Rivers.flac", "Glass rivers"},
		{"the sources can't be reached", nil, true, false, "Glass Rivers.flac", "Glass rivers"},
	} {
		t.Run(c.name, func(t *testing.T) {
			ls, _ := localLyricsSources(t, x)
			if c.unison != nil {
				ls.unison = c.unison
			}
			if c.off {
				lyricsOffline(ls)
			}
			l, err := ls.c.Lyrics(context.Background(), song)
			checkLocalLyrics(t, l, err, c.synced, c.source, c.first)
			if !c.synced && (len(l.Lines) != 2 || l.Lines[1].Text != "Run clear") {
				t.Errorf("lines %+v", l.Lines)
			}
		})
	}
	// A file of no artist is looked for nowhere: its own lyrics show.
	ls, asks := localLyricsSources(t, x)
	l, err := ls.c.Lyrics(context.Background(), trackTitled(t, x, "Small Hours"))
	checkLocalLyrics(t, l, err, false, "Small Hours.flac", "Small hours")
	if n := asks.Load() + ls.unisonAsks.Load() + ls.lrcAsks.Load(); n != 0 {
		t.Errorf("the sources were asked %d times for a file of no artist", n)
	}
}

// TestLocalLyricsOrder takes a file's synced lyrics before its plain ones,
// and, of each kind, those of the .lrc beside it before those of its tags.
func TestLocalLyricsOrder(t *testing.T) {
	dir := t.TempDir()
	music := filepath.Join(dir, "music")
	synced := "SYNCEDLYRICS=[00:01.00]Synced in the tags\n[00:02.00]Two"
	plain := "LYRICS=Plain in the tags\nTwo"
	songs := []struct {
		title  string
		tags   []string
		lrc    string
		synced bool
		source string
		first  string
	}{
		{"Plain Beside", []string{synced, plain}, "Plain beside\nTwo", true, "Plain Beside.flac", "Synced in the tags"},
		{"Synced Beside", []string{synced, plain}, "[00:01.00]Synced beside\n[00:02.00]Two", true, "Synced Beside.lrc", "Synced beside"},
		{"Both Plain", []string{plain}, "Plain beside\nTwo", false, "Both Plain.lrc", "Plain beside"},
		{"LRC Where Plain Goes", []string{"LYRICS=[00:01.00]Synced where plain goes\n[00:02.00]Two"}, "", true, "LRC Where Plain Goes.flac", "Synced where plain goes"},
		{"One Timed Line", []string{"LYRICS=[00:01.00]One timed line"}, "", false, "One Timed Line.flac", "One timed line"},
	}
	for _, s := range songs {
		writeMusicFile(t, music, s.title+".flac", flacTagged(t, append([]string{"TITLE=" + s.title, "ARTIST=Ada Field"}, s.tags...)...))
		if s.lrc != "" {
			writeMusicFile(t, music, s.title+".lrc", []byte(s.lrc))
		}
	}
	x := lyricsIndex(t, dir, music)
	for _, s := range songs {
		ls, _ := localLyricsSources(t, x)
		lyricsOffline(ls)
		l, err := ls.c.Lyrics(context.Background(), trackTitled(t, x, s.title))
		checkLocalLyrics(t, l, err, s.synced, s.source, s.first)
	}
}

// TestLocalLyricsLRCFiles reads a .lrc of the file's name in another case
// and Unicode form, in UTF-16 with a byte order mark, or in ISO-8859-1,
// with an offset and lines of several times, and leaves out one too large.
func TestLocalLyricsLRCFiles(t *testing.T) {
	dir := t.TempDir()
	music := filepath.Join(dir, "music")
	utf16le := func(s string) []byte {
		b := []byte{0xFF, 0xFE}
		for _, u := range utf16.Encode([]rune(s)) {
			b = binary.LittleEndian.AppendUint16(b, u)
		}
		return b
	}
	// The file's name composed, the .lrc's decomposed, as macOS writes it.
	writeMusicFile(t, music, norm.NFC.String("Été.flac"), flacTagged(t, "TITLE=Été", "ARTIST=Ada Field"))
	writeMusicFile(t, music, norm.NFD.String("ÉTÉ.LRC"), utf16le("[offset:+500]\r\n[00:01.50][00:11.50]L'été\r\n[00:05.50]Encore"))
	writeMusicFile(t, music, "Caf\xc3\xa9.flac", flacTagged(t, "TITLE=Café", "ARTIST=Ada Field"))
	writeMusicFile(t, music, "Caf\xc3\xa9.lrc", []byte("[00:01.00]Caf\xe9 cr\xe8me\n[00:02.00]Au lait"))
	writeMusicFile(t, music, "Large.flac", flacTagged(t, "TITLE=Large", "ARTIST=Ada Field", "LYRICS=From the tags"))
	writeMusicFile(t, music, "Large.lrc", []byte("[00:01.00]Too large\n[00:02.00]"+strings.Repeat("la ", maxLRC/3)))
	x := lyricsIndex(t, dir, music)

	ls, _ := localLyricsSources(t, x)
	lyricsOffline(ls)
	l, err := ls.c.Lyrics(context.Background(), trackTitled(t, x, "Été"))
	checkLocalLyrics(t, l, err, true, norm.NFD.String("ÉTÉ.LRC"), "L'été")
	if len(l.Lines) != 3 || l.Lines[0].TimeMs != 1000 || l.Lines[1].TimeMs != 5000 || l.Lines[2].TimeMs != 11000 || l.Lines[2].Text != "L'été" {
		t.Errorf("lines %+v", l.Lines)
	}
	l, err = ls.c.Lyrics(context.Background(), trackTitled(t, x, "Café"))
	checkLocalLyrics(t, l, err, true, "Caf\xc3\xa9.lrc", "Café crème")
	l, err = ls.c.Lyrics(context.Background(), trackTitled(t, x, "Large"))
	checkLocalLyrics(t, l, err, false, "Large.flac", "From the tags")
}

// TestLocalLyricsCopies reads the lyrics of a song's other copy when the
// one it plays holds none.
func TestLocalLyricsCopies(t *testing.T) {
	dir := t.TempDir()
	single, album := filepath.Join(dir, "single"), filepath.Join(dir, "album")
	writeMusicFile(t, single, "Paper Lanterns.flac", flacTagged(t, "TITLE=Paper Lanterns", "ARTIST=Ada Field"))
	writeMusicFile(t, album, "03 Paper Lanterns.flac", flacTagged(t, "TITLE=Paper Lanterns", "ARTIST=Ada Field", "ALBUM=Kites"))
	writeMusicFile(t, album, "03 Paper Lanterns.lrc", []byte("[00:01.00]Paper lanterns\n[00:03.00]Over the bay"))
	x := lyricsIndex(t, dir, single, album)
	song := trackTitled(t, x, "Paper Lanterns")
	if !localfiles.IsSong(song.ID) {
		t.Fatalf("the two files aren't copies of one song: %+v", x.Tracks())
	}
	ls, _ := localLyricsSources(t, x)
	lyricsOffline(ls)
	l, err := ls.c.Lyrics(context.Background(), song)
	checkLocalLyrics(t, l, err, true, "03 Paper Lanterns.lrc", "Paper lanterns")
}
