package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// importYT is YouTube Music as these tests have it: songs by the queries
// that find them, and the queries asked.
type importYT struct {
	mu      sync.Mutex
	songs   map[string][]media.Track
	queries []string
}

func (y *importYT) Search(_ context.Context, query string, filter youtube.Filter) ([]media.Item, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	y.queries = append(y.queries, query+"/"+string(filter))
	if filter != youtube.FilterSongs {
		return nil, nil
	}
	found := y.songs[query]
	items := make([]media.Item, len(found))
	for i := range found {
		items[i] = media.Item{Kind: media.ItemTrack, Track: &found[i]}
	}
	return items, nil
}

func (y *importYT) Song(_ context.Context, id string) (media.Track, error) {
	return media.Track{}, youtube.ErrNotFound
}

// importSongs are the songs the tests' YouTube Music finds.
var importSongs = map[string][]media.Track{
	"Harbour Lights Ola Brine": {testTrack("yt:lights00001", "Harbour Lights", "Ola Brine", "Coastline", media.SourceYouTube)},
	"Gulls at Dawn Ola Brine":  {testTrack("yt:gullslive01", "Gulls at Dawn (Live)", "Ola Brine", "Pier", media.SourceYouTube)},
	"Salt Wind":                {testTrack("yt:saltwind001", "Salt Wind", "Tam Reed", "Rooftops", media.SourceYouTube)},
}

// importCSV is a playlist whose songs YouTube Music has, sure and to
// check, and hasn't.
const importCSV = "Title,Artist\nHarbour Lights,Ola Brine\nGulls at Dawn,Ola Brine\nSalt Wind,Nobody Known\n"

// importShell is a test shell whose imports look for songs in the tests'
// YouTube Music, and the file of a playlist, Sea Songs.csv.
func importShell(t *testing.T) (*shell, *ui.Tester, string) {
	t.Helper()
	s := newTestShell(t)
	s.importsOf().UseServices(app.ImportServices{Search: &importYT{songs: importSongs}, Song: (&importYT{}).Song})
	path := filepath.Join(t.TempDir(), "Sea Songs.csv")
	if err := os.WriteFile(path, []byte(importCSV), 0o644); err != nil {
		t.Fatal(err)
	}
	return s, testerOf(t, s, 1280, 1400), path
}

// chooseFiles has the dialog that opens files choose paths, until the
// test ends.
func chooseFiles(t *testing.T, paths ...string) {
	was := openFileDialog
	t.Cleanup(func() { openFileDialog = was })
	openFileDialog = func(mygo.OpenDialogOptions) ([]string, error) { return paths, nil }
}

// saveTo has the dialog that saves files save to path, until the test
// ends, and keeps the options it was given.
func saveTo(t *testing.T, path string) *mygo.SaveDialogOptions {
	was := saveFileDialog
	t.Cleanup(func() { saveFileDialog = was })
	asked := &mygo.SaveDialogOptions{}
	saveFileDialog = func(o mygo.SaveDialogOptions) (string, error) {
		*asked = o
		return path, nil
	}
	return asked
}

// TestPlaylistImportTile opens the page that imports playlists from its
// tile beside New mix, and from the command palette.
func TestPlaylistImportTile(t *testing.T) {
	s, tt, _ := importShell(t)
	show(s, tt, "/library/mixes")
	texts := tt.Texts()
	i, j := slices.Index(texts, "new mix"), slices.Index(texts, "import a playlist")
	if i < 0 || j < i {
		t.Fatalf("Library › Mixes shows %q", texts)
	}
	click(t, s, tt, "Import a playlist")
	if got := s.router.Path(); got != importPagePath {
		t.Fatalf("the tile opened %q", got)
	}
	if !tt.HasText("Choose files…") || !tt.HasText("How to download your data from Spotify or Google") {
		t.Errorf("the page shows %q", tt.Texts())
	}
	show(s, tt, "/")
	for _, c := range s.commands() {
		if c.id == "library.importPlaylist" {
			c.run()
		}
	}
	s.drain()
	tt.Frame()
	if got := s.router.Path(); got != importPagePath {
		t.Errorf("the command opened %q", got)
	}
}

// TestPlaylistImportReview imports a CSV chosen in the dialog: the songs
// found, to check and not found, the counts, Replace… of the one not
// found, and the import into a new mix, which opens.
func TestPlaylistImportReview(t *testing.T) {
	s, tt, path := importShell(t)
	chooseFiles(t, path)
	show(s, tt, importPagePath)
	click(t, s, tt, "Choose files…")
	until(t, s, tt, "the review", func() bool {
		return tt.HasText("Found 1 · To check 1 · In your library already 0 · Not found 1")
	})
	for _, want := range []string{"Ola Brine - Harbour Lights", "Harbour Lights · Ola Brine", "To check",
		"Gulls at Dawn (Live) · Ola Brine", "Nobody Known - Salt Wind", "Not found", "Import 1 song"} {
		if !tt.HasText(want) {
			t.Errorf("the review doesn't show %q: %q", want, tt.Texts())
		}
	}
	// Replace… of the song not found searches it by its title and artist,
	// and the user searches by its title alone.
	s.openImportReplace(0, 2, "Nobody Known - Salt Wind", s.imports.lists[0].Rows[2])
	until(t, s, tt, "the search", func() bool { return tt.HasText("Nothing found for Salt Wind Nobody Known") })
	if !tt.HasText("Replace Nobody Known - Salt Wind") {
		t.Errorf("the dialog shows %q", tt.Texts())
	}
	s.imports.replace.query = "Salt Wind"
	s.searchImportReplace()
	until(t, s, tt, "the search", func() bool { return tt.HasText("Salt Wind · Tam Reed") })
	click(t, s, tt, "Salt Wind · Tam Reed")
	until(t, s, tt, "the song replaced", func() bool { return tt.HasText("Import 2 songs") })
	click(t, s, tt, "Import 2 songs")
	until(t, s, tt, "the mix", func() bool { return tt.HasText("Added 2 songs to Sea Songs") })
	if !strings.HasPrefix(s.router.Path(), "/playlist/") {
		t.Errorf("the import went to %q", s.router.Path())
	}
	mixes := s.svc.Collection.Playlists()
	if len(mixes) != 1 || mixes[0].Title != "Sea Songs" || mixes[0].TrackCount != 2 {
		t.Errorf("the mixes are %+v", mixes)
	}
}

// TestPlaylistImportDrop imports the playlists dropped on the window, and
// asks before it starts again while an import is under way.
func TestPlaylistImportDrop(t *testing.T) {
	s, tt, path := importShell(t)
	s.dropFiles(&mygo.FileDropEvent{Paths: []string{path}})
	until(t, s, tt, "the review", func() bool { return tt.HasText("Import 1 song") })
	s.dropFiles(&mygo.FileDropEvent{Paths: []string{path}})
	until(t, s, tt, "the question", func() bool { return tt.HasText("Start a new import?") })
	click(t, s, tt, "Start the new one")
	until(t, s, tt, "the review again", func() bool { return tt.HasText("Import 1 song") })
}

// TestPlaylistImportColumns has the user name the columns of a table
// whose names MixMusic doesn't know, from its first rows.
func TestPlaylistImportColumns(t *testing.T) {
	s, tt, _ := importShell(t)
	path := filepath.Join(t.TempDir(), "Liste.csv")
	if err := os.WriteFile(path, []byte("Titel,Künstler\nHarbour Lights,Ola Brine\nSalt Wind,Tam Reed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	show(s, tt, importPagePath)
	s.importFiles([]string{path})
	until(t, s, tt, "the columns", func() bool {
		return tt.HasText("MixMusic doesn't know the columns of Liste. Choose what each one holds:")
	})
	for _, want := range []string{"Titel", "Künstler", "Harbour Lights", "Tam Reed", "The first row names the columns",
		"Choose the column of the titles, or one of links or IDs, to continue."} {
		if !tt.HasText(want) {
			t.Errorf("the page doesn't show %q: %q", want, tt.Texts())
		}
	}
	s.imports.fields = []string{"Title", "Artists"}
	tt.Frame()
	click(t, s, tt, "Continue")
	until(t, s, tt, "the review", func() bool { return tt.HasText("Found 1 · To check 0 · In your library already 0 · Not found 1") })
}

// TestPlaylistImportStop stops the looking for the songs, whose page
// tells how far it is, and reviews what was found.
func TestPlaylistImportStop(t *testing.T) {
	s, tt, path := importShell(t)
	gate := make(chan struct{})
	s.importsOf().UseServices(app.ImportServices{Search: gatedSearch{gate}})
	show(s, tt, importPagePath)
	s.importFiles([]string{path})
	until(t, s, tt, "the looking", func() bool { return tt.HasText("Matching 0 of 3…") })
	click(t, s, tt, "Stop and review")
	until(t, s, tt, "the review", func() bool { return tt.HasText("3 songs weren't looked for yet.") })
	if !tt.HasText("Not looked for") {
		t.Errorf("the review shows %q", tt.Texts())
	}
	close(gate)
}

// gatedSearch is a search that waits for its gate.
type gatedSearch struct{ gate chan struct{} }

func (g gatedSearch) Search(ctx context.Context, _ string, _ youtube.Filter) ([]media.Item, error) {
	select {
	case <-g.gate:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// TestPlaylistImportFailures tells the files an import couldn't read.
func TestPlaylistImportFailures(t *testing.T) {
	s, tt, _ := importShell(t)
	empty := filepath.Join(t.TempDir(), "empty.m3u8")
	if err := os.WriteFile(empty, []byte("#EXTM3U\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.importFiles([]string{empty})
	until(t, s, tt, "the failure", func() bool { return tt.HasText("Couldn't import empty.m3u8: it holds no songs") })
	show(s, tt, importPagePath)
	s.imports.pasted = "# nothing but a comment"
	tt.Frame()
	click(t, s, tt, "Import the text")
	until(t, s, tt, "the failure", func() bool { return tt.HasText("Couldn't import the text: it holds no songs") })
}

// TestPlaylistExport exports a mix from its More menu, Liked songs from
// their page and the library from the command palette, as the file the
// dialog chose says: CSV, M3U8 or JSON.
func TestPlaylistExport(t *testing.T) {
	s, tt, _ := importShell(t)
	tracks := importSongs["Harbour Lights Ola Brine"]
	mix, err := s.svc.Collection.CreatePlaylist("Sea Songs", "", tracks)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.svc.Collection.SetLiked(context.Background(), importSongs["Salt Wind"][0], true); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	csv := filepath.Join(dir, "Sea Songs.csv")
	asked := saveTo(t, csv)
	show(s, tt, playlistPath(mix.ID))
	until(t, s, tt, "the mix", func() bool { return tt.HasText("Harbour Lights") })
	if err := tt.Click("More"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Export the list…"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	until(t, s, tt, "the export", func() bool { return tt.HasText("Exported Sea Songs to " + csv) })
	if asked.DefaultPath != "Sea Songs.csv" || len(asked.Filters) != 3 {
		t.Errorf("the dialog was asked %+v", asked)
	}
	data, err := os.ReadFile(csv)
	if err != nil || !strings.Contains(string(data), "Sea Songs,Harbour Lights,Ola Brine,Coastline") {
		t.Errorf("the file holds %q, %v", data, err)
	}

	m3u := filepath.Join(dir, "liked.m3u8")
	saveTo(t, m3u)
	show(s, tt, "/library/liked")
	click(t, s, tt, "Export the list…")
	until(t, s, tt, "the export", func() bool { return tt.HasText("Exported Liked songs to " + m3u) })
	if data, _ := os.ReadFile(m3u); !strings.Contains(string(data), "#PLAYLIST:Liked songs\n#EXTINF:") {
		t.Errorf("the M3U8 holds %q", data)
	}

	lib := filepath.Join(dir, "library")
	saveTo(t, lib)
	s.exportLibrary()
	until(t, s, tt, "the library's export", func() bool { return tt.HasText("Exported MixMusic library to " + lib + ".csv") })
	data, _ = os.ReadFile(lib + ".csv")
	if !strings.Contains(string(data), "Liked songs,Salt Wind") || !strings.Contains(string(data), "Sea Songs,Harbour Lights") {
		t.Errorf("the library holds %q", data)
	}

	saveFileDialog = func(mygo.SaveDialogOptions) (string, error) { return "", errors.New("the dialog couldn't open") }
	s.exportLibrary()
	until(t, s, tt, "the dialog's failure", func() bool { return tt.HasText("the dialog couldn't open") })
}
