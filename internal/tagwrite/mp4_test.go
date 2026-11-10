package tagwrite

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/mp4"
)

// mp4Packet is a packet of an MP4 file's audio track.
type mp4Packet struct {
	data       []byte
	time, dura time.Duration
}

// mp4Packets reads the packets of the audio track of an MP4 file through
// the repository's reader: each sample's data, from where the sample
// tables or the fragments say it is, and its time.
func mp4Packets(t *testing.T, b []byte) []mp4Packet {
	t.Helper()
	r, err := mp4.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	var out []mp4Packet
	for {
		p, err := r.ReadPacket()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, mp4Packet{p.Data, p.Time, p.Duration})
	}
}

// sameSamples checks that two MP4 files hold the same audio.
func sameSamples(t *testing.T, a, b []byte) {
	t.Helper()
	pa, pb := mp4Packets(t, a), mp4Packets(t, b)
	if len(pa) != len(pb) || len(pa) == 0 {
		t.Fatalf("%d packets, were %d", len(pb), len(pa))
	}
	for i := range pa {
		if !bytes.Equal(pa[i].data, pb[i].data) || pa[i].time != pb[i].time || pa[i].dura != pb[i].dura {
			t.Fatalf("packet %d changed", i)
		}
	}
}

// mdatAt is where an MP4 file's first mdat starts.
func mdatAt(t *testing.T, b []byte) int64 {
	t.Helper()
	f, _ := newFile(bytes.NewReader(b))
	top, _, err := f.topBoxes()
	if err != nil {
		t.Fatal(err)
	}
	for _, bx := range top {
		if bx.kind == "mdat" {
			return bx.at
		}
	}
	t.Fatal("no mdat")
	return 0
}

// TestMP4 writes every field of M4A files of the repository's testdata:
// one whose moov comes before its audio, which moves, and whose sample
// tables move with it; one whose moov comes after; and fragmented ones,
// whose fragments move. Then the tags shrink.
func TestMP4(t *testing.T) {
	for _, name := range []string{"faststart.m4a", "progressive.m4a", "fragmented.m4a", "sidx.m4a", "dash.mp4"} {
		t.Run(name, func(t *testing.T) {
			data := fixture(t, "../../mp4/testdata/"+name)
			path := save(t, name, data)
			tg := song(t)
			b := write(t, path, tg, All)
			matches(t, path, tg)
			sameSamples(t, data, b)

			tg.Cover, tg.Album, tg.Artists = nil, "", []string{"Just One"}
			b = write(t, path, tg, Cover|Album|Artists)
			matches(t, path, tg)
			sameSamples(t, data, b)
		})
	}
}

// TestMP4KeepsItems keeps the items it doesn't change, and a track's
// count.
func TestMP4KeepsItems(t *testing.T) {
	data := fixture(t, "../../mp4/testdata/faststart.m4a")
	path := save(t, "song.m4a", data)
	write(t, path, Tags{Title: "First", Track: 2, Genre: "Ambient", ISRC: "QZES82100002"}, Title|Track|Genre|ISRC)
	var out bytes.Buffer
	in := fixture(t, path)
	f, _ := newFile(bytes.NewReader(in))
	top, _, _ := f.topBoxes()
	for _, bx := range top {
		if bx.kind == "moov" {
			// Give the track a count, as a ripper would.
			out.Write(in[:bx.at])
			moov := bytes.Clone(in[bx.at:bx.end()])
			i := bytes.Index(moov, []byte("trkn"))
			moov[i+4+8+8+5] = 9 // the count's low byte
			out.Write(moov)
			out.Write(in[bx.end():])
		}
	}
	path = save(t, "counted.m4a", out.Bytes())
	write(t, path, Tags{Track: 5, Album: "Second"}, Track|Album)
	got := readTags(t, path)
	if got.Title != "First" || got.Track != 5 || got.Album != "Second" || got.Genre != "Ambient" || got.ISRC != "QZES82100002" {
		t.Errorf("read %+v", got)
	}
	b := fixture(t, path)
	f, _ = newFile(bytes.NewReader(b))
	top, _, _ = f.topBoxes()
	for _, bx := range top {
		if bx.kind == "moov" {
			kids, _ := boxesOf(b[bx.at+bx.head : bx.end()])
			if n := countIn(t, kids); n != 9 {
				t.Errorf("the track's count is %d", n)
			}
		}
	}
	sameSamples(t, data, b)
}

// countIn is the track's count of the ilst of a moov's udta's meta.
func countIn(t *testing.T, moov []box) int {
	for _, k := range moov {
		if k.kind != "udta" {
			continue
		}
		udta, _ := boxesOf(k.data)
		for _, m := range udta {
			if m.kind == "meta" {
				kids, _ := boxesOf(m.data[4:])
				for _, il := range kids {
					if il.kind == "ilst" {
						items, _ := boxesOf(il.data)
						return count(items, "trkn")
					}
				}
			}
		}
	}
	t.Fatal("no ilst")
	return 0
}

// TestMP4FreeTakesTheDifference leaves the audio where it was when a free
// box after the moov takes up its change in size.
func TestMP4FreeTakesTheDifference(t *testing.T) {
	src := fixture(t, "../../mp4/testdata/faststart.m4a")
	// The same file, with 64 KB free after its moov, its audio moved.
	f, _ := newFile(bytes.NewReader(src))
	top, _, err := f.topBoxes()
	if err != nil {
		t.Fatal(err)
	}
	var data []byte
	for _, bx := range top {
		data = append(data, src[bx.at:bx.end()]...)
		if bx.kind == "moov" {
			moov := data[bx.at+bx.head:]
			if err := shiftOffsets(moov, bx.end(), 64<<10); err != nil {
				t.Fatal(err)
			}
			data = append(data, boxBytes("free", make([]byte, 64<<10-8))...)
		}
	}
	sameSamples(t, src, data)
	before := mdatAt(t, data)
	path := save(t, "padded.m4a", data)
	tg := song(t)
	b := write(t, path, tg, All)
	matches(t, path, tg)
	sameSamples(t, src, b)
	if at := mdatAt(t, b); at != before {
		t.Errorf("the audio moved from %d to %d", before, at)
	}
	tg.Cover = nil
	b = write(t, path, tg, Cover)
	if at := mdatAt(t, b); at != before {
		t.Errorf("the audio moved from %d to %d", before, at)
	}
	sameSamples(t, src, b)
}
