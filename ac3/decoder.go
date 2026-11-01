package ac3

import "errors"

// Decoder decodes an AC-3 stream. It is not safe for concurrent use.
type Decoder struct {
	info Info
}

// NewDecoder returns a decoder.
func NewDecoder() *Decoder {
	d := &Decoder{}
	d.Reset()
	return d
}

// Info returns what the syncframes decoded last tell of the stream: the
// zero Info before the first.
func (d *Decoder) Info() Info { return d.info }

// Reset makes the decoder forget the stream, as for a new one or after a
// seek: the block decoded next overlaps silence.
func (d *Decoder) Reset() {}

// Decode decodes the syncframes of data, as an MP4 sample holds them, into
// pcm, and returns the samples per channel: FrameLength for an MP4
// sample. The samples are interleaved, of MaxChannels channels in the
// order of Left to RightSurround, those the stream lacks silent, as
// float32 of about [-1, 1], not clipped, but finite. pcm must have room
// for MaxChannels times the samples of data's frames: MaxChannels *
// FrameLength for an MP4 sample.
//
// A block decodes into the second half of the last one's window and the
// first half of its own: the first block of a stream, or the first after
// Reset, holds only half a window of audio. After an error, the decoder
// decodes the next frame as if the bad one never came.
func (d *Decoder) Decode(data []byte, pcm []float32) (int, error) {
	return 0, errors.ErrUnsupported
}
