package tagwrite

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"time"

	"github.com/fzn0x/mixmusic/opus"
	"github.com/fzn0x/mixmusic/webm"
)

// A remux may leave stretches of the audio out (#735), as the parts of a
// video SponsorBlock marks, which the player passes over: whole Opus
// packets go, none is decoded, and the copy stays one Ogg stream, which
// every player reads.
//
//   - A stretch from the start goes by the OpusHead's pre-skip: the copy
//     starts at the packet preRoll before the stretch's end, which a decoder
//     decodes to settle and drops, so that the audio starts on the sample
//     where the stretch ends, as it was.
//   - A stretch that reaches the end goes by the last page's granule
//     position, which ends the audio on the sample where the stretch starts.
//   - A stretch between goes by whole packets: those whose middle is in it,
//     so that at most half a packet of the audio around it goes or stays,
//     10 ms of YouTube's 20 ms packets. The decoder takes the packet after
//     it as the one after the packet before, and settles within a few.
//
// The pages' granule positions count the samples of the packets kept, and
// their sequence numbers follow on, so that a player tells the copy's own
// length, and seeks in it.

// Cut is a stretch of the audio a remux leaves out, from From to To, from
// the start of the audio as it plays.
type Cut struct{ From, To time.Duration }

// Remux tells RemuxWebMWith what more to do than RemuxWebM: its zero value
// does nothing more.
type Remux struct {
	// Leave returns the stretches of the audio to leave out of the copy,
	// of the length of the audio the WebM file tells, 0 when it tells none:
	// a stretch that reaches that length leaves out the rest. Nil leaves out
	// nothing.
	Leave func(length time.Duration) []Cut
}

// preRoll is how much audio a decoder decodes and drops before the start
// of a copy whose start was left out, to settle: RFC 7845 asks for 80 ms
// at least, after which the audio is still a few percent off, and 200 ms
// takes it within 0.3 % of the audio as it was.
const preRoll = opusRate * 200 / 1000

// errAllCut is the error of a remux that would leave out all of the audio.
var errAllCut = errors.New("tagwrite: it would leave out all of the audio")

// cutPlan is what a remux leaves out, in samples at 48 kHz from the start
// of the stream, the pre-skip among them.
type cutPlan struct {
	// start is where the audio starts after a stretch left out from the
	// start, 0 for none.
	start int64
	// mids are the stretches between, from their first sample to the one
	// after their last, in order.
	mids [][2]int64
	// tail is where the stretch that reaches the end starts, -1 for none.
	tail int64
}

// plan is what o leaves out of an audio of length, whose pre-skip is
// preSkip: the stretches Leave returns, in order, joined where they
// overlap or touch.
func (o Remux) plan(length time.Duration, preSkip int64) cutPlan {
	p := cutPlan{tail: -1}
	if o.Leave == nil {
		return p
	}
	cuts := slices.Clone(o.Leave(length))
	slices.SortFunc(cuts, func(a, b Cut) int { return cmp.Compare(a.From, b.From) })
	for _, c := range cuts {
		from, to := samplesOf(max(c.From, 0))+preSkip, samplesOf(c.To)+preSkip
		if length > 0 && c.To >= length {
			to = math.MaxInt64
		}
		if to <= from {
			continue
		}
		switch n := len(p.mids); {
		case from <= max(p.start, preSkip):
			p.start = max(p.start, to)
		case n > 0 && from <= p.mids[n-1][1]:
			p.mids[n-1][1] = max(p.mids[n-1][1], to)
		default:
			p.mids = append(p.mids, [2]int64{from, to})
		}
	}
	if n := len(p.mids); n > 0 && p.mids[n-1][1] == math.MaxInt64 {
		p.tail, p.mids = p.mids[n-1][0], p.mids[:n-1]
	}
	return p
}

// audioPacket is an Opus packet of a WebM file: where it starts in the
// stream, in samples at 48 kHz, the pre-skip among them, and how many it
// holds.
type audioPacket struct {
	webm.Packet
	at, n int64
}

// packetSource reads the Opus packets of a WebM file in turn.
type packetSource struct {
	r  *webm.Reader
	at int64 // where the next packet starts
}

// next reads the next packet: false after the last.
func (s *packetSource) next() (audioPacket, bool, error) {
	p, err := s.r.ReadPacket()
	switch {
	case err == io.EOF:
		return audioPacket{}, false, nil
	case err != nil:
		return audioPacket{}, false, fmt.Errorf("%w: %v", ErrFormat, err)
	}
	n, err := opus.PacketSamples(p.Data, opusRate)
	if err != nil {
		return audioPacket{}, false, fmt.Errorf("%w: an Opus packet at %v is broken", ErrFormat, p.Time)
	}
	out := audioPacket{Packet: p, at: s.at, n: int64(n)}
	s.at += int64(n)
	return out, true, nil
}

// keptPackets reads the packets of a WebM file a plan keeps, each where it
// starts in the copy.
type keptPackets struct {
	src  *packetSource
	plan cutPlan
	// first is the copy's first packet, once a stretch from the start was
	// passed over, which the next read returns, kept whatever the plan.
	first *audioPacket
	// dropped counts the samples left out before the next packet.
	dropped int64
}

// skipStart passes over the packets before the copy's first, when the
// plan leaves out a stretch from the start, and returns head with the
// pre-skip from that packet to the stretch's end: head as it is when the
// plan leaves out none.
func (k *keptPackets) skipStart(head []byte) ([]byte, error) {
	if k.plan.start == 0 {
		return head, nil
	}
	if k.plan.start == math.MaxInt64 {
		return nil, errAllCut
	}
	from := max(k.plan.start-preRoll, 0)
	for {
		p, ok, err := k.src.next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errAllCut
		}
		if p.at+p.n <= from {
			continue
		}
		skip := k.plan.start - p.at
		if skip > math.MaxUint16 {
			return nil, fmt.Errorf("%w: an Opus packet at %v is too long", ErrFormat, p.Time)
		}
		k.first, k.dropped = &p, p.at
		head = bytes.Clone(head)
		binary.LittleEndian.PutUint16(head[10:], uint16(skip))
		return head, nil
	}
}

// next reads the next packet the plan keeps: false after the last.
func (k *keptPackets) next() (audioPacket, bool, error) {
	for {
		var p audioPacket
		if k.first != nil {
			p, k.first = *k.first, nil
		} else {
			var ok bool
			var err error
			if p, ok, err = k.src.next(); !ok || err != nil {
				return p, ok, err
			}
			if k.drops(p) {
				k.dropped += p.n
				continue
			}
		}
		if k.plan.tail >= 0 && p.at >= k.plan.tail {
			return audioPacket{}, false, nil
		}
		p.at -= k.dropped
		return p, true, nil
	}
}

// drops reports whether the plan leaves p out: its middle is in a stretch
// between.
func (k *keptPackets) drops(p audioPacket) bool {
	mid := p.at + p.n/2
	for _, m := range k.plan.mids {
		if mid >= m[0] && mid < m[1] {
			return true
		}
	}
	return false
}

// end is where the copy's audio ends, in samples from its start, the
// pre-skip among them, after p, its last packet, of a copy of total
// samples: short of total by what the WebM's last block drops, or at the
// start of the stretch that reaches the end, as the last page's granule
// position tells.
func (k *keptPackets) end(p audioPacket, total int64) int64 {
	end := total
	if p.DiscardPadding > 0 {
		end -= samplesOf(p.DiscardPadding)
	}
	if k.plan.tail >= 0 {
		end = min(end, k.plan.tail-k.dropped)
	}
	return end
}
