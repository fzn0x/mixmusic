package audio

import (
	"io"
	"time"

	"github.com/fzn0x/mixmusic/opus"
	"github.com/fzn0x/mixmusic/webm"
)

// track is the audio track of a file as a deck plays it: the packets its
// reader reads, decoded into frames of its channels at Rate.
type track interface {
	// channels returns the channels of the frames it decodes: 2, or 6 for
	// 5.1 surround, in the order of ac3's Left to RightSurround.
	channels() int
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

// decoded is the audio of a packet: the frames of pcm, of the track's
// channels interleaved, at Rate, which follow those decoded before since
// the last seek. Those from from to to play; the others are padding.
type decoded struct {
	pcm []float32
	// downmix is, of a surround track, the stereo downmix its stream asks
	// for, as ac3.Info's Downmix: the same pointer while it doesn't
	// change. Nil for stereo.
	downmix *[2][6]float32
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
// audio track, and when it plays: the AAC, AC-3 or E-AC-3 of an MP4 file,
// the AAC of HLS packed audio, the audio of a file kept on a computer,
// FLAC, Opus or Vorbis in Ogg, WAV or MP3, else the Opus of a WebM file.
// Its errors are worded for the player.
func openTrack(src io.ReadSeeker) (track, timing, error) {
	kind, err := sniff(src)
	if err != nil {
		return nil, timing{}, readError(err)
	}
	switch kind {
	case mp4File:
		return openMP4(src)
	case packedAudio:
		return openPacked(src)
	case flacFile:
		return openFLAC(src)
	case oggFile:
		return openOgg(src)
	case wavFile:
		return openWAV(src)
	case mp3File:
		return openMP3(src)
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
	// The files kept on a computer, as the local library holds them.
	flacFile
	oggFile
	wavFile
	mp3File
)

// sniff tells the kind of a file by its first bytes, and goes back to its
// start: an MP4 file by the type of its first box, at byte 4, packed
// audio by the syncword of an ADTS frame, FLAC, Ogg and WAV files by
// their magic numbers, and MP3 by the syncword of an MPEG audio frame.
// After an ID3 tag, as packed audio and MP3 files start with, the first
// frame tells them apart, as it tells a FLAC file with a tag before it;
// what else follows a tag is packed audio. A WebM file starts with
// EBML's magic number, 1A 45 DF A3, instead.
func sniff(src io.ReadSeeker) (container, error) {
	var head [12]byte
	n, err := io.ReadFull(src, head[:])
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return webmFile, err
	}
	kind := webmFile
	switch {
	case n >= 10 && string(head[:3]) == "ID3":
		if kind, err = sniffTagged(src, head[:]); err != nil {
			return webmFile, err
		}
	case n >= 4 && string(head[:4]) == "fLaC":
		kind = flacFile
	case n >= 4 && string(head[:4]) == "OggS":
		kind = oggFile
	case n >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WAVE":
		kind = wavFile
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return webmFile, err
	}
	if kind != webmFile {
		return kind, nil
	}
	switch {
	case n >= 3 && string(head[:3]) == "ID3", n >= 2 && head[0] == 0xFF && head[1]&0xF6 == 0xF0:
		return packedAudio, nil
	case n >= 4 && mpegFrame(head[:4]):
		return mp3File, nil
	case n < 8:
		return webmFile, nil // the WebM reader tells what is wrong
	}
	switch string(head[4:8]) {
	case "ftyp", "styp", "moov", "mdat", "free", "skip", "wide", "pdin", "uuid":
		return mp4File, nil
	}
	return webmFile, nil
}

// sniffTagged tells the kind of a file that starts with the ID3 tag whose
// header head starts with, by what follows the tag: FLAC, MP3, else
// packed audio. A small tag, as those of packed audio, is read through;
// past a large one, as an MP3 file's with its cover, the file seeks.
func sniffTagged(src io.ReadSeeker, head []byte) (container, error) {
	if head[6]|head[7]|head[8]|head[9] >= 0x80 {
		return packedAudio, nil
	}
	size := int64(head[6])<<21 | int64(head[7])<<14 | int64(head[8])<<7 | int64(head[9])
	if head[5]&0x10 != 0 {
		size += 10 // a footer
	}
	var next [4]byte
	switch rest := 10 + size - int64(len(head)); {
	case rest <= -4:
		copy(next[:], head[10+size:])
	case rest <= 4096:
		if _, err := io.CopyN(io.Discard, src, max(0, rest)); err != nil {
			return packedAudio, nil
		}
		k := copy(next[:], head[min(int64(len(head)), 10+size):])
		if _, err := io.ReadFull(src, next[k:]); err != nil {
			return packedAudio, nil
		}
	default:
		if _, err := src.Seek(10+size, io.SeekStart); err != nil {
			return webmFile, err
		}
		if _, err := io.ReadFull(src, next[:]); err != nil {
			return packedAudio, nil
		}
	}
	switch {
	case string(next[:]) == "fLaC":
		return flacFile, nil
	case next[0] == 0xFF && next[1]&0xF6 == 0xF0:
		return packedAudio, nil
	case mpegFrame(next[:]):
		return mp3File, nil
	}
	return packedAudio, nil
}

// mpegFrame reports whether b starts the header of an MPEG audio frame of
// Layer III: its syncword, and a version, a bitrate and a sampling
// frequency that aren't reserved.
func mpegFrame(b []byte) bool {
	return b[0] == 0xFF && b[1]&0xE0 == 0xE0 && b[1]>>3&3 != 1 && b[1]>>1&3 == 1 &&
		b[2]>>4 != 15 && b[2]>>2&3 != 3
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

func (t *opusTrack) channels() int { return 2 }

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
