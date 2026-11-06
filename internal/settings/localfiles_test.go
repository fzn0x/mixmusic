package settings

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// TestLocalFilesClean keeps each folder of music once, cleaned, and drops
// those that aren't absolute.
func TestLocalFilesClean(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	got := LocalFiles{MusicFolders: []string{a, " " + b + string(filepath.Separator) + " ", `"` + a + `"`, "Music", ""}}.clean()
	if want := []string{filepath.Clean(a), filepath.Clean(b)}; !reflect.DeepEqual(got.MusicFolders, want) {
		t.Errorf("folders %q, want %q", got.MusicFolders, want)
	}
	if got := (LocalFiles{}).clean(); got.MusicFolders == nil {
		t.Error("no folders are nil, which settings.json writes as null")
	}
	if SamePath(a, b) || !SamePath(a, a+string(filepath.Separator)+".") {
		t.Error("SamePath")
	}
	if upper := filepath.Join(filepath.Dir(a), "MUSIC"); SamePath(upper, filepath.Join(filepath.Dir(a), "music")) != (runtime.GOOS != "linux") {
		t.Error("SamePath of names of other cases")
	}
}
