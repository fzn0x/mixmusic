package audio

import (
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/hajimehoshi/go-mp3"
)

// MP3License is the license of go-mp3, the MP3 decoder of Hajime Hoshi
// and its authors, which MixMusic is built with, and whose binary copies
// must give it.
//
//go:embed LICENSE.go-mp3
var MP3License string

// errMP3 is wrapped by the errors of an MP3 file MixMusic can't read.
var errMP3 = errors.New("mp3: invalid data")

// mp3Bytes is the bytes of the 16-bit stereo an MP3 track reads at a
// time: a frame of MPEG-1 Layer III.
const mp3Bytes = 4 * 1152

// mp3DecoderDelay is the frames an MP3 decoder's filter bank delays the
// audio by, which LAME's tag leaves out of the encoder's delay.
const mp3DecoderDelay = 529

// mp3Codec decodes the MP3 of a file kept on a computer, with go-mp3,
// which decodes MPEG-1, 2 and 2.5 Layer III into 16-bit stereo, a mono
// file's in both channels.
type mp3Codec struct {
	d *mp3.Decoder
	// at is the frame read next, and end the frame the audio ends at, as
	// LAME's tag tells, else 0.
	at, end int64
	buf     []byte
	pcm     []float32
}

// openMP3 returns the track of an MP3 file. Its errors are worded for the
// player. go-mp3 decodes the frame of a Xing or LAME tag, before the
// audio, as silence, and keeps the encoder's delay and padding: the tag's
// delay and padding, when it tells them, are the codec's delay, and where
// the audio ends, as players that play albums without gaps take them.
func openMP3(src io.ReadSeeker) (track, timing, error) {
	start, end := mp3Gap(src)
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return nil, timing{}, readError(err)
	}
	var d *mp3.Decoder
	err := guard(func() (err error) {
		d, err = mp3.NewDecoder(src)
		return err
	})
	if err == io.EOF {
		err = fmt.Errorf("%w: no frames", errMP3)
	}
	if err != nil {
		return nil, timing{}, readError(err)
	}
	rate := d.SampleRate()
	if rate <= 0 {
		return nil, timing{}, fmt.Errorf("%w (%v)", errUnsupported, errMP3)
	}
	c := &mp3Codec{d: d, buf: make([]byte, mp3Bytes)}
	n := d.Length() / 4
	if end > start && end <= n {
		c.end = end
		n = end
	} else {
		start = 0
	}
	tm := timing{delay: time.Duration(start) * time.Second / time.Duration(rate)}
	if n > 0 {
		tm.total = time.Duration(n) * time.Second / time.Duration(rate)
	}
	return newPCMTrack(c, rate, 2, tm)
}

// mp3Gap reads the Xing or Info tag of an MP3 file's first frame, and the
// LAME tag after it: it returns the frame the audio starts at, as go-mp3
// decodes the file, past the tag's frame, the encoder's delay and the
// decoder's, and the frame it ends at, before the encoder's padding. It
// returns zeros for a file without them.
func mp3Gap(src io.ReadSeeker) (start, end int64) {
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
	var b [512]byte
	n, _ := readAt(src, at, b[:])
	f := b[:n]
	if len(f) < 4 || !mpegFrame(f) {
		return 0, 0
	}
	mpeg1, mono := f[1]>>3&3 == 3, f[3]>>6 == 3
	side, frame := 17, int64(1152) // the side information's bytes, and a frame's samples
	switch {
	case mpeg1 && !mono:
		side = 32
	case !mpeg1 && mono:
		side, frame = 9, 576
	case !mpeg1:
		frame = 576
	}
	x := 4 + side
	if len(f) < x+8 || string(f[x:x+4]) != "Xing" && string(f[x:x+4]) != "Info" {
		return 0, 0
	}
	flags := binary.BigEndian.Uint32(f[x+4:])
	x += 8
	frames := int64(-1)
	if flags&1 != 0 {
		if len(f) < x+4 {
			return 0, 0
		}
		frames = int64(binary.BigEndian.Uint32(f[x:]))
		x += 4
	}
	for _, field := range []struct {
		flag uint32
		size int
	}{{2, 4}, {4, 100}, {8, 4}} {
		if flags&field.flag != 0 {
			x += field.size
		}
	}
	// The LAME tag: its encoder's name, then 12 bytes, then the delay and
	// the padding, 12 bits each. ffmpeg writes one too, as "Lavc".
	if frames < 0 || len(f) < x+24 || string(f[x:x+4]) != "LAME" && string(f[x:x+4]) != "Lavc" && string(f[x:x+4]) != "Lavf" {
		return 0, 0
	}
	g := f[x+21:]
	delay, padding := int64(g[0])<<4|int64(g[1])>>4, int64(g[1]&15)<<8|int64(g[2])
	start = frame + delay + mp3DecoderDelay
	end = start + frames*frame - delay - padding
	return start, end
}

// guard runs fn, which runs a decoder MixMusic didn't write: a panic of
// it, as at a file made to break it, is an error of the file.
func guard(fn func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%w: the decoder failed (%v)", errMP3, p)
		}
	}()
	err = fn()
	if err != nil && err != io.EOF && !errors.Is(err, errMP3) {
		err = fmt.Errorf("%w: %v", errMP3, err)
	}
	return err
}

func (c *mp3Codec) seek(f int64) (int64, error) {
	if l := c.d.Length() / 4; l > 0 && f >= l {
		f = l - 1
	}
	f = max(0, f)
	err := guard(func() error {
		_, err := c.d.Seek(4*f, io.SeekStart)
		return err
	})
	if err == io.EOF {
		err = nil
	}
	c.at = f
	return f, err
}

func (c *mp3Codec) read() ([]float32, error) {
	var n int
	err := guard(func() (err error) {
		n, err = io.ReadFull(c.d, c.buf)
		if err == io.ErrUnexpectedEOF {
			err = nil
		}
		return err
	})
	frames := int64(n / 4)
	if c.end > 0 {
		frames = max(0, min(frames, c.end-c.at))
	}
	if frames == 0 {
		if err == nil {
			err = io.EOF
		}
		return nil, err
	}
	c.at += frames
	c.pcm = grow(c.pcm, int(2*frames))
	for i := range c.pcm {
		c.pcm[i] = float32(int16(uint16(c.buf[2*i])|uint16(c.buf[2*i+1])<<8)) / 32768
	}
	return c.pcm, nil
}
