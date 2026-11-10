//go:build darwin && (amd64 || arm64)

package localfiles

import (
	"slices"
	"testing"
)

// fsItemIsDir is kFSEventStreamEventFlagItemIsDir, which FSEvents sets on
// the events of a folder.
const fsItemIsDir = 0x00020000

// TestFSEvent tells what each event of FSEvents becomes. The folder of
// music made tells nothing to scan: FSEvents tells a folder made just
// before its watch started as made, and the folder is there.
func TestFSEvent(t *testing.T) {
	const root = "/Users/a/Music"
	for _, c := range []struct {
		what  string
		path  string
		flags uint32
		want  []string
	}{
		{"the folder of music made", root, fsItemIsDir | fsItemCreated, []string{"changed " + root}},
		{"the folder of music written in", root, fsItemIsDir | fsItemMetaMod, []string{"changed " + root}},
		{"the folder of music renamed", root, fsItemIsDir | fsItemRenamed, []string{"moved " + root}},
		{"the folder of music removed", root, fsItemIsDir | fsItemCreated | fsItemRemoved, []string{"moved " + root}},
		{"the folder of music to scan", root, fsMustScanSubDirs, []string{"lost"}},
		{"a folder made in it", root + "/Live", fsItemIsDir | fsItemCreated, []string{"moved " + root + "/Live"}},
		{"a file made", root + "/Live/1.mp3", fsItemCreated | fsItemModified, []string{"moved " + root + "/Live/1.mp3"}},
		{"a file written", root + "/1.mp3", fsItemModified, []string{"changed " + root + "/1.mp3"}},
		{"a folder in it to scan", root + "/Live", fsMustScanSubDirs, []string{"moved " + root + "/Live"}},
		{"events dropped", root + "/1.mp3", fsKernelDropped, []string{"lost"}},
		{"beyond the folder", "/Users/a/Musical/1.mp3", fsItemCreated, nil},
	} {
		var got []string
		s := &fsStream{root: root, real: root, tell: Changes{
			Changed: func(p string) { got = append(got, "changed "+p) },
			Moved:   func(p string) { got = append(got, "moved "+p) },
			Lost:    func() { got = append(got, "lost") },
			Ended:   func(err error) { got = append(got, "ended") },
		}}
		s.event(c.path, c.flags)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s (%#x): told %q, want %q", c.what, c.flags, got, c.want)
		}
	}
}
