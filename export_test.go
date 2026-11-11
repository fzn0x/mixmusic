package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/tags"
)

// exportTracks are the made-up tracks the downloads of these tests hold.
var exportTracks = []media.Track{
	testTrack("yt:expaaaaaaaa", "Lighthouse Keeper", "Ola Brine", "Harbour Lights", media.SourceYouTube),
	testTrack("yt:expbbbbbbbb", "Gulls at Dawn", "Ola Brine", "Harbour Lights", media.SourceYouTube),
}

// exportShell is a shell whose downloads hold exportTracks, which exports
// to a folder of the test's, with the album's page made up, and shows the
// files in a folder as shown records them.
func exportShell(t *testing.T) (s *shell, tt *ui.Tester, dir string, shown *[]string) {
	t.Helper()
	s, _ = newKeptShell(t, exportTracks...)
	dir = t.TempDir()
	shown = new([]string)
	saved, savedAlbum, savedShow := chooseExportFolder, exportAlbum, showInFolder
	t.Cleanup(func() { chooseExportFolder, exportAlbum, showInFolder = saved, savedAlbum, savedShow })
	chooseExportFolder = func(*shell) (string, error) { return dir, nil }
	exportAlbum = func(_ *shell, _ context.Context, id string) (media.Album, error) {
		return media.Album{ID: id, Year: "2019", Artists: []media.Ref{{Name: "Ola Brine"}}, Tracks: exportTracks}, nil
	}
	showInFolder = func(path string) { *shown = append(*shown, path) }
	tt = testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/downloads")
	until(t, s, tt, "the downloads", func() bool { return tt.HasText(exportTracks[1].Title) })
	return s, tt, dir, shown
}

// exportedFiles are the names of the files in dir.
func exportedFiles(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

// TestExportFromMenu exports a download from its menu: a copy named
// "Artist - Title", of its tags and what its album's page tells, and a
// toast whose show shows it.
func TestExportFromMenu(t *testing.T) {
	s, tt, dir, shown := exportShell(t)
	if err := tt.RightClick(exportTracks[1].Title); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Export…"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	msg := "Exported " + exportTracks[1].Title + " to " + dir
	until(t, s, tt, "the export", func() bool { return tt.HasText(msg) })
	want := filepath.Join(dir, "Ola Brine - Gulls at Dawn.opus")
	if got := exportedFiles(dir); !slices.Equal(got, []string{filepath.Base(want)}) {
		t.Fatalf("the folder holds %q", got)
	}
	f, err := os.Open(want)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tg, err := tags.Read(f)
	if err != nil {
		t.Fatal(err)
	}
	if tg.Title != "Gulls at Dawn" || !slices.Equal(tg.Artists, []string{"Ola Brine"}) || tg.Album != "Harbour Lights" ||
		tg.Year != 2019 || tg.Track != 2 {
		t.Errorf("the copy reads back as %+v", tg)
	}
	click(t, s, tt, "show")
	if !slices.Equal(*shown, []string{want}) {
		t.Errorf("show showed %q", *shown)
	}
}

// TestExportFromBar exports the tracks selected from the bar's Export.
func TestExportFromBar(t *testing.T) {
	s, tt, dir, _ := exportShell(t)
	for _, tr := range exportTracks {
		if err := tt.ClickWith(ui.Cmd, tr.Title); err != nil {
			t.Fatal(err)
		}
		s.drain()
		tt.Frame()
	}
	click(t, s, tt, "Export the selected tracks")
	until(t, s, tt, "the export", func() bool { return tt.HasText("Exported 2 tracks to " + dir) })
	got := exportedFiles(dir)
	slices.Sort(got)
	if want := []string{"Ola Brine - Gulls at Dawn.opus", "Ola Brine - Lighthouse Keeper.opus"}; !slices.Equal(got, want) {
		t.Errorf("the folder holds %q, want %q", got, want)
	}
}

// TestExportOnlyKept offers Export… only of a track downloaded or on this
// computer, and an export the user didn't choose a folder for, or whose
// folder can't be chosen, writes nothing.
func TestExportOnlyKept(t *testing.T) {
	s, tt, dir, _ := exportShell(t)
	other := testTrack("yt:expcccccccc", "Tin Roof Rain", "Tam Reed", "Rooftops", media.SourceYouTube)
	s.loads["album:al-roof"] = &loaded[media.Album]{done: true, value: media.Album{ID: "al-roof", Title: "Rooftops",
		Source: media.SourceYouTube, Tracks: []media.Track{other}}}
	show(s, tt, "/album/al-roof")
	until(t, s, tt, "the album", func() bool { return tt.HasText(other.Title) })
	if err := tt.RightClick(other.Title); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(tt.Menu(), "Export…") {
		t.Errorf("a track not downloaded has Export…: %q", tt.Menu())
	}
	tt.CloseMenu()
	s.exportTracks([]media.Track{other})
	tt.Frame()
	if !tt.HasText("Only tracks downloaded or on this computer export") {
		t.Errorf("the window shows %q", tt.Texts())
	}

	chooseExportFolder = func(*shell) (string, error) { return "", nil }
	s.exportTracks(exportTracks)
	chooseExportFolder = func(*shell) (string, error) { return "", errors.New("the dialog couldn't open") }
	s.exportTracks(exportTracks)
	until(t, s, tt, "the dialog's failure", func() bool { return tt.HasText("the dialog couldn't open") })
	if got := exportedFiles(dir); len(got) != 0 {
		t.Errorf("the folder holds %q", got)
	}
}
