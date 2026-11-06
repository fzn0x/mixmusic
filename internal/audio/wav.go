package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"time"
)

// errWAV is wrapped by the errors of a WAV file MixMusic can't read.
var errWAV = errors.New("wav: invalid data")

const (
	// wavFrames is the frames a WAV track reads at a time.
	wavFrames = 4096
	// maxChunks bounds the chunks read before the audio.
	maxChunks = 256
)

// wavCodec reads the PCM of a WAV file: integers of 8 to 32 bits, or
// floats of 32 or 64.
type wavCodec struct {
	src io.ReadSeeker
	// data is the offset of the audio, frames how many frames it holds,
	// and at the frame read next.
	data, frames, at int64
	ch, size         int // the channels, and the bytes of a sample
	float            bool
	buf              []byte
	pcm              []float32
}

// openWAV returns the track of a WAV file. Its errors are worded for the
// player.
func openWAV(src io.ReadSeeker) (track, timing, error) {
	c, rate, err := readWAV(src)
	if err != nil {
		return nil, timing{}, readError(err)
	}
	total := time.Duration(c.frames) * time.Second / time.Duration(rate)
	return newPCMTrack(c, rate, c.ch, timing{total: total})
}

// readWAV reads the chunks of a WAV file up to its audio: its "fmt "
// chunk, of WAVE_FORMAT_PCM, WAVE_FORMAT_IEEE_FLOAT or an extensible
// format of either, and its "data" chunk.
func readWAV(src io.ReadSeeker) (*wavCodec, int, error) {
	end, err := src.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, 0, err
	}
	var h [12]byte
	if _, err := readAt(src, 0, h[:]); err != nil || string(h[:4]) != "RIFF" || string(h[8:]) != "WAVE" {
		return nil, 0, fmt.Errorf("%w: not a WAV file", errWAV)
	}
	c := &wavCodec{src: src}
	rate, format, bits := 0, 0, 0
	at := int64(12)
	for range maxChunks {
		var ch [8]byte
		if _, err := readAt(src, at, ch[:]); err != nil {
			break
		}
		size := int64(binary.LittleEndian.Uint32(ch[4:]))
		body := at + 8
		switch string(ch[:4]) {
		case "fmt ":
			if size < 16 || size > 1024 {
				return nil, 0, fmt.Errorf("%w: a format of %d bytes", errWAV, size)
			}
			f := make([]byte, size)
			if _, err := readAt(src, body, f); err != nil {
				return nil, 0, fmt.Errorf("%w: the format ends early", errWAV)
			}
			format = int(binary.LittleEndian.Uint16(f))
			c.ch = int(binary.LittleEndian.Uint16(f[2:]))
			rate = int(binary.LittleEndian.Uint32(f[4:]))
			bits = int(binary.LittleEndian.Uint16(f[14:]))
			if format == 0xFFFE && size >= 40 {
				// WAVE_FORMAT_EXTENSIBLE: the format is the first two bytes
				// of the subformat's GUID.
				format = int(binary.LittleEndian.Uint16(f[24:]))
			}
		case "data":
			if rate == 0 {
				return nil, 0, fmt.Errorf("%w: audio before its format", errWAV)
			}
			c.data = body
			// A size past the end, as of a file still being written, or
			// of 0xFFFFFFFF, is the rest of the file.
			if size > end-body || size == 0xFFFFFFFF {
				size = end - body
			}
			c.float = format == 3
			c.size = (bits + 7) / 8
			switch {
			case format != 1 && format != 3:
				return nil, 0, fmt.Errorf("%w (WAV of format %#x)", errors.ErrUnsupported, format)
			case c.ch < 1 || c.ch > 8 || rate < 1000 || rate > 768000:
				return nil, 0, fmt.Errorf("%w (WAV of %d channels at %d Hz)", errors.ErrUnsupported, c.ch, rate)
			case c.float && bits != 32 && bits != 64, !c.float && (bits < 8 || bits > 32):
				return nil, 0, fmt.Errorf("%w (WAV of %d-bit samples)", errors.ErrUnsupported, bits)
			}
			c.frames = size / int64(c.ch*c.size)
			return c, rate, nil
		}
		at = body + size + size&1
	}
	return nil, 0, fmt.Errorf("%w: no audio", errWAV)
}

// readAt reads len(b) bytes at offset off of r.
func readAt(r io.ReadSeeker, off int64, b []byte) (int, error) {
	if _, err := r.Seek(off, io.SeekStart); err != nil {
		return 0, err
	}
	return io.ReadFull(r, b)
}

func (c *wavCodec) seek(f int64) (int64, error) {
	f = max(0, min(f, c.frames))
	if _, err := c.src.Seek(c.data+f*int64(c.ch*c.size), io.SeekStart); err != nil {
		return 0, err
	}
	c.at = f
	return f, nil
}

func (c *wavCodec) read() ([]float32, error) {
	n := min(wavFrames, c.frames-c.at)
	if n <= 0 {
		return nil, io.EOF
	}
	frame := c.ch * c.size
	if cap(c.buf) < int(n)*frame {
		c.buf = make([]byte, int(n)*frame)
	}
	k, err := io.ReadFull(c.src, c.buf[:int(n)*frame])
	if k < frame {
		if err == nil || err == io.ErrUnexpectedEOF {
			err = io.EOF
		}
		return nil, err
	}
	n = int64(k / frame)
	c.at += n
	b := c.buf[:int(n)*frame]
	c.pcm = grow(c.pcm, int(n)*c.ch)
	switch {
	case c.float && c.size == 4:
		for i := range c.pcm {
			c.pcm[i] = finite(math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:])))
		}
	case c.float:
		for i := range c.pcm {
			c.pcm[i] = finite(float32(math.Float64frombits(binary.LittleEndian.Uint64(b[8*i:]))))
		}
	case c.size == 1:
		for i := range c.pcm {
			c.pcm[i] = float32(int(b[i])-128) / 128 // unsigned
		}
	default:
		scale := 1 / float32(int64(1)<<(8*c.size-1))
		for i := range c.pcm {
			s := b[c.size*i : c.size*(i+1)]
			v := int32(s[c.size-1]) << 24 >> 24 // the sign, of the last byte
			for k := c.size - 2; k >= 0; k-- {
				v = v<<8 | int32(s[k])
			}
			c.pcm[i] = float32(v) * scale
		}
	}
	return c.pcm, nil
}

// finite is v, or 0 for what isn't a number or is infinite.
func finite(v float32) float32 {
	if v != v || math.IsInf(float64(v), 0) {
		return 0
	}
	return v
}
