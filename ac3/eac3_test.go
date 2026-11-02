package ac3

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFFmpegEAC3 encodes the streams of TestFFmpeg with ffmpeg's E-AC-3
// encoder, and compares their decodes to ffmpeg's, their dithflags cleared.
func TestFFmpegEAC3(t *testing.T) {
	haveFFmpeg(t)
	dir := t.TempDir()
	all := toolUse{}
	for _, e := range encodings {
		t.Run(e.name, func(t *testing.T) {
			path := e.encodeAs(t, dir, "eac3")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			plain := filepath.Join(dir, e.name+"-undithered.eac3")
			if err := os.WriteFile(plain, undither(t, data), 0o644); err != nil {
				t.Fatal(err)
			}
			pdata, _ := os.ReadFile(plain)
			ours, d, use := decodeAll(t, pdata)
			diffs, ok := compare(t, ffmpegDecode(t, plain), ours, d.h.acmod, d.h.lfeon)
			t.Logf("acmod %d, LFE %v, %d blocks: %v; %v", d.h.acmod, d.h.lfeon, d.h.numblks, diffs, use)
			if !ok {
				t.Errorf("the decodes differ: %v", diffs)
			}
			for k, v := range use {
				all[k] += v
			}
		})
	}
	t.Logf("tools used: %v", all)
}
