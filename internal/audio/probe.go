package audio

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/hajimehoshi/go-mp3"
)

// ErrCantPlay is wrapped by Probe's errors of a file a deck can't play:
// one of another kind, or broken.
var ErrCantPlay = errors.New("the file can't be played")

// Probe reads the start of a file kept on a computer as a deck does, and
// decodes the first of its audio: it returns how long the file plays, 0
// when the file doesn't tell, or why a deck can't play it, wrapping
// ErrCantPlay, in the words of the player. It reads little of the file:
// of MP3, the headers of its first frame tell its length, else its
// bitrate does, where a deck that plays it reads every frame's header.
func Probe(src io.ReadSeeker) (time.Duration, error) {
	kind, err := sniff(src)
	if err != nil {
		return 0, err
	}
	switch kind {
	case mp3File:
		return probeMP3(src)
	case packedAudio, webmFile:
		// Packed audio has no length, and WebM isn't a kind of file the
		// library takes.
		return 0, fmt.Errorf("%w: %v", ErrCantPlay, errUnsupported)
	}
	tr, tm, err := openTrack(src)
	if err != nil {
		return 0, cantPlay(err)
	}
	if err := tr.seek(0); err != nil {
		return 0, cantPlay(readError(err))
	}
	if err := firstAudio(tr); err != nil {
		return 0, err
	}
	return max(0, tm.total-tm.delay), nil
}

// firstAudio reads and decodes the track's packets until one decodes to
// audio, as a deck starts to.
func firstAudio(tr track) error {
	for range 16 {
		err := tr.read()
		if err == io.EOF {
			return fmt.Errorf("%w: it holds no audio", ErrCantPlay)
		}
		if err != nil {
			return cantPlay(readError(err))
		}
		p, err := tr.decode()
		if err != nil {
			return cantPlay(err)
		}
		if len(p.pcm) > 0 {
			return nil
		}
	}
	return nil
}

// cantPlay wraps err, a deck's, in ErrCantPlay.
func cantPlay(err error) error { return fmt.Errorf("%w: %v", ErrCantPlay, err) }

// probeMP3 decodes the first frames of an MP3 file, without the scan of
// its frames go-mp3 makes of a file it can seek, and reads its length: of
// its LAME tag, else of the frames its Xing, Info or VBRI header counts,
// as a file of a variable bitrate has, else of its bitrate and size. Of a
// file of a variable bitrate whose header counts more than it holds, as
// one cut short, it counts the frames there are.
func probeMP3(src io.ReadSeeker) (time.Duration, error) {
	h := readMP3Header(src)
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	var d *mp3.Decoder
	err := guard(func() (err error) {
		// A reader that can't seek, which go-mp3 reads only as it decodes.
		d, err = mp3.NewDecoder(io.LimitReader(src, 1<<20))
		if err != nil {
			return err
		}
		_, err = io.ReadFull(d, make([]byte, mp3Bytes))
		return err
	})
	if err != nil && err != io.ErrUnexpectedEOF {
		if err == io.EOF {
			err = fmt.Errorf("%w: no frames", errMP3)
		}
		return 0, cantPlay(readError(err))
	}
	rate := d.SampleRate()
	if rate <= 0 {
		return 0, cantPlay(errUnsupported)
	}
	if length, ok := h.length(rate); ok {
		return length, nil
	}
	if h.cut {
		if length := mp3Frames(src); length > 0 {
			return length, nil
		}
	}
	return mp3Length(src), nil
}

// mp3MaxFrames bounds the frames mp3Frames counts: some 12 hours of
// MPEG-1 at 44.1 kHz.
const mp3MaxFrames = 1 << 20

// mp3Frames reads the length of an MP3 file from the headers of its
// frames, each of which tells its size and samples, as a deck plays them:
// the frame of a Xing header as silence, and the last, cut short, whole.
// 0 when it can't tell.
func mp3Frames(src io.ReadSeeker) time.Duration {
	at := int64(0)
	var h [10]byte
	if _, err := readAt(src, 0, h[:]); err == nil && string(h[:3]) == "ID3" && h[6]|h[7]|h[8]|h[9] < 0x80 {
		at = 10 + (int64(h[6])<<21 | int64(h[7])<<14 | int64(h[8])<<7 | int64(h[9]))
		if h[5]&0x10 != 0 {
			at += 10
		}
	}
	if _, err := src.Seek(at, io.SeekStart); err != nil {
		return 0
	}
	r := bufio.NewReaderSize(src, 64<<10)
	// The bitrates of Layer III, in kbit/s, of MPEG-1 and of MPEG-2 and
	// 2.5, and the sample rates of each version, by their indexes.
	kbps := [2][16]int64{
		{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0},
		{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
	}
	rates := map[byte][3]int64{3: {44100, 48000, 32000}, 2: {22050, 24000, 16000}, 0: {11025, 12000, 8000}}
	var samples, rate int64
	var f [4]byte
	for range mp3MaxFrames {
		if _, err := io.ReadFull(r, f[:]); err != nil || !mpegFrame(f[:]) {
			break
		}
		version := f[1] >> 3 & 3
		v, perFrame, coef := 1, int64(576), int64(72)
		if version == 3 {
			v, perFrame, coef = 0, 1152, 144
		}
		bitrate, sr := kbps[v][f[2]>>4]*1000, rates[version][f[2]>>2&3]
		if bitrate == 0 || sr == 0 || rate != 0 && sr != rate {
			break
		}
		rate = sr
		size := coef*bitrate/sr + int64(f[2]>>1&1)
		n, err := r.Discard(int(size - 4))
		samples += perFrame
		if err != nil || int64(n) != size-4 {
			break // the last frame, cut short, which a deck plays whole
		}
	}
	if rate == 0 {
		return 0
	}
	return time.Duration(samples) * time.Second / time.Duration(rate)
}

// mp3Length reads the length of an MP3 file of a constant bitrate, as its
// first frame's bitrate and the size of its frames tell: 0 when it can't
// tell.
func mp3Length(src io.ReadSeeker) time.Duration {
	size, err := src.Seek(0, io.SeekEnd)
	if err != nil {
		return 0
	}
	var h [10]byte
	at := int64(0)
	if _, err := readAt(src, 0, h[:]); err == nil && string(h[:3]) == "ID3" && h[6]|h[7]|h[8]|h[9] < 0x80 {
		at = 10 + (int64(h[6])<<21 | int64(h[7])<<14 | int64(h[8])<<7 | int64(h[9]))
	}
	var f [4]byte
	if _, err := readAt(src, at, f[:]); err != nil || !mpegFrame(f[:]) {
		return 0
	}
	var tag [3]byte
	if _, err := readAt(src, size-128, tag[:]); err == nil && string(tag[:]) == "TAG" {
		size -= 128
	}
	// The bitrates of Layer III, in kbit/s, of MPEG-1 and of MPEG-2 and
	// 2.5.
	rates := [2][16]int{
		{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0},
		{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
	}
	v := 1
	if f[1]>>3&3 == 3 {
		v = 0
	}
	kbps := rates[v][f[2]>>4]
	if kbps == 0 || size <= at {
		return 0
	}
	return time.Duration(size-at) * 8 * time.Millisecond / time.Duration(kbps)
}
