package mp4

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"
)

// The kinds of sizes of the sample tables testFile writes.
const (
	stszSizes  = iota // an stsz with the size of each sample
	stszSame          // an stsz with one size for every sample
	stz2Sizes4        // an stz2 of 4-bit sizes
	stz2Sizes8
	stz2Sizes16
)

// The kinds of sidx of fragmented test files.
const (
	noSidx    = iota
	sidxAll   // a sidx of every fragment, in presentation times, as YouTube's
	sidxMedia // in media times, which are off by the edit
	sidxGap   // with a free box between it and the first moof, its first_offset
	sidxBad   // pointing a byte after each fragment
)

// The base offsets of the trafs of fragmented test files.
const (
	moofBase     = iota // default-base-is-moof, as ffmpeg and YouTube
	explicitBase        // a base_data_offset in each tfhd
	implicitBase        // neither: the moof, or the end of the data of the traf before
)

// The places of the durations and sizes of the samples of fragments.
const (
	inTrun = iota // in the trun
	inTfhd        // durations in the tfhd, sizes in the trun
	inTrex        // both in the trex
)

// layout says how testFile lays a file out.
type layout struct {
	samples   int    // of 1024 ticks at 44.1 kHz, but the last, of 500
	perChunk  int    // samples in a chunk, or in a fragment
	frameSize int    // the largest sample: 40 when 0
	sizes     int    // stszSizes, stszSame, stz2Sizes4, 8 or 16
	co64      bool   // chunk offsets in a co64
	moovFirst bool   // the moov before the mdat, as with faststart
	video     bool   // a video track 1, whose chunks or trafs come first
	wide      bool   // 64-bit sizes for the moov, the mdat and the moof boxes, and version 1 boxes
	toEnd     bool   // a last mdat of size 0
	delay     uint32 // the media time of the edit list: none when 0

	fragmented bool
	sidx       int  // noSidx, sidxAll, sidxMedia, sidxGap or sidxBad
	noTfdt     bool // fragments without tfdt
	base       int  // moofBase, explicitBase or implicitBase
	defaults   int  // inTrun, inTfhd or inTrex
	twoRuns    bool // two truns in each traf
	styp       bool // an styp and a free box before each moof
}

// frame returns the data of sample i of a test file: of varied sizes, but
// for stszSame, and different from that of the samples around it.
func frame(i int, l layout) []byte {
	size := l.frameSize
	if size == 0 {
		size = 40
	}
	if l.sizes == stz2Sizes4 {
		size = 15
	}
	n := size
	if l.sizes != stszSame {
		n = 1 + i*37%size
	}
	b := make([]byte, n)
	for j := range b {
		b[j] = byte(i + j)
	}
	return b
}

// sampleTicks returns the duration of sample i of n.
func sampleTicks(i, n int) uint32 {
	if i == n-1 {
		return 500
	}
	return 1024
}

// tickTime is the time of k ticks at 44.1 kHz.
func tickTime(k int) time.Duration { return time.Duration(k) * time.Second / 44100 }

// asc is the AudioSpecificConfig of the test files: AAC-LC at 44.1 kHz,
// stereo.
var asc = []byte{0x12, 0x10}

// desc returns a descriptor of fewer than 128 bytes.
func desc(tag byte, data ...[]byte) []byte {
	d := slices.Concat(data...)
	return slices.Concat([]byte{tag, byte(len(d))}, d)
}

// esdsBox returns an esds of an object type and DecoderSpecificInfo.
func esdsBox(oti byte, dsi []byte) []byte {
	dec := slices.Concat([]byte{oti, 0x15, 0, 0, 0}, u32(0, 0), desc(tagDecoderInfo, dsi))
	return box("esds", full(0, 0, desc(tagES, u16(1), []byte{0}, desc(tagDecoderConfig, dec), desc(6, []byte{2}))))
}

// mp4aBox returns an mp4a sample entry, of version 0, with the given boxes.
func mp4aBox(channels uint16, rate uint32, boxes ...[]byte) []byte {
	return box("mp4a", make([]byte, 6), u16(1), make([]byte, 8), u16(channels, 16, 0, 0), u32(rate<<16), slices.Concat(boxes...))
}

// track is a trak testFile writes.
type track struct {
	id      uint32
	handler string // "soun" when empty
	media   uint64 // the duration of the mdhd
	elst    []byte // an elst, or nil
	entry   []byte // the sample entry: an mp4a of asc when nil
	tables  [][]byte
}

func (t track) trak() []byte {
	handler, entry := t.handler, t.entry
	if handler == "" {
		handler = "soun"
	}
	if entry == nil {
		entry = mp4aBox(2, 44100, esdsBox(0x40, asc))
	}
	var edts []byte
	if t.elst != nil {
		edts = box("edts", t.elst)
	}
	tkhd := box("tkhd", full(0, 3, u32(0, 0, t.id, 0, uint32(t.media)), make([]byte, 60)))
	mdhd := box("mdhd", full(0, 0, u32(0, 0, 44100, uint32(t.media)), u16(0x55C4, 0)))
	hdlr := box("hdlr", full(0, 0, u32(0), []byte(handler), make([]byte, 12), []byte("handler\x00")))
	stbl := box("stbl", box("stsd", full(0, 0, u32(1), entry)), slices.Concat(t.tables...))
	return box("trak", tkhd, edts, box("mdia", mdhd, hdlr, box("minf", box("smhd", full(0, 0, u32(0))), stbl)))
}

// emptyTables are those of a fragmented file.
var emptyTables = [][]byte{
	box("stts", full(0, 0, u32(0))), box("stsc", full(0, 0, u32(0))),
	box("stsz", full(0, 0, u32(0, 0))), box("stco", full(0, 0, u32(0))),
}

// mvhd returns the mvhd of a movie of a timescale of 44.1 kHz.
func mvhd(duration uint32) []byte {
	return box("mvhd", full(0, 0, u32(0, 0, 44100, duration), make([]byte, 80)))
}

// elst returns an edit list of an edit of the media from tick at on, for
// duration ticks.
func elst(version byte, duration uint64, at int64) []byte {
	if version == 1 {
		return box("elst", full(1, 0, u32(1), u64(duration, uint64(at)), u32(1<<16)))
	}
	return box("elst", full(0, 0, u32(1, uint32(duration), uint32(at)), u32(1<<16)))
}

var ftyp = box("ftyp", []byte("M4A "), u32(0), []byte("isomM4A "))

// testFile returns a file laid out as l, and the packets of its audio
// track.
func testFile(l layout) ([]byte, []Packet) {
	var want []Packet
	total := 0
	for i := range l.samples {
		d := sampleTicks(i, l.samples)
		want = append(want, Packet{Data: frame(i, l), Time: tickTime(total), Duration: tickTime(int(d))})
		total += int(d)
	}
	if l.fragmented {
		return fragmentedFile(l, want, total), want
	}
	return progressiveFile(l, want, total), want
}

func progressiveFile(l layout, want []Packet, total int) []byte {
	// The chunks, and those of the video before each.
	var chunks [][]byte
	var counts []int
	for first := 0; first < l.samples; first += l.perChunk {
		var c []byte
		for i := first; i < min(first+l.perChunk, l.samples); i++ {
			c = append(c, want[i].Data...)
		}
		chunks = append(chunks, c)
		counts = append(counts, min(l.perChunk, l.samples-first))
	}
	video := []byte("video")

	tables := func(offsets []uint64) [][]byte {
		stts := box("stts", full(0, 0, u32(2, uint32(l.samples-1), 1024, 1, 500)))
		var stsc []byte
		n := 0
		for i, c := range counts {
			if i == 0 || c != counts[i-1] {
				stsc = append(stsc, u32(uint32(i+1), uint32(c), 1)...)
				n++
			}
		}
		stsc = box("stsc", full(0, 0, u32(uint32(n)), stsc))
		var sizes []byte
		switch l.sizes {
		case stszSizes:
			for _, p := range want {
				sizes = append(sizes, u32(uint32(len(p.Data)))...)
			}
			sizes = box("stsz", full(0, 0, u32(0, uint32(l.samples)), sizes))
		case stszSame:
			sizes = box("stsz", full(0, 0, u32(uint32(len(want[0].Data)), uint32(l.samples))))
		case stz2Sizes4:
			sizes = make([]byte, (l.samples+1)/2)
			for i, p := range want {
				sizes[i/2] |= byte(len(p.Data)) << (4 - 4*(i&1))
			}
			sizes = box("stz2", full(0, 0, []byte{0, 0, 0, 4}, u32(uint32(l.samples)), sizes))
		case stz2Sizes8:
			for _, p := range want {
				sizes = append(sizes, byte(len(p.Data)))
			}
			sizes = box("stz2", full(0, 0, []byte{0, 0, 0, 8}, u32(uint32(l.samples)), sizes))
		case stz2Sizes16:
			for _, p := range want {
				sizes = append(sizes, u16(uint16(len(p.Data)))...)
			}
			sizes = box("stz2", full(0, 0, []byte{0, 0, 0, 16}, u32(uint32(l.samples)), sizes))
		}
		var co []byte
		if l.co64 {
			co = box("co64", full(0, 0, u32(uint32(len(offsets))), u64(offsets...)))
		} else {
			var o []uint32
			for _, v := range offsets {
				o = append(o, uint32(v))
			}
			co = box("stco", full(0, 0, u32(uint32(len(offsets))), u32(o...)))
		}
		return [][]byte{stts, stsc, sizes, co}
	}
	moov := func(offsets, videoOffsets []uint64) []byte {
		audio := track{id: 2, media: uint64(total), tables: tables(offsets)}
		if l.delay > 0 {
			audio.elst = elst(0, uint64(total)-uint64(l.delay), int64(l.delay))
		}
		var v []byte
		if l.video {
			var o []uint32
			for _, off := range videoOffsets {
				o = append(o, uint32(off))
			}
			v = track{id: 1, handler: "vide", entry: box("mp4v", make([]byte, 78)), tables: [][]byte{
				box("stts", full(0, 0, u32(1, uint32(len(o)), 2048))), box("stsc", full(0, 0, u32(1, 1, 1, 1))),
				box("stsz", full(0, 0, u32(uint32(len(video)), uint32(len(o))))), box("stco", full(0, 0, u32(uint32(len(o))), u32(o...))),
			}}.trak()
		}
		if l.wide {
			return box64("moov", mvhd(uint32(total)), v, audio.trak())
		}
		return box("moov", mvhd(uint32(total)), v, audio.trak())
	}
	mdat := func(data ...[]byte) []byte {
		switch {
		case l.toEnd:
			return boxToEnd("mdat", data...)
		case l.wide:
			return box64("mdat", data...)
		}
		return box("mdat", data...)
	}

	// The offsets take a fixed width, so that the size of the moov does
	// not depend on them: lay out with zeros first.
	offsets, videoOffsets := make([]uint64, len(chunks)), make([]uint64, len(chunks))
	var data [][]byte
	for _, c := range chunks {
		if l.video {
			data = append(data, video)
		}
		data = append(data, c)
	}
	start := uint64(len(ftyp)) + uint64(len(mdat())) // of the data of the mdat
	if l.moovFirst {
		start += uint64(len(moov(offsets, videoOffsets)))
	}
	at := start
	for i, c := range chunks {
		if l.video {
			videoOffsets[i] = at
			at += uint64(len(video))
		}
		offsets[i] = at
		at += uint64(len(c))
	}
	if l.moovFirst {
		return slices.Concat(ftyp, moov(offsets, videoOffsets), mdat(data...))
	}
	return slices.Concat(ftyp, mdat(data...), moov(offsets, videoOffsets))
}

func fragmentedFile(l layout, want []Packet, total int) []byte {
	baseFlags := uint32(0)
	switch l.base {
	case moofBase:
		baseFlags |= tfhdDefaultBaseIs
	case explicitBase:
		baseFlags |= tfhdBaseOffset
	}
	var trex1, trex2 []byte
	if l.defaults == inTrex {
		trex2 = box("trex", full(0, 0, u32(2, 1, 1024, uint32(len(want[0].Data)), 0)))
	} else {
		trex2 = box("trex", full(0, 0, u32(2, 1, 0, 0, 0)))
	}
	audio := track{id: 2, tables: emptyTables}
	if l.delay > 0 {
		version := byte(0)
		if l.wide {
			version = 1
		}
		audio.elst = elst(version, uint64(total)-uint64(l.delay), int64(l.delay))
	}
	var v []byte
	if l.video {
		v = track{id: 1, handler: "vide", entry: box("mp4v", make([]byte, 78)), tables: emptyTables}.trak()
		trex1 = box("trex", full(0, 0, u32(1, 1, 2048, 0, 0)))
	}
	moov := box("moov", mvhd(0), v, audio.trak(), box("mvex", trex1, trex2))
	head := slices.Concat(ftyp, moov)

	// A fragment: its moof, at offset at in the file, and its mdat.
	video := []byte("video")
	fragment := func(k, first, last int, at uint64) []byte {
		var data []byte
		for i := first; i < last; i++ {
			data = append(data, want[i].Data...)
		}
		moof := func(dataAt uint64) []byte {
			// The data of the mdat start at dataAt, those of the audio after
			// the video's.
			audioAt := dataAt
			if l.video {
				audioAt += uint64(len(video))
			}
			var vtraf []byte
			if l.video {
				h := u32(1)
				if l.base == explicitBase {
					h = slices.Concat(h, u64(dataAt))
				}
				run := box("trun", full(0, trunDataOffset|trunSize, u32(1, uint32(dataAt-at)), u32(uint32(len(video)))))
				if l.base == explicitBase {
					run = box("trun", full(0, trunSize, u32(1), u32(uint32(len(video)))))
				}
				vtraf = box("traf", box("tfhd", full(0, baseFlags|tfhdDuration, h, u32(2048))), run)
			}
			h := u32(2)
			if l.base == explicitBase {
				h = slices.Concat(h, u64(audioAt))
			}
			flags := baseFlags
			if l.defaults == inTfhd {
				flags |= tfhdDuration
				h = slices.Concat(h, u32(1024))
			}
			var tfdt []byte
			if !l.noTfdt {
				t := uint64(first) * 1024
				if l.wide {
					tfdt = box("tfdt", full(1, 0, u64(t)))
				} else {
					tfdt = box("tfdt", full(0, 0, u32(uint32(t))))
				}
			}
			// The runs: their fields, and whether the first gives its offset.
			runs := [][2]int{{first, last}}
			if l.twoRuns && last-first > 1 {
				mid := (first + last) / 2
				runs = [][2]int{{first, mid}, {mid, last}}
			}
			var truns []byte
			for j, r := range runs {
				fl := uint32(0)
				if l.defaults != inTrex {
					fl |= trunSize
				}
				if l.defaults == inTrun || r[1] == l.samples {
					fl |= trunDuration // the last sample is shorter
				}
				var offset []byte
				switch {
				case j > 0: // after the run before
				case l.base == moofBase || l.base == implicitBase && !l.video:
					fl |= trunDataOffset
					offset = u32(uint32(audioAt - at))
				}
				var entries []byte
				for i := r[0]; i < r[1]; i++ {
					if fl&trunDuration != 0 {
						entries = append(entries, u32(sampleTicks(i, l.samples))...)
					}
					if fl&trunSize != 0 {
						entries = append(entries, u32(uint32(len(want[i].Data)))...)
					}
				}
				truns = append(truns, box("trun", full(0, fl, u32(uint32(r[1]-r[0])), offset, entries))...)
			}
			traf := box("traf", box("tfhd", full(0, flags, h)), tfdt, truns)
			mfhd := box("mfhd", full(0, 0, u32(uint32(k+1))))
			if l.wide {
				return box64("moof", mfhd, vtraf, traf)
			}
			return box("moof", mfhd, vtraf, traf)
		}
		var vdata []byte
		if l.video {
			vdata = video
		}
		mdat := box("mdat", vdata, data)
		switch {
		case l.toEnd && last == l.samples:
			mdat = boxToEnd("mdat", vdata, data)
		case l.wide:
			mdat = box64("mdat", vdata, data)
		}
		size := uint64(len(moof(0)))
		return slices.Concat(moof(at+size+uint64(len(mdat)-len(vdata)-len(data))), mdat)
	}

	// The fragments, and what the sidx says of them.
	var frags [][]byte
	var sizes, durations []uint32
	var gap []byte
	if l.sidx == sidxGap {
		gap = box("free", []byte("gap"))
	}
	sidxLen := 0
	if l.sidx != noSidx {
		n := (l.samples + l.perChunk - 1) / l.perChunk
		sidxLen = 8 + 4 + 8 + 8 + 4 + 12*n
		if l.wide {
			sidxLen += 8
		}
	}
	at := uint64(len(head) + sidxLen + len(gap))
	for k, first := 0, 0; first < l.samples; k, first = k+1, first+l.perChunk {
		last := min(first+l.perChunk, l.samples)
		var pre []byte
		if l.styp {
			pre = slices.Concat(box("styp", []byte("msdh"), u32(0), []byte("msdhmsix")), box("free", []byte("x")))
		}
		f := slices.Concat(pre, fragment(k, first, last, at+uint64(len(pre))))
		frags = append(frags, f)
		sizes = append(sizes, uint32(len(f)))
		d := 0
		for i := first; i < last; i++ {
			d += int(sampleTicks(i, l.samples))
		}
		if k == 0 && l.sidx != sidxMedia {
			d -= int(l.delay)
		}
		durations = append(durations, uint32(d))
		at += uint64(len(f))
	}
	var sidx []byte
	if l.sidx != noSidx {
		var refs []byte
		for i := range sizes {
			size := sizes[i]
			if l.sidx == sidxBad {
				size++
			}
			refs = append(refs, u32(size, durations[i], 0x90000000)...)
		}
		// The reference_ID and the timescale, the earliest_presentation_time
		// and the first_offset.
		fields := u32(2, 44100)
		if l.wide {
			fields = slices.Concat(fields, u64(0, uint64(len(gap))))
		} else {
			fields = slices.Concat(fields, u32(0, uint32(len(gap))))
		}
		version := byte(0)
		if l.wide {
			version = 1
		}
		sidx = box("sidx", full(version, 0, fields, u16(0, uint16(len(sizes))), refs))
		if len(sidx) != sidxLen {
			panic("sidx of the wrong size")
		}
	}
	return slices.Concat(head, sidx, gap, slices.Concat(frags...))
}

// read returns the packets of a file and the error that ended them.
func read(file []byte) ([]Packet, error) {
	r, err := NewReader(bytes.NewReader(file))
	if err != nil {
		return nil, err
	}
	var packets []Packet
	for {
		p, err := r.ReadPacket()
		if err != nil {
			return packets, err
		}
		packets = append(packets, p)
	}
}

func samePacket(a, b Packet) bool {
	return bytes.Equal(a.Data, b.Data) && a.Time == b.Time && a.Duration == b.Duration
}

func checkPackets(t *testing.T, got, want []Packet) {
	t.Helper()
	for i := range min(len(got), len(want)) {
		if g, w := got[i], want[i]; !samePacket(g, w) {
			t.Fatalf("packet %d: %v, %d bytes, %v; want %v, %d bytes, %v", i, g.Time, len(g.Data), g.Duration, w.Time, len(w.Data), w.Duration)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("%d packets, want %d", len(got), len(want))
	}
}

// layouts are those the tests read every way: progressive and fragmented,
// with each kind of table, box and fragment.
var layouts = []layout{
	{samples: 50, perChunk: 7},
	{samples: 50, perChunk: 7, moovFirst: true, delay: 1024},
	{samples: 50, perChunk: 50, sizes: stszSame, co64: true},
	{samples: 50, perChunk: 1, sizes: stz2Sizes4, video: true, moovFirst: true},
	{samples: 50, perChunk: 9, sizes: stz2Sizes8, wide: true, delay: 2112},
	{samples: 50, perChunk: 9, sizes: stz2Sizes16, video: true, co64: true},
	{samples: 50, perChunk: 9, moovFirst: true, toEnd: true},

	{samples: 50, perChunk: 7, fragmented: true},
	{samples: 50, perChunk: 7, fragmented: true, sidx: sidxAll, delay: 1600},
	{samples: 50, perChunk: 7, fragmented: true, sidx: sidxMedia, delay: 1024, styp: true},
	{samples: 50, perChunk: 7, fragmented: true, sidx: sidxGap, wide: true, twoRuns: true},
	{samples: 50, perChunk: 7, fragmented: true, sidx: sidxBad, noTfdt: true},
	{samples: 50, perChunk: 7, fragmented: true, base: explicitBase, video: true, defaults: inTfhd},
	{samples: 50, perChunk: 7, fragmented: true, base: implicitBase, video: true, twoRuns: true},
	{samples: 50, perChunk: 7, fragmented: true, base: implicitBase, noTfdt: true},
	{samples: 50, perChunk: 7, fragmented: true, defaults: inTrex, sizes: stszSame, toEnd: true},
	{samples: 50, perChunk: 50, fragmented: true, defaults: inTfhd, sidx: sidxAll},
}

func TestReadPackets(t *testing.T) {
	for _, l := range layouts {
		t.Run(fmt.Sprintf("%+v", l), func(t *testing.T) {
			file, want := testFile(l)
			r, err := NewReader(bytes.NewReader(file))
			if err != nil {
				t.Fatal(err)
			}
			tr := r.Track()
			delay := tickTime(int(l.delay))
			if tr.ID != 2 || tr.Codec != "mp4a.40.2" || !bytes.Equal(tr.Config, asc) || tr.CodecDelay != delay ||
				tr.SampleRate != 44100 || tr.Channels != 2 || tr.Timescale != 44100 {
				t.Errorf("track %+v", tr)
			}
			// Fragmented files give their duration by their edit list, or by
			// a sidx that indexes them to their end.
			end := tickTime((l.samples-1)*1024 + 500)
			if d := r.Duration(); d != end && (!l.fragmented || l.delay > 0 || l.sidx == sidxAll || l.sidx == sidxGap || d != 0) {
				t.Errorf("duration %v, want %v", d, end)
			}
			checkPackets(t, readAll(t, r), want)
			if _, err := r.ReadPacket(); err != io.EOF {
				t.Errorf("after the end: %v", err)
			}
		})
	}
}

// seekCase is a Seek of TestSeek, and the sample it must go to.
type seekCase struct {
	at     time.Duration
	sample int
}

// seekCases are those of a file of 400 samples of 1024 ticks.
var seekCases = []seekCase{
	{tickTime(200 * 1024), 200}, {tickTime(200*1024) - 1, 199}, {0, 0}, {-time.Second, 0},
	{tickTime(399 * 1024), 399}, {time.Hour, 399}, {tickTime(150*1024 + 1023), 150}, {tickTime(20*1024 + 1), 20},
	{tickTime(7 * 1024), 7}, {tickTime(6*1024) - 1, 5}, {tickTime(398 * 1024), 398}, {tickTime(100 * 1024), 100},
}

// checkSeek seeks r to c.at, and checks that it reads the packets of want
// from sample c.sample on.
func checkSeek(t *testing.T, r *Reader, want []Packet, c seekCase) {
	t.Helper()
	if err := r.Seek(c.at); err != nil {
		t.Fatalf("Seek(%v): %v", c.at, err)
	}
	for i := c.sample; i < min(c.sample+12, len(want)); i++ {
		p, err := r.ReadPacket()
		if err != nil || !samePacket(p, want[i]) {
			t.Fatalf("Seek(%v), then a packet at %v, of %d bytes, %v long, %v; want packet %d at %v, of %d bytes, %v long",
				c.at, p.Time, len(p.Data), p.Duration, err, i, want[i].Time, len(want[i].Data), want[i].Duration)
		}
	}
	if c.sample+12 >= len(want) {
		if _, err := r.ReadPacket(); err != io.EOF {
			t.Fatalf("Seek(%v), then at the end: %v", c.at, err)
		}
	}
}

func TestSeek(t *testing.T) {
	for _, l := range layouts {
		t.Run(fmt.Sprintf("%+v", l), func(t *testing.T) {
			// 400 samples of 1.5 kB on average, in chunks or fragments of 10:
			// 600 kB, more than the reads.
			l.samples, l.perChunk, l.frameSize = 400, 10, 3000
			if l.sizes == stz2Sizes4 || l.sizes == stz2Sizes8 {
				l.frameSize = 0
			}
			file, want := testFile(l)
			c := &countingReader{r: bytes.NewReader(file)}
			r, err := NewReader(c)
			if err != nil {
				t.Fatal(err)
			}
			if p, err := r.ReadPacket(); err != nil || !samePacket(p, want[0]) {
				t.Fatalf("first packet: %v", err)
			}
			for _, sc := range seekCases {
				seeks := c.seeks
				checkSeek(t, r, want, sc)
				// A seek, of the file or to where the sidx says, and another
				// when the sidx is off, or to look past what it indexes; a scan
				// without a sidx reads the moof boxes.
				limit := 1
				switch {
				case !l.fragmented:
				case l.sidx == sidxAll || l.sidx == sidxGap:
					if sc.at == time.Hour {
						limit = 2
					}
				case l.sidx == sidxMedia:
					limit = 2
				default:
					limit = len(want)
				}
				if n := c.seeks - seeks; n > limit {
					t.Errorf("Seek(%v): %d seeks", sc.at, n)
				}
			}
			// What a scan learnt, the next Seeks know: seeking back reads a
			// fragment at most.
			c.read = 0
			checkSeek(t, r, want, seekCase{tickTime(50 * 1024), 50})
			if c.read > 2*bufSize {
				t.Errorf("seeking back read %d bytes", c.read)
			}
		})
	}
}

// TestSeekReadsLittle checks that a Seek of a fragmented file with a sidx
// reads the moof of the fragment and its samples, not the fragments
// between; and that one of a progressive file reads nothing.
func TestSeekReadsLittle(t *testing.T) {
	for _, l := range []layout{
		{fragmented: true, sidx: sidxAll, delay: 1600},
		{moovFirst: true},
	} {
		l.samples, l.perChunk, l.frameSize = 400, 10, 3000
		file, want := testFile(l)
		c := &countingReader{r: bytes.NewReader(file)}
		r, err := NewReader(c)
		if err != nil {
			t.Fatal(err)
		}
		if c.seeks != 1 || c.read > bufSize {
			t.Errorf("%+v: NewReader: %d seeks, %d bytes read", l, c.seeks, c.read)
		}
		if err := r.Seek(tickTime(300 * 1024)); err != nil {
			t.Fatal(err)
		}
		if l.fragmented && (c.seeks != 2 || c.read > 2*bufSize) || !l.fragmented && (c.seeks != 1 || c.read > bufSize) {
			t.Errorf("%+v: NewReader and a Seek: %d seeks, %d bytes read", l, c.seeks, c.read)
		}
		checkSeek(t, r, want, seekCase{tickTime(300 * 1024), 300})
	}
}

// TestMoovAtEnd checks that a file whose moov comes after its media data,
// as ffmpeg writes them without +faststart, costs a seek to the moov and
// another back.
func TestMoovAtEnd(t *testing.T) {
	file, want := testFile(layout{samples: 400, perChunk: 10, frameSize: 3000})
	c := &countingReader{r: bytes.NewReader(file)}
	r, err := NewReader(c)
	if err != nil {
		t.Fatal(err)
	}
	p, err := r.ReadPacket()
	if err != nil || !samePacket(p, want[0]) {
		t.Fatalf("first packet: %v", err)
	}
	if c.seeks != 3 {
		t.Errorf("%d seeks", c.seeks)
	}
	checkPackets(t, append([]Packet{p}, readAll(t, r)...), want)
}

// shortReader returns at most a few bytes from each Read, as a network
// may.
type shortReader struct {
	r io.ReadSeeker
	n int
}

func (s *shortReader) Read(p []byte) (int, error) {
	s.n++
	return s.r.Read(p[:min(len(p), 1+s.n%7)])
}

func (s *shortReader) Seek(off int64, whence int) (int64, error) { return s.r.Seek(off, whence) }

func TestShortReads(t *testing.T) {
	for _, l := range layouts {
		t.Run(fmt.Sprintf("%+v", l), func(t *testing.T) {
			l.samples = 100
			file, want := testFile(l)
			r, err := NewReader(&shortReader{r: bytes.NewReader(file)})
			if err != nil {
				t.Fatal(err)
			}
			checkPackets(t, readAll(t, r), want)
			for _, at := range []int{60, 3, 99, 0, 1000} {
				i := min(at, 99)
				checkSeek(t, r, want, seekCase{tickTime(at * 1024), i})
			}
		})
	}
}

// flakyReader fails while fail is set, as a network may.
type flakyReader struct {
	r    io.ReadSeeker
	fail bool
}

var errFlaky = errors.New("the network is down")

func (f *flakyReader) Read(p []byte) (int, error) {
	if f.fail {
		return 0, errFlaky
	}
	return f.r.Read(p)
}

func (f *flakyReader) Seek(off int64, whence int) (int64, error) {
	if f.fail {
		return 0, errFlaky
	}
	return f.r.Seek(off, whence)
}

// TestReadErrors checks that the errors of reading reach the caller, and
// that the Reader works again once reading does.
func TestReadErrors(t *testing.T) {
	for _, l := range []layout{
		{samples: 400, perChunk: 10, frameSize: 3000, fragmented: true},
		{samples: 400, perChunk: 10, frameSize: 3000},
	} {
		file, want := testFile(l)
		f := &flakyReader{r: bytes.NewReader(file), fail: true}
		if _, err := NewReader(f); err != errFlaky {
			t.Fatalf("NewReader: %v", err)
		}
		f.fail = false
		r, err := NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		f.fail = true // once what is buffered is read
		var got []Packet
		for {
			p, err := r.ReadPacket()
			if err != nil {
				if err != errFlaky {
					t.Fatalf("after %d packets: %v", len(got), err)
				}
				break
			}
			got = append(got, p)
		}
		checkPackets(t, got, want[:len(got)])
		if _, err := r.ReadPacket(); err != errFlaky {
			t.Errorf("ReadPacket again: %v", err)
		}
		if l.fragmented {
			// Seeking scans the fragments, which reads.
			if err := r.Seek(2 * time.Second); err != errFlaky {
				t.Errorf("Seek: %v", err)
			}
		}
		f.fail = false
		checkSeek(t, r, want, seekCase{tickTime(86 * 1024), 86})
		f.fail = true
		r.Seek(tickTime(350 * 1024))
		if _, err := r.ReadPacket(); err != errFlaky {
			t.Errorf("ReadPacket out of the buffer: %v", err)
		}
		f.fail = false
		checkSeek(t, r, want, seekCase{tickTime(350 * 1024), 350})
	}
}

// TestBoxes reads past boxes that the Reader does not know, uuid boxes,
// and boxes of 64-bit sizes and of size 0.
func TestBoxes(t *testing.T) {
	uuid := slices.Concat(u32(28), []byte("uuid"), make([]byte, 16), []byte("data"))
	data := []byte("abcdef")
	stbl := [][]byte{
		box("stts", full(0, 0, u32(1, 2, 1024))), box("stsc", full(0, 0, u32(1, 1, 2, 1))),
		box("stsz", full(0, 0, u32(3, 2))), box64("stco", full(0, 0, u32(1, 0))), uuid,
	}
	moov := box("moov", uuid, mvhd(0), box("iods", []byte("?")), track{id: 7, tables: stbl}.trak(), box("udta", []byte("meta")))
	at := len(ftyp) + len(uuid) + len(moov) + len(box("free")) + 16
	stbl[3] = box64("stco", full(0, 0, u32(1, uint32(at))))
	moov = box("moov", uuid, mvhd(0), box("iods", []byte("?")), track{id: 7, tables: stbl}.trak(), box("udta", []byte("meta")))
	file := slices.Concat(ftyp, uuid, moov, box("free"), box64("mdat", data))
	got, err := read(file)
	if err != io.EOF {
		t.Fatal(err)
	}
	checkPackets(t, got, []Packet{{Data: data[:3], Duration: tickTime(1024)}, {Data: data[3:], Time: tickTime(1024), Duration: tickTime(1024)}})
}

// TestEditLists reads the priming of edit lists of either version, after
// an empty edit, and the duration of their edits.
func TestEditLists(t *testing.T) {
	stbl := [][]byte{
		box("stts", full(0, 0, u32(1, 4, 1024))), box("stsc", full(0, 0, u32(1, 1, 4, 1))),
		box("stsz", full(0, 0, u32(1, 4))), box("stco", full(0, 0, u32(1, 0))),
	}
	for _, c := range []struct {
		elst     []byte
		delay    int
		duration int // in ticks: 0 for the media's, of 4096
	}{
		{elst(0, 2000, 1024), 1024, 3024},
		{elst(1, 2000, 2112), 2112, 4112},
		{box("elst", full(0, 0, u32(2, 500, 0xFFFFFFFF, 1<<16, 3000, 1600, 1<<16))), 1600, 4600}, // an empty edit first
		{box("elst", full(0, 0, u32(2, 1000, 1024, 1<<16, 1000, 3000, 1<<16))), 1024, 3024},      // two edits
		{box("elst", full(0, 0, u32(1, 0, 1024, 1<<16))), 1024, 0},                               // of no duration
		{box("elst", full(0, 0, u32(3, 1000, 1024, 1<<16, 5000))), 1024, 2024},                   // shorter than it says
		{box("elst", full(0, 0, u32(0))), 0, 0},
	} {
		moov := box("moov", mvhd(0), track{id: 1, media: 4096, elst: c.elst, tables: stbl}.trak())
		r, err := NewReader(bytes.NewReader(slices.Concat(ftyp, moov, box("mdat", []byte("abcd")))))
		if err != nil {
			t.Fatalf("% x: %v", c.elst, err)
		}
		duration := c.duration
		if duration == 0 {
			duration = 4096
		}
		if r.Track().CodecDelay != tickTime(c.delay) || r.Duration() != tickTime(duration) {
			t.Errorf("% x: CodecDelay %v, duration %v; want %v, %v", c.elst, r.Track().CodecDelay, r.Duration(), tickTime(c.delay), tickTime(duration))
		}
	}
}

// TestTrackEntries reads the codecs and configurations of sample entries
// of each kind.
func TestTrackEntries(t *testing.T) {
	heaac := []byte{0x2B, 0x92, 0x08, 0x00}
	longDesc := func(tag byte, data ...[]byte) []byte {
		d := slices.Concat(data...)
		return slices.Concat([]byte{tag, 0x80, 0x80, 0x80, byte(len(d))}, d)
	}
	// An ES_Descriptor of every flag, of 4-byte sizes as ffmpeg writes them.
	flagged := box("esds", full(0, 0, longDesc(tagES, u16(1), []byte{0xE0}, u16(9), []byte{3}, []byte("url"), u16(4),
		longDesc(tagDecoderConfig, []byte{0x40, 0x15, 0, 0, 0}, u32(0, 0), longDesc(tagDecoderInfo, heaac)))))
	quicktime1 := box("mp4a", make([]byte, 6), u16(1), u16(1, 0), u32(0), u16(2, 16, 0xFFFE, 0), u32(44100<<16), make([]byte, 16),
		box("wave", box("frma", []byte("mp4a")), esdsBox(0x40, asc)))
	quicktime2 := box("mp4a", make([]byte, 6), u16(1), u16(2, 0), u32(0), u16(3, 16, 0xFFFE, 0), u32(1<<16),
		u32(72), u64(0x40E7700000000000), u32(6), u32(0x7F000000, 16, 0, 0, 1024), esdsBox(0x40, []byte{0x11, 0x80})) // 48 kHz, configuration 0
	for _, c := range []struct {
		name     string
		entry    []byte
		codec    string
		config   []byte
		rate     int
		channels int
	}{
		{"AAC-LC", mp4aBox(2, 44100, esdsBox(0x40, asc)), "mp4a.40.2", asc, 44100, 2},
		{"HE-AAC", mp4aBox(2, 44100, box("btrt", u32(0, 0, 0)), flagged), "mp4a.40.5", heaac, 44100, 2},
		{"MP3", mp4aBox(2, 44100, esdsBox(0x6B, nil)), "mp4a.6B", nil, 44100, 2},
		{"MPEG-2 AAC", mp4aBox(6, 32000, esdsBox(0x67, []byte{0x13, 0x88})), "mp4a.67", []byte{0x13, 0x88}, 22050, 1},
		{"no esds", mp4aBox(2, 32000), "mp4a", nil, 32000, 2},
		{"QuickTime version 1", quicktime1, "mp4a.40.2", asc, 44100, 2},
		{"QuickTime version 2", quicktime2, "mp4a.40.2", []byte{0x11, 0x80}, 48000, 6},
		{"Opus", box("Opus", make([]byte, 6), u16(1), make([]byte, 8), u16(2, 16, 0, 0), u32(48000<<16), box("dOps", []byte("x"))), "Opus", nil, 48000, 2},
	} {
		moov := box("moov", mvhd(0), track{id: 1, entry: c.entry, tables: emptyTables}.trak())
		r, err := NewReader(bytes.NewReader(slices.Concat(ftyp, moov)))
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		tr := r.Track()
		if tr.Codec != c.codec || !bytes.Equal(tr.Config, c.config) || tr.SampleRate != c.rate || tr.Channels != c.channels {
			t.Errorf("%s: %+v", c.name, tr)
		}
		if _, err := r.ReadPacket(); err != io.EOF {
			t.Errorf("%s: a packet: %v", c.name, err)
		}
	}
}

func TestUnreadableFiles(t *testing.T) {
	audio := track{id: 1, tables: emptyTables}.trak()
	moov := box("moov", mvhd(0), audio)
	for _, c := range []struct {
		name string
		file []byte
		want error
	}{
		{"empty", nil, ErrFormat},
		{"text", []byte("not an MP4 file at all"), ErrFormat},
		{"WebM", []byte{0x1A, 0x45, 0xDF, 0xA3, 0x9F, 0x42, 0x86, 0x81, 0x01, 0x42, 0xF7, 0x81}, ErrFormat},
		{"no moov", slices.Concat(ftyp, box("mdat", []byte("data"))), ErrFormat},
		{"a moov after an mdat to the end", slices.Concat(ftyp, boxToEnd("mdat", moov)), ErrFormat},
		{"video only", slices.Concat(ftyp, box("moov", mvhd(0), track{id: 1, handler: "vide", tables: emptyTables}.trak())), ErrNoAudio},
		{"encrypted", slices.Concat(ftyp, box("moov", track{id: 1, entry: box("enca", make([]byte, 28)), tables: emptyTables}.trak())), errors.ErrUnsupported},
		{"no timescale", slices.Concat(ftyp, bytes.Replace(moov, u32(0, 0, 44100, 0), u32(0, 0, 0, 0), 2)), ErrFormat},
		{"no sample table", slices.Concat(ftyp, box("moov", box("trak", box("mdia", box("mdhd", full(0, 0, u32(0, 0, 44100, 0, 0))),
			box("hdlr", full(0, 0, u32(0), []byte("soun"))))))), ErrFormat},
		{"no sample entry", slices.Concat(ftyp, box("moov", track{id: 1, entry: []byte{}, tables: emptyTables}.trak())), ErrFormat},
		{"a short sample entry", slices.Concat(ftyp, box("moov", track{id: 1, entry: box("mp4a", make([]byte, 20)), tables: emptyTables}.trak())), ErrFormat},
		{"an esds without DecoderConfigDescriptor", slices.Concat(ftyp, box("moov", track{id: 1,
			entry: mp4aBox(2, 44100, box("esds", full(0, 0, desc(tagES, u16(1), []byte{0})))), tables: emptyTables}.trak())), ErrFormat},
		{"an incomplete sample table", slices.Concat(ftyp, box("moov", track{id: 1, tables: [][]byte{box("stsz", full(0, 0, u32(5, 3)))}}.trak())), ErrFormat},
		{"a trak larger than the moov", slices.Concat(ftyp, box("moov", audio[:40])), ErrFormat},
		{"a moov larger than the file", slices.Concat(ftyp, u32(4000), []byte("moov"), audio), io.ErrUnexpectedEOF},
	} {
		if _, err := NewReader(bytes.NewReader(c.file)); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
}

// TestBadTables reads sample tables that disagree, or that are corrupt.
func TestBadTables(t *testing.T) {
	data := []byte("abcdefghij")
	open := func(tables ...[]byte) (*Reader, error) {
		at := uint32(len(ftyp) + 8)
		for i, b := range tables {
			tables[i] = bytes.ReplaceAll(b, []byte("@@@@"), u32(at))
		}
		moov := box("moov", mvhd(0), track{id: 1, tables: tables}.trak())
		return NewReader(bytes.NewReader(slices.Concat(ftyp, box("mdat", data), moov)))
	}
	stts := box("stts", full(0, 0, u32(1, 10, 1024)))
	stsc := box("stsc", full(0, 0, u32(1, 1, 10, 1)))
	stsz := box("stsz", full(0, 0, u32(1, 10)))
	stco := box("stco", full(0, 0, u32(1), []byte("@@@@")))
	for _, c := range []struct {
		name    string
		tables  [][]byte
		samples int
	}{
		{"agreeing", [][]byte{stts, stsc, stsz, stco}, 10},
		{"fewer times", [][]byte{box("stts", full(0, 0, u32(1, 6, 1024))), stsc, stsz, stco}, 6},
		{"fewer sizes", [][]byte{stts, stsc, box("stsz", full(0, 0, u32(1, 4))), stco}, 4},
		{"sizes shorter than they say", [][]byte{stts, stsc, box("stsz", full(0, 0, u32(0, 10, 1, 1, 1))), stco}, 3},
		{"fewer chunks", [][]byte{stts, box("stsc", full(0, 0, u32(1, 1, 3, 1))), stsz, box("stco", full(0, 0, u32(2), []byte("@@@@")))}, 3},
		{"a stsc past the chunks", [][]byte{stts, box("stsc", full(0, 0, u32(2, 1, 5, 1, 3, 1, 1))), stsz, stco}, 5},
		{"a stts shorter than it says", [][]byte{box("stts", full(0, 0, u32(3, 4, 1024))), stsc, stsz, stco}, 4},
		{"chunks of no samples", [][]byte{stts, box("stsc", full(0, 0, u32(3, 1, 0, 1, 3, 10, 1, 4, 0, 1))),
			stsz, box("stco", full(0, 0, u32(4), u32(0), u32(0), []byte("@@@@"), u32(0)))}, 10},
	} {
		r, err := open(slices.Clone(c.tables)...)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		got := readAll(t, r)
		if len(got) != c.samples {
			t.Errorf("%s: %d packets, want %d", c.name, len(got), c.samples)
		}
		for i, p := range got {
			if string(p.Data) != string(data[i]) || p.Time != tickTime(i*1024) {
				t.Errorf("%s: packet %d: %q at %v", c.name, i, p.Data, p.Time)
			}
		}
		r.Seek(time.Hour)
		if p, err := r.ReadPacket(); err != nil || p.Time != tickTime((c.samples-1)*1024) {
			t.Errorf("%s: Seek to the end, then %v, %v", c.name, p.Time, err)
		}
	}
	for _, c := range []struct {
		name   string
		tables [][]byte
	}{
		{"an stsc of chunk 0", [][]byte{stts, box("stsc", full(0, 0, u32(1, 0, 10, 1))), stsz, stco}},
		{"an stsc going back", [][]byte{stts, box("stsc", full(0, 0, u32(2, 1, 5, 1, 1, 5, 1))), stsz,
			box("stco", full(0, 0, u32(2), []byte("@@@@"), u32(0)))}},
		{"no stsc entry", [][]byte{stts, box("stsc", full(0, 0, u32(0))), stsz, stco}},
		{"a stz2 of 12-bit sizes", [][]byte{stts, stsc, box("stz2", full(0, 0, []byte{0, 0, 0, 12}, u32(10), make([]byte, 15))), stco}},
		{"samples of 4 GB", [][]byte{stts, stsc, box("stsz", full(0, 0, u32(1<<32-1, 10))), stco}},
		{"times past 34 years", [][]byte{box("stts", full(0, 0, u32(1, 1<<20, 1<<32-1))), box("stsc", full(0, 0, u32(1, 1, 1<<20, 1))),
			box("stsz", full(0, 0, u32(1, 1<<20))), stco}},
		{"a chunk at 2⁶⁴ - 1", [][]byte{stts, stsc, stsz, box("co64", full(0, 0, u32(1), u64(1<<64-1)))}},
	} {
		if _, err := open(slices.Clone(c.tables)...); !errors.Is(err, ErrFormat) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}

// TestBadFragments reads fragments that are corrupt, or that the Reader
// skips.
func TestBadFragments(t *testing.T) {
	head := slices.Concat(ftyp, box("moov", mvhd(0), track{id: 1, tables: emptyTables}.trak(), box("mvex", box("trex", full(0, 0, u32(1, 1, 1024, 0, 0))))))
	data := []byte("abcdefgh")
	// moof returns a moof of one traf, of the given boxes, and its mdat.
	moof := func(boxes ...[]byte) []byte {
		return slices.Concat(box("moof", box("traf", boxes...)), box("mdat", data))
	}
	tfhd := box("tfhd", full(0, tfhdDefaultBaseIs, u32(1)))
	trun := func(n uint32, sizes ...uint32) []byte {
		size := 8 + 8 + len(tfhd) + 8 + 12 + 4*len(sizes) + 8 // the data of the mdat follow the moof
		return box("trun", full(0, trunDataOffset|trunSize, u32(n, uint32(size)), u32(sizes...)))
	}
	for _, c := range []struct {
		name string
		file []byte
		want []string // the packets read
		err  error
	}{
		{"good", slices.Concat(head, moof(tfhd, trun(2, 3, 5))), []string{"abc", "defgh"}, io.EOF},
		{"of another track", slices.Concat(head, moof(box("tfhd", full(0, tfhdDefaultBaseIs, u32(2))), trun(2, 3, 5)),
			moof(tfhd, trun(1, 8))), []string{"abcdefgh"}, io.EOF},
		{"empty", slices.Concat(head, box("moof"), moof(tfhd, trun(0)), moof(tfhd, trun(1, 2))), []string{"ab"}, io.EOF},
		{"a traf without tfhd", slices.Concat(head, moof(trun(2, 3, 5))), nil, ErrFormat},
		{"a trun of more samples than it holds", slices.Concat(head, moof(tfhd, trun(3, 3, 5))), nil, ErrFormat},
		{"a sample past the file", slices.Concat(head, moof(tfhd, trun(2, 3, 50))), []string{"abc"}, io.ErrUnexpectedEOF},
		{"a sample of 4 GB", slices.Concat(head, moof(tfhd, trun(2, 3, 1<<32-1))), []string{"abc"}, ErrFormat},
		{"a trun before the file", slices.Concat(head, moof(tfhd, box("trun", full(0, trunDataOffset, u32(1), u32(1<<31))))), nil, ErrFormat},
		{"a base past 2⁴⁸", slices.Concat(head, moof(box("tfhd", full(0, tfhdBaseOffset, u32(1), u64(1<<50))), trun(1, 1))), nil, ErrFormat},
		{"a tfdt past 34 years", slices.Concat(head, moof(tfhd, box("tfdt", full(1, 0, u64(1<<62))), trun(1, 1))), nil, ErrFormat},
		{"samples of 4 GB by default", slices.Concat(head, moof(box("tfhd", full(0, tfhdDefaultBaseIs|tfhdSize, u32(1), u32(1<<31))),
			box("trun", full(0, 0, u32(10))))), nil, ErrFormat},
		{"2³² samples of no data", slices.Concat(head, moof(tfhd, box("trun", full(0, 0, u32(1<<32-1))))), make([]string, 1000), nil},
		{"a moof larger than the file", slices.Concat(head, u32(1000), []byte("moof"), box("traf", tfhd)), nil, io.ErrUnexpectedEOF},
		{"a file chained after", slices.Concat(head, moof(tfhd, trun(1, 1)), head, moof(tfhd, trun(1, 2))), []string{"a"}, io.EOF},
	} {
		r, err := NewReader(bytes.NewReader(c.file))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		var got []string
		for len(got) < 1000 {
			var p Packet
			if p, err = r.ReadPacket(); err != nil {
				break
			}
			got = append(got, string(p.Data))
		}
		if !slices.Equal(got, c.want) || !errors.Is(err, c.err) {
			t.Errorf("%s: %d packets %.40q, then %v; want %.40q, %v", c.name, len(got), got, err, c.want, c.err)
		}
	}
}

// TestHybrid reads a fragmented file whose moov describes the first
// samples, in the mdat after it, as ffmpeg writes with frag_keyframe
// alone.
func TestHybrid(t *testing.T) {
	data := []byte("abcdefgh")
	stbl := [][]byte{
		box("stts", full(0, 0, u32(1, 3, 1024))), box("stsc", full(0, 0, u32(1, 1, 3, 1))),
		box("stsz", full(0, 0, u32(1, 3))), box("stco", full(0, 0, u32(1, 0))),
	}
	mvex := box("mvex", box("trex", full(0, 0, u32(1, 1, 1024, 1, 0))))
	moov := box("moov", mvhd(0), track{id: 1, tables: stbl}.trak(), mvex)
	stbl[3] = box("stco", full(0, 0, u32(1, uint32(len(ftyp)+len(moov)+8))))
	moov = box("moov", mvhd(0), track{id: 1, tables: stbl}.trak(), mvex)
	traf := box("traf", box("tfhd", full(0, tfhdDefaultBaseIs, u32(1))), box("trun", full(0, trunDataOffset, u32(5, 0))))
	n := len(box("moof", traf)) + 8
	traf = box("traf", box("tfhd", full(0, tfhdDefaultBaseIs, u32(1))), box("trun", full(0, trunDataOffset, u32(5, uint32(n)))))
	file := slices.Concat(ftyp, moov, box("mdat", data[:3]), box("moof", traf), box("mdat", data[3:]))
	var want []Packet
	for i := range data {
		want = append(want, Packet{Data: data[i : i+1], Time: tickTime(i * 1024), Duration: tickTime(1024)})
	}
	got, err := read(file)
	if err != io.EOF {
		t.Fatal(err)
	}
	checkPackets(t, got, want)
	r, err := NewReader(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range []int{6, 1, 3, 2, 7, 0} {
		checkSeek(t, r, want, seekCase{tickTime(i * 1024), i})
	}
}

// TestLargeSizes reads sizes larger than the file, or than anything in it
// can be, as errors, without allocating what they say.
func TestLargeSizes(t *testing.T) {
	audio := track{id: 1, tables: emptyTables}.trak()
	head := slices.Concat(ftyp, box("moov", mvhd(0), audio, box("mvex", box("trex", full(0, 0, u32(1, 1, 1024, 0, 0))))))
	tfhd := box("tfhd", full(0, tfhdDefaultBaseIs, u32(1)))
	for _, c := range []struct {
		name string
		file []byte
		want error // from NewReader, or else ReadPacket
	}{
		{"a moov of 15 MB, of a file of less", slices.Concat(ftyp, u32(15<<20), []byte("moov"), audio), io.ErrUnexpectedEOF},
		{"a moov of 1 GB", slices.Concat(ftyp, u32(1<<30), []byte("moov"), audio), ErrFormat},
		{"a moov of 2⁶² bytes", slices.Concat(ftyp, u32(1), []byte("moov"), u64(1<<62), audio), ErrFormat},
		{"a moof of 15 MB", slices.Concat(head, u32(15<<20), []byte("moof"), box("traf", tfhd)), io.ErrUnexpectedEOF},
		{"a moof of 1 GB", slices.Concat(head, u32(1<<30), []byte("moof"), box("traf", tfhd)), ErrFormat},
		{"a sample of 15 MB", slices.Concat(head, box("moof", box("traf", tfhd, box("trun", full(0, trunSize, u32(1, 15<<20)))))), io.ErrUnexpectedEOF},
		{"a trun of 2³² samples", slices.Concat(head, box("moof", box("traf", tfhd, box("trun", full(0, trunSize, u32(1<<32-1, 1)))))), ErrFormat},
		{"a stsz of 2³² samples", slices.Concat(ftyp, box("moov", track{id: 1, tables: [][]byte{
			box("stts", full(0, 0, u32(1, 1<<32-1, 1))), box("stsc", full(0, 0, u32(1, 1, 1<<32-1, 1))),
			box("stsz", full(0, 0, u32(1, 1<<32-1))), box("stco", full(0, 0, u32(1<<32-1, 0)))}}.trak())), io.ErrUnexpectedEOF},
		{"a sidx of 65535 fragments", slices.Concat(head, box("sidx", full(0, 0, u32(1, 44100, 0, 0), u16(0, 0xFFFF)))), io.EOF},
		{"an elst of 2³² edits", slices.Concat(ftyp, box("moov", track{id: 1, elst: box("elst", full(0, 0, u32(1<<32-1))), tables: emptyTables}.trak())), io.EOF},
	} {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		r, err := NewReader(bytes.NewReader(c.file))
		if err == nil {
			_, err = r.ReadPacket()
		}
		if err == nil {
			// The samples of a stsz of 2³² samples, of 1 byte each.
			for range 1000 {
				if _, err = r.ReadPacket(); err != nil {
					break
				}
			}
		}
		runtime.ReadMemStats(&after)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
		if n := after.TotalAlloc - before.TotalAlloc; n > 1<<20 {
			t.Errorf("%s: allocated %d bytes", c.name, n)
		}
	}
}

// TestTruncated reads every beginning of files, which must give the
// packets they hold whole, then an error: io.EOF only once all the packets
// are read, as when the file ends between fragments, or no packets when
// the file ends before its moov does.
func TestTruncated(t *testing.T) {
	for _, l := range []layout{
		{samples: 12, perChunk: 5, moovFirst: true, video: true},
		{samples: 12, perChunk: 5, fragmented: true, sidx: sidxAll, video: true, delay: 1024},
	} {
		file, want := testFile(l)
		for n := range len(file) {
			r, err := NewReader(bytes.NewReader(file[:n]))
			if err != nil {
				continue
			}
			var got []Packet
			for len(got) <= len(want) {
				var p Packet
				if p, err = r.ReadPacket(); err != nil {
					break
				}
				got = append(got, p)
			}
			if err == nil || err == io.EOF && len(got) != len(want) && !l.fragmented {
				t.Fatalf("%+v: cut at %d of %d bytes: %d packets, then %v", l, n, len(file), len(got), err)
			}
			if len(got) > len(want) || !slices.EqualFunc(got, want[:len(got)], samePacket) {
				t.Fatalf("%+v: cut at %d of %d bytes: wrong packets", l, n, len(file))
			}
			if _, again := r.ReadPacket(); again != err {
				t.Fatalf("%+v: cut at %d: %v, then %v", l, n, err, again)
			}
			// Seeking still works within what is there.
			if err := r.Seek(tickTime(5 * 1024)); err != nil {
				t.Fatalf("%+v: cut at %d: Seek: %v", l, n, err)
			}
			if p, err := r.ReadPacket(); err == nil && !slices.ContainsFunc(want, func(w Packet) bool { return samePacket(p, w) }) {
				t.Fatalf("%+v: cut at %d: Seek, then a packet at %v", l, n, p.Time)
			}
		}
	}
}

// exercise reads a file every way a Reader does, to find panics and
// endless loops.
func exercise(file []byte) {
	r, err := NewReader(bytes.NewReader(file))
	if err != nil {
		return
	}
	r.Track()
	r.Duration()
	for _, at := range []time.Duration{0, 150 * time.Millisecond, time.Hour, 50 * time.Millisecond, -time.Second} {
		for range 1000 {
			if _, err := r.ReadPacket(); err != nil {
				break
			}
		}
		r.Seek(at)
	}
}

// TestCorrupt changes each byte of files, to values that most often make
// sense to a reader: no change may make it panic or loop.
func TestCorrupt(t *testing.T) {
	for _, l := range []layout{
		{samples: 12, perChunk: 5, moovFirst: true, video: true, delay: 1024},
		{samples: 12, perChunk: 5, sizes: stz2Sizes4, co64: true},
		{samples: 12, perChunk: 5, fragmented: true, sidx: sidxAll, video: true, delay: 1024, twoRuns: true},
		{samples: 12, perChunk: 5, fragmented: true, base: implicitBase, video: true, noTfdt: true},
		{samples: 12, perChunk: 5, fragmented: true, defaults: inTrex, sizes: stszSame, wide: true},
	} {
		file, _ := testFile(l)
		for i := range file {
			for _, v := range []byte{0x00, 0xFF, 0x01, 0x80, file[i] ^ 0x40, file[i] + 1} {
				b := bytes.Clone(file)
				b[i] = v
				exercise(b)
			}
		}
	}
}

func FuzzReader(f *testing.F) {
	for _, l := range []layout{
		{samples: 6, perChunk: 4, moovFirst: true, video: true, delay: 1024},
		{samples: 6, perChunk: 4, sizes: stz2Sizes4, co64: true, wide: true},
		{samples: 6, perChunk: 4, fragmented: true, sidx: sidxAll, video: true, delay: 1024, twoRuns: true},
		{samples: 6, perChunk: 4, fragmented: true, base: implicitBase, video: true, noTfdt: true},
		{samples: 6, perChunk: 4, fragmented: true, defaults: inTrex, sizes: stszSame},
	} {
		file, _ := testFile(l)
		f.Add(file)
	}
	for _, name := range []string{"faststart.m4a", "sidx.m4a"} {
		b, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b[:min(len(b), 4096)])
	}
	f.Fuzz(func(t *testing.T, b []byte) { exercise(b) })
}
