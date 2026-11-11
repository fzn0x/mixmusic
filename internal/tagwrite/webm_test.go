package tagwrite

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/ogg"
	"github.com/fzn0x/mixmusic/opus"
	"github.com/fzn0x/mixmusic/webm"
)

// The elements of the WebM files the tests make.
const (
	ebmlID           = 0x1A45DFA3
	ebmlDocType      = 0x4282
	segmentID        = 0x18538067
	infoID           = 0x1549A966
	timestampScaleID = 0x2AD7B1
	durationID       = 0x4489
	tracksID         = 0x1654AE6B
	trackEntryID     = 0xAE
	trackNumberID    = 0xD7
	trackTypeID      = 0x83
	codecIDID        = 0x86
	codecPrivateID   = 0x63A2
	codecDelayID     = 0x56AA
	audioID          = 0xE1
	samplingID       = 0xB5
	channelsID       = 0x9F
	clusterID        = 0x1F43B675
	timestampID      = 0xE7
	simpleBlockID    = 0xA3
	blockGroupID     = 0xA0
	blockID          = 0xA1
	discardID        = 0x75A2
)

// ebml is an element of id holding data, with a size of 8 bytes.
func ebml(id uint32, data ...[]byte) []byte {
	idb := binary.BigEndian.AppendUint32(nil, id)
	for len(idb) > 1 && idb[0] == 0 {
		idb = idb[1:]
	}
	d := slices.Concat(data...)
	size := binary.BigEndian.AppendUint64(nil, uint64(len(d)))
	size[0] = 0x01 // a size of 8 bytes
	return slices.Concat(idb, size, d)
}

func ebmlUint(id uint32, v uint64) []byte { return ebml(id, binary.BigEndian.AppendUint64(nil, v)) }
func ebmlInt(id uint32, v int64) []byte {
	return ebml(id, binary.BigEndian.AppendUint64(nil, uint64(v)))
}
func ebmlFloat(id uint32, f float64) []byte {
	return ebml(id, binary.BigEndian.AppendUint64(nil, math.Float64bits(f)))
}

// testHead is the OpusHead of the files the tests make: stereo, with a
// pre-skip of 312 samples.
var testHead = []byte("OpusHead\x01\x02\x38\x01\x80\xbb\x00\x00\x00\x00\x00")

// opusPacket is a made-up Opus packet of size bytes whose TOC byte says it
// holds frames frames of 20 ms of CELT, stereo, and data that differs from
// packet to packet: what a remux copies, without decoding it.
func opusPacket(i, frames, size int) []byte {
	toc := byte(31<<3 | 0x04)
	head := []byte{toc}
	if frames > 1 {
		head = []byte{toc | 3, byte(frames)}
	}
	b := append(head, make([]byte, size-len(head))...)
	for j := len(head); j < len(b); j++ {
		b[j] = byte(i*7 + j)
	}
	return b
}

// webmFile makes a WebM file of packets, of codec, in clusters of ten,
// whose last block drops discard samples at its end, and returns it with
// the samples of each packet.
func webmFile(t *testing.T, codec string, packets [][]byte, discard int64) ([]byte, []int) {
	t.Helper()
	var clusters [][]byte
	var samples []int
	at := 0 // ms
	for first := 0; first < len(packets); first += 10 {
		children := [][]byte{ebmlUint(timestampID, uint64(at))}
		for i := first; i < min(first+10, len(packets)); i++ {
			n, err := opus.PacketSamples(packets[i], opusRate)
			if err != nil {
				t.Fatal(err)
			}
			samples = append(samples, n)
			rel := binary.BigEndian.AppendUint16(nil, uint16(at-int(binary.BigEndian.Uint64(children[0][len(children[0])-8:]))))
			block := slices.Concat([]byte{0x81}, rel, []byte{0x80}, packets[i])
			if i == len(packets)-1 && discard > 0 {
				children = append(children, ebml(blockGroupID, ebml(blockID, slices.Concat([]byte{0x81}, rel, []byte{0}, packets[i])),
					ebmlInt(discardID, discard*1e9/opusRate)))
			} else {
				children = append(children, ebml(simpleBlockID, block))
			}
			at += n * 1000 / opusRate
		}
		clusters = append(clusters, ebml(clusterID, children...))
	}
	track := ebml(trackEntryID, ebmlUint(trackNumberID, 1), ebmlUint(trackTypeID, 2), ebml(codecIDID, []byte(codec)),
		ebml(codecPrivateID, testHead), ebmlUint(codecDelayID, 6_500_000),
		ebml(audioID, ebmlFloat(samplingID, 48000), ebmlUint(channelsID, 2)))
	segment := ebml(segmentID, slices.Concat(ebml(infoID, ebmlUint(timestampScaleID, 1_000_000), ebmlFloat(durationID, float64(at))),
		ebml(tracksID, track), slices.Concat(clusters...)))
	return slices.Concat(ebml(ebmlID, ebml(ebmlDocType, []byte("webm"))), segment), samples
}

// oggPages reads every page of an Ogg file, and fails the test on one
// whose checksum is wrong, or whose number doesn't follow the one before.
func oggPages(t *testing.T, b []byte) []oggPage {
	t.Helper()
	pr := &pageReader{r: bufio.NewReader(bytes.NewReader(b))}
	var pages []oggPage
	for {
		p, err := pr.next()
		if err == io.EOF {
			return pages
		}
		if err != nil {
			t.Fatal(err)
		}
		if !p.sound {
			t.Fatalf("page %d's checksum is wrong", len(pages))
		}
		if p.seq != uint32(len(pages)) || len(pages) > 0 && p.serial != pages[0].serial {
			t.Fatalf("page %d is numbered %d of stream %x", len(pages), p.seq, p.serial)
		}
		pages = append(pages, p)
	}
}

func remux(t *testing.T, file []byte, tg Tags, fields Fields) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := RemuxWebM(&out, bytes.NewReader(file), tg, fields); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// TestRemuxWebM writes the Opus of a WebM file as an Ogg Opus file: its
// header as the track's, its tags and cover in its comment header, every
// packet as it was, the granule positions counting the samples to the end
// of each page, the pre-skip among them, and its end trimmed as the
// WebM's last block trimmed it.
func TestRemuxWebM(t *testing.T) {
	var packets [][]byte
	for i := range 130 {
		size := 120 + i%40
		frames := 1
		switch i {
		case 7:
			size = 510 // two whole segments, and an empty one after
		case 8:
			size = 700
		case 20:
			frames = 3 // 60 ms
		}
		packets = append(packets, opusPacket(i, frames, size))
	}
	file, samples := webmFile(t, "A_OPUS", packets, 400)
	tg := song(t)
	out := remux(t, file, tg, All)

	pages := oggPages(t, out)
	if first := pages[0]; first.flags != pageFirst || first.granule != 0 || !bytes.Equal(first.body, testHead) || len(first.segs) != 1 {
		t.Errorf("the first page is %+v", first)
	}
	if last := pages[len(pages)-1]; last.flags&pageLast == 0 {
		t.Error("the last page isn't flagged so")
	}
	for i, p := range pages[:len(pages)-1] {
		if p.flags&pageLast != 0 {
			t.Errorf("page %d is flagged last", i)
		}
	}

	// The packets, through the repository's reader.
	r, err := ogg.NewReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	var got [][]byte
	var granules []int64
	for {
		p, err := r.ReadPacket()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, bytes.Clone(p.Data))
		granules = append(granules, p.Granule)
	}
	if len(got) != len(packets)+2 {
		t.Fatalf("%d packets, want %d", len(got), len(packets)+2)
	}
	if !bytes.HasPrefix(got[1], []byte("OpusTags")) || granules[1] != 0 {
		t.Errorf("the second packet is %q at %d", got[1][:min(len(got[1]), 16)], granules[1])
	}
	total := int64(0)
	for i, p := range packets {
		if !bytes.Equal(got[i+2], p) {
			t.Fatalf("packet %d changed", i)
		}
		total += int64(samples[i])
		want := total
		if i == len(packets)-1 {
			want -= 400
		}
		if g := granules[i+2]; g != -1 && g != want {
			t.Errorf("packet %d ends a page at granule %d, want %d", i, g, want)
		}
	}
	if g := granules[len(granules)-1]; g != total-400 {
		t.Errorf("the stream ends at granule %d, want %d", g, total-400)
	}
	// Pages of about a second, each ending a packet but the 255 segments
	// allow, as YouTube's 20 ms packets never fill.
	audio := pages[2:]
	if len(audio) < 2 {
		t.Fatalf("%d pages of audio", len(audio))
	}
	from := uint64(0)
	for i, p := range audio[:len(audio)-1] {
		if p.granule == ^uint64(0) {
			t.Fatalf("audio page %d ends no packet", i)
		}
		if n := p.granule - from; n < pageSamples || n > pageSamples+2880 {
			t.Errorf("audio page %d holds %d samples", i, n)
		}
		from = p.granule
	}

	path := save(t, "out.opus", out)
	matches(t, path, tg)
	if kind, err := KindOf(bytes.NewReader(out)); err != nil || kind != Opus {
		t.Errorf("it's %q, %v", kind, err)
	}
}

// TestRemuxWebMOfYouTube remuxes a recorded WebM file of Opus, as the
// downloads keep YouTube's, packet for packet.
func TestRemuxWebMOfYouTube(t *testing.T) {
	file, err := os.ReadFile(filepath.Join("..", "..", "webm", "testdata", "stereo-cues-front.webm"))
	if err != nil {
		t.Fatal(err)
	}
	out := remux(t, file, Tags{Title: "Harbour Tone", Artists: []string{"Ola Brine"}}, Title|Artists)
	oggPages(t, out)
	r, err := webm.NewReader(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	want := [][]byte{r.Track().CodecPrivate, nil}
	total := int64(0)
	for {
		p, err := r.ReadPacket()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		n, _ := opus.PacketSamples(p.Data, opusRate)
		total += int64(n)
		want = append(want, p.Data)
	}
	got := oggPackets(t, out)
	if len(got) != len(want) {
		t.Fatalf("%d packets, want %d", len(got), len(want))
	}
	for i, w := range want {
		if i != 1 && !bytes.Equal(got[i][:len(got[i])-8], w) {
			t.Fatalf("packet %d changed", i)
		}
	}
	last := got[len(got)-1]
	if g := int64(binary.LittleEndian.Uint64(last[len(last)-8:])); g > total || g < total-opusRate/10 {
		t.Errorf("it ends at granule %d, of %d samples", g, total)
	}
	path := save(t, "tone.opus", out)
	if tg := readTags(t, path); tg.Title != "Harbour Tone" || !slices.Equal(tg.Artists, []string{"Ola Brine"}) {
		t.Errorf("it reads back as %+v", tg)
	}
}

// TestRemuxWebMRefuses refuses a WebM file of another codec, and one that
// isn't WebM, and tells WebM files apart.
func TestRemuxWebMRefuses(t *testing.T) {
	vorbis, _ := webmFile(t, "A_VORBIS", [][]byte{opusPacket(0, 1, 40)}, 0)
	if err := RemuxWebM(io.Discard, bytes.NewReader(vorbis), Tags{}, All); !errors.Is(err, ErrUnsupported) {
		t.Errorf("a Vorbis track: %v", err)
	}
	if !IsWebM(bytes.NewReader(vorbis)) {
		t.Error("a WebM file isn't told one")
	}
	ogg := fixture(t, filepath.Join("..", "..", "ogg", "testdata", "tone.opus"))
	if IsWebM(bytes.NewReader(ogg)) {
		t.Error("an Ogg file is told WebM")
	}
	if err := RemuxWebM(io.Discard, bytes.NewReader(ogg), Tags{}, All); !errors.Is(err, ErrFormat) {
		t.Errorf("an Ogg file: %v", err)
	}
	opusFile, _ := webmFile(t, "A_OPUS", [][]byte{opusPacket(0, 1, 40)}, 0)
	if err := RemuxWebM(io.Discard, bytes.NewReader(opusFile), Tags{Cover: &Picture{Data: []byte("GIF89a")}}, Cover); !errors.Is(err, ErrCover) {
		t.Errorf("a GIF cover: %v", err)
	}
}

// TestFFmpegRemuxWebM has ffmpeg make a WebM file of Opus, of a tone, as
// YouTube's are, remuxes it, and has ffmpeg decode both: the same samples,
// the start's pre-skip and the end's trim alike, and ffprobe read the tags.
func TestFFmpegRemuxWebM(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg isn't on the PATH")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "tone.webm")
	out, err := exec.Command("ffmpeg", "-v", "error", "-nostdin", "-f", "lavfi", "-i", "sine=frequency=330:duration=2.37",
		"-ac", "2", "-c:a", "libopus", "-b:a", "96k", "-f", "webm", src).CombinedOutput()
	if err != nil {
		t.Skipf("ffmpeg can't make it: %v\n%s", err, out)
	}
	tg := song(t)
	dst := save(t, "tone.opus", remux(t, fixture(t, src), tg, All))
	pcm := func(path string) []byte {
		t.Helper()
		b, err := exec.Command("ffmpeg", "-v", "error", "-nostdin", "-i", path, "-f", "s16le", "-ac", "2", "-ar", "48000", "-").Output()
		if err != nil {
			t.Fatalf("ffmpeg decodes %s: %v", filepath.Base(path), err)
		}
		return b
	}
	a, b := pcm(src), pcm(dst)
	if len(a) == 0 || !bytes.Equal(a, b) {
		t.Errorf("the WebM decodes to %d bytes and the Ogg Opus to %d, which differ", len(a), len(b))
	}
	// 2.37 s, which no whole number of 20 ms packets lasts: the end is
	// trimmed, of two channels of 16 bits.
	if want := 237 * opusRate / 100 * 4; len(b) != want {
		t.Errorf("the Ogg Opus decodes to %d bytes, want %d", len(b), want)
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return
	}
	probed, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream_tags=title,album,artist", "-of", "default=nw=1", dst).Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{tg.Title, tg.Album, "Ada Field"} {
		if !bytes.Contains(probed, []byte(want)) {
			t.Errorf("ffprobe reads %q, without %q", probed, want)
		}
	}
}
