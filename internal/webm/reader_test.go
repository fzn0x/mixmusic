package webm

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

// layout says how testFile lays a file out.
type layout struct {
	frames     int // of 20 ms, in clusters of perCluster
	perCluster int
	frameSize  int // the size of the largest frames: 40 when 0
	sizeLen    int // the length of the sizes of the Segment, Clusters and blocks, at least
	cues       int // noCues, cuesFront (as YouTube) or cuesEnd (as ffmpeg)
	video      bool
	badCues    bool // Cues that point one byte after the clusters
	unknownSeg bool // a Segment of unknown size
	unknownCl  bool // Clusters of unknown size
}

const (
	noCues = iota
	cuesFront
	cuesEnd
)

// frame returns the data of frame i of a test file: of varied sizes, and
// different from that of the frames around it.
func frame(i, size int) []byte {
	if size == 0 {
		size = 40
	}
	b := make([]byte, 1+i*37%size)
	for j := range b {
		b[j] = byte(i + j)
	}
	return b
}

// testFile returns a file laid out as l, of a SeekHead, Info, Tracks,
// clusters and Cues, and the packets of its Opus track 2. Its Tracks have
// a video track 1 too, whose blocks come before the Opus ones when
// l.video.
func testFile(l layout) ([]byte, []Packet) {
	var want []Packet
	var clusters [][]byte
	var times []uint64
	for first := 0; first < l.frames; first += l.perCluster {
		ts := uint64(first * 20)
		children := [][]byte{uintEl(idTimestamp, ts)}
		for i := first; i < min(first+l.perCluster, l.frames); i++ {
			rel := int16(i*20 - int(ts))
			if l.video {
				children = append(children, elSize(idSimpleBlock, l.sizeLen, block(1, rel, 0x80, []byte("video"))))
			}
			f := frame(i, l.frameSize)
			children = append(children, elSize(idSimpleBlock, l.sizeLen, block(2, rel, 0x80, f)))
			want = append(want, Packet{Data: f, Time: time.Duration(i) * 20 * time.Millisecond, Keyframe: true})
		}
		if l.unknownCl {
			clusters = append(clusters, elUnknown(idCluster, children...))
		} else {
			clusters = append(clusters, elSize(idCluster, l.sizeLen, children...))
		}
		times = append(times, ts)
	}
	info := infoEl(1_000_000, float64(l.frames*20))
	tracks := el(idTracks, videoTrack(1), opusTrack(2))
	cues := func(clusterPos []uint64) []byte {
		var points [][]byte
		for i, ts := range times {
			p := clusterPos[i]
			if l.badCues {
				p++
			}
			points = append(points, el(idCuePoint, uintEl(idCueTime, ts),
				el(idCueTrackPositions, uintEl(idCueTrack, 2), el(idCueClusterPosition, pos(p)))))
		}
		return el(idCues, points...)
	}
	index := func(info, tracks, cues uint64) []byte {
		if l.cues == noCues {
			return seekHead(idInfo, info, idTracks, tracks)
		}
		return seekHead(idInfo, info, idTracks, tracks, idCues, cues)
	}

	// The positions take 8 bytes, so that the sizes of the SeekHead and the
	// Cues do not depend on them: lay out with zeros first.
	clusterPos := make([]uint64, len(clusters))
	children := [][]byte{index(0, 0, 0), info, tracks}
	if l.cues == cuesFront {
		children = append(children, cues(clusterPos))
	}
	children = append(children, clusters...)
	offs := offsets(children...)
	copy(clusterPos, offs[len(offs)-len(clusters):])
	end := offs[len(offs)-1] + uint64(len(children[len(children)-1]))
	cuesPos := end
	if l.cues == cuesFront {
		cuesPos = offs[3]
		children[3] = cues(clusterPos)
	}
	children[0] = index(offs[1], offs[2], cuesPos)
	if l.cues == cuesEnd {
		children = append(children, cues(clusterPos))
	}
	segment := elSize(idSegment, l.sizeLen, children...)
	if l.unknownSeg {
		segment = elUnknown(idSegment, children...)
	}
	return slices.Concat(ebmlHeader("webm"), segment), want
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
	return bytes.Equal(a.Data, b.Data) && a.Time == b.Time && a.Duration == b.Duration &&
		a.DiscardPadding == b.DiscardPadding && a.Keyframe == b.Keyframe
}

func checkPackets(t *testing.T, got, want []Packet) {
	t.Helper()
	for i := range min(len(got), len(want)) {
		if g, w := got[i], want[i]; !samePacket(g, w) {
			t.Fatalf("packet %d: %v, %d bytes, %v, %v, keyframe %v; want %v, %d bytes, %v, %v, keyframe %v", i,
				g.Time, len(g.Data), g.Duration, g.DiscardPadding, g.Keyframe, w.Time, len(w.Data), w.Duration, w.DiscardPadding, w.Keyframe)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("%d packets, want %d", len(got), len(want))
	}
}

func TestReadPackets(t *testing.T) {
	file, want := testFile(layout{frames: 50, perCluster: 10, cues: cuesEnd, video: true})
	r, err := NewReader(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	tr := r.Track()
	if tr.Number != 2 || tr.Codec != "A_OPUS" || !bytes.Equal(tr.CodecPrivate, opusHead) || tr.CodecDelay != 6500*time.Microsecond ||
		tr.SeekPreRoll != 80*time.Millisecond || tr.SampleRate != 48000 || tr.Channels != 2 {
		t.Errorf("track %+v", tr)
	}
	if r.Duration() != time.Second {
		t.Errorf("duration %v", r.Duration())
	}
	checkPackets(t, readAll(t, r), want)
	if _, err := r.ReadPacket(); err != io.EOF {
		t.Errorf("after the end: %v", err)
	}
}

// TestSizeLengths reads sizes of every length, from 1 to 8 bytes. IDs of
// every length, from 1 to 4 bytes, are in every file.
func TestSizeLengths(t *testing.T) {
	for n := 1; n <= 8; n++ {
		file, want := testFile(layout{frames: 30, perCluster: 7, sizeLen: n, cues: cuesFront, video: true})
		got, err := read(file)
		if err != io.EOF {
			t.Fatalf("sizes of %d bytes: %v", n, err)
		}
		checkPackets(t, got, want)
	}
}

func TestUnknownSizes(t *testing.T) {
	for _, l := range []layout{
		{unknownCl: true},
		{unknownCl: true, cues: cuesEnd},
		{unknownSeg: true},
		{unknownSeg: true, unknownCl: true},
		{unknownSeg: true, unknownCl: true, cues: cuesEnd},
	} {
		l.frames, l.perCluster, l.video = 40, 8, true
		file, want := testFile(l)
		got, err := read(file)
		if err != io.EOF {
			t.Fatalf("%+v: %v", l, err)
		}
		checkPackets(t, got, want)
	}
}

// xiphLace laces frames as Xiph does: each size but the last's in a run
// of 255s ended by a smaller byte.
func xiphLace(frames ...[]byte) []byte {
	b := []byte{byte(len(frames) - 1)}
	for _, f := range frames[:len(frames)-1] {
		n := len(f)
		for ; n >= 255; n -= 255 {
			b = append(b, 255)
		}
		b = append(b, byte(n))
	}
	return slices.Concat(b, slices.Concat(frames...))
}

// ebmlLace laces frames as EBML does: the first size as a vint, the next
// as signed differences.
func ebmlLace(frames ...[]byte) []byte {
	b := []byte{byte(len(frames) - 1)}
	for i, f := range frames[:len(frames)-1] {
		if i == 0 {
			b = append(b, vint(uint64(len(f)), sizeLen(len(f)))...)
			continue
		}
		d := int64(len(f) - len(frames[i-1]))
		n := 1
		for bias := int64(63); d < -bias || d > bias; bias = 1<<(7*n-1) - 1 {
			n++
		}
		b = append(b, vint(uint64(d+1<<(7*n-1)-1), n)...)
	}
	return slices.Concat(b, slices.Concat(frames...))
}

func fixedLace(frames ...[]byte) []byte {
	return slices.Concat([]byte{byte(len(frames) - 1)}, slices.Concat(frames...))
}

// sized returns frames of the given sizes.
func sized(sizes ...int) [][]byte {
	var frames [][]byte
	for i, n := range sizes {
		frames = append(frames, bytes.Repeat([]byte{byte(i + 1)}, n))
	}
	return frames
}

// simpleFile returns a file of an Opus track 1 and the given clusters.
func simpleFile(clusters ...[]byte) []byte {
	return slices.Concat(ebmlHeader("webm"), el(idSegment, slices.Concat(
		[][]byte{infoEl(1_000_000, 0), el(idTracks, opusTrack(1))}, clusters)...))
}

func TestLacing(t *testing.T) {
	xiph := sized(1, 300, 510, 0, 7)
	ebml := sized(10, 200, 5000, 3, 70003, 9)
	fixed := sized(25, 25, 25, 25)
	group := sized(6, 9, 4)
	ms := time.Millisecond
	file := simpleFile(el(idCluster, uintEl(idTimestamp, 1000),
		el(idSimpleBlock, block(1, 0, 0x80|lacingXiph<<1, xiphLace(xiph...))),
		el(idSimpleBlock, block(1, 20, 0x80|lacingEBML<<1, ebmlLace(ebml...))),
		el(idSimpleBlock, block(1, 40, 0x80|lacingFixed<<1, fixedLace(fixed...))),
		el(idSimpleBlock, block(1, 60, 0x80|lacingXiph<<1, xiphLace(sized(5)...))),
		el(idSimpleBlock, block(1, 80, 0x80|lacingEBML<<1, ebmlLace(sized(6)...))),
		el(idSimpleBlock, block(1, 100, 0x80|lacingFixed<<1, fixedLace(sized(7)...))),
		el(idBlockGroup, el(idBlock, block(1, 120, lacingEBML<<1, ebmlLace(group...))),
			uintEl(idBlockDuration, 60), intEl(idDiscardPadding, int64(3*ms))),
	))
	var want []Packet
	add := func(at time.Duration, frames [][]byte) {
		for _, f := range frames {
			want = append(want, Packet{Data: f, Time: at, Keyframe: true})
		}
	}
	add(1000*ms, xiph)
	add(1020*ms, ebml)
	add(1040*ms, fixed)
	add(1060*ms, sized(5))
	add(1080*ms, sized(6))
	add(1100*ms, sized(7))
	add(1120*ms, group)
	want[len(want)-1].DiscardPadding = 3 * ms // of the last frame
	got, err := read(file)
	if err != io.EOF {
		t.Fatal(err)
	}
	checkPackets(t, got, want)
}

func TestLacingErrors(t *testing.T) {
	for _, c := range []struct {
		lacing byte
		data   []byte
	}{
		{lacingXiph, nil},
		{lacingXiph, []byte{1, 255, 255}},            // a size that runs past the data
		{lacingXiph, []byte{1, 10, 1, 2}},            // a frame of 10 bytes of 2
		{lacingEBML, []byte{2, 0x85, 0x80, 1, 2, 3}}, // 5 bytes, then 5 - 63
		{lacingEBML, []byte{1, 0x00, 1}},             // an invalid size
		{lacingEBML, []byte{1, 0x88, 1, 2, 3}},       // 8 bytes of 3
		{lacingFixed, []byte{2, 1, 2, 3, 4}},         // 4 bytes in 3 frames
		{lacingFixed, nil},
	} {
		if frames, err := unlace(c.lacing, c.data); !errors.Is(err, ErrFormat) {
			t.Errorf("lacing %d of % x: %d frames, %v", c.lacing, c.data, len(frames), err)
		}
	}
}

func TestBlockGroups(t *testing.T) {
	ms := time.Millisecond
	file := simpleFile(el(idCluster, uintEl(idTimestamp, 0),
		el(idBlockGroup, el(idBlock, block(1, 0, 0, []byte("a"))), uintEl(idBlockDuration, 20), intEl(idDiscardPadding, int64(ms))),
		el(idBlockGroup, el(idBlock, block(1, 20, 0, []byte("b"))), intEl(idReferenceBlock, -20)),
		el(idBlockGroup, intEl(idDiscardPadding, int64(-2*ms)), el(idBlock, block(1, 40, 0, []byte("c")))),
		el(idBlockGroup, uintEl(idBlockDuration, 20)), // no Block
		el(idBlockGroup, el(idBlock, block(2, 60, 0, []byte("other track")))),
		el(idBlockGroup, el(idBlock, block(1, 60, lacingXiph<<1, xiphLace([]byte("d"), []byte("e")))),
			uintEl(idBlockDuration, 40), intEl(idDiscardPadding, int64(-ms))),
		el(idSimpleBlock, block(1, 100, 0, []byte("f"))), // not a keyframe
	))
	want := []Packet{
		{Data: []byte("a"), Time: 0, Duration: 20 * ms, DiscardPadding: ms, Keyframe: true},
		{Data: []byte("b"), Time: 20 * ms},
		{Data: []byte("c"), Time: 40 * ms, DiscardPadding: -2 * ms, Keyframe: true},
		{Data: []byte("d"), Time: 60 * ms, DiscardPadding: -ms, Keyframe: true}, // the first, as it is negative
		{Data: []byte("e"), Time: 60 * ms, Keyframe: true},
		{Data: []byte("f"), Time: 100 * ms},
	}
	got, err := read(file)
	if err != io.EOF {
		t.Fatal(err)
	}
	checkPackets(t, got, want)
}

func TestNegativeTime(t *testing.T) {
	got, err := read(simpleFile(el(idCluster, uintEl(idTimestamp, 5), el(idSimpleBlock, block(1, -15, 0x80, []byte("x"))))))
	if err != io.EOF || len(got) != 1 || got[0].Time != -10*time.Millisecond {
		t.Errorf("%v, %v", got, err)
	}
}

func TestTimestampScale(t *testing.T) {
	tracks := el(idTracks, opusTrack(1))
	clusters := el(idCluster, uintEl(idTimestamp, 10000), el(idSimpleBlock, block(1, 200, 0x80, []byte("x"))))
	for _, c := range []struct {
		info     []byte
		duration time.Duration
		time     time.Duration
	}{
		// Ticks of 0.1 ms, and a Duration as a float32.
		{el(idInfo, el(idDuration, []byte{0x46, 0xEA, 0x60, 0x00}), uintEl(idTimestampScale, 100_000)), 3 * time.Second, 1020 * time.Millisecond},
		{el(idInfo, uintEl(idTimestampScale, 0)), 0, 10200 * time.Millisecond}, // invalid: 1 ms, as ffmpeg reads it
		{nil, 0, 10200 * time.Millisecond},
		{el(idInfo, floatEl(idDuration, -5)), 0, 10200 * time.Millisecond},
	} {
		file := slices.Concat(ebmlHeader("matroska"), el(idSegment, c.info, tracks, clusters))
		r, err := NewReader(bytes.NewReader(file))
		if err != nil {
			t.Fatal(err)
		}
		p, err := r.ReadPacket()
		if err != nil || p.Time != c.time || r.Duration() != c.duration {
			t.Errorf("Info % x: duration %v, packet at %v, %v; want %v and %v", c.info, r.Duration(), p.Time, err, c.duration, c.time)
		}
	}
}

// TestSkippedElements reads past Void and CRC-32 elements, which may be
// anywhere, Tags, Chapters, Attachments and elements of unknown IDs.
func TestSkippedElements(t *testing.T) {
	void := el(idVoid, make([]byte, 5))
	crc := el(idCRC32, []byte{1, 2, 3, 4})
	unknown := el(0x4E4E, []byte("?"))
	file := slices.Concat(
		el(idEBML, crc, uintEl(idEBMLReadVersion, 1), void, strEl(idDocType, "webm")),
		void,
		el(idSegment,
			crc, void,
			el(idInfo, crc, uintEl(idTimestampScale, 1_000_000), void, unknown),
			el(idTags, el(0x7373, strEl(0x45A3, "TITLE"))),
			el(idTracks, crc, void, el(idTrackEntry, void, uintEl(idTrackNumber, 1), uintEl(idTrackType, trackAudio),
				unknown, strEl(idCodecID, "A_OPUS"), el(idAudio, void, uintEl(idChannels, 1)))),
			el(idChapters, void),
			el(idAttachments, make([]byte, 2*skipLimit)),
			unknown,
			el(idCluster, crc, uintEl(idTimestamp, 0), void,
				el(idSimpleBlock, block(1, 0, 0x80, []byte("a"))),
				unknown,
				el(idBlockGroup, crc, void, el(idBlock, block(1, 20, 0, []byte("b"))), unknown),
			),
			void, el(idTags, void),
		),
	)
	got, err := read(file)
	if err != io.EOF {
		t.Fatal(err)
	}
	checkPackets(t, got, []Packet{{Data: []byte("a"), Keyframe: true}, {Data: []byte("b"), Time: 20 * time.Millisecond, Keyframe: true}})
}

// TestChainedSegments ends the stream at the Segment after the first.
func TestChainedSegments(t *testing.T) {
	first, want := testFile(layout{frames: 20, perCluster: 5, unknownSeg: true, unknownCl: true})
	second, _ := testFile(layout{frames: 10, perCluster: 5})
	got, err := read(slices.Concat(first, second))
	if err != io.EOF {
		t.Fatal(err)
	}
	checkPackets(t, got, want)
}

// TestHeadersAfterClusters finds Info, Tracks and Cues after the clusters,
// through a SeekHead that the first one points to.
func TestHeadersAfterClusters(t *testing.T) {
	clusters := [][]byte{
		el(idCluster, uintEl(idTimestamp, 0), el(idSimpleBlock, block(1, 0, 0x80, []byte("a")))),
		el(idCluster, uintEl(idTimestamp, 1000), el(idSimpleBlock, block(1, 0, 0x80, []byte("b")))),
	}
	cues := func(c0, c1 uint64) []byte {
		return el(idCues,
			el(idCuePoint, uintEl(idCueTime, 0), el(idCueTrackPositions, uintEl(idCueTrack, 1), el(idCueClusterPosition, pos(c0)))),
			el(idCuePoint, uintEl(idCueTime, 1000), el(idCueTrackPositions, uintEl(idCueTrack, 1), el(idCueClusterPosition, pos(c1)))))
	}
	info, tracks := infoEl(1_000_000, 2000), el(idTracks, opusTrack(1))
	// The positions take 8 bytes: laying out with zeros gives them.
	lay := func(o []uint64) [][]byte {
		return [][]byte{seekHead(idSeekHead, o[6]), clusters[0], clusters[1], info, tracks, cues(o[1], o[2]),
			seekHead(idInfo, o[3], idTracks, o[4], idCues, o[5], idSeekHead, 0)}
	}
	children := lay(offsets(lay(make([]uint64, 7))...))
	file := slices.Concat(ebmlHeader("webm"), el(idSegment, children...))

	r, err := NewReader(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	if r.Track().Codec != "A_OPUS" || r.Duration() != 2*time.Second {
		t.Errorf("track %+v, duration %v", r.Track(), r.Duration())
	}
	want := []Packet{{Data: []byte("a"), Keyframe: true}, {Data: []byte("b"), Time: time.Second, Keyframe: true}}
	checkPackets(t, readAll(t, r), want)
	if err := r.Seek(1500 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if len(r.cues) != 2 {
		t.Errorf("%d cue points", len(r.cues))
	}
	checkPackets(t, readAll(t, r), want[1:])
}

func TestUnreadableFiles(t *testing.T) {
	tracks := el(idTracks, opusTrack(1))
	segment := el(idSegment, tracks)
	encrypted := el(idTracks, el(idTrackEntry, uintEl(idTrackNumber, 1), uintEl(idTrackType, trackAudio),
		strEl(idCodecID, "A_OPUS"), el(idContentEncodings, el(idContentEncoding))))
	for _, c := range []struct {
		name string
		file []byte
		want error
	}{
		{"empty", nil, ErrFormat},
		{"text", []byte("not a WebM file at all"), ErrFormat},
		{"MKV of another DocType", slices.Concat(ebmlHeader("mkv3d"), segment), ErrFormat},
		{"EBMLReadVersion 2", slices.Concat(el(idEBML, uintEl(idEBMLReadVersion, 2)), segment), errors.ErrUnsupported},
		{"no Segment", ebmlHeader("webm"), ErrFormat},
		{"no Tracks", slices.Concat(ebmlHeader("webm"), el(idSegment, infoEl(1_000_000, 0))), ErrFormat},
		{"video only", slices.Concat(ebmlHeader("webm"), el(idSegment, el(idTracks, videoTrack(1)))), ErrNoAudio},
		{"encrypted audio", slices.Concat(ebmlHeader("webm"), el(idSegment, encrypted)), errors.ErrUnsupported},
		{"Tracks of unknown size", slices.Concat(ebmlHeader("webm"), el(idSegment, elUnknown(idTracks, opusTrack(1)))), ErrFormat},
		{"a TrackEntry larger than Tracks", slices.Concat(ebmlHeader("webm"), el(idSegment, el(idTracks, opusTrack(1)[:20]))), ErrFormat},
	} {
		if _, err := NewReader(bytes.NewReader(c.file)); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}

	// A track without TrackType, which is mandatory, is audio by its codec.
	untyped := el(idTracks, videoTrack(1), el(idTrackEntry, uintEl(idTrackNumber, 2), strEl(idCodecID, "A_OPUS")))
	r, err := NewReader(bytes.NewReader(slices.Concat(ebmlHeader("webm"), el(idSegment, untyped))))
	if err != nil || r.Track().Number != 2 || r.Track().SampleRate != 8000 || r.Track().Channels != 1 {
		t.Errorf("untyped track: %v", err)
	}
}

// TestLargeSizes reads sizes larger than the file, or than anything in it
// can be, as errors, without allocating what they say.
func TestLargeSizes(t *testing.T) {
	tracks := el(idTracks, opusTrack(1))
	hugeBlock := slices.Concat(idBytes(idSimpleBlock), vint(15<<20, 4), block(1, 0, 0x80, []byte("data")))
	for _, c := range []struct {
		name string
		file []byte
		want error // from NewReader, or else ReadPacket
	}{
		{"a block larger than the file", slices.Concat(ebmlHeader("webm"),
			elUnknown(idSegment, tracks, elUnknown(idCluster, uintEl(idTimestamp, 0), hugeBlock))), io.ErrUnexpectedEOF},
		{"a block larger than any", slices.Concat(ebmlHeader("webm"), elUnknown(idSegment, tracks, elUnknown(idCluster,
			slices.Concat(idBytes(idSimpleBlock), vint(1<<40, 8), block(1, 0, 0x80, []byte("data")))))), ErrFormat},
		{"a cluster larger than its Segment", slices.Concat(ebmlHeader("webm"), el(idSegment, tracks,
			slices.Concat(idBytes(idCluster), vint(1<<30, 4), uintEl(idTimestamp, 0)))), ErrFormat},
		{"a block larger than its cluster", slices.Concat(ebmlHeader("webm"), el(idSegment, tracks,
			el(idCluster, uintEl(idTimestamp, 0), hugeBlock[:8]))), ErrFormat},
		{"a Segment larger than the file", slices.Concat(ebmlHeader("webm"),
			idBytes(idSegment), vint(1<<50, 8), tracks, el(idCluster, uintEl(idTimestamp, 0))), io.ErrUnexpectedEOF},
		{"Tracks larger than the file", slices.Concat(ebmlHeader("webm"),
			el(idSegment, slices.Concat(idBytes(idTracks), vint(10<<20, 4), opusTrack(1)))), ErrFormat},
		{"Tracks larger than the file, in a Segment of unknown size", slices.Concat(ebmlHeader("webm"),
			elUnknown(idSegment, slices.Concat(idBytes(idTracks), vint(10<<20, 4), opusTrack(1)))), io.ErrUnexpectedEOF},
		{"an EBML header larger than the file", slices.Concat(idBytes(idEBML), vint(1<<20, 4), strEl(idDocType, "webm")), io.ErrUnexpectedEOF},
		{"a SeekHead pointing past the end", slices.Concat(ebmlHeader("webm"),
			el(idSegment, seekHead(idTracks, 1<<40))), ErrFormat},
		{"a SeekHead pointing past the file", slices.Concat(ebmlHeader("webm"),
			elUnknown(idSegment, seekHead(idTracks, 1<<40))), io.ErrUnexpectedEOF},
	} {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		r, err := NewReader(bytes.NewReader(c.file))
		if err == nil {
			_, err = r.ReadPacket()
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

// TestTruncated reads every beginning of a file, which must give the
// packets it holds whole, then an error, as the Segment's size says more
// follows: io.EOF only once all the packets are read, as when the file
// ends in the Cues after them.
func TestTruncated(t *testing.T) {
	file, want := testFile(layout{frames: 12, perCluster: 5, cues: cuesEnd, video: true})
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
		if err == nil || err == io.EOF && len(got) != len(want) {
			t.Fatalf("cut at %d of %d bytes: %d packets, then %v", n, len(file), len(got), err)
		}
		if len(got) > len(want) || !slices.EqualFunc(got, want[:len(got)], samePacket) {
			t.Fatalf("cut at %d of %d bytes: wrong packets", n, len(file))
		}
		if _, again := r.ReadPacket(); again != err {
			t.Fatalf("cut at %d of %d bytes: %v, then %v", n, len(file), err, again)
		}
		// Seeking still works within what is there.
		if err := r.Seek(150 * time.Millisecond); err != nil {
			t.Fatalf("cut at %d: Seek: %v", n, err)
		}
		if p, err := r.ReadPacket(); err == nil && !slices.ContainsFunc(want, func(w Packet) bool { return samePacket(p, w) }) {
			t.Fatalf("cut at %d: Seek, then a packet at %v", n, p.Time)
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
	for _, at := range []time.Duration{0, 150 * time.Millisecond, time.Hour, 50 * time.Millisecond} {
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
		{frames: 12, perCluster: 5, cues: cuesEnd, video: true},
		{frames: 12, perCluster: 5, cues: cuesFront, unknownSeg: true, unknownCl: true},
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
		{frames: 6, perCluster: 4, cues: cuesEnd, video: true},
		{frames: 6, perCluster: 4, cues: cuesFront, unknownSeg: true, unknownCl: true},
	} {
		file, _ := testFile(l)
		f.Add(file)
	}
	for _, name := range ffmpegFiles {
		b, err := os.ReadFile(filepath.Join("testdata", name+".webm"))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) { exercise(b) })
}

// seekCase is a Seek of TestSeek: the cluster of 10 frames it must go to.
type seekCase struct {
	at      time.Duration
	cluster int
}

var seekCases = []seekCase{
	{3 * time.Second, 15}, {3199 * time.Millisecond, 15}, {0, 0}, {-time.Second, 0}, {7990 * time.Millisecond, 39},
	{time.Hour, 39}, {1500 * time.Millisecond, 7}, {200 * time.Millisecond, 1}, {199 * time.Millisecond, 0},
}

// checkSeek seeks r to c.at, and checks that it reads the packets of want
// from the start of cluster c.cluster on.
func checkSeek(t *testing.T, r *Reader, want []Packet, c seekCase) {
	t.Helper()
	if err := r.Seek(c.at); err != nil {
		t.Fatalf("Seek(%v): %v", c.at, err)
	}
	for i := c.cluster * 10; i < min(c.cluster*10+12, len(want)); i++ {
		p, err := r.ReadPacket()
		if err != nil || !samePacket(p, want[i]) {
			t.Fatalf("Seek(%v), then a packet at %v, %v; want packet %d at %v", c.at, p.Time, err, i, want[i].Time)
		}
	}
}

func TestSeek(t *testing.T) {
	for _, l := range []layout{
		{cues: cuesFront},
		{cues: cuesEnd},
		{cues: cuesEnd, video: true, sizeLen: 8},
		{cues: cuesEnd, unknownSeg: true},
		{},
		{unknownCl: true},
		{unknownSeg: true, unknownCl: true, video: true},
		{cues: cuesEnd, badCues: true},
		{cues: cuesFront, badCues: true, unknownCl: true},
	} {
		t.Run(fmt.Sprintf("%+v", l), func(t *testing.T) {
			// 40 clusters of 10 frames, of 15 kB on average: 600 kB.
			l.frames, l.perCluster, l.frameSize = 400, 10, 3000
			file, want := testFile(l)
			c := &countingReader{r: bytes.NewReader(file)}
			r, err := NewReader(c)
			if err != nil {
				t.Fatal(err)
			}
			if p, err := r.ReadPacket(); err != nil || !samePacket(p, want[0]) {
				t.Fatalf("first packet: %v", err)
			}
			if c.seeks != 1 {
				t.Errorf("%d seeks to read the first packet", c.seeks)
			}
			for i, sc := range seekCases {
				seeks := c.seeks
				checkSeek(t, r, want, sc)
				// A seek, and another to the Cues at the end the first time.
				limit := 1
				if i == 0 && l.cues == cuesEnd {
					limit = 2
				}
				if n := c.seeks - seeks; l.cues != noCues && !l.badCues && n > limit {
					t.Errorf("Seek(%v): %d seeks", sc.at, n)
				}
			}

			// What a scan learnt, the next Seeks know: seeking back reads a
			// cluster at most, and seeking anew reads nothing more.
			c.read = 0
			checkSeek(t, r, want, seekCase{time.Second, 5})
			if c.read > bufSize {
				t.Errorf("seeking back read %d bytes", c.read)
			}
		})
	}
}

// TestSeekReadsLittle checks that the first Seek of a file with Cues at
// its end reads the Cues and a cluster, not the clusters between.
func TestSeekReadsLittle(t *testing.T) {
	file, want := testFile(layout{frames: 400, perCluster: 10, frameSize: 3000, cues: cuesEnd})
	c := &countingReader{r: bytes.NewReader(file)}
	r, err := NewReader(c)
	if err != nil {
		t.Fatal(err)
	}
	if r.cuesRead {
		t.Error("NewReader read the Cues at the end")
	}
	checkSeek(t, r, want, seekCase{4 * time.Second, 20})
	if c.seeks != 3 || c.read > 3*bufSize {
		t.Errorf("NewReader and a Seek: %d seeks, %d bytes read", c.seeks, c.read)
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
	file, want := testFile(layout{frames: 400, perCluster: 10, frameSize: 3000, cues: cuesEnd})
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
	if err := r.Seek(2 * time.Second); err != errFlaky {
		t.Errorf("Seek, which reads the Cues: %v", err)
	}
	f.fail = false
	checkSeek(t, r, want, seekCase{2 * time.Second, 10})
	f.fail = true
	if err := r.Seek(6 * time.Second); err != errFlaky {
		t.Errorf("Seek to a cluster out of the buffer: %v", err)
	}
	f.fail = false
	checkSeek(t, r, want, seekCase{6 * time.Second, 30})
}

func TestOutOfRangeValues(t *testing.T) {
	tracks := el(idTracks, opusTrack(1))
	for _, c := range []struct {
		name string
		file []byte
	}{
		{"a TimestampScale of 8 s", slices.Concat(ebmlHeader("webm"), el(idSegment, el(idInfo, uintEl(idTimestampScale, 8e9)), tracks))},
		{"a CodecDelay of 2⁶⁴ - 1 ns", slices.Concat(ebmlHeader("webm"), el(idSegment, el(idTracks,
			el(idTrackEntry, uintEl(idTrackNumber, 1), uintEl(idTrackType, trackAudio), uintEl(idCodecDelay, 1<<64-1)))))},
		{"a cluster timestamp of 2⁶³ ms", simpleFile(el(idCluster, uintEl(idTimestamp, 1<<63)))},
		{"a Timestamp of 9 bytes", simpleFile(el(idCluster, el(idTimestamp, make([]byte, 9))))},
		{"a Block of 2 bytes", simpleFile(el(idCluster, el(idBlockGroup, el(idBlock, []byte{0x81, 0}))))},
		{"a Block of an invalid track", simpleFile(el(idCluster, el(idBlockGroup, el(idBlock, []byte{0, 0, 0, 0}))))},
		{"a SimpleBlock of an invalid track", simpleFile(el(idCluster, el(idSimpleBlock, []byte{0, 0, 0, 0})))},
		{"a SimpleBlock of 2 bytes", simpleFile(el(idCluster, el(idSimpleBlock, []byte{0x81, 0})))},
		{"a SimpleBlock shorter than its track", simpleFile(el(idCluster, el(idSimpleBlock, []byte{0x41})))},
		{"a BlockDuration of 9 bytes", simpleFile(el(idCluster, el(idBlockGroup, el(idBlockDuration, make([]byte, 9)))))},
	} {
		if _, err := read(c.file); !errors.Is(err, ErrFormat) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	// A BlockDuration out of range is unknown.
	got, err := read(simpleFile(el(idCluster, el(idBlockGroup, el(idBlock, block(1, 0, 0, []byte("a"))), uintEl(idBlockDuration, 1<<63)))))
	if err != io.EOF || len(got) != 1 || got[0].Duration != 0 {
		t.Errorf("BlockDuration of 2⁶³ ms: %v, %v", got, err)
	}
}

func TestSeekNoClusters(t *testing.T) {
	file := slices.Concat(ebmlHeader("webm"), el(idSegment, el(idTracks, opusTrack(1))))
	r, err := NewReader(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := r.ReadPacket(); err != io.EOF {
			t.Errorf("ReadPacket: %v", err)
		}
		if err := r.Seek(time.Second); err != nil {
			t.Errorf("Seek: %v", err)
		}
	}
}
