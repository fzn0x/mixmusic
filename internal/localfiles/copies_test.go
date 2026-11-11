package localfiles

import (
	"bytes"
	"encoding/json"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// The songs here are made up. A test file's length, in seconds, is its
// number of bytes, as the fake probe of newIndex reads it: tagged pads its
// tags with spaces to a length.

// tagged is the text of a test file of tags "title|artist|album||1", which
// plays seconds long.
func tagged(t *testing.T, title, artist, album string, seconds int) string {
	t.Helper()
	s := title + "|" + artist + "|" + album + "||1"
	if len(s) > seconds {
		t.Fatalf("%q is longer than %d seconds", s, seconds)
	}
	return s + strings.Repeat(" ", seconds-len(s))
}

// TestCopyOrder orders copies best first: lossless before lossy, then the
// higher bitrate, then the higher sample rate, whatever order they come
// in.
func TestCopyOrder(t *testing.T) {
	want := []Copy{
		{Path: "/music/e.wav", Facts: Facts{Codec: "PCM", SampleRate: 44100, BitDepth: 16}, Bitrate: 1411},
		{Path: "/music/d.flac", Facts: Facts{Codec: "FLAC", SampleRate: 96000, BitDepth: 24}, Bitrate: 900},
		{Path: "/music/c.flac", Facts: Facts{Codec: "FLAC", SampleRate: 44100, BitDepth: 16}, Bitrate: 900},
		{Path: "/music/f.opus", Facts: Facts{Codec: "Opus", SampleRate: 48000}, Bitrate: 320},
		{Path: "/music/a.mp3", Facts: Facts{Codec: "MP3", SampleRate: 44100}, Bitrate: 320},
		{Path: "/music/g.m4a", Facts: Facts{Codec: "AAC", SampleRate: 44100}, Bitrate: 320},
		{Path: "/music/b.mp3", Facts: Facts{Codec: "MP3", SampleRate: 48000}, Bitrate: 128},
	}
	r := rand.New(rand.NewPCG(1, 2))
	for range 20 {
		got := slices.Clone(want)
		r.Shuffle(len(got), func(i, j int) { got[i], got[j] = got[j], got[i] })
		slices.SortFunc(got, compareCopies)
		if !slices.EqualFunc(got, want, func(a, b Copy) bool { return a.Path == b.Path }) {
			t.Fatalf("the order is %v", paths(got))
		}
	}
	// Without their audio read, their extensions tell their codecs.
	for path, lossless := range map[string]bool{"a.flac": true, "a.WAV": true, "a.wave": true, "a.mp3": false,
		"a.m4a": false, "a.opus": false, "a.ogg": false} {
		if got := guessFacts(path).Lossless(); got != lossless {
			t.Errorf("%s is lossless: %v", path, got)
		}
	}
}

func paths(cs []Copy) []string {
	var out []string
	for _, c := range cs {
		out = append(out, filepath.Base(c.Path))
	}
	return out
}

// TestCopiesShowOnce shows files of the same title and artists, a few
// seconds apart at most, once in the songs and the search, which find it
// by the album of each; each album shows its own file; and the song plays
// its best copy, the FLAC. Files of another artist, of a length further
// apart, or without tags stay apart. The links are kept in the data
// folder; the music files stay as they were.
func TestCopiesShowOnce(t *testing.T) {
	ti := newIndex(t)
	flac := ti.write(t, "Ada Field/Kites/01.flac", tagged(t, "Paper Kites", "Ada Field", "Kites", 60), time.Hour)
	mp3 := ti.write(t, "Singles/Paper Kites.mp3", tagged(t, "paper  kites", "ADA FIELD", "Paper Kites - Single", 62), time.Hour)
	ti.write(t, "Others/kites.mp3", tagged(t, "Paper Kites", "Orin Vale", "Covers", 60), time.Hour)
	ti.write(t, "Live/kites.mp3", tagged(t, "Paper Kites", "Ada Field", "Live", 90), time.Hour)
	ti.write(t, "Memos/a/memo.mp3", strings.Repeat(" ", 60), time.Hour)
	ti.write(t, "Memos/b/memo.mp3", strings.Repeat(" ", 60), time.Hour)
	before, _ := os.ReadFile(mp3)
	ti.scan(t)

	tracks := ti.Tracks()
	var songs []media.Track
	for _, tr := range tracks {
		if IsSong(tr.ID) {
			songs = append(songs, tr)
		}
	}
	if len(tracks) != 5 || len(songs) != 1 {
		t.Fatalf("the songs are %+v", tracks)
	}
	song := songs[0]
	if song.Title != "Paper Kites" || song.Album == nil || song.Album.Name != "Kites" {
		t.Errorf("the song shows as %+v", song)
	}
	for _, q := range []string{"paper kites ada", "single"} {
		found := ti.Search(q, 10).Tracks
		if n := slices.IndexFunc(found, func(tr media.Track) bool { return tr.ID == song.ID }); n < 0 ||
			slices.ContainsFunc(found, func(tr media.Track) bool { return tr.ID == TrackID(mp3) || tr.ID == TrackID(flac) }) {
			t.Errorf("a search of %q finds %+v", q, found)
		}
	}
	for _, a := range ti.Albums() {
		al, _ := ti.Album(a.ID)
		if a.Title == "Kites" && (len(al.Tracks) != 1 || al.Tracks[0].ID != TrackID(flac)) {
			t.Errorf("the album Kites holds %+v", al.Tracks)
		}
		if a.Title == "Paper Kites - Single" && (len(al.Tracks) != 1 || al.Tracks[0].ID != TrackID(mp3)) {
			t.Errorf("the single holds %+v", al.Tracks)
		}
	}
	artist, _ := ti.Artist(artistID("Ada Field"))
	if n := len(artist.TopTracks); n != 2 {
		t.Errorf("Ada Field has %d songs: %+v", n, artist.TopTracks)
	}
	if p, ok := ti.FileOf(song.ID); !ok || p != flac {
		t.Errorf("the song plays %q", p)
	}
	s, ok := ti.Song(TrackID(mp3))
	if !ok || s.Track.ID != song.ID || len(s.Copies) != 2 || s.Copies[0].Path != flac || !s.Copies[0].Best || s.Copies[1].Best {
		t.Errorf("the song of the MP3 is %+v", s)
	}
	if got := ti.SongTrack(fileTrack(mp3)); got.ID != song.ID {
		t.Errorf("the MP3's song is %q", got.ID)
	}
	if kept := keptLinks(t, ti); len(kept.Songs) != 1 || !slices.Equal(kept.Songs[0].Copies, []string{flac, mp3}) {
		t.Errorf("the links kept: %+v", kept)
	}
	if after, _ := os.ReadFile(mp3); !bytes.Equal(before, after) {
		t.Error("a file of music changed")
	}
}

// fileTrack is the track of the file at path.
func fileTrack(path string) media.Track {
	return media.Track{ID: TrackID(path), Source: media.SourceFile}
}

// keptLinks reads the links the index keeps in its data folder.
func keptLinks(t *testing.T, ti *testIndex) savedLinks {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(filepath.Dir(ti.dir), "local-copies.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s savedLinks
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	return decodePaths(s)
}

// songsOf are the IDs of the songs with copies of an index's songs.
func songsOf(x *Index) []string {
	var out []string
	for _, tr := range x.Tracks() {
		if IsSong(tr.ID) {
			out = append(out, tr.ID)
		}
	}
	return out
}

// TestCopyGoes keeps a song's ID as its copies go: the one left plays it,
// and with none left, the song is missing, until a file of it comes back,
// elsewhere, as one moved.
func TestCopyGoes(t *testing.T) {
	ti := newIndex(t)
	flac := ti.write(t, "Kites/01.flac", tagged(t, "Paper Kites", "Ada Field", "Kites", 60), time.Hour)
	mp3 := ti.write(t, "Singles/kites.mp3", tagged(t, "Paper Kites", "Ada Field", "Single", 61), time.Hour)
	ti.scan(t)
	ids := songsOf(ti.Index)
	if len(ids) != 1 {
		t.Fatalf("songs %q", ids)
	}
	id := ids[0]
	if err := os.Remove(flac); err != nil {
		t.Fatal(err)
	}
	ti.scan(t)
	if got := songsOf(ti.Index); !slices.Equal(got, ids) {
		t.Fatalf("with one copy gone, the songs are %q", got)
	}
	if p, ok := ti.FileOf(id); !ok || p != mp3 || ti.Missing(id) {
		t.Errorf("the song plays %q, %v; missing: %v", p, ok, ti.Missing(id))
	}
	away := filepath.Join(filepath.Dir(ti.dir), "away")
	if err := os.Rename(filepath.Join(ti.dir, "Singles"), away); err != nil {
		t.Fatal(err)
	}
	ti.scan(t)
	if got := songsOf(ti.Index); len(got) != 0 || !ti.Missing(id) {
		t.Fatalf("with no copy, the songs are %q, missing %v", got, ti.Missing(id))
	}
	moved := filepath.Join(ti.dir, "Moved")
	if err := os.Rename(away, moved); err != nil {
		t.Fatal(err)
	}
	ti.scan(t)
	if got := songsOf(ti.Index); !slices.Equal(got, ids) {
		t.Errorf("its file back, elsewhere, the songs are %q, want %q", got, ids)
	}
	if p, _ := ti.FileOf(id); p != filepath.Join(moved, "kites.mp3") {
		t.Errorf("the song plays %q", p)
	}
}

// TestCopiesMoved keeps a song's ID when its copies move together, as a
// folder renamed.
func TestCopiesMoved(t *testing.T) {
	ti := newIndex(t)
	ti.write(t, "Ada/Kites/01.flac", tagged(t, "Paper Kites", "Ada Field", "Kites", 60), time.Hour)
	ti.write(t, "Ada/Single/01.mp3", tagged(t, "Paper Kites", "Ada Field", "Single", 61), time.Hour)
	ti.scan(t)
	ids := songsOf(ti.Index)
	if err := os.Rename(filepath.Join(ti.dir, "Ada"), filepath.Join(ti.dir, "Ada Field")); err != nil {
		t.Fatal(err)
	}
	ti.scan(t)
	if got := songsOf(ti.Index); len(ids) != 1 || !slices.Equal(got, ids) {
		t.Errorf("moved, the songs are %q, were %q", got, ids)
	}
}

// TestCopiesFollowTags takes a copy out of its song once its tags say
// another song, and back once they say it again; the song keeps its ID.
func TestCopiesFollowTags(t *testing.T) {
	ti := newIndex(t)
	flac := ti.write(t, "Kites/01.flac", tagged(t, "Paper Kites", "Ada Field", "Kites", 60), time.Hour)
	mp3 := ti.write(t, "Single/01.mp3", tagged(t, "Paper Kites", "Ada Field", "Single", 61), time.Hour)
	ti.scan(t)
	ids := songsOf(ti.Index)
	retag := func(text string) {
		t.Helper()
		if err := os.WriteFile(mp3, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		ti.Reread([]string{mp3})
	}
	retag(tagged(t, "Kites (Demo)", "Ada Field", "Single", 61))
	if got := songsOf(ti.Index); !slices.Equal(got, ids) || len(ti.Tracks()) != 2 {
		t.Fatalf("retagged, the songs are %q of %+v", got, ti.Tracks())
	}
	if s, _ := ti.Song(ids[0]); len(s.Copies) != 1 || s.Copies[0].Path != flac {
		t.Errorf("the song's copies are %+v", s.Copies)
	}
	retag(tagged(t, "Paper Kites", "Ada Field", "Single", 61))
	if s, _ := ti.Song(ids[0]); len(s.Copies) != 2 || len(ti.Tracks()) != 1 {
		t.Errorf("tagged back, the song's copies are %+v", s.Copies)
	}
}

// TestLinkByHand links files of other titles, which stay linked whatever
// their tags say; and unlinks a copy, which only a link by hand puts back,
// the song keeping its ID.
func TestLinkByHand(t *testing.T) {
	ti := newIndex(t)
	a := ti.write(t, "Kites/01.flac", tagged(t, "Paper Kites", "Ada Field", "Kites", 60), time.Hour)
	b := ti.write(t, "Live/01.mp3", tagged(t, "Kites, live at the pier", "Ada Field", "Live", 240), time.Hour)
	c := ti.write(t, "Single/01.mp3", tagged(t, "Paper Kites", "Ada Field", "Single", 61), time.Hour)
	ti.scan(t)
	ids := songsOf(ti.Index)
	if len(ids) != 1 {
		t.Fatalf("songs %q", ids)
	}
	if err := ti.Link([]string{ids[0], TrackID(b)}); err != nil {
		t.Fatal(err)
	}
	s, _ := ti.Song(TrackID(b))
	if s.Track.ID != ids[0] || len(s.Copies) != 3 || len(ti.Tracks()) != 1 ||
		!slices.ContainsFunc(s.Copies, func(cp Copy) bool { return cp.Path == b && cp.Linked }) {
		t.Fatalf("linked, the song is %+v of %d tracks", s, len(ti.Tracks()))
	}
	if err := os.WriteFile(b, []byte(tagged(t, "Kites (Pier)", "Ada Field", "Live", 240)), 0o644); err != nil {
		t.Fatal(err)
	}
	ti.Reread([]string{b})
	if s, _ := ti.Song(ids[0]); len(s.Copies) != 3 {
		t.Errorf("retagged, a file linked by hand left: %+v", s.Copies)
	}
	if err := ti.Unlink(TrackID(c)); err != nil {
		t.Fatal(err)
	}
	ti.scan(t)
	if s, _ := ti.Song(ids[0]); len(s.Copies) != 2 || slices.ContainsFunc(s.Copies, func(cp Copy) bool { return cp.Path == c }) {
		t.Errorf("unlinked, the song's copies are %+v", s.Copies)
	}
	if got := ti.SongTrack(fileTrack(c)); got.ID != TrackID(c) || len(ti.Tracks()) != 2 {
		t.Errorf("the copy unlinked is %q, of %d tracks", got.ID, len(ti.Tracks()))
	}
	if err := ti.Unlink(TrackID(a)); err != nil {
		t.Fatal(err)
	}
	if err := ti.Unlink(TrackID(b)); err == nil {
		t.Error("the last copy of a song was unlinked")
	}
	if err := ti.Link([]string{TrackID(c), ids[0]}); err != nil {
		t.Fatal(err)
	}
	if s, _ := ti.Song(TrackID(c)); s.Track.ID != ids[0] || len(s.Copies) != 2 {
		t.Errorf("linked again, the song is %+v", s)
	}
	if err := ti.Link([]string{TrackID(a)}); err == nil {
		t.Error("a file was linked alone")
	}
	opened := filepath.Join(t.TempDir(), "opened.mp3")
	if err := ti.Link([]string{TrackID(a), TrackID(opened)}); err != errOutside {
		t.Errorf("a file beyond the folders of music linked: %v", err)
	}
}

// TestLinkSongs makes two songs with copies one, by hand: OnLinked hears
// that the second became the first, and that each copy's file is the
// first's.
func TestLinkSongs(t *testing.T) {
	ti := newIndex(t)
	var told []map[string]media.Track
	ti.OnLinked(func(m map[string]media.Track) { told = append(told, m) })
	a := ti.write(t, "Kites/01.flac", tagged(t, "Paper Kites", "Ada Field", "Kites", 60), time.Hour)
	b := ti.write(t, "Single/01.mp3", tagged(t, "Paper Kites", "Ada Field", "Single", 61), time.Hour)
	c := ti.write(t, "Live/01.flac", tagged(t, "Kites (Live)", "Ada Field", "Live", 120), time.Hour)
	d := ti.write(t, "Live/02.mp3", tagged(t, "Kites (Live)", "Ada Field", "Live", 121), time.Hour)
	ti.scan(t)
	ids := songsOf(ti.Index)
	if len(ids) != 2 || len(told) != 1 {
		t.Fatalf("songs %q, told %d times", ids, len(told))
	}
	for _, p := range []string{a, b, c, d} {
		if _, ok := told[0][TrackID(p)]; !ok {
			t.Errorf("OnLinked didn't hear of %s", filepath.Base(p))
		}
	}
	first, _ := ti.Song(TrackID(a))
	second, _ := ti.Song(TrackID(c))
	if err := ti.Link([]string{first.Track.ID, second.Track.ID}); err != nil {
		t.Fatal(err)
	}
	last := told[len(told)-1]
	if got := last[second.Track.ID]; got.ID != first.Track.ID {
		t.Errorf("the second song became %q", got.ID)
	}
	if got := last[TrackID(d)]; got.ID != first.Track.ID {
		t.Errorf("a copy of the second is %q", got.ID)
	}
	if got := songsOf(ti.Index); !slices.Equal(got, []string{first.Track.ID}) {
		t.Errorf("the songs are %q", got)
	}
	ti.scan(t)
	if got := songsOf(ti.Index); !slices.Equal(got, []string{first.Track.ID}) {
		t.Errorf("after a scan, the songs are %q", got)
	}
}

// TestLinksKept keeps the songs across runs: a new run shows them before
// any scan, and tells OnLinked of them once, after its first scan; and
// one whose links were lost makes the songs of the same IDs again.
func TestLinksKept(t *testing.T) {
	ti := newIndex(t)
	ti.write(t, "Kites/01.flac", tagged(t, "Paper Kites", "Ada Field", "Kites", 60), time.Hour)
	ti.write(t, "Single/01.mp3", tagged(t, "Paper Kites", "Ada Field", "Single", 61), time.Hour)
	ti.scan(t)
	ids := songsOf(ti.Index)
	dir := filepath.Dir(ti.dir)
	reopen := func() *Index {
		return ti.open(t, filepath.Join(dir, "local-files.json"), filepath.Join(dir, "cache", "local-art"))
	}
	x := reopen()
	var told int
	x.OnLinked(func(map[string]media.Track) { told++ })
	if got := songsOf(x); len(ids) != 1 || !slices.Equal(got, ids) {
		t.Errorf("a new run shows %q, want %q", got, ids)
	}
	for range 2 {
		if err := x.Scan(t.Context(), []string{ti.dir}); err != nil {
			t.Fatal(err)
		}
	}
	if told != 1 {
		t.Errorf("OnLinked told %d times", told)
	}
	if err := os.Remove(filepath.Join(dir, "local-copies.json")); err != nil {
		t.Fatal(err)
	}
	x = reopen()
	if err := x.Scan(t.Context(), []string{ti.dir}); err != nil {
		t.Fatal(err)
	}
	if got := songsOf(x); !slices.Equal(got, ids) {
		t.Errorf("its links lost, a run shows %q, want %q", got, ids)
	}
}

// TestCopyFacts reads what the copies' audio is, once each while their
// files don't change, which orders two of the same bitrate by their sample
// rates; and a song the index doesn't know, as when its links were lost,
// plays the copy its ID names.
func TestCopyFacts(t *testing.T) {
	ti := newIndex(t)
	var read int
	ti.Inspect(func(r io.ReadSeeker) (Facts, error) {
		read++
		b, _ := io.ReadAll(r)
		f := Facts{Codec: "FLAC", BitDepth: 16}
		if _, rate, ok := strings.Cut(string(b), "rate="); ok {
			f.SampleRate, _ = strconv.Atoi(strings.TrimSpace(rate))
		}
		return f, nil
	})
	low := ti.write(t, "A/01.flac", tagged(t, "Paper Kites", "Ada Field", "Kites", 60)+"rate=44100", time.Hour)
	high := ti.write(t, "B/01.flac", tagged(t, "Paper Kites", "Ada Field", "Kites", 60)+"rate=96000", time.Hour)
	ti.scan(t)
	s, ok := ti.Song(TrackID(low))
	if !ok || len(s.Copies) != 2 || s.Copies[0].Path != high || s.Copies[0].SampleRate != 96000 || read != 2 {
		t.Fatalf("the song is %+v, of %d reads", s, read)
	}
	if s.Track.ID != SongID(high) {
		t.Errorf("the song's ID is %q, of its best copy %q", s.Track.ID, SongID(high))
	}
	ti.scan(t)
	if read != 2 {
		t.Errorf("read the copies' audio %d times", read)
	}
	if kept := keptLinks(t, ti); len(kept.Facts) != 2 {
		t.Errorf("the facts kept are %+v", kept.Facts)
	}
	if p, ok := ti.FileOf(SongID(low)); !ok || p != low {
		t.Errorf("a song the index doesn't know plays %q, %v", p, ok)
	}
}
