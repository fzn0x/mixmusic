package audio

import (
	"io"
	"time"
)

// Stretches reads stretches of the audio of a file, mixed down to mono at
// Rate, to measure it rather than play it: where a song starts in its
// music video, to time the song's lyrics to the video (#565). It decodes
// as a deck does, from the same frame a seek to the same time plays from,
// so that what it measures keeps time with what plays. Its methods are
// not safe for concurrent use.
type Stretches struct {
	tr   track
	tm   timing
	mono []float32
}

// OpenStretches reads the headers of a file, as a deck opens it: any file
// a deck plays. Its errors are worded for the player.
func OpenStretches(src io.ReadSeeker) (*Stretches, error) {
	tr, tm, err := openTrack(src)
	if err != nil {
		return nil, err
	}
	if tm.live {
		return nil, errUnsupported // no stretch of it stays
	}
	return &Stretches{tr: tr, tm: tm}, nil
}

// Length returns the length of the audio, as the file tells it: 0 when it
// doesn't.
func (s *Stretches) Length() time.Duration {
	if s.tm.total <= s.tm.delay {
		return 0
	}
	return s.tm.total - s.tm.delay
}

// Read decodes the audio from from to to, in the time of the track, and
// gives it to fn as it comes, a packet's worth at a time, mixed down to
// mono at Rate: fn must not keep the slice. It stops at the end of the
// file, before to when the file ends first.
func (s *Stretches) Read(from, to time.Duration, fn func(mono []float32)) error {
	ch := s.tr.channels()
	target := from + s.tm.delay
	if err := s.tr.seek(max(0, target-s.tm.preRoll)); err != nil {
		return readError(err)
	}
	// next is the frame of the stream, codec delay included, of the next
	// frame decoded, once known, as a deck counts them; the frames from
	// skip to end are those of the stretch.
	next, skip, end := int64(-1), frames(target), frames(to+s.tm.delay)
	// give gives fn the frames of pcm from a to b, of those from skip to
	// end, and reports whether the stretch goes on after them.
	give := func(pcm []float32, a, b int64) bool {
		if next < skip {
			a = max(a, min(skip-next, b))
		}
		b = min(b, max(a, end-next))
		next += int64(len(pcm) / ch)
		if a < b {
			fn(s.mix(pcm[int64(ch)*a:int64(ch)*b], ch))
		}
		return next < end
	}
	for {
		err := s.tr.read()
		if err == io.EOF {
			if pcm := s.tr.end(); len(pcm) > 0 && next >= 0 {
				give(pcm, 0, int64(len(pcm)/ch))
			}
			return nil
		}
		if err != nil {
			return readError(err)
		}
		p, err := s.tr.decode()
		if err != nil {
			return err
		}
		if next < 0 {
			next = p.at
		}
		if !give(p.pcm, int64(p.from), int64(p.to)) {
			return nil
		}
	}
}

// mix mixes the interleaved frames of pcm, of ch channels, down to mono:
// the mean of the channels.
func (s *Stretches) mix(pcm []float32, ch int) []float32 {
	n := len(pcm) / ch
	if cap(s.mono) < n {
		s.mono = make([]float32, n)
	}
	out := s.mono[:n]
	scale := 1 / float32(ch)
	for i := range out {
		var v float32
		for _, x := range pcm[i*ch : (i+1)*ch] {
			v += x
		}
		out[i] = v * scale
	}
	return out
}
