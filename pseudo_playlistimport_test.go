package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// refusingSearch is YouTube Music refusing for being asked too often.
type refusingSearch struct{}

func (refusingSearch) Search(context.Context, string, youtube.Filter) ([]media.Item, error) {
	return nil, &youtube.APIError{Status: http.StatusTooManyRequests}
}

// TestPseudoLocalePlaylistImport shows each step of an import in en-XA:
// the tile, the page before a file, a table's columns, the lists to
// pick, the looking stopped by YouTube Music, the review of two lists,
// the dialog that replaces a song, and the failures: only the names of
// songs, artists, lists and files are English.
func TestPseudoLocalePlaylistImport(t *testing.T) {
	usePseudoLocale(t)
	s, tt, path := importShell(t)
	tt.SetLocale("en-XA")
	names := []string{"Ola Brine", "Harbour Lights", "Gulls at Dawn", "Gulls at Dawn (Live)", "Nobody Known", "Salt Wind",
		"Tam Reed", "Sea Songs", "Liste", "Titel", "Künstler", "Night Drive", "empty.m3u8", "Pier", "Coastline", "Rooftops"}

	show(s, tt, "/library/mixes")
	checkPseudo(t, "Library › Mixes", tt.Texts(), names...)
	show(s, tt, importPagePath)
	checkPseudo(t, "The page before a file", tt.Texts(), names...)

	dir := t.TempDir()
	table := filepath.Join(dir, "Liste.csv")
	if err := os.WriteFile(table, []byte("Titel,Künstler\nHarbour Lights,Ola Brine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.importFiles([]string{table})
	until(t, s, tt, "the columns", func() bool { return s.importsOf().Status().Step == app.ImportColumns })
	tt.Frame()
	checkPseudo(t, "The columns of a table", tt.Texts(), names...)
	s.importsOf().Cancel()

	second := filepath.Join(dir, "Night Drive.txt")
	if err := os.WriteFile(second, []byte("Ola Brine - Harbour Lights\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.importsOf().UseServices(app.ImportServices{Search: refusingSearch{}})
	s.importFiles([]string{path, second})
	until(t, s, tt, "the lists", func() bool { return s.importsOf().Status().Step == app.ImportPick })
	tt.Frame()
	checkPseudo(t, "The lists to pick", tt.Texts(), names...)
	s.importsOf().Pick([]bool{true, true})
	until(t, s, tt, "the halt", func() bool { return s.importsOf().Status().Step == app.ImportHalted })
	tt.Frame()
	checkPseudo(t, "The looking stopped", tt.Texts(), names...)

	s.importsOf().UseServices(app.ImportServices{Search: &importYT{songs: importSongs}})
	s.importsOf().TryAgain()
	until(t, s, tt, "the review", func() bool { return s.importsOf().Status().Step == app.ImportReview })
	tt.Frame()
	checkPseudo(t, "The review", tt.Texts(), names...)
	s.openImportReplace(0, 2, "Nobody Known - Salt Wind", s.imports.lists[0].Rows[2])
	until(t, s, tt, "the search", func() bool { return !s.imports.replace.searching })
	tt.Frame()
	checkPseudo(t, "The dialog that replaces a song", tt.Texts(), names...)
	s.imports.replace.open = false
	s.importsOf().Cancel()

	empty := filepath.Join(dir, "empty.m3u8")
	if err := os.WriteFile(empty, []byte("#EXTM3U\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.importFiles([]string{empty})
	until(t, s, tt, "the failure", func() bool {
		return slices.ContainsFunc(tt.Texts(), func(txt string) bool { return strings.Contains(txt, "empty.m3u8") })
	})
	tt.Frame()
	checkPseudo(t, "A file that holds no songs", tt.Texts(), names...)
}
