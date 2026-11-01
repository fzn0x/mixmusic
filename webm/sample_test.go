package webm

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestSample checks the Reader against ffprobe on a file the repository
// does not hold, such as the Opus audio of a YouTube video, and logs how
// the file is laid out. It runs when MIXMUSIC_WEBM_SAMPLE names the file,
// with ffprobe on the PATH:
//
//	yt-dlp -f 251 -o zoo.webm https://www.youtube.com/watch?v=jNQXAC9IVRw
//	MIXMUSIC_WEBM_SAMPLE=zoo.webm go test ./webm -run Sample -v
func TestSample(t *testing.T) {
	path := os.Getenv("MIXMUSIC_WEBM_SAMPLE")
	if path == "" {
		t.Skip("set MIXMUSIC_WEBM_SAMPLE to a WebM file to test against ffprobe")
	}
	out, err := exec.Command("ffprobe", append(probeArgs, path)...).Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	var p probe
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatal(err)
	}
	if desc, err := describe(path); err != nil {
		t.Errorf("layout: %v", err)
	} else {
		t.Logf("layout of %s:\n%s", path, desc)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	c := &countingReader{r: f}
	r, err := NewReader(c)
	if err != nil {
		t.Fatal(err)
	}
	checkProbe(t, r, p)
	t.Logf("NewReader and reading every packet: %d seeks, %d bytes read", c.seeks, c.read)

	// Seek to each cue point, or cluster, and around it.
	want := p.want(r)
	var times []time.Duration
	if err := r.loadCues(); err != nil {
		t.Fatal(err)
	}
	points := r.cues
	if len(points) == 0 {
		r.Seek(time.Duration(1 << 62)) // scan every cluster
		points = r.clusters
	}
	for _, c := range points {
		times = append(times, c.time-time.Millisecond, c.time, c.time+time.Millisecond)
	}
	times = append(times, -time.Second, r.Duration()/3, r.Duration()+time.Minute)
	c.seeks = 0
	checkSeeks(t, r, want, times)
	t.Logf("%d seeks to %d cue points: %d seeks of the file", len(times), len(points), c.seeks)
}

// probeArgs are the arguments of ffprobe that report what checkProbe
// compares the Reader with, as JSON.
var probeArgs = []string{"-v", "error", "-select_streams", "a:0",
	"-show_entries", "packet=pts,size,flags:stream=codec_name,sample_rate,channels,initial_padding:format=duration",
	"-of", "json=compact=1"}

// probe is what ffprobe reports of the audio of a file.
type probe struct {
	Packets []struct {
		Pts   int64  `json:"pts"`
		Size  string `json:"size"`
		Flags string `json:"flags"`
	} `json:"packets"`
	Streams []struct {
		CodecName      string `json:"codec_name"`
		SampleRate     string `json:"sample_rate"`
		Channels       int    `json:"channels"`
		InitialPadding int    `json:"initial_padding"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// expected is a packet as ffprobe reports it.
type expected struct {
	time     time.Duration
	size     int
	keyframe bool
}

// want returns the packets of the probe, with their times as the Reader
// gives them: ffmpeg subtracts the CodecDelay, rounded to ticks, from the
// timestamps of the blocks, so that they start below 0.
func (p probe) want(r *Reader) []expected {
	delay := (int64(r.Track().CodecDelay) + r.scale/2) / r.scale
	var want []expected
	for _, pkt := range p.Packets {
		var size int
		fmt.Sscan(pkt.Size, &size)
		want = append(want, expected{
			time:     time.Duration((pkt.Pts + delay) * r.scale),
			size:     size,
			keyframe: strings.HasPrefix(pkt.Flags, "K"),
		})
	}
	return want
}

// checkProbe compares the track, the duration and every packet of r with
// what ffprobe reports.
func checkProbe(t *testing.T, r *Reader, p probe) {
	t.Helper()
	track := r.Track()
	if len(p.Streams) != 1 {
		t.Fatalf("ffprobe reports %d audio streams", len(p.Streams))
	}
	s := p.Streams[0]
	if s.CodecName != "opus" || track.Codec != "A_OPUS" {
		t.Errorf("codec %q, ffprobe says %q", track.Codec, s.CodecName)
	}
	if fmt.Sprint(track.SampleRate) != s.SampleRate || track.Channels != s.Channels {
		t.Errorf("%v Hz, %d channels; ffprobe says %s Hz, %d channels", track.SampleRate, track.Channels, s.SampleRate, s.Channels)
	}
	// ffmpeg reads the CodecDelay into initial_padding, in samples at 48 kHz.
	if d := time.Duration(s.InitialPadding) * time.Second / 48000; track.CodecDelay != d {
		t.Errorf("CodecDelay %v, ffprobe says %v", track.CodecDelay, d)
	}
	if !strings.HasPrefix(string(track.CodecPrivate), "OpusHead") {
		t.Errorf("CodecPrivate %q is no OpusHead", track.CodecPrivate)
	}
	var seconds float64
	fmt.Sscan(p.Format.Duration, &seconds)
	if d := r.Duration(); d != 0 && (d-time.Duration(seconds*1e9)).Abs() > time.Millisecond {
		t.Errorf("duration %v, ffprobe says %vs", d, seconds)
	}

	want := p.want(r)
	got := readAll(t, r)
	if len(got) != len(want) {
		t.Errorf("%d packets, ffprobe reports %d", len(got), len(want))
	}
	for i := range min(len(got), len(want)) {
		if g, w := got[i], want[i]; g.Time != w.time || len(g.Data) != w.size || g.Keyframe != w.keyframe {
			t.Fatalf("packet %d: %v, %d bytes, keyframe %v; ffprobe reports %v, %d bytes, keyframe %v",
				i, g.Time, len(g.Data), g.Keyframe, w.time, w.size, w.keyframe)
		}
	}
}

// readAll reads the packets of r up to io.EOF.
func readAll(t *testing.T, r *Reader) []Packet {
	t.Helper()
	var packets []Packet
	for {
		p, err := r.ReadPacket()
		if err == io.EOF {
			return packets
		}
		if err != nil {
			t.Fatalf("after %d packets: %v", len(packets), err)
		}
		packets = append(packets, p)
	}
}

// checkSeeks seeks r to each of times, checking that the packets read
// after are those of want from a cue point, or cluster, at or before the
// time: the last one, from the cues or the clusters scanned.
func checkSeeks(t *testing.T, r *Reader, want []expected, times []time.Duration) {
	t.Helper()
	for _, at := range times {
		if err := r.Seek(at); err != nil {
			t.Fatalf("Seek(%v): %v", at, err)
		}
		var first time.Duration
		if len(want) > 0 {
			first = want[0].time
		}
		points := r.cues
		if len(points) == 0 {
			points = r.clusters
		}
		for _, c := range points {
			if c.time <= at {
				first = c.time
			}
		}
		i := slices.IndexFunc(want, func(e expected) bool { return e.time >= first })
		if i < 0 {
			i = len(want)
		}
		for n := 0; ; n++ {
			p, err := r.ReadPacket()
			if err == io.EOF && i+n == len(want) {
				break
			}
			if err != nil {
				t.Fatalf("Seek(%v), then packet %d: %v", at, n, err)
			}
			if i+n >= len(want) {
				t.Fatalf("Seek(%v): more packets than ffprobe reports", at)
			}
			if w := want[i+n]; p.Time != w.time || len(p.Data) != w.size {
				t.Fatalf("Seek(%v), then packet %d: %v, %d bytes; want %v, %d bytes", at, n, p.Time, len(p.Data), w.time, w.size)
			}
			if n == 3 && len(times) > 20 {
				break // the start is what matters; save the time of long files
			}
		}
	}
}

// countingReader counts the seeks and the bytes read of a file.
type countingReader struct {
	r     io.ReadSeeker
	seeks int
	read  int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.read += int64(n)
	return n, err
}

func (c *countingReader) Seek(off int64, whence int) (int64, error) {
	c.seeks++
	return c.r.Seek(off, whence)
}

// names names the elements describe lists.
var names = map[uint32]string{
	idEBML: "EBML", idSegment: "Segment", idSeekHead: "SeekHead", idInfo: "Info", idTracks: "Tracks",
	idCues: "Cues", idCluster: "Cluster", idTags: "Tags", idChapters: "Chapters", idAttachments: "Attachments",
	idVoid: "Void", idCRC32: "CRC-32", idBlock: "Block", idBlockDuration: "BlockDuration",
	idReferenceBlock: "ReferenceBlock", idDiscardPadding: "DiscardPadding",
}

func name(id uint32) string {
	if n, ok := names[id]; ok {
		return n
	}
	return fmt.Sprintf("%#x", id)
}

// describe lists the elements of a file, summing up its clusters.
func describe(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	r, err := NewReader(f)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	tr := r.Track()
	fmt.Fprintf(&b, "track %d %s, %v Hz, %d channels, CodecDelay %v, SeekPreRoll %v, CodecPrivate % x\n",
		tr.Number, tr.Codec, tr.SampleRate, tr.Channels, tr.CodecDelay, tr.SeekPreRoll, tr.CodecPrivate)
	fmt.Fprintf(&b, "TimestampScale %d ns, Duration %v\n", r.scale, r.Duration())

	s := r.src
	s.seekTo(r.segStart)
	size := "unknown size"
	if r.segEnd >= 0 {
		size = fmt.Sprintf("%d bytes", r.segEnd-r.segStart)
	}
	fmt.Fprintf(&b, "Segment: data at byte %d, %s\n", r.segStart, size)

	var clusters, unknown, blocks, groups, laced, keyframes, others int
	var minSize, maxSize, total int64
	var lacings [4]int
	var firstTimes, inGroups, paddings []string
	for r.segEnd < 0 || s.offset() < r.segEnd {
		h, err := s.readHeader()
		if err == io.EOF {
			break
		}
		if err != nil {
			return b.String(), err
		}
		if h.id != idCluster {
			if clusters > 0 && h.id != idVoid && h.id != idCRC32 {
				fmt.Fprintf(&b, "  ... %d clusters\n", clusters)
			}
			fmt.Fprintf(&b, "%s at byte %d (segment position %d), %d bytes\n", name(h.id), h.start, h.start-r.segStart, h.size)
			if h.size == unknownSize {
				return b.String(), fmt.Errorf("level-1 element of unknown size")
			}
			s.skip(h.size)
			continue
		}
		if clusters == 0 {
			fmt.Fprintf(&b, "first Cluster at byte %d (segment position %d)\n", h.start, h.start-r.segStart)
		}
		clusters++
		if h.size == unknownSize {
			unknown++
		} else {
			if minSize == 0 || h.size < minSize {
				minSize = h.size
			}
			maxSize = max(maxSize, h.size)
			total += h.size
		}
		r.inCluster, r.clusterEnd = true, h.end()
		for {
			c, ok, err := r.clusterChild()
			if err != nil {
				return b.String(), err
			}
			if !ok {
				break
			}
			data, err := s.readBody(c)
			if err != nil {
				return b.String(), err
			}
			switch c.id {
			case idTimestamp:
				if len(firstTimes) < 6 {
					v, _ := readUint(data)
					firstTimes = append(firstTimes, fmt.Sprint(v))
				}
				continue
			case idBlockGroup:
				groups++
				eachChild(data, func(id uint32, d []byte) error {
					if !slices.Contains(inGroups, name(id)) {
						inGroups = append(inGroups, name(id))
					}
					switch id {
					case idBlock:
						data = d
					case idDiscardPadding:
						v, _ := readInt(d)
						paddings = append(paddings, time.Duration(v).String())
					}
					return nil
				})
			case idSimpleBlock:
				blocks++
			default:
				continue
			}
			track, n, err := readVint(data)
			if err != nil || len(data) < n+3 {
				return b.String(), fmt.Errorf("bad block at byte %d", c.start)
			}
			if track != tr.Number {
				others++
				continue
			}
			flags := data[n+2]
			lacings[flags>>1&3]++
			if flags>>1&3 != lacingNone {
				laced++
			}
			if c.id == idSimpleBlock && flags&flagKeyframe != 0 {
				keyframes++
			}
		}
	}
	fmt.Fprintf(&b, "%d clusters (%d of unknown size), %d to %d bytes, %d on average; first timestamps %s\n",
		clusters, unknown, minSize, maxSize, total/int64(max(clusters-unknown, 1)), strings.Join(firstTimes, ", "))
	fmt.Fprintf(&b, "%d SimpleBlocks (%d flagged keyframes), %d blocks of other tracks\n", blocks, keyframes, others)
	fmt.Fprintf(&b, "%d BlockGroups, of %s; DiscardPadding %s\n", groups, strings.Join(inGroups, ", "), strings.Join(paddings, ", "))
	fmt.Fprintf(&b, "lacing of the track's blocks: none %d, Xiph %d, fixed %d, EBML %d\n",
		lacings[lacingNone], lacings[lacingXiph], lacings[lacingFixed], lacings[lacingEBML])
	if err := r.loadCues(); err != nil {
		return b.String(), err
	}
	fmt.Fprintf(&b, "%d cue points", len(r.cues))
	for i, c := range r.cues {
		if i == 8 {
			fmt.Fprintf(&b, " ...")
			break
		}
		fmt.Fprintf(&b, " %v@%d", c.time, c.pos)
	}
	return b.String(), nil
}
