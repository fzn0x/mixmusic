package localfiles

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/tags"
)

// mp3Of is a made-up MP3 file whose ID3v2.4 tag holds a title, and the
// content advisory a TXXX frame gives when advisory isn't "".
func mp3Of(title, advisory string) []byte {
	frame := func(id string, data []byte) []byte {
		n := len(data)
		h := append([]byte(id), byte(n>>21&127), byte(n>>14&127), byte(n>>7&127), byte(n&127), 0, 0)
		return append(h, data...)
	}
	body := frame("TIT2", append([]byte{3}, title...))
	if advisory != "" {
		body = append(body, frame("TXXX", append([]byte{3}, "ITUNESADVISORY\x00"+advisory...))...)
	}
	n := len(body)
	tag := append([]byte{'I', 'D', '3', 4, 0, 0, byte(n >> 21 & 127), byte(n >> 14 & 127), byte(n >> 7 & 127), byte(n & 127)}, body...)
	frameHead := binary.BigEndian.AppendUint32(nil, 0xFFFB9064)
	return append(append(tag, frameHead...), make([]byte, 200)...)
}

// explicitTitles are the titles of the tracks marked explicit.
func explicitTitles(tracks []media.Track) []string {
	var out []string
	for _, t := range tracks {
		if t.Explicit {
			out = append(out, t.Title)
		}
	}
	return out
}

// TestExplicitFiles marks explicit the songs of this computer whose tags'
// content advisory says so, and not those marked clean or untagged; and an
// index kept before it read the advisory reads its files again.
func TestExplicitFiles(t *testing.T) {
	ti := newIndex(t)
	ti.readTags = tags.Read
	salt := ti.write(t, "Gulls/01.mp3", string(mp3Of("Salt on the Wind", "1")), time.Hour)
	ti.write(t, "Gulls/02.mp3", string(mp3Of("Low Water", "2")), time.Hour)
	ti.write(t, "Gulls/03.mp3", string(mp3Of("Paper Boats", "")), time.Hour)
	if n := ti.scan(t); n != 3 {
		t.Fatalf("read %d files", n)
	}
	if got := explicitTitles(ti.Tracks()); !slices.Equal(got, []string{"Salt on the Wind"}) {
		t.Errorf("explicit: %q of %q", got, titles(ti.Tracks()))
	}

	// The index as the version before kept it, which didn't keep the
	// advisory, is read again from the files.
	info, err := os.Stat(salt)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "local-files.json")
	old, _ := json.Marshal(map[string]any{"version": 2, "files": []map[string]any{
		{"path": salt, "size": info.Size(), "modified": info.ModTime().UnixNano(), "title": "Salt on the Wind"}}})
	if err := os.WriteFile(path, old, 0o644); err != nil {
		t.Fatal(err)
	}
	ti.Index = ti.open(t, path, filepath.Join(t.TempDir(), "art"))
	ti.readTags = tags.Read
	if n := ti.scan(t); n != 3 {
		t.Errorf("read %d files of an index of version 2, not all 3", n)
	}
	if got := explicitTitles(ti.Tracks()); !slices.Equal(got, []string{"Salt on the Wind"}) {
		t.Errorf("read again, explicit: %q", got)
	}
}
