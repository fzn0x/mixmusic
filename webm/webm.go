// Package webm reads the audio of WebM and Matroska files, such as the
// Opus audio YouTube serves (itags 249, 250 and 251): the packets of their
// first audio track, with their times, and seeking by their Cues.
//
// A Reader reads its io.ReadSeeker a little at a time, through a buffer,
// and seeks it only when it must: to go back, to skip far ahead, and to
// load the Cues or headers that come after the clusters. The sizes it
// reads are bounded, so that a corrupt one cannot allocate much memory,
// and malformed files make errors, never panics.
package webm

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"time"
)

var (
	// ErrFormat is wrapped by the errors of data that is not WebM or
	// Matroska, or is corrupt.
	ErrFormat = errors.New("webm: invalid data")
	// ErrNoAudio is returned by NewReader for a file without an audio track.
	ErrNoAudio = errors.New("webm: no audio track")
)

const (
	// defaultScale is the TimestampScale of files that give none: 1 ms.
	defaultScale = 1_000_000
	// maxScale bounds the TimestampScale, so that times in nanoseconds
	// cannot overflow: a tick of 4 seconds is already absurd.
	maxScale = 1 << 32
	// maxPosition bounds the positions the SeekHead and the Cues give.
	maxPosition = 1 << 48
	// maxSeekHeads bounds the SeekHeads read, which may point to others.
	maxSeekHeads = 8
)

// Track is the audio track a Reader reads.
type Track struct {
	Number       uint64 // TrackNumber, which the blocks name it by
	Codec        string // CodecID, as "A_OPUS"
	CodecPrivate []byte // for Opus, the OpusHead
	// CodecDelay is the decoded audio to drop at the start of the stream
	// (for Opus, the pre-skip of the OpusHead): the audio of a packet
	// plays at its Time minus CodecDelay.
	CodecDelay  time.Duration
	SeekPreRoll time.Duration // to decode before the time of a seek
	SampleRate  float64       // Audio/SamplingFrequency: 8000 when not given
	Channels    int           // Audio/Channels: 1 when not given
}

// Packet is a frame of the track: for Opus, an Opus packet.
type Packet struct {
	// Data is the frame, which the Reader does not reuse.
	Data []byte
	// Time is when the frame starts, from the start of the stream: the
	// timestamp of its cluster plus that of its block, scaled. A laced
	// block gives all its frames its own time, as the specification
	// leaves the time of the frames after the first unspecified: an Opus
	// decoder can count their samples instead.
	Time time.Duration
	// Duration is the BlockDuration of the frame's block, when it gives
	// one and holds a single frame; else 0.
	Duration time.Duration
	// DiscardPadding is the duration of the samples to drop from the end
	// of the frame, as the last frame of a stream has, or, when negative,
	// from its start. Of a laced block, the last frame has it, or the
	// first when it is negative.
	DiscardPadding time.Duration
	// Keyframe is set for the frames of a SimpleBlock flagged as a
	// keyframe and of a BlockGroup without ReferenceBlock: all Opus
	// frames are.
	Keyframe bool
}

// Reader reads the packets of the first audio track of a WebM or Matroska
// file. It is not safe for concurrent use.
type Reader struct {
	src      *source
	track    Track
	scale    int64 // the nanoseconds of a timestamp tick: TimestampScale
	duration time.Duration

	segStart int64 // the offset of the Segment's data
	segEnd   int64 // the offset of its end, or -1 when its size is unknown
	start    int64 // the offset of the first Cluster, or of the end of the headers

	index map[uint32]int64 // where the SeekHeads read say elements are
	heads []int64          // the SeekHeads they point to, to read when need be
	seen  []int64          // the SeekHeads read or to read

	cuesRead bool       // whether the Cues were read, or found unusable
	cues     []cuePoint // the cue points of the track, by time
	clusters []cuePoint // the clusters a Seek without cues scanned, in order
	scanAt   int64      // where the next cluster to scan starts
	scanned  bool       // whether the scan reached the end

	inCluster   bool
	clusterEnd  int64    // -1 when the size of the cluster is unknown
	clusterTime int64    // its timestamp, in ticks
	queue       []Packet // the frames of the last block read
	qi          int      // the next of them to return
	err         error    // returned by ReadPacket until the next Seek
}

// cuePoint is a cluster to seek to and the time it starts at.
type cuePoint struct {
	time time.Duration
	pos  int64 // the offset of the cluster
}

// NewReader reads the headers of a file, from the start of r up to the
// first Cluster: the EBML header, Info and Tracks, found through the
// SeekHead when they come after the clusters, and the Cues when they come
// before, as in YouTube's files. Cues after the clusters are only read at
// the first Seek, so that a file played from its start is read straight
// through.
//
// A file without an audio track returns ErrNoAudio, and one whose audio
// is compressed or encrypted (by ContentEncodings), or of an EBML version
// above 1, an error wrapping errors.ErrUnsupported.
func NewReader(r io.ReadSeeker) (*Reader, error) {
	rd := &Reader{src: newSource(r), scale: defaultScale, index: map[uint32]int64{}}
	if err := rd.readHeaders(); err != nil {
		return nil, err
	}
	return rd, nil
}

// Track returns the track the Reader reads: the first audio track.
func (r *Reader) Track() Track { return r.track }

// Duration returns the duration of the file, as its Info gives it, or 0
// when unknown.
func (r *Reader) Duration() time.Duration { return r.duration }

func (r *Reader) readHeaders() error {
	s := r.src
	h, err := s.readHeader()
	if err != nil {
		return eofFormat(err, "empty file")
	}
	if h.id != idEBML {
		return fmt.Errorf("%w: not an EBML file", ErrFormat)
	}
	b, err := s.readBody(h)
	if err != nil {
		return err
	}
	if err := checkEBMLHeader(b); err != nil {
		return err
	}

	for {
		if h, err = s.readHeader(); err != nil {
			return eofFormat(err, "no Segment")
		}
		if h.id == idSegment {
			break
		}
		if h.size == unknownSize {
			return fmt.Errorf("%w: element %#x at byte %d has an unknown size", ErrFormat, h.id, h.start)
		}
		s.skip(h.size)
	}
	r.segStart, r.segEnd = h.data, h.end()

	// The elements before the first Cluster.
	var info, tracks, cues []byte
	for r.segEnd < 0 || s.offset() < r.segEnd {
		h, err := s.readHeader()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if h.id == idCluster || h.id == idEBML || h.id == idSegment {
			s.seekTo(h.start)
			break
		}
		if err := checkChild(h, r.segEnd); err != nil {
			return err
		}
		switch h.id {
		case idSeekHead, idInfo, idTracks:
			b, err := s.readBody(h)
			if err != nil {
				return err
			}
			switch h.id {
			case idSeekHead:
				r.addSeekHead(h.start, b)
			case idInfo:
				info = b
			case idTracks:
				tracks = b
			}
		case idCues:
			b, err := s.readBody(h)
			switch {
			case err == nil:
				cues = b
			case errors.Is(err, ErrFormat):
				s.seekTo(h.end()) // too large: done without
			default:
				return err
			}
		default:
			s.skip(h.size)
		}
	}
	r.start, r.scanAt = s.offset(), s.offset()

	if info == nil {
		if info, err = r.loadIndexed(idInfo); err != nil {
			return err
		}
	}
	if err := r.parseInfo(info); err != nil {
		return err
	}
	if tracks == nil {
		if tracks, err = r.loadIndexed(idTracks); err != nil {
			return err
		}
		if tracks == nil {
			return fmt.Errorf("%w: no Tracks", ErrFormat)
		}
	}
	if r.track, err = parseTracks(tracks); err != nil {
		return err
	}
	if cues != nil {
		r.setCues(cues)
	}
	s.seekTo(r.start)
	return nil
}

// eofFormat turns the end of the file, where something is missing, into
// an ErrFormat.
func eofFormat(err error, missing string) error {
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return fmt.Errorf("%w: %s", ErrFormat, missing)
	}
	return err
}

// checkChild checks that the element of header h, of known size, ends
// within its parent, which ends at end (-1 when unknown).
func checkChild(h header, end int64) error {
	if h.size == unknownSize {
		return fmt.Errorf("%w: element %#x at byte %d has an unknown size", ErrFormat, h.id, h.start)
	}
	if end >= 0 && h.size > end-h.data {
		return fmt.Errorf("%w: element %#x at byte %d overflows its parent", ErrFormat, h.id, h.start)
	}
	return nil
}

// checkEBMLHeader checks that the EBML header is of a WebM or Matroska
// file that this package can read.
func checkEBMLHeader(b []byte) error {
	readVersion, docType := uint64(1), "matroska"
	err := eachChild(b, func(id uint32, data []byte) error {
		var err error
		switch id {
		case idEBMLReadVersion:
			readVersion, err = readUint(data)
		case idDocType:
			docType = readString(data)
		}
		return err
	})
	switch {
	case err != nil:
		return err
	case docType != "webm" && docType != "matroska":
		return fmt.Errorf("%w: DocType %q is not WebM or Matroska", ErrFormat, docType)
	case readVersion > 1:
		return fmt.Errorf("webm: %w: EBMLReadVersion %d", errors.ErrUnsupported, readVersion)
	}
	return nil
}

// parseInfo reads the TimestampScale and the Duration of Info, which may
// be nil.
func (r *Reader) parseInfo(b []byte) error {
	var duration float64
	err := eachChild(b, func(id uint32, data []byte) error {
		var err error
		switch id {
		case idTimestampScale:
			var v uint64
			if v, err = readUint(data); v > maxScale {
				err = fmt.Errorf("%w: TimestampScale %d", ErrFormat, v)
			} else if v > 0 { // 0 is invalid: keep the default, as ffmpeg does
				r.scale = int64(v)
			}
		case idDuration:
			duration, err = readFloat(data)
		}
		return err
	})
	if err != nil {
		return err
	}
	// In ticks, as a float: NaN, infinite or out-of-range ones are unknown.
	if d := duration * float64(r.scale); d > 0 && d < math.MaxInt64 {
		r.duration = time.Duration(d)
	}
	return nil
}

// parseTracks returns the first audio track of Tracks.
func parseTracks(b []byte) (Track, error) {
	var track Track
	found := false
	err := eachChild(b, func(id uint32, data []byte) error {
		if id != idTrackEntry || found {
			return nil
		}
		t, audio, err := parseTrackEntry(data)
		if audio {
			track, found = t, true
		}
		return err
	})
	switch {
	case err != nil:
		return Track{}, err
	case !found:
		return Track{}, ErrNoAudio
	}
	return track, nil
}

// parseTrackEntry reads a TrackEntry, and reports whether it is of a
// usable audio track. It returns errors.ErrUnsupported for audio that is
// compressed or encrypted.
func parseTrackEntry(b []byte) (t Track, audio bool, err error) {
	t.SampleRate, t.Channels = 8000, 1
	var kind uint64
	encoded := false
	err = eachChild(b, func(id uint32, data []byte) error {
		var err error
		switch id {
		case idTrackNumber:
			t.Number, err = readUint(data)
		case idTrackType:
			kind, err = readUint(data)
		case idCodecID:
			t.Codec = readString(data)
		case idCodecPrivate:
			t.CodecPrivate = bytes.Clone(data)
		case idCodecDelay:
			t.CodecDelay, err = readNanoseconds(data)
		case idSeekPreRoll:
			t.SeekPreRoll, err = readNanoseconds(data)
		case idAudio:
			err = eachChild(data, func(id uint32, data []byte) error {
				switch id {
				case idSamplingFrequency:
					f, err := readFloat(data)
					if f > 0 && !math.IsInf(f, 0) {
						t.SampleRate = f
					}
					return err
				case idChannels:
					n, err := readUint(data)
					t.Channels = int(min(n, math.MaxInt32))
					return err
				}
				return nil
			})
		case idContentEncodings:
			encoded = true
		}
		return err
	})
	if err != nil {
		return Track{}, false, err
	}
	// A track without TrackType, which is mandatory, is audio by its codec.
	audio = (kind == trackAudio || kind == 0 && strings.HasPrefix(t.Codec, "A_")) && t.Number > 0
	if audio && encoded {
		return Track{}, false, fmt.Errorf("webm: %w: track %d is compressed or encrypted", errors.ErrUnsupported, t.Number)
	}
	return t, audio, nil
}

// readNanoseconds reads an unsigned integer of nanoseconds.
func readNanoseconds(b []byte) (time.Duration, error) {
	v, err := readUint(b)
	if v > math.MaxInt64 {
		return 0, fmt.Errorf("%w: %d ns", ErrFormat, v)
	}
	return time.Duration(v), err
}

// segmentOffset returns the offset in the file of a position in the
// Segment, as the SeekHead and the Cues give them, and whether it is in
// the Segment.
func (r *Reader) segmentOffset(p uint64) (int64, bool) {
	if p >= maxPosition {
		return 0, false
	}
	off := r.segStart + int64(p)
	return off, r.segEnd < 0 || off < r.segEnd
}

// ticks converts a timestamp in ticks to a time, and reports whether it
// is in range.
func (r *Reader) ticks(v uint64) (time.Duration, bool) {
	if v > uint64(math.MaxInt64/r.scale) {
		return 0, false
	}
	return time.Duration(int64(v) * r.scale), true
}

// addSeekHead notes where the SeekHead read at offset at, of data b, says
// the elements are. A corrupt SeekHead gives what it holds before the
// corruption.
func (r *Reader) addSeekHead(at int64, b []byte) {
	if !slices.Contains(r.seen, at) {
		r.seen = append(r.seen, at)
	}
	eachChild(b, func(id uint32, data []byte) error {
		if id != idSeek {
			return nil
		}
		var target uint32
		var pos uint64
		err := eachChild(data, func(id uint32, data []byte) error {
			var err error
			switch id {
			case idSeekID:
				// The ID, as its element starts with it.
				if v, e := readUint(data); e == nil && v <= math.MaxUint32 {
					target = uint32(v)
				}
			case idSeekPosition:
				pos, err = readUint(data)
			}
			return err
		})
		off, ok := r.segmentOffset(pos)
		switch {
		case err != nil || target == 0 || !ok:
		case target == idSeekHead:
			if !slices.Contains(r.seen, off) && len(r.seen) < maxSeekHeads {
				r.seen = append(r.seen, off)
				r.heads = append(r.heads, off)
			}
		default:
			if _, dup := r.index[target]; !dup {
				r.index[target] = off
			}
		}
		return err
	})
}

// lookup returns the offset the SeekHeads give for the element of ID id,
// reading the further SeekHeads they point to as need be, or -1.
func (r *Reader) lookup(id uint32) (int64, error) {
	for {
		if off, ok := r.index[id]; ok {
			return off, nil
		}
		if len(r.heads) == 0 {
			return -1, nil
		}
		at := r.heads[0]
		r.heads = r.heads[1:]
		b, err := r.loadElement(at, idSeekHead)
		switch {
		case err == nil:
			r.addSeekHead(at, b)
		case !isDataError(err):
			return -1, err
		}
	}
}

// loadIndexed reads the element of ID id the SeekHeads point to, or
// returns nil when they point to none.
func (r *Reader) loadIndexed(id uint32) ([]byte, error) {
	off, err := r.lookup(id)
	if err != nil || off < 0 {
		return nil, err
	}
	return r.loadElement(off, id)
}

// loadElement reads the data of the element at offset off, which must be
// of ID id.
func (r *Reader) loadElement(off int64, id uint32) ([]byte, error) {
	r.src.seekTo(off)
	h, err := r.src.readHeader()
	if err != nil {
		return nil, unexpected(err)
	}
	if h.id != id {
		return nil, fmt.Errorf("%w: no element %#x at byte %d, where the SeekHead points", ErrFormat, id, off)
	}
	if err := checkChild(h, r.segEnd); err != nil {
		return nil, err
	}
	return r.src.readBody(h)
}

// isDataError reports whether err is about the data of the file rather
// than the reading of it.
func isDataError(err error) bool {
	return errors.Is(err, ErrFormat) || err == io.EOF || err == io.ErrUnexpectedEOF
}
