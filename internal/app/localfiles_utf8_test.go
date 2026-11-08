package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestLocalNamesNotUTF8 plays a song whose file's name and folder's name
// aren't UTF-8, as on a Linux drive they may be, after a restart, before
// the scan: the settings name its folder, the index its file, and the
// liked songs its track. Only Linux names files with such bytes:
// Windows' names are UTF-16, which always reads as UTF-8, and macOS's
// disks refuse them.
func TestLocalNamesNotUTF8(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only Linux names files with bytes that aren't UTF-8")
	}
	dir := t.TempDir()
	music := filepath.Join(dir, "Mus\xefk")
	if err := os.MkdirAll(music, 0o755); err != nil {
		t.Fatal(err)
	}
	flacData, err := os.ReadFile(filepath.Join("..", "..", "flac", "testdata", "stereo-16.flac"))
	if err != nil {
		t.Fatal(err)
	}
	song := filepath.Join(music, "caf\xe9.flac")
	if err := os.WriteFile(song, flacData, 0o644); err != nil {
		t.Fatal(err)
	}
	opts := Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"), Builtin: builtin.FS, Version: "test"}
	a, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	st := a.settings.Get()
	st.LocalFiles = settings.LocalFiles{MusicFolders: []string{music}}
	if err := a.settings.Set(st); err != nil {
		t.Fatal(err)
	}
	scanned(t, a)
	tracks := (&LocalFiles{a}).Tracks()
	if len(tracks) != 1 {
		t.Fatalf("tracks %+v", tracks)
	}
	if err := a.library.SetLiked(tracks[0], true); err != nil {
		t.Fatal(err)
	}
	a.Close()

	b, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Close)
	if got := b.settings.Get().MusicFolders; !slices.Equal(got, []string{music}) {
		t.Errorf("the folders after a restart: %q", got)
	}
	tracks = (&LocalFiles{b}).Tracks()
	if len(tracks) != 1 {
		t.Fatalf("tracks after a restart %+v", tracks)
	}
	if path, ok := localfiles.PathOf(tracks[0].ID); !ok || path != song {
		t.Errorf("the track after a restart names %q", path)
	}
	if _, err := (&Player{b}).Prepare(context.Background(), tracks[0]); err != nil {
		t.Errorf("the track after a restart, before the scan: %v", err)
	}
	liked := b.library.Liked()
	if len(liked) != 1 {
		t.Fatalf("liked %+v", liked)
	}
	if path, ok := localfiles.PathOf(liked[0].ID); !ok || path != song || b.localFiles.Missing(liked[0].ID) {
		t.Errorf("the liked track after a restart names %q", path)
	}
	scanned(t, b)
	if st := (&LocalFiles{b}).Status(); len(st.Folders) != 1 || st.Folders[0].Problem != "" || st.Tracks != 1 {
		t.Errorf("the folder after a restart: %+v", st)
	}
}
