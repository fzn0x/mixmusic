package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/text/unicode/norm"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/playlistio"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// fakeImportYT is YouTube Music as the imports' tests have it: songs and
// videos by the queries that find them, videos by their IDs. It keeps
// the queries, how many ran at once at most, and when each started.
type fakeImportYT struct {
	mu            sync.Mutex
	songs, videos map[string][]media.Track
	byID          map[string]media.Track
	queries       []string
	starts        []time.Time
	active, most  int
	// fail, when set, is what every search answers; gate, when set,
	// holds each search until it closes.
	fail error
	gate chan struct{}
}

func (f *fakeImportYT) Search(ctx context.Context, query string, filter youtube.Filter) ([]media.Item, error) {
	f.mu.Lock()
	f.queries = append(f.queries, query+"/"+string(filter))
	f.starts = append(f.starts, time.Now())
	f.active++
	f.most = max(f.most, f.active)
	gate, fail := f.gate, f.fail
	var found []media.Track
	if filter == youtube.FilterSongs {
		found = f.songs[query]
	} else {
		found = f.videos[query]
	}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.active--
		f.mu.Unlock()
	}()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if fail != nil {
		return nil, fail
	}
	time.Sleep(5 * time.Millisecond)
	items := make([]media.Item, len(found))
	for i := range found {
		items[i] = media.Item{Kind: media.ItemTrack, Track: &found[i]}
	}
	return items, nil
}

func (f *fakeImportYT) Song(_ context.Context, id string) (media.Track, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, "song:"+id)
	if t, ok := f.byID[id]; ok {
		return t, nil
	}
	return media.Track{}, youtube.ErrNotFound
}

func (f *fakeImportYT) asked() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.queries)
}

// importSong is a song of YouTube Music.
func importSong(id, title, artist string, ms int) media.Track {
	return media.Track{ID: "yt:" + id, Source: media.SourceYouTube, Title: title, Artists: []media.Ref{{Name: artist}},
		DurationMs: ms, Artwork: []media.Image{}}
}

// importApp is an app of a folder of its own, whose imports look for
// songs in yt.
func importApp(t *testing.T, dir string, yt *fakeImportYT) *App {
	t.Helper()
	a, err := New(Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"), Builtin: builtin.FS, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	(&Collection{a}).Imports().UseServices(ImportServices{Search: yt, Song: yt.Song})
	return a
}

// untilStep waits for the import to come to step.
func untilStep(t *testing.T, i *Imports, step ImportStep) ImportStatus {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		st := i.Status()
		if st.Step == step {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("the import is at %q, not %q: %+v", st.Step, step, st)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// writeImportFile writes text to a file of dir named name, and returns its path.
func writeImportFile(t *testing.T, dir, name, text string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// seaYT is YouTube Music of the songs of seaCSV.
func seaYT() *fakeImportYT {
	return &fakeImportYT{
		songs: map[string][]media.Track{
			"Harbour Lights Ola Brine": {importSong("lights00001", "Harbour Lights", "Ola Brine", 215000)},
			"Gulls at Dawn Ola Brine":  {importSong("gullslive01", "Gulls at Dawn (Live)", "Ola Brine", 241000)},
		},
		videos: map[string][]media.Track{},
	}
}

// seaCSV is a playlist of a CSV file: a Spotify track by its URI, songs
// YouTube Music has, sure and to check, and one it hasn't.
const seaCSV = "Track URI,Track Name,Artist Name(s),Track Duration (ms)\n" +
	"spotify:track:0harbourLights00000001,Low Tide,Tam Reed,198000\n" +
	",Harbour Lights,Ola Brine,215000\n" +
	",Gulls at Dawn,Ola Brine,241000\n" +
	",Salt Wind,Nobody Known,100000\n"

// TestPlaylistImportFinds reads a CSV, finds its songs, and adds those found to a
// new mix named after the file, in their order; importing it again, with
// a song more, adds only that one, to the same mix.
func TestPlaylistImportFinds(t *testing.T) {
	dir := t.TempDir()
	yt := seaYT()
	a := importApp(t, dir, yt)
	im := (&Collection{a}).Imports()
	path := writeImportFile(t, dir, "Sea Songs.csv", seaCSV)
	opened, err := im.Open([]string{path})
	if err != nil || opened.Songs != 4 || len(opened.Failed) != 0 {
		t.Fatalf("opened %+v, %v", opened, err)
	}
	st := untilStep(t, im, ImportReview)
	if want := (ImportCounts{Found: 2, Check: 1, Missing: 1, Ticked: 2}); st.Counts != want || st.Done != 4 || st.Total != 4 {
		t.Errorf("counts %+v, %d of %d, want %+v", st.Counts, st.Done, st.Total, want)
	}
	rows := im.Lists()[0].Rows
	if r := rows[0]; r.State != ImportSure || r.Track.ID != "sp:0harbourLights00000001" || r.Track.Title != "Low Tide" {
		t.Errorf("the Spotify track is %+v", r)
	}
	if r := rows[2]; r.State != ImportCheck || r.Ticked || r.Track.ID != "yt:gullslive01" {
		t.Errorf("the song to check is %+v", r)
	}
	if r := rows[3]; r.State != ImportMissing || r.Track != nil {
		t.Errorf("the song not found is %+v", r)
	}
	for _, q := range yt.asked() {
		if strings.Contains(q, "Low Tide") {
			t.Errorf("the Spotify track was searched: %q", q)
		}
	}
	res, err := im.Finish(context.Background(), ImportTarget{})
	if err != nil || len(res.Mixes) != 1 || res.Added != 2 {
		t.Fatalf("finished %+v, %v", res, err)
	}
	mix, _ := (&Collection{a}).a.library.Playlist(strings.TrimPrefix(res.Mixes[0].ID, "mx:"))
	if mix.Title != "Sea Songs" || len(mix.Tracks) != 2 || mix.Tracks[0].Title != "Low Tide" || mix.Tracks[1].Title != "Harbour Lights" {
		t.Errorf("the mix is %q of %+v", mix.Title, mix.Tracks)
	}
	if im.Status().Step != ImportNone {
		t.Errorf("the import is still at %q", im.Status().Step)
	}

	// Again, with a song more.
	yt.mu.Lock()
	yt.songs["Driftwood Ola Brine"] = []media.Track{importSong("driftwood01", "Driftwood", "Ola Brine", 180000)}
	yt.mu.Unlock()
	writeImportFile(t, dir, "Sea Songs.csv", seaCSV+",Driftwood,Ola Brine,180000\n")
	if _, err := im.Open([]string{path}); err != nil {
		t.Fatal(err)
	}
	untilStep(t, im, ImportReview)
	res, err = im.Finish(context.Background(), ImportTarget{})
	if err != nil || len(res.Mixes) != 1 || res.Mixes[0].ID != mix.ID || res.Added != 1 {
		t.Fatalf("finished again %+v, %v", res, err)
	}
	if got := a.library.Playlists(); len(got) != 1 || got[0].TrackCount != 3 {
		t.Errorf("the mixes are %+v", got)
	}
}

// TestPlaylistImportPace searches 4 songs at once at most, each starting 100 ms
// after the one before at least, and keeps what it found: the songs
// imported again aren't searched.
func TestPlaylistImportPace(t *testing.T) {
	dir := t.TempDir()
	yt := &fakeImportYT{songs: map[string][]media.Track{}}
	var text strings.Builder
	for n := range 8 {
		title := fmt.Sprintf("Song %d", n)
		text.WriteString("Ola Brine - " + title + "\n")
		yt.songs[title+" Ola Brine"] = []media.Track{importSong(fmt.Sprintf("song%07d", n), title, "Ola Brine", 0)}
	}
	a := importApp(t, dir, yt)
	im := (&Collection{a}).Imports()
	if _, err := im.Paste(text.String()); err != nil {
		t.Fatal(err)
	}
	if st := untilStep(t, im, ImportReview); st.Counts.Found != 8 {
		t.Fatalf("counts %+v", st.Counts)
	}
	yt.mu.Lock()
	starts, most := slices.Clone(yt.starts), yt.most
	yt.mu.Unlock()
	if most > importSearches {
		t.Errorf("%d searches ran at once", most)
	}
	for k := 1; k < len(starts); k++ {
		if gap := starts[k].Sub(starts[k-1]); gap < importGap-10*time.Millisecond {
			t.Errorf("search %d started %v after the one before", k, gap)
		}
	}
	im.Cancel()
	n := len(yt.asked())
	if _, err := im.Paste(text.String()); err != nil {
		t.Fatal(err)
	}
	untilStep(t, im, ImportReview)
	if got := yt.asked(); len(got) != n {
		t.Errorf("searched again: %q", got[n:])
	}
}

// TestPlaylistImportHalts stops looking when YouTube Music refuses for asking too
// often, and after 3 failures in a row, until the user tries again.
func TestPlaylistImportHalts(t *testing.T) {
	dir := t.TempDir()
	yt := seaYT()
	yt.fail = &youtube.APIError{Status: http.StatusTooManyRequests}
	a := importApp(t, dir, yt)
	im := (&Collection{a}).Imports()
	if _, err := im.Open([]string{writeImportFile(t, dir, "Sea Songs.csv", seaCSV)}); err != nil {
		t.Fatal(err)
	}
	st := untilStep(t, im, ImportHalted)
	if got := ErrorWords(st.Halt); !strings.Contains(got, "slow down") {
		t.Errorf("halted for %q", got)
	}
	yt.mu.Lock()
	yt.fail = errors.New("connection reset")
	yt.mu.Unlock()
	im.TryAgain()
	st = untilStep(t, im, ImportHalted)
	if got := ErrorWords(st.Halt); got != "3 searches in a row failed: connection reset" {
		t.Errorf("halted for %q", got)
	}
	yt.mu.Lock()
	yt.fail = nil
	yt.mu.Unlock()
	im.TryAgain()
	if st := untilStep(t, im, ImportReview); st.Counts.Found != 2 {
		t.Errorf("counts %+v after trying again", st.Counts)
	}
}

// TestPlaylistImportResumes keeps an import under way as the app closes, and
// goes on with it at the next start.
func TestPlaylistImportResumes(t *testing.T) {
	dir := t.TempDir()
	yt := seaYT()
	yt.gate = make(chan struct{})
	a := importApp(t, dir, yt)
	im := (&Collection{a}).Imports()
	if _, err := im.Open([]string{writeImportFile(t, dir, "Sea Songs.csv", seaCSV)}); err != nil {
		t.Fatal(err)
	}
	for len(yt.asked()) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	a.stopImport()
	if st := im.Status(); st.Step != ImportHalted || ErrorWords(st.Halt) != "MixMusic quit while it looked for the songs" {
		t.Errorf("closed, the import is %+v", st)
	}

	yt2 := seaYT()
	b := importApp(t, dir, yt2)
	b.resumeImport()
	st := untilStep(t, (&Collection{b}).Imports(), ImportReview)
	if st.Counts.Found != 2 || st.Counts.Missing != 1 {
		t.Errorf("resumed, counts %+v", st.Counts)
	}
}

// TestPlaylistImportLiked likes the songs of an import, first, in their order.
func TestPlaylistImportLiked(t *testing.T) {
	dir := t.TempDir()
	a := importApp(t, dir, seaYT())
	im := (&Collection{a}).Imports()
	if _, err := im.Open([]string{writeImportFile(t, dir, "Sea Songs.csv", seaCSV)}); err != nil {
		t.Fatal(err)
	}
	untilStep(t, im, ImportReview)
	im.SetTicked(0, 2, true)
	res, err := im.Finish(context.Background(), ImportTarget{Liked: true})
	if err != nil || !res.Liked || res.Added != 3 {
		t.Fatalf("finished %+v, %v", res, err)
	}
	var got []string
	for _, tr := range a.library.Liked() {
		got = append(got, tr.Title)
	}
	if want := []string{"Low Tide", "Harbour Lights", "Gulls at Dawn (Live)"}; !slices.Equal(got, want) {
		t.Errorf("liked %q, want %q", got, want)
	}
}

// TestPlaylistImportOwnExports exports the library as CSV, M3U8 and JSON, which
// import back every track in its order, without asking the network.
func TestPlaylistImportOwnExports(t *testing.T) {
	dir := t.TempDir()
	yt := &fakeImportYT{fail: errors.New("no network in this test")}
	a := importApp(t, dir, yt)
	tracks := []media.Track{importSong("lights00001", "Harbour Lights", "Ola Brine", 215000),
		{ID: "sp:0gullsAtDawn0000000002", Source: media.SourceSpotify, Title: "=Gulls at Dawn",
			Artists: []media.Ref{{Name: "Ola Brine"}, {Name: "Tam Reed"}}, DurationMs: 241000, ISRC: "GBAYE0601498"}}
	if _, err := a.library.CreatePlaylist("Sea Songs", "", tracks); err != nil {
		t.Fatal(err)
	}
	if err := a.library.SetLiked(tracks[1], true); err != nil {
		t.Fatal(err)
	}
	im := (&Collection{a}).Imports()
	for _, ext := range []string{".csv", ".m3u8", ".json"} {
		path := filepath.Join(dir, "library"+ext)
		if err := im.Export(path, im.Library()); err != nil {
			t.Fatal(err)
		}
		if _, err := im.Open([]string{path}); err != nil {
			t.Fatalf("%s: %v", ext, err)
		}
		st := untilStep(t, im, ImportPick)
		if len(st.Lists) != 2 || st.Lists[0].Name != "Liked songs" || st.Lists[1].Name != "Sea Songs" {
			t.Fatalf("%s: lists %+v", ext, st.Lists)
		}
		im.Pick([]bool{true, true})
		st = untilStep(t, im, ImportReview)
		if st.Counts.InLibrary != 3 || st.Counts.Ticked != 3 {
			t.Errorf("%s: counts %+v", ext, st.Counts)
		}
		mix := im.Lists()[1].Rows
		if len(mix) != 2 || mix[0].Track.ID != tracks[0].ID || mix[1].Track.ID != tracks[1].ID ||
			mix[1].Track.Title != "=Gulls at Dawn" || !slices.Equal(refNamesOf(mix[1].Track.Artists), []string{"Ola Brine", "Tam Reed"}) {
			t.Errorf("%s: the mix reads %+v", ext, mix)
		}
		im.Cancel()
	}
	if got := yt.asked(); len(got) != 0 {
		t.Errorf("asked the network %q", got)
	}
}

func refNamesOf(refs []media.Ref) []string {
	var out []string
	for _, r := range refs {
		out = append(out, r.Name)
	}
	return out
}

// TestPlaylistImportColumns has the user name the columns a table's names don't
// tell, then imports its songs.
func TestPlaylistImportColumns(t *testing.T) {
	dir := t.TempDir()
	a := importApp(t, dir, seaYT())
	im := (&Collection{a}).Imports()
	path := writeImportFile(t, dir, "Liste.csv", "Titel,Künstler\nHarbour Lights,Ola Brine\nSalt Wind,Nobody Known\n")
	if _, err := im.Open([]string{path}); err != nil {
		t.Fatal(err)
	}
	st := untilStep(t, im, ImportColumns)
	if st.Table == nil || st.Table.Width() != 2 {
		t.Fatalf("the table is %+v", st.Table)
	}
	im.SetColumns([]playlistio.Field{playlistio.FieldTitle, playlistio.FieldArtists}, true)
	st = untilStep(t, im, ImportReview)
	if st.Counts.Found != 1 || st.Counts.Missing != 1 || st.Lists[0].Name != "Liste" {
		t.Errorf("status %+v", st)
	}
}

// TestPlaylistImportFiles finds the files a list names: by their paths, and of a
// path of another computer, by a name of the music on this computer, the
// same once Unicode normalizes it, as macOS keeps names decomposed.
func TestPlaylistImportFiles(t *testing.T) {
	a, music := localApp(t)
	flac, err := os.ReadFile(filepath.Join(music, "Shore", "01 Slow Tide.flac"))
	if err != nil {
		t.Fatal(err)
	}
	cafe := norm.NFC.String("03 Café Noir.flac")
	if err := os.WriteFile(filepath.Join(music, "Shore", cafe), flac, 0o644); err != nil {
		t.Fatal(err)
	}
	scanned(t, a)
	im := (&Collection{a}).Imports()
	im.UseServices(ImportServices{Search: &fakeImportYT{fail: errors.New("no network in this test")}})
	here := filepath.Join(music, "Shore", "01 Slow Tide.flac")
	far := norm.NFD.String(`D:\Elsewhere\Shore\` + cafe)
	if far == `D:\Elsewhere\Shore\`+cafe {
		t.Fatal("the name doesn't decompose")
	}
	if _, err := im.Open([]string{writeImportFile(t, t.TempDir(), "files.m3u", "#EXTM3U\n"+here+"\n"+far+"\n")}); err != nil {
		t.Fatal(err)
	}
	st := untilStep(t, im, ImportReview)
	if st.Counts.InLibrary != 2 {
		t.Errorf("counts %+v", st.Counts)
	}
	rows := im.Lists()[0].Rows
	for k, r := range rows {
		if r.Track == nil || r.Track.Source != media.SourceFile || r.State != ImportSure {
			t.Errorf("row %d is %+v", k, r)
		}
	}
	if p, _ := a.localFiles.FileOf(rows[1].Track.ID); filepath.Base(p) != cafe {
		t.Errorf("the far file is %q", p)
	}
}

// TestPlaylistImportRefuses tells the files it couldn't read, and refuses more
// than 100 at once.
func TestPlaylistImportRefuses(t *testing.T) {
	dir := t.TempDir()
	a := importApp(t, dir, seaYT())
	im := (&Collection{a}).Imports()
	empty := writeImportFile(t, dir, "empty.m3u8", "#EXTM3U\n")
	good := writeImportFile(t, dir, "good.txt", "Ola Brine - Harbour Lights\n")
	opened, err := im.Open([]string{empty, good})
	if err != nil || len(opened.Failed) != 1 || ErrorWords(opened.Failed[0].Err) != "it holds no songs" {
		t.Errorf("opened %+v, %v", opened, err)
	}
	if _, err := im.Open([]string{empty}); err == nil || ErrorWords(err) != "it holds no songs" {
		t.Errorf("an empty list opens with %v", err)
	}
	if _, err := im.Open(make([]string, 101)); err == nil || ErrorWords(err) != "MixMusic imports 100 files at once at most" {
		t.Errorf("101 files open with %v", err)
	}
}
