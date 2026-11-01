package mp4

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
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
// does not hold, such as the AAC audio of a YouTube video, and logs how
// the file is laid out. It runs when MIXMUSIC_MP4_SAMPLE names the file,
// with ffprobe on the PATH:
//
//	yt-dlp -f 140 --fixup never -o zoo.m4a -- jNQXAC9IVRw
//	MIXMUSIC_MP4_SAMPLE=zoo.m4a go test ./internal/mp4 -run Sample -v
//
// yt-dlp's --fixup never keeps the file as YouTube serves it, which it
// would otherwise remux.
func TestSample(t *testing.T) {
	path := os.Getenv("MIXMUSIC_MP4_SAMPLE")
	if path == "" {
		t.Skip("set MIXMUSIC_MP4_SAMPLE to an MP4 file to test against ffprobe")
	}
	p := runProbe(t, path)
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

	// Seek to each fragment, and around it, and across the file.
	want := p.want(r)
	var times []time.Duration
	for _, f := range r.frags {
		at := r.time(f.time)
		times = append(times, at-time.Millisecond, at, at+time.Millisecond)
	}
	d := r.Duration()
	if d == 0 && len(want) > 0 {
		d = want[len(want)-1].time
	}
	for i := range 20 {
		times = append(times, d*time.Duration(i)/19)
	}
	times = append(times, -time.Second, d+time.Minute, d/3, 0)
	c.seeks, c.read = 0, 0
	checkSeeks(t, r, want, times, 3)
	t.Logf("%d seeks, by %d fragments known: %d seeks of the file, %d bytes read", len(times), len(r.frags), c.seeks, c.read)
}

// probeArgs are the arguments of ffprobe that report what checkProbe
// compares the Reader with, as JSON.
var probeArgs = []string{"-v", "error", "-select_streams", "a:0",
	"-show_entries", "packet=pts,duration,size:stream=codec_name,profile,sample_rate,channels,time_base,extradata:format=duration",
	"-show_data", "-of", "json=compact=1"}

// runProbe runs ffprobe on the file at path, or skips the test when
// ffprobe is not on the PATH.
func runProbe(t *testing.T, path string) probe {
	t.Helper()
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe is not on the PATH")
	}
	out, err := exec.Command("ffprobe", append(probeArgs, path)...).Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	var p probe
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// probe is what ffprobe reports of the audio of a file.
type probe struct {
	Packets []struct {
		Pts      int64  `json:"pts"`
		Duration *int64 `json:"duration"` // missing for some
		Size     string `json:"size"`
	} `json:"packets"`
	Streams []struct {
		CodecName  string `json:"codec_name"`
		Profile    string `json:"profile"`
		SampleRate string `json:"sample_rate"`
		Channels   int    `json:"channels"`
		TimeBase   string `json:"time_base"`
		Extradata  string `json:"extradata"` // as a hex dump
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// expected is a packet as ffprobe reports it.
type expected struct {
	time     time.Duration
	size     int
	duration time.Duration // -1 when ffprobe does not say
}

// want returns the packets of the probe, with their times as the Reader
// gives them: ffmpeg subtracts the media time of the edit list from the
// times of the samples, which its time base counts in ticks, so that they
// start below 0.
func (p probe) want(r *Reader) []expected {
	var want []expected
	for _, pkt := range p.Packets {
		var size int
		fmt.Sscan(pkt.Size, &size)
		e := expected{time: r.time(pkt.Pts + r.delay), size: size, duration: -1}
		if pkt.Duration != nil {
			e.duration = r.time(*pkt.Duration)
		}
		want = append(want, e)
	}
	return want
}

// unhex reads the bytes of a hex dump of ffprobe: lines of an offset, 16
// bytes in groups of 2, and their characters.
func unhex(dump string) ([]byte, error) {
	var b []byte
	for _, line := range strings.Split(dump, "\n") {
		if len(line) < 10 {
			continue
		}
		digits := strings.ReplaceAll(line[10:min(len(line), 50)], " ", "")
		d, err := hex.DecodeString(digits)
		if err != nil {
			return nil, err
		}
		b = append(b, d...)
	}
	return b, nil
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
	if s.CodecName != "aac" || s.Profile != "LC" || track.Codec != "mp4a.40.2" {
		t.Errorf("codec %q, ffprobe says %q, %q", track.Codec, s.CodecName, s.Profile)
	}
	if fmt.Sprint(track.SampleRate) != s.SampleRate || track.Channels != s.Channels {
		t.Errorf("%d Hz, %d channels; ffprobe says %s Hz, %d channels", track.SampleRate, track.Channels, s.SampleRate, s.Channels)
	}
	if want := fmt.Sprintf("1/%d", track.Timescale); s.TimeBase != want {
		t.Errorf("timescale %d, ffprobe says a time base of %s", track.Timescale, s.TimeBase)
	}
	if b, err := unhex(s.Extradata); err != nil || !bytes.Equal(b, track.Config) {
		t.Errorf("AudioSpecificConfig % x, ffprobe says % x (%v)", track.Config, b, err)
	}
	// ffmpeg starts the stream at the edit, so that the first packet is at
	// minus the priming.
	if len(p.Packets) > 0 && r.time(-p.Packets[0].Pts) != track.CodecDelay {
		t.Errorf("CodecDelay %v, ffprobe says the first packet is at %d", track.CodecDelay, p.Packets[0].Pts)
	}
	var seconds float64
	fmt.Sscan(p.Format.Duration, &seconds)
	if d := r.Duration(); d != 0 && (d-track.CodecDelay-time.Duration(seconds*1e9)).Abs() > time.Millisecond {
		t.Errorf("duration %v less CodecDelay %v, ffprobe says %vs", d, track.CodecDelay, seconds)
	}

	want := p.want(r)
	if err := r.Seek(0); err != nil {
		t.Fatal(err)
	}
	got := readAll(t, r)
	if len(got) != len(want) {
		t.Errorf("%d packets, ffprobe reports %d", len(got), len(want))
	}
	for i := range min(len(got), len(want)) {
		g, w := got[i], want[i]
		// ffmpeg gives the last packet of a fragmented file the duration of
		// an AAC frame, rather than its trun's.
		known := w.duration >= 0 && !(r.fragmented && i == len(want)-1)
		if g.Time != w.time || len(g.Data) != w.size || known && g.Duration != w.duration {
			t.Fatalf("packet %d: %v, %d bytes, %v long; ffprobe reports %v, %d bytes, %v long",
				i, g.Time, len(g.Data), g.Duration, w.time, w.size, w.duration)
		}
	}
	// The last packet ends with the track.
	if n := len(got); n > 0 && r.Duration() != 0 {
		if end := got[n-1].Time + got[n-1].Duration; (end - r.Duration()).Abs() > time.Millisecond {
			t.Errorf("the last packet ends at %v, the track at %v", end, r.Duration())
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
// after are those of want from the last at or before the time on, or from
// the first: n of them, or all when n is 0.
func checkSeeks(t *testing.T, r *Reader, want []expected, times []time.Duration, n int) {
	t.Helper()
	for _, at := range times {
		if err := r.Seek(at); err != nil {
			t.Fatalf("Seek(%v): %v", at, err)
		}
		i := max(slices.IndexFunc(want, func(e expected) bool { return e.time > at })-1, 0)
		if want[len(want)-1].time <= at {
			i = len(want) - 1
		}
		for k := 0; n == 0 || k < n; k++ {
			p, err := r.ReadPacket()
			if err == io.EOF && i+k == len(want) {
				break
			}
			if err != nil {
				t.Fatalf("Seek(%v), then packet %d: %v", at, k, err)
			}
			if i+k >= len(want) {
				t.Fatalf("Seek(%v): more packets than ffprobe reports", at)
			}
			if w := want[i+k]; p.Time != w.time || len(p.Data) != w.size {
				t.Fatalf("Seek(%v), then packet %d: %v, %d bytes; want packet %d, %v, %d bytes", at, k, p.Time, len(p.Data), i+k, w.time, w.size)
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

// containers are the boxes describe lists the boxes of.
var containers = []uint32{typeMoov, typeTrak, typeEdts, typeMdia, typeMinf, typeStbl, typeMvex, typeMoof, typeTraf, typeStsd, typeMp4a}

// describe lists the boxes of a file, summing up its fragments.
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
	fmt.Fprintf(&b, "track %d %s, %d Hz, %d channels, timescale %d, CodecDelay %v (%d ticks), duration %v, config % x\n",
		tr.ID, tr.Codec, tr.SampleRate, tr.Channels, tr.Timescale, tr.CodecDelay, r.delay, r.Duration(), tr.Config)
	fmt.Fprintf(&b, "%d samples in the moov; fragmented %v, from byte %d; %d fragments in the sidx\n",
		r.table.count, r.fragmented, r.fragStart, r.indexed)

	s := r.src
	s.seekTo(0)
	var moofs, mdats, samples int
	var firstMoof int64 = -1
	var runFlags, trafFlags []string
	var minSamples, maxSamples int
	flush := func() {
		if moofs+mdats == 0 {
			return
		}
		fmt.Fprintf(&b, "  %d moof and %d mdat; first moof at byte %d; %d samples, %d to %d a fragment; tfhd flags %s; trun flags %s\n",
			moofs, mdats, firstMoof, samples, minSamples, maxSamples, strings.Join(trafFlags, " "), strings.Join(runFlags, " "))
		moofs, mdats, samples, firstMoof = 0, 0, 0, -1
	}
	for {
		h, err := s.readHeader()
		if err == io.EOF {
			break
		}
		if err != nil {
			return b.String(), err
		}
		switch h.typ {
		case typeMoof:
			if firstMoof < 0 {
				firstMoof = h.start
			}
			moofs++
			data, err := s.readBody(h)
			if err != nil {
				return b.String(), err
			}
			f, err := r.parseMoof(h.start, data, 0)
			if err != nil {
				return b.String(), err
			}
			n := int(f.count())
			if samples == 0 || n < minSamples {
				minSamples = n
			}
			maxSamples = max(maxSamples, n)
			samples += n
			eachBox(data, func(typ uint32, data []byte) error {
				if typ == typeTraf {
					eachBox(data, func(typ uint32, data []byte) error {
						_, flags, _ := fullBox(data)
						name := fmt.Sprintf("%#x", flags)
						switch {
						case typ == typeTfhd && !slices.Contains(trafFlags, name):
							trafFlags = append(trafFlags, name)
						case typ == typeTrun && !slices.Contains(runFlags, name):
							runFlags = append(runFlags, name)
						}
						return nil
					})
				}
				return nil
			})
			continue
		case typeMdat:
			mdats++
			if moofs > 0 {
				if h.size == toEnd {
					flush()
					fmt.Fprintf(&b, "mdat at byte %d, to the end\n", h.start)
					return b.String(), nil
				}
				s.skip(h.size)
				continue
			}
		}
		flush()
		fmt.Fprintf(&b, "%s at byte %d, %d bytes\n", typeName(h.typ), h.start, h.end()-h.start)
		if h.size == toEnd {
			break
		}
		if h.typ == typeMoov || h.typ == typeSidx {
			data, err := s.readBody(h)
			if err != nil {
				return b.String(), err
			}
			if h.typ == typeSidx {
				describeSidx(&b, data)
			} else {
				describeBoxes(&b, data, 1)
			}
			continue
		}
		s.skip(h.size)
	}
	flush()
	return b.String(), nil
}

// describeBoxes lists the boxes of data b, at a depth of indent.
func describeBoxes(b *strings.Builder, data []byte, indent int) {
	eachBox(data, func(typ uint32, data []byte) error {
		pad := strings.Repeat("  ", indent)
		fmt.Fprintf(b, "%s%s, %d bytes", pad, typeName(typ), len(data))
		switch typ {
		case typeStsd:
			fmt.Fprintln(b)
			describeBoxes(b, data[8:], indent+1)
			return nil
		case typeMp4a:
			fmt.Fprintln(b)
			if len(data) > 28 {
				describeBoxes(b, data[28:], indent+1)
			}
			return nil
		case typeHdlr, typeTkhd, typeMvhd, typeMdhd, typeElst, typeTrex, typeMehd, typeStts, typeStsc, typeStco, typeStsz, typeEsds:
			fmt.Fprintf(b, ": % x", data[:min(len(data), 40)])
		}
		fmt.Fprintln(b)
		if slices.Contains(containers, typ) {
			describeBoxes(b, data, indent+1)
		}
		return nil
	})
}

// describeSidx sums up a sidx.
func describeSidx(b *strings.Builder, data []byte) {
	v, _, f := fullBox(data)
	id, scale := f.u32(), f.u32()
	ept, first := f.uint(v == 1), f.uint(v == 1)
	f.skip(2)
	n := f.u16()
	fmt.Fprintf(b, "  version %d, reference_ID %d, timescale %d, earliest_presentation_time %d, first_offset %d, %d references:", v, id, scale, ept, first, n)
	for i := range n {
		if i == 4 {
			fmt.Fprintf(b, " ...")
			break
		}
		e := f.next(12)
		if e == nil {
			break
		}
		fmt.Fprintf(b, " %d bytes %d ticks SAP %#x;", binary.BigEndian.Uint32(e)&0x7FFFFFFF, binary.BigEndian.Uint32(e[4:]), binary.BigEndian.Uint32(e[8:])>>28)
	}
	fmt.Fprintln(b)
}
