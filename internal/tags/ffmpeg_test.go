package tags

import (
	"bytes"
	"image"
	"image/color"
	imgpng "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestFFmpegFiles reads the tags ffmpeg writes in a file of each kind, of
// made-up songs, with a cover where it writes one.
func TestFFmpegFiles(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg isn't on the PATH")
	}
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := range img.Pix {
		img.Pix[i] = byte(i)
	}
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var cover bytes.Buffer
	if err := imgpng.Encode(&cover, img); err != nil {
		t.Fatal(err)
	}
	coverPath := filepath.Join(dir, "cover.png")
	if err := os.WriteFile(coverPath, cover.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	meta := []string{
		"-metadata", "title=Lanterns over the Bay", "-metadata", "artist=Ada Field",
		"-metadata", "album=Café Nights", "-metadata", "album_artist=Various Artists",
		"-metadata", "track=3/12", "-metadata", "disc=2/2", "-metadata", "date=2021",
	}
	withCover := []string{"-i", coverPath, "-map", "0:a", "-map", "1", "-c:v", "copy", "-disposition:v", "attached_pic"}
	for _, c := range []struct {
		name  string
		args  []string
		cover bool
		disc  bool
	}{
		{"id3v24.mp3", append(withCover, "-c:a", "libmp3lame"), true, true},
		{"id3v23.mp3", append(withCover, "-c:a", "libmp3lame", "-id3v2_version", "3"), true, true},
		{"tags.flac", append(withCover, "-c:a", "flac"), true, true},
		{"tags.m4a", append(withCover, "-c:a", "aac"), true, true},
		{"tags.opus", []string{"-c:a", "libopus"}, false, true},
		{"tags.ogg", []string{"-c:a", "libvorbis"}, false, true},
		// WAV's INFO list has no album artist or disc.
		{"tags.wav", []string{"-c:a", "pcm_s16le"}, false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(dir, c.name)
			args := append([]string{"-v", "error", "-nostdin", "-f", "lavfi", "-i", "sine=duration=0.2"}, c.args...)
			args = append(append(args, meta...), path)
			if out, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
				t.Fatalf("ffmpeg: %v\n%s", err, out)
			}
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			got, err := Read(f)
			if err != nil {
				t.Fatal(err)
			}
			want := Tags{Title: "Lanterns over the Bay", Artists: []string{"Ada Field"}, Album: "Café Nights", Track: 3, Year: 2021}
			if c.disc {
				want.AlbumArtists, want.Disc = []string{"Various Artists"}, 2
			}
			pic := got.Picture
			got.Picture = nil
			check(t, got, want)
			switch {
			case c.cover && (pic == nil || !bytes.Equal(pic.Data, cover.Bytes()) || pic.MIME != "image/png"):
				t.Errorf("cover %+v", pic)
			case !c.cover && pic != nil:
				t.Errorf("a cover of %q", pic.MIME)
			}
		})
	}
}
