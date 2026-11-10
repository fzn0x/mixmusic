package tagwrite

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestFFmpegReads has ffmpeg make a file of each kind, of a tone, writes
// its tags, and has ffprobe read them back and ffmpeg decode it whole
// without an error: another program's reading of what is written.
func TestFFmpegReads(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " isn't on the PATH")
		}
	}
	dir := t.TempDir()
	tg := song(t)
	for _, c := range []struct {
		name, codec string
		args        []string
	}{
		{"tone.mp3", "libmp3lame", nil},
		{"tone.flac", "flac", nil},
		{"tone.opus", "libopus", nil},
		{"tone.ogg", "libvorbis", nil},
		{"tone.m4a", "aac", nil},
		// Its moov before its audio, which moves.
		{"faststart.m4a", "aac", []string{"-movflags", "+faststart"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(dir, c.name)
			args := append([]string{"-v", "error", "-nostdin", "-f", "lavfi", "-i", "sine=duration=0.5", "-c:a", c.codec}, c.args...)
			out, err := exec.Command("ffmpeg", append(args, "-metadata", "title=Before", path)...).CombinedOutput()
			if err != nil {
				t.Skipf("ffmpeg can't make it: %v\n%s", err, out)
			}
			write(t, path, tg, All)
			out, err = exec.Command("ffprobe", "-v", "error", "-show_entries", "format_tags:stream_tags", "-of", "json", path).Output()
			if err != nil {
				t.Fatalf("ffprobe: %v", err)
			}
			var probed struct {
				Format  struct{ Tags map[string]string }
				Streams []struct{ Tags map[string]string }
			}
			if err := json.Unmarshal(out, &probed); err != nil {
				t.Fatal(err)
			}
			got := map[string]string{}
			all := []map[string]string{probed.Format.Tags}
			for _, s := range probed.Streams {
				all = append(all, s.Tags)
			}
			for _, m := range all {
				for k, v := range m {
					got[strings.ToLower(k)] = v
				}
			}
			for k, want := range map[string]string{"title": tg.Title, "album": tg.Album, "genre": tg.Genre} {
				if got[k] != want {
					t.Errorf("ffprobe reads %s %q, want %q; all: %q", k, got[k], want, got)
				}
			}
			if !strings.HasPrefix(got["date"], "2021") || !strings.HasPrefix(got["track"], "3") || !strings.Contains(got["artist"], "Ada Field") {
				t.Errorf("ffprobe reads %q", got)
			}
			if out, err := exec.Command("ffmpeg", "-v", "error", "-nostdin", "-i", path, "-f", "null", "-").CombinedOutput(); err != nil || len(out) > 0 {
				t.Errorf("ffmpeg decodes it with %v: %s", err, out)
			}
		})
	}
}
