package webm

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"time"
)

// Lacing, in bits 1 and 2 of the flags of a block, is how a block holds
// several frames.
const (
	lacingNone  = 0
	lacingXiph  = 1
	lacingFixed = 2
	lacingEBML  = 3
)

// flagKeyframe is the flag of a SimpleBlock that holds keyframes only.
const flagKeyframe = 0x80

var (
	errLacing = fmt.Errorf("%w: invalid lacing", ErrFormat)
	errBlock  = fmt.Errorf("%w: block too short", ErrFormat)
)

// ReadPacket returns the next packet of the track, or io.EOF after the
// last. A file that ends before its Segment does returns
// io.ErrUnexpectedEOF. After an error, ReadPacket returns it again until a
// Seek. A Segment chained after the first, which may have other tracks,
// ends the stream.
func (r *Reader) ReadPacket() (Packet, error) {
	for r.qi == len(r.queue) {
		if r.err != nil {
			return Packet{}, r.err
		}
		r.queue, r.qi = r.queue[:0], 0
		r.err = r.next()
	}
	p := r.queue[r.qi]
	r.queue[r.qi] = Packet{} // the caller's now
	r.qi++
	return p, nil
}

// next reads the next element, of the Segment or of the cluster being
// read, queueing the frames of the track's blocks.
func (r *Reader) next() error {
	if !r.inCluster {
		return r.nextCluster()
	}
	h, ok, err := r.clusterChild()
	if err != nil || !ok {
		return err
	}
	switch h.id {
	case idTimestamp:
		return r.readTimestamp(h)
	case idSimpleBlock:
		return r.readSimpleBlock(h)
	case idBlockGroup:
		return r.readBlockGroup(h)
	}
	r.src.skip(h.size)
	return nil
}

// nextCluster reads the next element of the Segment, entering it if it is
// a Cluster and skipping it otherwise.
func (r *Reader) nextCluster() error {
	s := r.src
	if r.segEnd >= 0 && s.offset() >= r.segEnd {
		return io.EOF
	}
	h, err := s.readHeader()
	if err != nil {
		return endError(err, r.segEnd)
	}
	switch h.id {
	case idCluster:
		if h.size != unknownSize {
			if err := checkChild(h, r.segEnd); err != nil {
				return err
			}
		}
		r.inCluster, r.clusterEnd, r.clusterTime = true, h.end(), 0
		return nil
	case idEBML, idSegment:
		// A chained Segment, whose tracks may differ: the stream ends.
		s.seekTo(h.start)
		return io.EOF
	}
	if err := checkChild(h, r.segEnd); err != nil {
		return err
	}
	s.skip(h.size)
	return nil
}

// clusterChild reads the header of the next element of the cluster being
// read, or returns false at the end of the cluster. A cluster of unknown
// size ends at an element that can only be in a Segment, at the end of the
// Segment, or at the end of the file.
func (r *Reader) clusterChild() (header, bool, error) {
	s := r.src
	end := r.clusterEnd
	if end < 0 {
		end = r.segEnd
	}
	if end >= 0 && s.offset() >= end {
		r.inCluster = false
		return header{}, false, nil
	}
	h, err := s.readHeader()
	if err == io.EOF && end < 0 {
		r.inCluster = false
		return header{}, false, nil
	}
	if err != nil {
		return header{}, false, endError(err, end)
	}
	if r.clusterEnd < 0 && isLevel1(h.id) {
		s.seekTo(h.start)
		r.inCluster = false
		return header{}, false, nil
	}
	if err := checkChild(h, end); err != nil {
		return header{}, false, err
	}
	return h, true, nil
}

// isLevel1 reports whether elements of ID id can only be in a Segment, or
// start one: they end a Cluster of unknown size.
func isLevel1(id uint32) bool {
	switch id {
	case idCluster, idCues, idSeekHead, idInfo, idTracks, idTags, idChapters, idAttachments, idEBML, idSegment:
		return true
	}
	return false
}

// endError returns err, met reading the next element of a parent that ends
// at end, as io.ErrUnexpectedEOF when it is io.EOF and that end is known:
// the file ends before its parent does.
func endError(err error, end int64) error {
	if end >= 0 {
		return unexpected(err)
	}
	return err
}

// readTimestamp reads the Timestamp of the cluster.
func (r *Reader) readTimestamp(h header) error {
	v, err := r.src.readUintElement(h)
	if err != nil {
		return err
	}
	// Leave room for the timestamps of the blocks, relative to it: as the
	// scale is at most maxScale, the bound is positive.
	if v > uint64(math.MaxInt64/r.scale-math.MaxInt16) {
		return fmt.Errorf("%w: cluster timestamp %d at byte %d", ErrFormat, v, h.start)
	}
	r.clusterTime = int64(v)
	return nil
}

// readSimpleBlock queues the frames of a SimpleBlock of the track, and
// skips those of other tracks without reading them.
func (r *Reader) readSimpleBlock(h header) error {
	b, err := r.src.peek(int(min(h.size, 8)))
	if err != nil {
		return err
	}
	track, _, err := readVint(b)
	if err == errShort {
		err = errBlock
	}
	if err != nil {
		return err
	}
	if track != r.track.Number {
		r.src.skip(h.size)
		return nil
	}
	if b, err = r.src.readBody(h); err != nil {
		return err
	}
	return r.queueBlock(b, true, -1, 0, false)
}

// readBlockGroup queues the frames of the Block of a BlockGroup of the
// track.
func (r *Reader) readBlockGroup(h header) error {
	b, err := r.src.readBody(h)
	if err != nil {
		return err
	}
	var block []byte
	var duration, padding int64 = -1, 0
	reference := false
	err = eachChild(b, func(id uint32, data []byte) error {
		var err error
		switch id {
		case idBlock:
			if block == nil {
				block = data
			}
		case idBlockDuration:
			var v uint64
			if v, err = readUint(data); v <= math.MaxInt64 {
				duration = int64(v)
			}
		case idReferenceBlock:
			reference = true
		case idDiscardPadding:
			padding, err = readInt(data)
		}
		return err
	})
	if err != nil || block == nil {
		return err
	}
	return r.queueBlock(block, false, duration, time.Duration(padding), reference)
}

// queueBlock queues the frames of block b, if it is of the track. A
// SimpleBlock says whether it holds keyframes; the Block of a BlockGroup
// does by the absence of ReferenceBlock. duration is the BlockDuration in
// ticks, or -1.
func (r *Reader) queueBlock(b []byte, simple bool, duration int64, padding time.Duration, reference bool) error {
	track, n, err := readVint(b)
	if err == errShort || err == nil && len(b) < n+3 {
		return errBlock
	}
	if err != nil {
		return err
	}
	if track != r.track.Number {
		return nil
	}
	ticks := r.clusterTime + int64(int16(binary.BigEndian.Uint16(b[n:])))
	flags := b[n+2]
	frames, err := unlace(flags>>1&3, b[n+3:])
	if err != nil {
		return err
	}
	p := Packet{Time: time.Duration(ticks * r.scale), Keyframe: !reference}
	if simple {
		p.Keyframe = flags&flagKeyframe != 0
	}
	if d, ok := r.ticks(uint64(duration)); duration >= 0 && ok && len(frames) == 1 {
		p.Duration = d
	}
	for i, f := range frames {
		p.Data, p.DiscardPadding = f, 0
		if padding > 0 && i == len(frames)-1 || padding < 0 && i == 0 {
			p.DiscardPadding = padding
		}
		r.queue = append(r.queue, p)
	}
	return nil
}

// unlace splits the data of a block, after its header, into its frames.
func unlace(lacing byte, b []byte) ([][]byte, error) {
	if lacing == lacingNone {
		return [][]byte{b}, nil
	}
	if len(b) == 0 {
		return nil, errLacing
	}
	count := int(b[0]) + 1
	b = b[1:]
	sizes := make([]int, count)
	switch lacing {
	case lacingXiph:
		// Each size but the last's is a run of 255s ended by a smaller byte,
		// which add up.
		for i := range count - 1 {
			for {
				if len(b) == 0 || sizes[i] > maxElement {
					return nil, errLacing
				}
				c := b[0]
				b = b[1:]
				sizes[i] += int(c)
				if c != 255 {
					break
				}
			}
		}
	case lacingEBML:
		// The first size is a variable-size integer, those after it but the
		// last's signed ones, which add to the size before them.
		size := int64(0)
		for i := range count - 1 {
			v, n, err := readVint(b)
			if err != nil {
				return nil, errLacing
			}
			b = b[n:]
			if i == 0 {
				size = int64(v)
			} else {
				size += int64(v) - (1<<(7*n-1) - 1)
			}
			if size < 0 || size > maxElement {
				return nil, errLacing
			}
			sizes[i] = int(size)
		}
	case lacingFixed:
		if len(b)%count != 0 {
			return nil, errLacing
		}
		for i := range sizes {
			sizes[i] = len(b) / count
		}
	}
	if lacing != lacingFixed {
		rest := len(b)
		for _, size := range sizes[:count-1] {
			if rest -= size; rest < 0 {
				return nil, errLacing
			}
		}
		sizes[count-1] = rest
	}
	frames := make([][]byte, count)
	for i, size := range sizes {
		frames[i] = b[:size:size]
		b = b[size:]
	}
	return frames, nil
}
