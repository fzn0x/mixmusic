package audio

import (
	"io"
	"math"
	"time"
)

// Format is what a file of audio holds, as a deck reads it: Track info
// shows it of the file a track plays from, and of what the engine plays.
type Format struct {
	// Container is the kind of file: "MP4", "WebM", "Ogg", "FLAC", "WAV",
	// "MP3", or "ADTS" for the packed audio of a live stream.
	Container string
	// Codec is the audio's: "AAC", "AC-3", "E-AC-3", "Opus", "Vorbis",
	// "FLAC", "PCM" or "MP3".
	Codec string
	// SampleRate is the audio's own, in Hz, which the engine converts to
	// Rate. Opus codes at 48 kHz, whatever its file tells of what it was
	// made from.
	SampleRate int
	// BitDepth is the bits of a sample of FLAC and WAV, whose samples
	// keep them; 0 for the other codecs, whose samples have none. Float
	// is set for a WAV file of floating-point samples.
	BitDepth int
	Float    bool
	// Channels is how many channels the audio has: 1, 2, or 6 for 5.1
	// surround; 0 when the deck doesn't tell, as of an MP3 it plays, which
	// its decoder gives in stereo.
	Channels int
	// Duration is how long the audio plays, 0 when the file doesn't tell,
	// as a live stream's.
	Duration time.Duration
}

// Inspect reads the headers of a file of audio as a deck opens it, and
// returns what it holds, or why a deck can't play it, wrapping
// ErrCantPlay. As Probe does, it reads only the first frame of an MP3
// file, where a deck reads the header of every frame.
func Inspect(src io.ReadSeeker) (Format, error) {
	kind, err := sniff(src)
	if err != nil {
		return Format{}, err
	}
	if kind == mp3File {
		return inspectMP3(src)
	}
	tr, tm, err := openTrack(src)
	if err != nil {
		return Format{}, cantPlay(err)
	}
	f := describe(tr)
	f.Duration = max(0, tm.total-tm.delay)
	return f, nil
}

// inspectMP3 is Inspect of an MP3 file: its length as Probe reads it, and
// its rate and channels as its first frame's header tells them.
func inspectMP3(src io.ReadSeeker) (Format, error) {
	length, err := probeMP3(src)
	if err != nil {
		return Format{}, err
	}
	f := Format{Container: "MP3", Codec: "MP3", Duration: length}
	f.SampleRate, f.Channels = mp3Frame(src)
	return f, nil
}

// mp3Frame reads the header of an MP3 file's first frame, past its ID3v2
// tag, as readMP3Header finds it: the sample rate and the channels it
// tells, or zeros when there's no frame there.
func mp3Frame(src io.ReadSeeker) (rate, channels int) {
	var h [10]byte
	at := int64(0)
	if _, err := readAt(src, 0, h[:]); err != nil {
		return 0, 0
	}
	if string(h[:3]) == "ID3" && h[6]|h[7]|h[8]|h[9] < 0x80 {
		at = 10 + (int64(h[6])<<21 | int64(h[7])<<14 | int64(h[8])<<7 | int64(h[9]))
		if h[5]&0x10 != 0 {
			at += 10
		}
	}
	var f [4]byte
	if _, err := readAt(src, at, f[:]); err != nil || !mpegFrame(f[:]) {
		return 0, 0
	}
	// The rates of MPEG-1; MPEG-2 has half of each, and MPEG-2.5 a
	// quarter.
	rate = [3]int{44100, 48000, 32000}[f[2]>>2&3]
	switch f[1] >> 3 & 3 {
	case 2:
		rate /= 2
	case 0:
		rate /= 4
	}
	channels = 2
	if f[3]>>6 == 3 {
		channels = 1
	}
	return rate, channels
}

// describe tells what a track a deck opened holds: its file's kind, its
// codec, rate, bit depth and channels. It reads only what the track's
// headers told, before any packet: the deck's goroutine calls it as it
// opens the track.
func describe(tr track) Format {
	switch t := tr.(type) {
	case *mp4Track:
		mt := t.r.Track()
		f := Format{Container: "MP4", Codec: "AAC", SampleRate: int(t.rate), Channels: mt.Channels}
		switch mt.Codec {
		case "ac-3":
			f.Codec = "AC-3"
		case "ec-3":
			f.Codec = "E-AC-3"
		}
		return f
	case *packedTrack:
		return Format{Container: "ADTS", Codec: "AAC", SampleRate: t.cfg.SampleRate, Channels: t.cfg.Channels}
	case *opusTrack:
		f := Format{Container: "WebM", Codec: "Opus", SampleRate: Rate}
		if h, err := parseOpusHead(t.r.Track().CodecPrivate); err == nil {
			f.Channels = h.channels
		}
		return f
	case *oggOpusTrack:
		return Format{Container: "Ogg", Codec: "Opus", SampleRate: Rate, Channels: t.ch}
	case *pcmTrack:
		f := Format{SampleRate: int(t.conv.rate), Channels: t.in}
		switch c := t.codec.(type) {
		case *flacCodec:
			f.Container, f.Codec, f.BitDepth = "FLAC", "FLAC", c.r.Info().BitsPerSample
		case *wavCodec:
			f.Container, f.Codec, f.BitDepth, f.Float = "WAV", "PCM", 8*c.size, c.float
		case *vorbisCodec:
			f.Container, f.Codec = "Ogg", "Vorbis"
		case *mp3Codec:
			f.Container, f.Codec, f.Channels = "MP3", "MP3", 0
		}
		return f
	}
	return Format{}
}

// format returns what the deck's track holds, and whether the deck read
// its headers yet.
func (d *deck) format() (Format, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.holds, d.ready
}

// Loaded is what the engine's active deck plays: the track it loaded,
// what its file holds, and how loud it plays it.
type Loaded struct {
	// URL is the address the deck loaded, as Load or Crossfade had it.
	URL string
	// Format is what the track's file holds, once Ready: the deck read
	// its headers.
	Format Format
	Ready  bool
	// LKFS is the track's loudness, when Measured: its source told it.
	LKFS     float64
	Measured bool
	// EvenLoudness and Levelling tell whether Even loudness and Volume
	// levelling are on.
	EvenLoudness, Levelling bool
	// Trim is the gain, in dB, that Even loudness plays the track at: 0,
	// or less for a track louder than its target. It is 0 while Even
	// loudness is off, or the track's loudness is unknown.
	Trim float64
}

// Loaded returns what the active deck plays, and false while no deck
// does.
func (e *Engine) Loaded() (Loaded, bool) {
	e.mu.Lock()
	d := e.decks[e.active]
	l := Loaded{EvenLoudness: e.loudness, Levelling: e.normalize}
	e.mu.Unlock()
	if d == nil {
		return Loaded{}, false
	}
	l.URL = d.url
	l.Format, l.Ready = d.format()
	if lkfs := math.Float64frombits(d.lkfs.Load()); !math.IsNaN(lkfs) {
		l.LKFS, l.Measured = lkfs, true
		if l.EvenLoudness {
			l.Trim = loudnessTrim(lkfs)
		}
	}
	return l, true
}

// LoudnessTrim returns the gain, in dB, that Even loudness plays a track
// of lkfs at, as YouTube measured it: 0, or less for a track louder than
// the target.
func LoudnessTrim(lkfs float64) float64 { return loudnessTrim(lkfs) }
