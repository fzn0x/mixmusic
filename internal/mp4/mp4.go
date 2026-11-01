// Package mp4 reads the audio of MP4 files, of the ISO base media file
// format (ISO/IEC 14496-12 and 14496-14), such as the AAC audio YouTube
// serves (itags 139 and 140): the packets of their first audio track, with
// their times, and seeking by their sample tables or segment indexes.
//
// It reads progressive files, whose moov describes every sample, whether
// it comes before the media data or after, and fragmented ones, whose moof
// boxes each describe the samples of the mdat after them, as YouTube's
// DASH files are, with a sidx to seek by.
//
// A Reader reads its io.ReadSeeker a little at a time, through a buffer,
// and seeks it only when it must: to go back, to skip far ahead, and to
// load a moov that comes after the media data. The sizes it reads are
// bounded, so that a corrupt one cannot allocate much memory, and
// malformed files make errors, never panics.
package mp4

import (
	"errors"
	"fmt"
	"io"
	"time"
)

var (
	// ErrFormat is wrapped by the errors of data that is not MP4, or is
	// corrupt.
	ErrFormat = errors.New("mp4: invalid data")
	// ErrNoAudio is returned by NewReader for a file without an audio track.
	ErrNoAudio = errors.New("mp4: no audio track")
)

// maxSeconds bounds the times of the media, in seconds, so that they
// cannot overflow in ticks or in nanoseconds: 34 years.
const maxSeconds = 1 << 30

// Track is the audio track a Reader reads.
type Track struct {
	ID uint32 // track_ID, which the fragments name it by
	// Codec names the codec as RFC 6381 does. For MPEG-4 audio it is
	// "mp4a.40." and the audio object type of the AudioSpecificConfig, as
	// "mp4a.40.2" for AAC-LC; for another object type of the esds, "mp4a."
	// and its number in hexadecimal; for another sample entry, its four
	// characters, as "Opus".
	Codec string
	// Config is the decoder's configuration: for MPEG-4 audio, the
	// AudioSpecificConfig, the DecoderSpecificInfo of the esds.
	Config []byte
	// CodecDelay is the media to drop at the start of the stream, the
	// encoder's priming, which the edit list skips: the audio of a packet
	// plays at its Time minus CodecDelay.
	CodecDelay time.Duration
	// SampleRate and Channels are those of the AudioSpecificConfig, or of
	// the sample entry when it does not give them. For HE-AAC signalled
	// explicitly, the rate is that of the decoded audio, twice the core's.
	SampleRate int
	Channels   int
	Timescale  uint32 // the ticks per second of the media's times, of its mdhd
}

// Packet is a sample of the track: for AAC, an access unit.
type Packet struct {
	// Data is the sample, which the Reader does not reuse.
	Data []byte
	// Time is when the sample is decoded, from the start of the media: for
	// audio, when it starts.
	Time time.Duration
	// Duration is its duration, as the sample table or the fragment gives
	// it.
	Duration time.Duration
}

// sample is where a sample of the track is, and when.
type sample struct {
	pos      int64  // the offset of its data
	size     uint32 // the size of its data
	time     int64  // its time, in ticks
	duration uint32 // in ticks
}

// Reader reads the packets of the first audio track of an MP4 file. It is
// not safe for concurrent use.
type Reader struct {
	src      *source
	track    Track
	scale    int64 // the timescale of the media: ticks per second
	maxTicks int64 // the bound of its times, maxSeconds in ticks
	delay    int64 // the CodecDelay, in ticks
	duration time.Duration

	// The samples of the moov's sample table, which come first.
	table   table
	inTable bool // whether the next sample is of the table

	// The fragments, when the moov has an mvex.
	fragmented bool
	trex       map[uint32]trex // the defaults of each track's fragments
	fragStart  int64           // where the fragments start: after the moov and its mdat
	frags      []fragPoint     // the fragments known, by time: from the sidx, then a scan
	indexed    int             // those of r.frags the sidx gives, which come first
	known      int64           // when the fragments known end, in ticks
	scanAt     int64           // where the next fragment to scan starts, or -1
	scanned    bool            // whether the scan reached the end

	frag  fragment // the fragment being read
	after int64    // the offset of the box after it and its mdat, or -1 at the end
	err   error    // returned by ReadPacket until the next Seek
}

// NewReader reads the headers of a file, from its start up to its media
// data: the moov, and in a fragmented file the sidx after it. A moov after
// the media data is found by skipping them, which costs a seek of the
// file, and another back to the first sample.
//
// A file without an audio track returns ErrNoAudio, and one whose audio
// is encrypted an error wrapping errors.ErrUnsupported.
func NewReader(r io.ReadSeeker) (*Reader, error) {
	rd := &Reader{src: newSource(r), trex: map[uint32]trex{}, after: -1, scanAt: -1}
	if err := rd.readHeaders(); err != nil {
		return nil, err
	}
	return rd, nil
}

// Track returns the track the Reader reads: the first audio track.
func (r *Reader) Track() Track { return r.track }

// Duration returns when the track ends, in the timeline of the packets'
// Times, as the file says: by its edit list, which starts at CodecDelay,
// or else by its media header, its movie header or its sidx. It is 0 when
// the file does not say, as fragmented files may not. The audio of the
// last packet after it is the encoder's padding.
func (r *Reader) Duration() time.Duration { return r.duration }

func (r *Reader) readHeaders() error {
	s := r.src
	h, err := s.readHeader()
	if err != nil {
		return eofFormat(err, "empty file")
	}
	switch h.typ {
	case typeFtyp, typeMoov, typeMdat, typeFree, typeSkip, typeWide, typePdin, typeUUID:
	default:
		return fmt.Errorf("%w: not an MP4 file", ErrFormat)
	}

	// The boxes up to the moov, which may come after the media data.
	for h.typ != typeMoov {
		if h.size == toEnd {
			return fmt.Errorf("%w: no moov", ErrFormat)
		}
		s.skip(h.size)
		if h, err = s.readHeader(); err != nil {
			return eofFormat(err, "no moov")
		}
	}
	moov, err := s.readBody(h)
	if err != nil {
		return err
	}
	m, err := r.parseMoov(moov)
	if err != nil {
		return err
	}
	r.fragStart = h.end()
	if r.fragmented && r.fragStart >= 0 {
		if err := r.readAfterMoov(); err != nil {
			return err
		}
	}
	if !r.fragmented || r.fragStart < 0 {
		r.fragStart, r.scanAt, r.scanned = -1, -1, true
	}
	r.setDuration(m)
	r.inTable = true
	r.frag, r.after = fragment{start: r.table.end, end: r.table.end}, r.fragStart
	return nil
}

// readAfterMoov reads the boxes after the moov of a fragmented file up
// to the first moof: the sidx of the track, which it keeps, and the mdat
// of the samples of the moov, if any, after which the fragments start.
func (r *Reader) readAfterMoov() error {
	s := r.src
	r.known = r.table.end
	// start sets where the fragments start, which a scan starts at without
	// a sidx.
	start := func(at int64) error {
		r.fragStart = at
		if r.indexed == 0 {
			r.scanAt = at
		}
		return nil
	}
	for {
		at := s.offset()
		h, err := s.readHeader()
		if err != nil {
			if isDataError(err) {
				// No fragments, or a corrupt one, which ReadPacket finds.
				return start(at)
			}
			return err
		}
		switch h.typ {
		case typeMoof:
			return start(h.start)
		case typeMdat:
			return start(h.end())
		case typeSidx:
			if r.indexed > 0 {
				break
			}
			b, err := s.readBody(h)
			switch {
			case err == nil:
				r.setIndex(b, s.offset())
				continue
			case !isDataError(err):
				return err
			}
		}
		if h.size == toEnd {
			return start(-1)
		}
		s.seekTo(h.end())
	}
}

// setDuration sets the duration of the track, in the packets' timeline,
// from the first of the durations the file gives: of the edit list, of
// the fragments, of the media, of the movie, of the sidx when it indexes
// the file to its end, or of the sample table. Only the sidx may cost a
// read, at the end of what it indexes.
func (r *Reader) setDuration(m movie) {
	// movie returns when the track ends after a duration of the movie,
	// which counts from the edit.
	movie := func(v uint64) (int64, bool) {
		t, ok := rescale(v, m.scale, r.track.Timescale, r.maxTicks-r.delay)
		return r.delay + t, ok && v > 0
	}
	ticks, ok := movie(m.edits)
	if !ok {
		ticks, ok = movie(m.fragments)
	}
	if !ok && m.media > 0 && m.media <= uint64(r.maxTicks) {
		ticks, ok = int64(m.media), true
	}
	if !ok {
		ticks, ok = movie(m.duration)
	}
	if !ok && r.indexed > 0 && r.indexesAll() {
		ticks, ok = r.known, true
	}
	if !ok && r.table.end > 0 {
		ticks, ok = r.table.end, true
	}
	if ok {
		r.duration = r.time(ticks)
	}
}

// ReadPacket returns the next packet of the track, or io.EOF after the
// last. A file that ends within a sample, or within the moof of one,
// returns io.ErrUnexpectedEOF. After an error, ReadPacket returns it again
// until a Seek. In a fragmented file, a file chained after the first, which
// starts with an ftyp or a moov, ends the stream.
func (r *Reader) ReadPacket() (Packet, error) {
	if r.err != nil {
		return Packet{}, r.err
	}
	s, err := r.nextSample()
	var data []byte
	if err == nil {
		data, err = r.readSample(s)
	}
	if err != nil {
		r.err = err
		return Packet{}, err
	}
	return Packet{Data: data, Time: r.time(s.time), Duration: r.time(int64(s.duration))}, nil
}

// nextSample returns where the next sample is: in the sample table, then
// in the fragments.
func (r *Reader) nextSample() (sample, error) {
	if r.inTable {
		if s, ok := r.table.next(); ok {
			return s, nil
		}
		r.inTable = false
		if !r.fragmented {
			return sample{}, io.EOF
		}
	}
	for {
		if s, ok := r.frag.next(); ok {
			return s, nil
		}
		if err := r.nextFragment(); err != nil {
			return sample{}, err
		}
	}
}

// readSample reads the data of sample s.
func (r *Reader) readSample(s sample) ([]byte, error) {
	if s.size > maxBox {
		return nil, fmt.Errorf("%w: sample of %d bytes at byte %d", ErrFormat, s.size, s.pos)
	}
	r.src.seekTo(s.pos)
	b, err := r.src.read(int(s.size))
	if err != nil {
		return nil, err
	}
	return b, nil
}

// eofFormat turns the end of the file, where something is missing, into
// an ErrFormat.
func eofFormat(err error, missing string) error {
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return fmt.Errorf("%w: %s", ErrFormat, missing)
	}
	return err
}

// isDataError reports whether err is about the data of the file rather
// than the reading of it.
func isDataError(err error) bool {
	return errors.Is(err, ErrFormat) || err == io.EOF || err == io.ErrUnexpectedEOF
}

// ticks converts a time to the last tick whose time, as the Reader gives
// it, is at or before t: the time of tick k is k seconds over the
// timescale, rounded down to a nanosecond. It is -1 before tick 0.
func (r *Reader) ticks(t time.Duration) int64 {
	if t < 0 {
		return -1
	}
	if t >= maxSeconds*time.Second {
		return r.maxTicks
	}
	u := int64(t) + 1
	secs, rem := u/int64(time.Second), u%int64(time.Second)
	if rem == 0 {
		return secs*r.scale - 1
	}
	return secs*r.scale + (rem*r.scale-1)/int64(time.Second)
}

// time converts ticks, at most maxTicks, to a time, rounded down.
func (r *Reader) time(ticks int64) time.Duration {
	secs, rem := ticks/r.scale, ticks%r.scale
	return time.Duration(secs)*time.Second + time.Duration(rem*int64(time.Second)/r.scale)
}
