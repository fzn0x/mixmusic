package app

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/colorfile"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestColorFileReadsInOrder has a read of the colour file that began
// before the file changed end after one that began since, as the watch's
// and Reload's may: the newer colours stay.
func TestColorFileReadsInOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "colors.json")
	if err := os.WriteFile(path, []byte(pywalFile("#0d1117")), 0o644); err != nil {
		t.Fatal(err)
	}
	c := newColorTest(t, func(s *settings.Settings) {
		s.Appearance.ColorSource, s.Appearance.ColorFile = settings.ColorsFile, path
	})
	read := readColors
	t.Cleanup(func() { readColors = read })
	began, release := make(chan struct{}), make(chan struct{})
	var reads atomic.Int32
	readColors = func(p string) (colorfile.Colors, []byte, error) {
		colors, raw, err := read(p)
		if reads.Add(1) == 1 {
			close(began)
			<-release
		}
		return colors, raw, err
	}
	c.a.colorFile.mu.Lock()
	gen := c.a.colorFile.gen
	c.a.colorFile.mu.Unlock()
	older, newer := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(older)
		c.a.readColorFile(context.Background(), gen, path, false)
	}()
	<-began
	if err := os.WriteFile(path, []byte(pywalFile("#606870")), 0o644); err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(newer)
		c.a.readColorFile(context.Background(), gen, path, false)
	}()
	// The newer read waits for the older one; were it not to, this gives
	// it the time to land first.
	select {
	case <-newer:
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	<-older
	<-newer
	if st := c.status(); groundIn(st) != "#606870" {
		t.Errorf("the older read landed last: %+v", st)
	}
}
