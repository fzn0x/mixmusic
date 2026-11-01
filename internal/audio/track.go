package audio

import (
	"io"
	"time"

	"github.com/fzn0x/mixmusic/opus"
	"github.com/fzn0x/mixmusic/webm"
)

// track is the audio track of a file as a deck plays it: the packets its
// reader reads, decoded into stereo at Rate.
type track interface {
	// seek moves the track to the last packet that starts at or before t,
	// in the time of the packets, and resets the decoder: the packet read
	// next decodes as the first of a stream. A live track can't move: the
	// packets it reads next are at t.
	seek(t time.Duration) error
	// read reads the next packet, which decode decodes: io.EOF after the
	// last one.
	read() error
	// decode decodes the packet read last. Its errors are worded for the
	// player.
	decode() (decoded, error)
	// end returns the frames the decoder holds back after the last packet,
	// at the end of the stream.
	end() []float32
}

// decoded is the audio of a packet: the frames of pcm, interleaved stereo
// at Rate, which follow those decoded before since the last seek. Those
// from from to to play; the others are padding.
type decoded struct {
	pcm []float32
	// at is, of the first packet after a seek, the frame of the stream,
	// codec delay included, of the first frame of pcm: the deck reads it of
	// that packet only.
	at       int64
	from, to int
}

// timing is when the audio of a track plays.
type timing struct {
	// delay is the codec delay: the audio of a packet plays at its time
	// less delay.
	delay time.Duration
	// preRoll is how long before a seek's target the decoding starts, for
	// the decoder to settle.
	preRoll time.Duration
	// total is the duration of the file, codec delay included, 0 when it
	// doesn't tell.
	total time.Duration
	// live is set for a live stream, which plays on from where it starts:
	// it has no length, and seeks only tell the time of what comes next.
	live bool
}

// openTrack reads the headers of a file, from its start, and returns its
// audio track, and when it plays: the AAC of an MP4 file, or of HLS packed
// audio, else the Opus of a WebM one. Its errors are worded for the
// player.
func openTrack(src io.ReadSeeker) (track, timing, error) {
	kind, err := sniff(src)
	if err != nil {
		return nil, timing{}, readError(err)
	}
	switch kind {
	case mp4File:
		return openAAC(src)
	case packedAudio:
		return openPacked(src)
	}
	r, err := webm.NewReader(src)
	if err != nil {
		return nil, timing{}, readError(err)
	}
	return openOpus(r)
}

// container is the kind of a file of audio.
type container int

const (
	webmFile container = iota
	mp4File
	// packedAudio is the AAC of HLS packed audio, in ADTS frames.
	packedAudio
)

// sniff tells the kind of a file by its first bytes, and goes back to its
// start: an MP4 file by the type of its first box, at byte 4, and packed
// audio by its ID3 tag, or the syncword of an ADTS frame. A WebM file
// starts with EBML's magic number, 1A 45 DF A3, instead.
func sniff(src io.ReadSeeker) (container, error) {
	var head [8]byte
	n, err := io.ReadFull(src, head[:])
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return webmFile, err
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return webmFile, err
	}
	switch {
	case n >= 3 && string(head[:3]) == "ID3", n >= 2 && head[0] == 0xFF && head[1]&0xF6 == 0xF0:
		return packedAudio, nil
	case n < len(head):
		return webmFile, nil // the WebM reader tells what is wrong
	}
	switch string(head[4:]) {
	case "ftyp", "styp", "moov", "mdat", "free", "skip", "wide", "pdin", "uuid":
		return mp4File, nil
	}
	return webmFile, nil
}

// opusTrack is the Opus track of a WebM file.
type opusTrack struct {
	r   *webm.Reader
	dec *opus.Decoder
	p   webm.Packet // the packet read last
	pcm []float32   // room for the longest packet, of 120 ms
}

// openOpus returns the Opus track r reads.
func openOpus(r *webm.Reader) (track, timing, error) {
	t := r.Track()
	if t.Codec != "A_OPUS" {
		return nil, timing{}, errUnsupported
	}
	head, err := parseOpusHead(t.CodecPrivate)
	if err != nil {
		return nil, timing{}, err
	}
	dec, err := opus.NewDecoder(Rate, 2)
	if err != nil {
		return nil, timing{}, errDecode
	}
	dec.SetGain(head.gain)
	// The audio of a packet plays at its time less the codec delay, which
	// is the pre-skip of the OpusHead when the file doesn't give it.
	tm := timing{delay: t.CodecDelay, preRoll: t.SeekPreRoll, total: r.Duration()}
	if tm.delay == 0 {
		tm.delay = time.Duration(head.preSkip) * time.Second / Rate
	}
	if tm.preRoll == 0 {
		tm.preRoll = 80 * time.Millisecond
	}
	return &opusTrack{r: r, dec: dec, pcm: make([]float32, 2*5760)}, tm, nil
}

func (t *opusTrack) seek(at time.Duration) error {
	if err := t.r.Seek(at); err != nil {
		return err
	}
	t.dec.Reset()
	return nil
}

func (t *opusTrack) read() error {
	var err error
	t.p, err = t.r.ReadPacket()
	return err
}

func (t *opusTrack) decode() (decoded, error) {
	n, err := t.dec.Decode(t.p.Data, t.pcm, false)
	if err != nil {
		return decoded{}, errDecode
	}
	d := decoded{pcm: t.pcm[:2*n], at: frames(t.p.Time), to: n}
	if pad := t.p.DiscardPadding; pad > 0 {
		d.to = max(0, n-int(frames(pad)))
	} else if pad < 0 {
		d.from = int(min(frames(-pad), int64(n)))
	}
	return d, nil
}

func (t *opusTrack) end() []float32 { return nil }
