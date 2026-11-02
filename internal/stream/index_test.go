package stream

import (
	"encoding/binary"
	"errors"
	"os"
	"testing"
	"time"
)

// The head files are the first bytes of two streams of the picture of
// musicVideoID, to the end of their index, and the sizes of the streams.
const (
	size243 = 21115748 // VP9 in WebM, 640 × 360
	size134 = 31051398 // H.264 in MP4, 640 × 360
)

func readHead(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// checkSegments checks that the segments of ix follow one another from
// its initialization to the end of a stream of size bytes, as long as
// length together, of about five seconds each.
func checkSegments(t *testing.T, ix Index, first, size int64, length time.Duration) {
	t.Helper()
	if len(ix.Segments) < 90 {
		t.Fatalf("%d segments", len(ix.Segments))
	}
	if s := ix.Segments[0]; s.Start != first || s.Time != 0 {
		t.Errorf("the first segment is %+v", s)
	}
	var total time.Duration
	for i, s := range ix.Segments {
		if i > 0 {
			prev := ix.Segments[i-1]
			if s.Start != prev.End+1 || s.Time != prev.Time+prev.Duration {
				t.Fatalf("segment %d, %+v, doesn't follow %+v", i, s, prev)
			}
		}
		if s.Duration <= 0 || s.Duration > 10*time.Second {
			t.Errorf("segment %d lasts %s", i, s.Duration)
		}
		total += s.Duration
	}
	if last := ix.Segments[len(ix.Segments)-1]; last.End != size-1 {
		t.Errorf("the last segment ends at %d of %d bytes", last.End, size)
	}
	if d := total - length; d < -time.Second || d > time.Second {
		t.Errorf("the segments last %s, the stream %s", total, length)
	}
}

func TestWebMIndex(t *testing.T) {
	head := readHead(t, "video-243-head.webm")
	ix, err := parseIndex(head, `video/webm; codecs="vp9"`, size243)
	if err != nil {
		t.Fatal(err)
	}
	// YouTube's initRange: the EBML header to the end of the Tracks, but
	// the Cues.
	if ix.Init != 220 {
		t.Errorf("the initialization ends at %d", ix.Init)
	}
	checkSegments(t, ix, int64(len(head)), size243, 474316*time.Millisecond)
}

func TestMP4Index(t *testing.T) {
	head := readHead(t, "video-134-head.mp4")
	ix, err := parseIndex(head, `video/mp4; codecs="avc1.4D401E"`, size134)
	if err != nil {
		t.Fatal(err)
	}
	if ix.Init != 741 { // ftyp and moov
		t.Errorf("the initialization ends at %d", ix.Init)
	}
	checkSegments(t, ix, int64(len(head)), size134, 474316*time.Millisecond)
}

// TestIndexNeedsMore asks for the bytes the index takes when fewer came.
func TestIndexNeedsMore(t *testing.T) {
	for _, c := range []struct {
		name, mime string
		size       int64
	}{
		{"video-243-head.webm", "video/webm", size243},
		{"video-134-head.mp4", "video/mp4", size134},
	} {
		head := readHead(t, c.name)
		for _, n := range []int{40, 200, 1000} {
			_, err := parseIndex(head[:n], c.mime, c.size)
			var short shortHead
			if !errors.As(err, &short) || short.need <= int64(n) || short.need > maxHead {
				t.Errorf("%s, %d bytes: %v", c.name, n, err)
				continue
			}
			// What it asks for is enough, or tells how much is.
			more := head
			if short.need < int64(len(head)) {
				more = head[:short.need]
			}
			if _, err := parseIndex(more, c.mime, c.size); err != nil && !errors.As(err, &short) {
				t.Errorf("%s, %d bytes, then %d: %v", c.name, n, len(more), err)
			}
		}
	}
}

func TestIndexRefusesOthers(t *testing.T) {
	webm := readHead(t, "video-243-head.webm")
	mp4 := readHead(t, "video-134-head.mp4")
	// An MP4 whose fragments come without a sidx.
	noSidx := append([]byte(nil), mp4[:741]...)
	noSidx = binary.BigEndian.AppendUint32(noSidx, 16)
	noSidx = append(noSidx, "moof\x00\x00\x00\x00\x00\x00\x00\x00"...)
	// A WebM whose first cluster comes before any Cues.
	noCues := append([]byte(nil), webm[:220]...)
	noCues = append(noCues, 0x1F, 0x43, 0xB6, 0x75, 0x81, 0x00)
	for _, c := range []struct {
		name string
		head []byte
		mime string
		want error
	}{
		{"no sidx", noSidx, "video/mp4", errNoIndex},
		{"no cues", noCues, "video/webm", errNoIndex},
		{"not webm", mp4, "video/webm", errBadIndex},
		{"garbage", []byte("not a video at all, of more than sixteen bytes"), "video/mp4", errBadIndex},
	} {
		if _, err := parseIndex(c.head, c.mime, 1<<20); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
	if _, err := parseIndex(webm, "video/ogg", size243); err == nil {
		t.Error("an index of Ogg")
	}
}

// FuzzParseIndex reads whatever a stream's first bytes are without
// panicking: they come from the network.
func FuzzParseIndex(f *testing.F) {
	for _, name := range []string{"video-243-head.webm", "video-134-head.mp4"} {
		b, err := os.ReadFile("testdata/" + name)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b, true)
		f.Add(b, false)
	}
	f.Fuzz(func(t *testing.T, head []byte, webm bool) {
		mime := "video/mp4"
		if webm {
			mime = "video/webm"
		}
		ix, err := parseIndex(head, mime, 1<<30)
		if err == nil && (len(ix.Segments) == 0 || ix.Init <= 0) {
			t.Errorf("an index without parts: %+v", ix)
		}
	})
}
