package settings

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/store"
)

// pathSettings are settings of each path setting, of a folder of dir:
// one name of each is UTF-8, decomposed as macOS's names are, and one
// isn't, as a name on a Linux drive may be.
func pathSettings(dir string) Settings {
	s := Default()
	s.ColorFile = filepath.Join(dir, "colo\xefrs.json")
	s.DownloadsFolder = filepath.Join(dir, "Café")
	s.MusicFolders = []string{filepath.Join(dir, "Café"), filepath.Join(dir, "caf\xe9")}
	s.YTDLPPath = filepath.Join(dir, "\xffyt-dlp")
	return s
}

// TestPathsNotUTF8 keeps the paths of the settings in settings.json so
// that they name the same files when it's read again, as after a restart:
// in base64 those that aren't UTF-8, as they are those that are.
func TestPathsNotUTF8(t *testing.T) {
	s := pathSettings(t.TempDir())
	data := stored(t, s)
	for _, want := range []string{"Café", "Café", `"base64:`} {
		if !strings.Contains(string(data), want) || strings.Contains(string(data), `�`) {
			t.Errorf("settings.json holds %s", data)
		}
	}
	got, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, s.Clean()) {
		t.Errorf("read back: %q %q %q %q", got.ColorFile, got.DownloadsFolder, got.MusicFolders, got.YTDLPPath)
	}
	if r := Inspect(data, got); !r.Empty() {
		t.Errorf("the file read back is reported: %+v", r)
	}
	// Writing them leaves the settings written as they were.
	if want := pathSettings(filepath.Dir(s.DownloadsFolder)); !reflect.DeepEqual(s, want) {
		t.Errorf("writing changed the settings: %q", s.MusicFolders)
	}
}

// TestPathsKept keeps the paths of the settings through the store, which
// copies them through JSON as they change, and in the file it saves, as
// MixMusic keeps settings.json.
func TestPathsKept(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	f, err := store.OpenWith(path, Default(), Decode)
	if err != nil {
		t.Fatal(err)
	}
	want := pathSettings(dir).Clean()
	if err := f.Set(want); err != nil {
		t.Fatal(err)
	}
	if err := f.Update(func(s *Settings) error { s.Volume = 0.5; return nil }); err != nil {
		t.Fatal(err)
	}
	want.Volume = 0.5
	if got := f.Get(); !reflect.DeepEqual(got, want) {
		t.Errorf("updated: %q, want %q", got.MusicFolders, want.MusicFolders)
	}
	again, err := store.OpenWith(path, Default(), Decode)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Get(); !reflect.DeepEqual(got, want) {
		t.Errorf("read back: %q, want %q", got.MusicFolders, want.MusicFolders)
	}
	// An edit of the file merges in.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), `"volume": 0.5`, `"volume": 0.25`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := again.Reload(); err != nil {
		t.Fatal(err)
	}
	if got := again.Get(); got.Volume != 0.25 || !reflect.DeepEqual(got.MusicFolders, want.MusicFolders) {
		t.Errorf("after an edit: %v, %q", got.Volume, got.MusicFolders)
	}
}

// TestPathsOfBefore reads the paths of the versions before, which kept
// them all as text, one that wasn't UTF-8 with replacement characters
// in place of its bytes, and still tells which key holds a mistake.
func TestPathsOfBefore(t *testing.T) {
	dir := t.TempDir()
	text, odd := filepath.Join(dir, "Café"), filepath.Join(dir, "caf�")
	before := strings.ReplaceAll(`{"musicFolders": ["`+text+`", "`+odd+`"], "downloadsFolder": "`+text+`"}`, `\`, `\\`)
	got, err := Decode([]byte(before))
	if err != nil || !reflect.DeepEqual(got.MusicFolders, []string{text, odd}) || got.DownloadsFolder != text {
		t.Errorf("a file of before: %q, %q, %v", got.MusicFolders, got.DownloadsFolder, err)
	}
	var fe *FileError
	if _, err := Decode([]byte(`{"musicFolders": "/home/ada/Music"}`)); !errors.As(err, &fe) || fe.Key != "musicFolders" {
		t.Errorf("text in place of a list: %v", err)
	}
}
