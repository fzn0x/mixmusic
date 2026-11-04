package settings

import (
	"path/filepath"
	"testing"
)

// TestDownloadsFolder keeps an absolute folder of the downloads, cleaned,
// and drops a relative one, which would follow the working directory.
func TestDownloadsFolder(t *testing.T) {
	dir := t.TempDir()
	s := Default()
	if s.DownloadsFolder != "" || s.DownloadOnMetered {
		t.Errorf("the defaults keep the downloads in %q, on metered connections %v", s.DownloadsFolder, s.DownloadOnMetered)
	}
	s.DownloadsFolder = "  " + filepath.Join(dir, "a", "..", "music") + " "
	if got := s.Clean().DownloadsFolder; got != filepath.Join(dir, "music") {
		t.Errorf("an absolute folder came out as %q", got)
	}
	s.DownloadsFolder = filepath.Join("music", "downloads")
	if got := s.Clean().DownloadsFolder; got != "" {
		t.Errorf("a relative folder came out as %q", got)
	}
}

// TestDownloadAdded downloads the tracks added to collections downloaded
// by default, settings of before it too, and keeps it off once turned off.
func TestDownloadAdded(t *testing.T) {
	if !Default().DownloadAdded {
		t.Error("a new install doesn't download the tracks added")
	}
	if s, err := Decode([]byte(`{"downloadOnMetered": true}`)); err != nil || !s.DownloadAdded {
		t.Errorf("settings of before download the tracks added %v: %v", s.DownloadAdded, err)
	}
	if s, err := Decode([]byte(`{"downloadAdded": false}`)); err != nil || s.DownloadAdded {
		t.Errorf("turned off, the tracks added download %v: %v", s.DownloadAdded, err)
	}
}
