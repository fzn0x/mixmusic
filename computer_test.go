package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// The songs of these tests are made up, as their tones are.

// localWAV is a WAV file of a second of a tone, of made-up tags in its
// INFO list.
func localWAV(title, artist, album, year string, hz float64) []byte {
	le := func(b *bytes.Buffer, v any) { binary.Write(b, binary.LittleEndian, v) }
	sub := func(id, v string) []byte {
		var b bytes.Buffer
		b.WriteString(id)
		le(&b, uint32(len(v)+1))
		b.WriteString(v)
		b.WriteByte(0)
		if b.Len()%2 != 0 {
			b.WriteByte(0)
		}
		return b.Bytes()
	}
	var info bytes.Buffer
	info.WriteString("INFO")
	for _, kv := range [][2]string{{"INAM", title}, {"IART", artist}, {"IPRD", album}, {"ICRD", year}} {
		if kv[1] != "" {
			info.Write(sub(kv[0], kv[1]))
		}
	}
	const rate = 8000
	var data bytes.Buffer
	for i := range rate {
		le(&data, int16(8000*math.Sin(2*math.Pi*hz*float64(i)/rate)))
	}
	var b bytes.Buffer
	body := func() {
		b.WriteString("WAVEfmt ")
		le(&b, uint32(16))
		le(&b, uint16(1))
		le(&b, uint16(1))
		le(&b, uint32(rate))
		le(&b, uint32(2*rate))
		le(&b, uint16(2))
		le(&b, uint16(16))
		b.WriteString("data")
		le(&b, uint32(data.Len()))
		b.Write(data.Bytes())
		b.WriteString("LIST")
		le(&b, uint32(info.Len()))
		b.Write(info.Bytes())
	}
	body()
	all := append([]byte("RIFF"), binary.LittleEndian.AppendUint32(nil, uint32(b.Len()))...)
	return append(all, b.Bytes()...)
}

// computerShell is a test shell with a folder of music on this computer,
// scanned: two albums, one with a cover beside its files, and a file
// without tags.
func computerShell(t *testing.T, layout settings.Layout, theme string) (*shell, string) {
	t.Helper()
	s := newTestShell(t)
	music := filepath.Join(t.TempDir(), "Music")
	files := []struct{ path, title, artist, album, year string }{
		{"Harbour Lights/Shore/01.wav", "Slow Tide", "The Harbour Lights", "Shore", "2019"},
		{"Harbour Lights/Shore/02.wav", "Low Water", "The Harbour Lights", "Shore", "2019"},
		{"Harbour Lights/Shore/03.wav", "Gulls at Noon", "The Harbour Lights", "Shore", "2019"},
		{"Ada Field/Kites/01.wav", "Paper Kites", "Ada Field", "Kites", "2023"},
		{"Ada Field/Kites/02.wav", "String Theory", "Ada Field", "Kites", "2023"},
		{"Voice memo 14.wav", "", "", "", ""},
	}
	for i, f := range files {
		path := filepath.Join(music, filepath.FromSlash(f.path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, localWAV(f.title, f.artist, f.album, f.year, 220+float64(40*i)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for dir, n := range map[string]int{"Harbour Lights/Shore": 3, "Ada Field/Kites": 7} {
		if err := os.WriteFile(filepath.Join(music, filepath.FromSlash(dir), "cover.png"), coverOf(n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st := s.svc.Preferences.Get()
	st.FollowSystem, st.Theme, st.Layout, st.DynamicAccent = false, theme, layout, false
	st.LocalFiles = settings.LocalFiles{MusicFolders: []string{music}}
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	v := s.svc.LocalFiles.Version()
	s.svc.LocalFiles.Rescan()
	for deadline := time.Now().Add(10 * time.Second); s.svc.LocalFiles.Status().Scanning || s.svc.LocalFiles.Version() == v; {
		if time.Now().After(deadline) {
			t.Fatal("the scan didn't end")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return s, music
}

// TestComputerLibrary lists the music on this computer in its pivot of
// the library: its songs, a file without tags by its name and an unknown
// artist, its albums and its artists.
func TestComputerLibrary(t *testing.T) {
	s, _ := computerShell(t, settings.LayoutTiles, "mix")
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/computer")
	for _, want := range []string{"Slow Tide", "Paper Kites", "Voice memo 14", localfiles.UnknownArtist, "The Harbour Lights", "Shore"} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	// The library's own pivots are named as the sections are.
	s.computer.view = 1
	tt.Frame()
	if !tt.HasText("Kites") || !tt.HasText("Shore") || tt.HasText("Slow Tide") {
		t.Errorf("the albums: %q", tt.Texts())
	}
	s.computer.view = 2
	tt.Frame()
	if !tt.HasText("Ada Field") || !tt.HasText("The Harbour Lights") || tt.HasText("Shore") {
		t.Errorf("the artists: %q", tt.Texts())
	}
}

// TestComputerMissingFile greys a track whose file is gone, as File not
// found, in its list.
func TestComputerMissingFile(t *testing.T) {
	s, music := computerShell(t, settings.LayoutTiles, "mix")
	gone := filepath.Join(music, "Ada Field", "Kites", "02.wav")
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	// The index holds it until a scan: playing it finds it gone.
	tr := localfiles.TrackOf(localfiles.Entry{Path: gone, Title: "String Theory"})
	if _, err := s.svc.Player.Prepare(t.Context(), tr); err == nil {
		t.Fatal("a file gone prepared to play")
	}
	if !s.missingFile(tr) {
		t.Fatal("the file gone isn't missing")
	}
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/computer")
	if !tt.HasText("File not found") {
		t.Errorf("no File not found in %q", tt.Texts())
	}
}

// TestComputerSettings lists the folders of music, with how many tracks
// each holds, in Settings › Playback, and the palette opens files.
func TestComputerSettings(t *testing.T) {
	s, music := computerShell(t, settings.LayoutTiles, "mix")
	tt := testerOf(t, s, 1280, 4000)
	show(s, tt, "/settings/playback")
	for _, want := range []string{"music on this computer", music, "6 tracks"} {
		if !tt.HasText(want) {
			t.Errorf("no %q in Settings", want)
		}
	}
	found := false
	for _, cmd := range s.commands() {
		if cmd.id == "player.openFiles" && strings.HasPrefix(cmd.title, "Open music files") {
			found = true
		}
	}
	if !found {
		t.Error("no command opens music files")
	}
}

// TestComputerSearch finds the music on this computer in its own section
// of a search, before the services answer.
func TestComputerSearch(t *testing.T) {
	s, _ := computerShell(t, settings.LayoutTiles, "mix")
	tt := testerOf(t, s, 1280, 1400)
	s.loads["search:kites"] = &loaded[app.SearchResponse]{done: true}
	show(s, tt, searchPath("kites"))
	if !tt.HasText("on this computer") || !tt.HasText("Paper Kites") {
		t.Errorf("the search: %q", tt.Texts())
	}
}

// TestComputerPages opens the page of an album and of an artist of the
// music on this computer, which follows nothing.
func TestComputerPages(t *testing.T) {
	s, _ := computerShell(t, settings.LayoutTiles, "mix")
	var kites media.Album
	for _, a := range s.svc.LocalFiles.Albums() {
		if a.Title == "Kites" {
			kites = a
		}
	}
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, albumPath(kites.ID))
	s.settle(t, nil)
	tt.Frame()
	if !tt.HasText("Paper Kites") || !tt.HasText("String Theory") {
		t.Errorf("the album: %q", tt.Texts())
	}
	show(s, tt, artistPath(kites.Artists[0].ID))
	s.settle(t, nil)
	tt.Frame()
	if !tt.HasText("Paper Kites") || tt.HasText("follow") {
		t.Errorf("the artist: %q", tt.Texts())
	}
}

// TestRenderComputer draws Library › On this computer, its songs, albums
// and artists, a search that finds it, an album of it, and its group of
// Settings, in each layout, in Mix and Daylight, to MIXMUSIC_RENDER, for a
// look.
func TestRenderComputer(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	for _, layout := range settings.Layouts {
		for _, theme := range []string{"mix", "daylight"} {
			s, music := computerShell(t, layout, theme)
			os.Remove(filepath.Join(music, "Ada Field", "Kites", "02.wav"))
			s.svc.Player.Prepare(t.Context(), localfiles.TrackOf(localfiles.Entry{Path: filepath.Join(music, "Ada Field", "Kites", "02.wav")}))
			name := string(layout) + "-" + theme + "-computer-"
			// Each twice: the covers load beside the first frame.
			twice := func(tt *ui.Tester, shot string) {
				renderTo(t, s, tt, dir, name+shot)
				time.Sleep(100 * time.Millisecond)
				renderTo(t, s, tt, dir, name+shot)
			}
			tt := ui.NewTester(s.view, 1280, 900)
			show(s, tt, "/library/computer")
			twice(tt, "songs")
			s.computer.view = 1
			twice(tt, "albums")
			s.computer.view = 2
			twice(tt, "artists")
			s.computer.view = 0
			// The services found nothing; the computer does.
			s.loads["search:kites"] = &loaded[app.SearchResponse]{done: true}
			show(s, tt, searchPath("kites"))
			twice(tt, "search")
			show(s, tt, "/settings/playback?q=music+on+this+computer")
			twice(tt, "settings")
		}
	}
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 900)
	show(s, tt, "/library/computer")
	renderTo(t, s, tt, dir, "computer-empty")
}
