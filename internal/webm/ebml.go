package webm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/bits"
)

// The IDs of the elements the Reader reads, marker bits included, as the
// EBML and Matroska specifications number them.
const (
	idEBML            = 0x1A45DFA3
	idEBMLReadVersion = 0x42F7
	idDocType         = 0x4282

	idSegment = 0x18538067

	idSeekHead     = 0x114D9B74
	idSeek         = 0x4DBB
	idSeekID       = 0x53AB
	idSeekPosition = 0x53AC

	idInfo           = 0x1549A966
	idTimestampScale = 0x2AD7B1
	idDuration       = 0x4489

	idTracks            = 0x1654AE6B
	idTrackEntry        = 0xAE
	idTrackNumber       = 0xD7
	idTrackType         = 0x83
	idCodecID           = 0x86
	idCodecPrivate      = 0x63A2
	idCodecDelay        = 0x56AA
	idSeekPreRoll       = 0x56BB
	idAudio             = 0xE1
	idSamplingFrequency = 0xB5
	idChannels          = 0x9F
	idContentEncodings  = 0x6D80

	idCues               = 0x1C53BB6B
	idCuePoint           = 0xBB
	idCueTime            = 0xB3
	idCueTrackPositions  = 0xB7
	idCueTrack           = 0xF7
	idCueClusterPosition = 0xF1

	idCluster        = 0x1F43B675
	idTimestamp      = 0xE7
	idSimpleBlock    = 0xA3
	idBlockGroup     = 0xA0
	idBlock          = 0xA1
	idBlockDuration  = 0x9B
	idReferenceBlock = 0xFB
	idDiscardPadding = 0x75A2

	idTags        = 0x1254C367
	idChapters    = 0x1043A770
	idAttachments = 0x1941A469
)

// trackAudio is the TrackType of audio tracks.
const trackAudio = 2

// maxHeaderLen is the length of the longest element header the Reader
// reads: a 4-byte ID and an 8-byte size.
const maxHeaderLen = 12

// unknownSize is the size of an element its muxer did not know the size
// of, as of the Segment and Clusters of a live stream.
const unknownSize = -1

var (
	// errShort is returned by the parsers of headers given too few bytes.
	errShort = errors.New("webm: short header")

	errVint     = fmt.Errorf("%w: invalid variable-size integer", ErrFormat)
	errID       = fmt.Errorf("%w: invalid element ID", ErrFormat)
	errOverflow = fmt.Errorf("%w: an element overflows its parent", ErrFormat)
)

// readVint reads a variable-size integer: as many bytes as the leading
// zero bits of the first, plus one, up to 8, hold its value after the
// first 1 bit, the marker.
func readVint(b []byte) (v uint64, n int, err error) {
	if len(b) == 0 {
		return 0, 0, errShort
	}
	n = bits.LeadingZeros8(b[0]) + 1
	if n > 8 {
		return 0, 0, errVint
	}
	if len(b) < n {
		return 0, 0, errShort
	}
	v = uint64(b[0] & (0xFF >> n))
	for _, c := range b[1:n] {
		v = v<<8 | uint64(c)
	}
	return v, n, nil
}

// readID reads an element ID, which keeps its marker bit, of up to 4
// bytes as in Matroska.
func readID(b []byte) (id uint32, n int, err error) {
	if len(b) == 0 {
		return 0, 0, errShort
	}
	n = bits.LeadingZeros8(b[0]) + 1
	if n > 4 {
		return 0, 0, errID
	}
	if len(b) < n {
		return 0, 0, errShort
	}
	for _, c := range b[:n] {
		id = id<<8 | uint32(c)
	}
	// The bits after the marker are neither all zeros nor all ones.
	marker := uint32(1) << (7 * n)
	if v := id &^ marker; v == 0 || v == marker-1 {
		return 0, 0, errID
	}
	return id, n, nil
}

// readSize reads the size of an element: a variable-size integer, whose
// value bits all set mean that the size is unknown.
func readSize(b []byte) (size int64, n int, err error) {
	v, n, err := readVint(b)
	if err != nil {
		return 0, 0, err
	}
	if v == 1<<(7*n)-1 {
		return unknownSize, n, nil
	}
	return int64(v), n, nil
}

// parseHeader reads the header of an element: its ID and the size of its
// data, and returns the length of the header.
func parseHeader(b []byte) (id uint32, size int64, n int, err error) {
	id, n, err = readID(b)
	if err != nil {
		return 0, 0, 0, err
	}
	size, m, err := readSize(b[n:])
	if err != nil {
		return 0, 0, 0, err
	}
	return id, size, n + m, nil
}

// eachChild calls f with the ID and the data of each element in b, the
// data of a master element read into memory, until f returns an error.
func eachChild(b []byte, f func(id uint32, data []byte) error) error {
	for len(b) > 0 {
		id, size, n, err := parseHeader(b)
		if err == errShort || err == nil && (size == unknownSize || size > int64(len(b)-n)) {
			return errOverflow
		}
		if err != nil {
			return err
		}
		end := n + int(size)
		if err := f(id, b[n:end:end]); err != nil {
			return err
		}
		b = b[end:]
	}
	return nil
}

// readUint reads an unsigned integer element: empty means 0.
func readUint(b []byte) (uint64, error) {
	if len(b) > 8 {
		return 0, fmt.Errorf("%w: integer of %d bytes", ErrFormat, len(b))
	}
	var v uint64
	for _, c := range b {
		v = v<<8 | uint64(c)
	}
	return v, nil
}

// readInt reads a signed integer element.
func readInt(b []byte) (int64, error) {
	v, err := readUint(b)
	if err != nil || len(b) == 0 {
		return 0, err
	}
	shift := 64 - 8*len(b)
	return int64(v<<shift) >> shift, nil
}

// readFloat reads a float element, of 4 or 8 bytes; empty means 0.
func readFloat(b []byte) (float64, error) {
	switch len(b) {
	case 0:
		return 0, nil
	case 4:
		return float64(math.Float32frombits(binary.BigEndian.Uint32(b))), nil
	case 8:
		return math.Float64frombits(binary.BigEndian.Uint64(b)), nil
	}
	return 0, fmt.Errorf("%w: float of %d bytes", ErrFormat, len(b))
}

// readString reads a string element, which may be padded with zeros.
func readString(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}
