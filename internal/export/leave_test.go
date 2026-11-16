package export

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/tagwrite"
	"github.com/fzn0x/mixmusic/ogg"
)

// lengthOf is how many samples an Ogg Opus file plays, as its last page's
// granule position and its pre-skip tell.
func lengthOf(t *testing.T, path string) int64 {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := ogg.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	head, err := r.ReadPacket()
	if err != nil {
		t.Fatal(err)
	}
	preSkip := int64(binary.LittleEndian.Uint16(head.Data[10:]))
	end := int64(-1)
	for {
		p, err := r.ReadPacket()
		if errors.Is(err, io.EOF) {
			return end - preSkip
		}
		if err != nil {
			t.Fatal(err)
		}
		if p.Last {
			end = p.Granule
		}
	}
}

// TestWriteLeaves leaves out of the copy of a download of Opus in WebM
// what Leave tells, of the length of its audio, and nothing out of an M4A
// copy, whose sample table isn't written anew.
func TestWriteLeaves(t *testing.T) {
	dir := t.TempDir()
	src := testdata(t, "webm/testdata/stereo-cues-front.webm")
	whole, err := Write(dir, Track{Path: src, Title: "Whole"})
	if err != nil {
		t.Fatal(err)
	}
	var told time.Duration
	leave := func(length time.Duration) []tagwrite.Cut {
		told = length
		return []tagwrite.Cut{{From: 0, To: time.Second}}
	}
	cut, err := Write(dir, Track{Path: src, Title: "Cut", Leave: leave})
	if err != nil {
		t.Fatal(err)
	}
	if told != 3008*time.Millisecond {
		t.Errorf("Leave was told a length of %v", told)
	}
	if a, b := lengthOf(t, whole), lengthOf(t, cut); a-b != 48000 {
		t.Errorf("the copy plays %d samples, of %d whole: want a second less", b, a)
	}

	m4a := filepath.Join(t.TempDir(), "x.m4a")
	data, err := os.ReadFile(testdata(t, "mp4/testdata/dash.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m4a, data, 0o644); err != nil {
		t.Fatal(err)
	}
	told = 0
	p, err := Write(dir, Track{Path: m4a, Title: "Kept", Tags: tagwrite.Tags{Title: "Kept"}, Leave: leave})
	if err != nil {
		t.Fatal(err)
	}
	if told != 0 {
		t.Error("an M4A copy asked what to leave out")
	}
	plain, err := Write(dir, Track{Path: m4a, Title: "Plain", Tags: tagwrite.Tags{Title: "Kept"}})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(p)
	b, _ := os.ReadFile(plain)
	if !bytes.Equal(a, b) {
		t.Error("the M4A copy changed with Leave")
	}
}
