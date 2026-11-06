package ogg

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// fixtures are the files of testdata, sine waves of 440 and 660 Hz in
// stereo, as ffmpeg 7.0 encodes them: tone.opus, 3 seconds of Opus in
// pages of 200 ms, and tone.ogg, 2 seconds of Vorbis at 44.1 kHz. They
// were made with
//
//	ffmpeg -fflags +bitexact -f lavfi -i "aevalsrc=0.4*sin(2*PI*440*t)|0.4*sin(2*PI*660*t):s=48000:d=3" -c:a libopus -b:a 24k -page_duration 200000 -flags:a +bitexact tone.opus
//	ffmpeg -fflags +bitexact -f lavfi -i "aevalsrc=0.4*sin(2*PI*440*t)|0.4*sin(2*PI*660*t):s=44100:d=2" -c:a libvorbis -q:a 0 -flags:a +bitexact tone.ogg
var fixtures = []struct {
	name    string
	headers int
	magic   string
	last    int64 // the granule position of the last page
}{
	{"tone.opus", 2, "OpusHead", 144312},
	{"tone.ogg", 3, "\x01vorbis", 88200},
}

func readFixture(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// packets reads the packets of r on to the end, copies of them.
func packets(t testing.TB, r *Reader) []Packet {
	t.Helper()
	var out []Packet
	for {
		p, err := r.ReadPacket()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		p.Data = bytes.Clone(p.Data)
		out = append(out, p)
	}
}

// openAudio opens data, and reads the headers of its stream.
func openAudio(t testing.TB, data []byte, headers int) (*Reader, []Packet) {
	t.Helper()
	r, err := NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var hs []Packet
	for range headers {
		p, err := r.ReadPacket()
		if err != nil {
			t.Fatal(err)
		}
		p.Data = bytes.Clone(p.Data)
		hs = append(hs, p)
	}
	r.StartAudio()
	return r, hs
}

// TestPackets reads the fixtures' packets: their headers, then their
// audio, the granule positions of the pages they end, going up, and
// their last page's, which LastGranule tells too.
func TestPackets(t *testing.T) {
	for _, c := range fixtures {
		t.Run(c.name, func(t *testing.T) {
			r, hs := openAudio(t, readFixture(t, c.name), c.headers)
			if !bytes.HasPrefix(hs[0].Data, []byte(c.magic)) {
				t.Fatalf("first packet %q", hs[0].Data[:8])
			}
			last, err := r.LastGranule()
			if err != nil || last != c.last {
				t.Fatalf("LastGranule = %d, %v; want %d", last, err, c.last)
			}
			ps := packets(t, r)
			if len(ps) < 50 {
				t.Fatalf("%d packets", len(ps))
			}
			g := int64(0)
			for i, p := range ps {
				if p.Granule >= 0 {
					if p.Granule < g {
						t.Fatalf("packet %d ends at %d, after %d", i, p.Granule, g)
					}
					g = p.Granule
				}
				if p.Last != (i == len(ps)-1) {
					t.Fatalf("packet %d of %d: Last %v", i, len(ps), p.Last)
				}
			}
			if g != c.last {
				t.Errorf("the last granule position %d, want %d", g, c.last)
			}
		})
	}
}

// TestSeekGranule seeks to granule positions across the fixtures, by the
// search for pages and from the start, and reads on from the packet after
// the last that ends at or before them.
func TestSeekGranule(t *testing.T) {
	defer func(s int64) { seekSpan = s }(seekSpan)
	for _, span := range []int64{64 << 10, 512} {
		seekSpan = span
		for _, c := range fixtures {
			data := readFixture(t, c.name)
			r, _ := openAudio(t, data, c.headers)
			all := packets(t, r)
			for _, g := range []int64{c.last / 2, 0, c.last - 1, c.last / 7, c.last + 100, 1, all[0].Granule} {
				if g < 0 {
					continue
				}
				want, from := 0, int64(0)
				for i, p := range all {
					if p.Granule >= 0 && p.Granule <= g {
						want, from = i+1, p.Granule
					}
				}
				start, err := r.SeekGranule(g)
				if err != nil {
					t.Fatalf("%s: SeekGranule(%d): %v", c.name, g, err)
				}
				if start != from {
					t.Fatalf("%s: SeekGranule(%d) = %d, want %d", c.name, g, start, from)
				}
				got := packets(t, r)
				if len(got) != len(all)-want {
					t.Fatalf("%s: after SeekGranule(%d), %d packets, want %d", c.name, g, len(got), len(all)-want)
				}
				for i, p := range got {
					if !bytes.Equal(p.Data, all[want+i].Data) || p.Granule != all[want+i].Granule {
						t.Fatalf("%s: after SeekGranule(%d), packet %d differs", c.name, g, i)
					}
				}
			}
		}
	}
}

// TestPassesOverBrokenPages reads on past a page whose checksum fails,
// without the packets it holds parts of, and past junk between pages.
func TestPassesOverBrokenPages(t *testing.T) {
	data := readFixture(t, "tone.opus")
	r, _ := openAudio(t, data, 2)
	all := packets(t, r)
	broken := bytes.Clone(data)
	broken[len(broken)/2] ^= 0x55
	at := bytes.Index(broken[len(broken)/3:], []byte("OggS")) + len(broken)/3
	broken = append(broken[:at:at], append([]byte("junk OggS junk"), broken[at:]...)...)
	r, _ = openAudio(t, broken, 2)
	got := packets(t, r)
	if len(got) >= len(all) || len(got) < len(all)-20 {
		t.Fatalf("%d packets of %d", len(got), len(all))
	}
	// What is read is what the file held, in its order.
	j := 0
	for _, p := range got {
		for j < len(all) && !bytes.Equal(all[j].Data, p.Data) {
			j++
		}
		if j == len(all) {
			t.Fatal("a packet the file didn't hold")
		}
		j++
	}
}

// TestErrors refuses what isn't Ogg, with ErrFormat.
func TestErrors(t *testing.T) {
	good := readFixture(t, "tone.ogg")
	for _, c := range []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"not Ogg", []byte("fLaC\x00\x00\x00\x22 and so on, and on")},
		{"cut", good[:30]},
		{"a broken first page", append(bytes.Clone(good[:22]), append([]byte{1, 2, 3, 4}, good[26:]...)...)},
	} {
		_, err := NewReader(bytes.NewReader(c.data))
		if !errors.Is(err, ErrFormat) {
			t.Errorf("%s: %v, want ErrFormat", c.name, err)
		}
	}
}

// FuzzReader reads any bytes as an Ogg file, its packets, the end of its
// stream, and seeks: it never panics, and fails only with ErrFormat or
// the end of the file.
func FuzzReader(f *testing.F) {
	for _, c := range fixtures {
		b := readFixture(f, c.name)
		f.Add(b[:4096], int64(1000))
		f.Add(b, int64(c.last/2))
	}
	f.Fuzz(func(t *testing.T, b []byte, g int64) {
		r, err := NewReader(bytes.NewReader(b))
		if err != nil {
			if !errors.Is(err, ErrFormat) {
				t.Fatalf("not ErrFormat: %v", err)
			}
			return
		}
		read := func() {
			for range 300 {
				if _, err := r.ReadPacket(); err != nil {
					if err != io.EOF && !errors.Is(err, ErrFormat) {
						t.Fatalf("ReadPacket: %v", err)
					}
					return
				}
			}
		}
		read()
		r.StartAudio()
		if _, err := r.LastGranule(); err != nil {
			t.Fatalf("LastGranule: %v", err)
		}
		if _, err := r.SeekGranule(g); err != nil {
			t.Fatalf("SeekGranule: %v", err)
		}
		read()
	})
}
