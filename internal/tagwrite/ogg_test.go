package tagwrite

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/fzn0x/mixmusic/ogg"
)

// oggPackets reads the packets of an Ogg file's first stream through the
// repository's reader, which checks each page's checksum and passes over
// a page whose checksum fails: each packet's data, and the granule
// position of the packets that end a page.
func oggPackets(t *testing.T, b []byte) [][]byte {
	t.Helper()
	r, err := ogg.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	var out [][]byte
	for {
		p, err := r.ReadPacket()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		g := binary.LittleEndian.AppendUint64(nil, uint64(p.Granule))
		out = append(out, append(bytes.Clone(p.Data), g...))
	}
}

// samePackets checks that two Ogg files hold the same packets but their
// comment headers, at the same granule positions, and that every page of
// b is sound, in order.
func samePackets(t *testing.T, a, b []byte) {
	t.Helper()
	pa, pb := oggPackets(t, a), oggPackets(t, b)
	if len(pa) != len(pb) || len(pa) < 3 {
		t.Fatalf("%d packets, were %d", len(pb), len(pa))
	}
	for i := range pa {
		if i == 1 {
			continue // the comment header
		}
		if !bytes.Equal(pa[i], pb[i]) {
			t.Fatalf("packet %d changed", i)
		}
	}
	pr := &pageReader{r: bufio.NewReader(bytes.NewReader(b))}
	seq := map[uint32]uint32{}
	for n := 0; ; n++ {
		p, err := pr.next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if !p.sound {
			t.Errorf("the checksum of page %d fails", n)
		}
		if want, ok := seq[p.serial]; ok && p.seq != want {
			t.Errorf("page %d is number %d, want %d", n, p.seq, want)
		}
		seq[p.serial] = p.seq + 1
	}
}

// TestOpus writes every field into the OpusTags of an Ogg Opus file of the
// repository's testdata, with a cover large enough to take several pages,
// then takes it away again: the audio's packets stay as they were.
func TestOpus(t *testing.T) {
	data := fixture(t, "../../ogg/testdata/tone.opus")
	path := save(t, "tone.opus", data)
	tg := song(t)
	tg.Cover = &Picture{Data: jpegCoverOf(t, 2*255*255)}
	b := write(t, path, tg, All)
	matches(t, path, tg)
	samePackets(t, data, b)
	head := oggPackets(t, b)[1]
	if !bytes.HasPrefix(head, []byte("OpusTags")) {
		t.Fatalf("the comment header starts %q", head[:8])
	}

	tg.Cover, tg.Artists = nil, []string{"Only One"}
	b = write(t, path, tg, Cover|Artists)
	matches(t, path, tg)
	samePackets(t, data, b)
}

// jpegCoverOf is a JPEG image of at least n bytes.
func jpegCoverOf(t testing.TB, n int) []byte {
	img := jpegCover(t, 8, 8)
	// A JPEG reader stops at its end marker: what follows is passed over.
	return append(img, make([]byte, max(n-len(img), 0))...)
}

// TestVorbis writes the comment header of an Ogg Vorbis file of the
// repository's testdata, which shares its pages with the setup header:
// that stays as it was, and so do the audio's packets.
func TestVorbis(t *testing.T) {
	data := fixture(t, "../../ogg/testdata/tone.ogg")
	path := save(t, "tone.ogg", data)
	tg := song(t)
	b := write(t, path, tg, All)
	matches(t, path, tg)
	samePackets(t, data, b)
	p := oggPackets(t, b)
	if !bytes.HasPrefix(p[1], []byte("\x03vorbis")) || p[1][len(p[1])-9] != 1 {
		t.Errorf("the comment header doesn't end with its framing bit")
	}
	if !bytes.HasPrefix(p[2], []byte("\x05vorbis")) {
		t.Errorf("the setup header starts %q", p[2][:7])
	}
	tg.Title = "A Much Shorter Title"
	b = write(t, path, tg, Title)
	matches(t, path, tg)
	samePackets(t, data, b)
}

// opusFile is an Ogg Opus file built here: its headers, with the comments
// of pairs, then n pages of made-up packets, each page's granule position
// growing by 960 samples a packet, the last page the stream's last.
func opusFile(pairs [][2]string, n int) []byte {
	head := []byte("OpusHead\x01\x02\x38\x01\x80\xbb\x00\x00\x00\x00\x00")
	tagsPacket := append([]byte("OpusTags"), vorbisComments("libopus 1.5", pairs)...)
	var b []byte
	first := paginate([][]byte{head}, 7, 0)
	first[0].flags = pageFirst
	b = append(b, first[0].bytes()...)
	pages := paginate([][]byte{tagsPacket}, 7, 1)
	for _, p := range pages {
		b = append(b, p.bytes()...)
	}
	seq := uint32(1 + len(pages))
	granule := uint64(312)
	for i := range n {
		var packets [][]byte
		for j := range 5 {
			p := make([]byte, 60+j)
			for k := range p {
				p[k] = byte(i*5 + j + k)
			}
			packets = append(packets, p)
		}
		pg := paginate(packets, 7, seq)[0]
		granule += 5 * 960
		pg.granule = granule
		if i == n-1 {
			pg.flags |= pageLast
		}
		b = append(b, pg.bytes()...)
		seq++
	}
	return b
}

// TestOggBuilt writes a file built here, its comment header growing to
// pages more and shrinking back to one, and refuses one whose page is
// damaged once its pages have to be numbered again.
func TestOggBuilt(t *testing.T) {
	data := opusFile([][2]string{{"TITLE", "Built"}, {"ENCODER", "made by hand"}}, 6)
	path := save(t, "built.opus", data)
	if got := readTags(t, path); got.Title != "Built" {
		t.Fatalf("the file built reads %+v", got)
	}
	tg := Tags{Title: "Built", Cover: &Picture{Data: jpegCoverOf(t, 70000)}}
	b := write(t, path, tg, Cover)
	matches(t, path, tg)
	samePackets(t, data, b)
	// Without the cover, the comments are those of the file built, on as
	// many pages, and so the file is as it was.
	b = write(t, path, Tags{}, Cover)
	matches(t, path, Tags{Title: "Built"})
	if !bytes.Equal(b, data) {
		t.Errorf("%d bytes, not the %d of the file built", len(b), len(data))
	}

	damaged := bytes.Clone(data)
	damaged[len(damaged)-10] ^= 0xFF // in the last page's body
	path = save(t, "damaged.opus", damaged)
	err := Write(path, Tags{Cover: &Picture{Data: jpegCoverOf(t, 70000)}}, Cover, Options{})
	if !errors.Is(err, ErrFormat) {
		t.Errorf("a damaged page: %v", err)
	}
	unchanged(t, path, damaged)
}
