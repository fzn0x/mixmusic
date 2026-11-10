package tagwrite

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/flac"
)

// flacAudio is made-up bytes, as a FLAC file's frames after its metadata.
var flacAudio = func() []byte {
	b := make([]byte, 6000)
	for i := range b {
		b[i] = byte(i*13 + i/7)
	}
	b[0], b[1] = 0xFF, 0xF8
	return b
}()

// flacBlockOf is a metadata block, the last one if last.
func flacBlockOf(kind byte, data []byte, last bool) []byte {
	if last {
		kind |= 0x80
	}
	return append([]byte{kind, byte(len(data) >> 16), byte(len(data) >> 8), byte(len(data))}, data...)
}

// vorbisComments are Vorbis comments of a vendor and pairs.
func vorbisComments(vendor string, pairs [][2]string) []byte {
	c := comments{vendor: []byte(vendor)}
	for _, p := range pairs {
		c.list = append(c.list, []byte(p[0]+"="+p[1]))
	}
	return c.bytes()
}

// flacFile is a FLAC file of STREAMINFO, a seek table, the comments of
// pairs, unless nil, padding of its bytes, unless 0, and flacAudio.
func flacFile(t testing.TB, pairs [][2]string, padding int) []byte {
	info := make([]byte, 34)
	binary.BigEndian.PutUint16(info, 4096)
	binary.BigEndian.PutUint16(info[2:], 4096)
	info[10], info[11], info[12] = 0x0A, 0xC4, 0x42 // 44.1 kHz, stereo, 16 bits
	blocks := [][]byte{flacBlockOf(blockStreamInfo, info, false), flacBlockOf(3, make([]byte, 18), false)}
	if pairs != nil {
		blocks = append(blocks, flacBlockOf(blockComments, vorbisComments("reference libFLAC 1.4.3", pairs), false))
	}
	if padding > 0 {
		blocks = append(blocks, flacBlockOf(blockPadding, make([]byte, padding), false))
	}
	blocks[len(blocks)-1][0] |= 0x80
	return append(append([]byte("fLaC"), bytes.Join(blocks, nil)...), flacAudio...)
}

// flacLayout is the types of a FLAC file's metadata blocks, and where its
// audio starts.
func flacLayout(t *testing.T, b []byte) ([]byte, int64) {
	t.Helper()
	f, _ := newFile(bytes.NewReader(b))
	start, _ := f.id3End()
	blocks, audio, err := f.flacBlocks(start + 4)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []byte
	for _, bl := range blocks {
		kinds = append(kinds, bl.kind)
	}
	return kinds, audio
}

// TestFLAC writes every field into a file's comments and a PICTURE block,
// keeping its other comments and blocks, within its padding, so that the
// audio stays where it was; then more than the padding holds.
func TestFLAC(t *testing.T) {
	data := flacFile(t, [][2]string{{"title", "Old"}, {"COMMENT", "ripped in 2009"}, {"Album Artist", "Someone"},
		{"TRACKNUMBER", "1/11"}, {"REPLAYGAIN_TRACK_GAIN", "-6.2 dB"}}, 8192)
	_, before := flacLayout(t, data)
	path := save(t, "song.flac", data)
	tg := song(t)
	tg.Cover = &Picture{Data: jpegCover(t, 20, 20)}
	b := write(t, path, tg, All)
	matches(t, path, tg)
	kinds, audio := flacLayout(t, b)
	if !slices.Equal(kinds, []byte{blockStreamInfo, 3, blockComments, blockPicture, blockPadding}) || audio != before {
		t.Errorf("blocks %v, the audio at %d, was at %d", kinds, audio, before)
	}
	if !bytes.Equal(b[audio:], flacAudio) {
		t.Error("the audio changed")
	}
	f, _ := newFile(bytes.NewReader(b))
	blocks, _, _ := f.flacBlocks(4)
	vc, _, err := parseComments(blocks[2].data)
	if err != nil {
		t.Fatal(err)
	}
	if string(vc.vendor) != "reference libFLAC 1.4.3" || vc.value("COMMENT") != "ripped in 2009" ||
		vc.value("REPLAYGAIN_TRACK_GAIN") != "-6.2 dB" || vc.value("TRACKNUMBER") != "3/11" || vc.value("ALBUM ARTIST") != "" {
		t.Errorf("the comments: %q", vc.list)
	}

	// A cover larger than the padding moves the audio, which stays as it
	// was.
	tg.Cover = &Picture{Data: pngCover(t, 200, 200, 1)}
	b = write(t, path, tg, Cover)
	matches(t, path, tg)
	if _, audio := flacLayout(t, b); !bytes.Equal(b[audio:], flacAudio) {
		t.Error("the audio changed")
	}

	// And it shrinks.
	tg.Cover, tg.Title = nil, "Short"
	b = write(t, path, tg, Cover|Title)
	matches(t, path, tg)
	if kinds, audio := flacLayout(t, b); !bytes.Equal(b[audio:], flacAudio) || slices.Contains(kinds, blockPicture) {
		t.Errorf("blocks %v, or the audio changed", kinds)
	}
}

// TestFLACWithoutComments gives a FLAC file comments after STREAMINFO.
func TestFLACWithoutComments(t *testing.T) {
	data := flacFile(t, nil, 0)
	path := save(t, "bare.flac", data)
	b := write(t, path, Tags{Title: "Bare", Artists: []string{"Nobody"}}, Title|Artists)
	matches(t, path, Tags{Title: "Bare", Artists: []string{"Nobody"}})
	kinds, audio := flacLayout(t, b)
	if !slices.Equal(kinds, []byte{blockStreamInfo, blockComments, 3, blockPadding}) || !bytes.Equal(b[audio:], flacAudio) {
		t.Errorf("blocks %v, or the audio changed", kinds)
	}
}

// TestFLACAfterID3 drops the ID3 tag some programs write before "fLaC",
// keeping in the comments what it told that they didn't.
func TestFLACAfterID3(t *testing.T) {
	cover := pngCover(t, 6, 6, 77)
	tag := id3Tag4(textFrame4("TIT2", "From ID3"), textFrame4("TALB", "Album of ID3"), textFrame4("TDRC", "2004"),
		frame4("APIC", append(append([]byte{0}, "image/png\x00\x03\x00"...), cover...)))
	data := append(tag, flacFile(t, [][2]string{{"TITLE", "From the comments"}}, 0)...)
	path := save(t, "id3.flac", data)
	b := write(t, path, Tags{Artists: []string{"New Artist"}}, Artists)
	if string(b[:4]) != "fLaC" {
		t.Fatalf("the file starts %q", b[:4])
	}
	matches(t, path, Tags{Title: "From ID3", Artists: []string{"New Artist"}, Album: "Album of ID3", Year: 2004,
		Cover: &Picture{Data: cover}})
	if _, audio := flacLayout(t, b); !bytes.Equal(b[audio:], flacAudio) {
		t.Error("the audio changed")
	}
}

// TestFLACDecodes writes the tags of a FLAC file of the repository's
// testdata, which decodes to the same samples after.
func TestFLACDecodes(t *testing.T) {
	data := fixture(t, "../../flac/testdata/stereo-16.flac")
	path := save(t, "tone.flac", data)
	tg := song(t)
	b := write(t, path, tg, All)
	matches(t, path, tg)
	if a, b := flacSamples(t, data), flacSamples(t, b); !slices.Equal(a, b) || len(a) == 0 {
		t.Errorf("%d samples, were %d", len(b), len(a))
	}
}

// flacSamples decodes a FLAC file's samples, channel after channel of each
// frame.
func flacSamples(t *testing.T, b []byte) []int32 {
	t.Helper()
	r, err := flac.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	var out []int32
	for {
		fr, err := r.ReadFrame()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, ch := range fr.Samples {
			out = append(out, ch...)
		}
	}
}
