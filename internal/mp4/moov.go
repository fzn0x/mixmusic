package mp4

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/bits"
)

// movie is what the moov says of the durations of the movie and of the
// track.
type movie struct {
	scale     uint32 // the timescale of the movie, of its mvhd
	duration  uint64 // of the mvhd, in the movie's timescale
	fragments uint64 // of the mehd, with the fragments, likewise
	media     uint64 // of the track's mdhd, in the media's timescale
	edits     uint64 // of the edits of the track's edit list, in the movie's timescale
}

// trex is the defaults of the samples of a track's fragments.
type trex struct {
	duration, size uint32
}

// parseMoov reads the moov: the movie's header, its first audio track,
// and the defaults of the fragments, when its mvex says fragments follow.
func (r *Reader) parseMoov(b []byte) (movie, error) {
	var m movie
	var trak []byte
	err := eachBox(b, func(typ uint32, data []byte) error {
		switch typ {
		case typeMvhd:
			v, _, f := fullBox(data)
			if v == 1 {
				f.skip(16) // the creation and modification times
			} else {
				f.skip(8)
			}
			m.scale = f.u32()
			m.duration = f.duration(v == 1)
			return f.check(typ)
		case typeTrak:
			if trak != nil {
				return nil
			}
			audio, err := isAudio(data)
			if audio {
				trak = data
			}
			return err
		case typeMvex:
			r.fragmented = true
			return r.parseMvex(data, &m)
		}
		return nil
	})
	if err != nil {
		return m, err
	}
	if trak == nil {
		return m, ErrNoAudio
	}
	return m, r.parseTrak(trak, &m)
}

// isAudio reports whether the trak of data b is of audio, as the handler
// of its mdia says.
func isAudio(b []byte) (bool, error) {
	mdia, err := findBox(b, typeMdia)
	if err != nil || mdia == nil {
		return false, err
	}
	hdlr, err := findBox(mdia, typeHdlr)
	if err != nil || hdlr == nil {
		return false, err
	}
	_, _, f := fullBox(hdlr)
	f.skip(4) // pre_defined
	return f.u32() == handlerSound, f.check(typeHdlr)
}

// parseMvex reads the defaults of the fragments of each track, and the
// duration of the fragmented movie.
func (r *Reader) parseMvex(b []byte, m *movie) error {
	return eachBox(b, func(typ uint32, data []byte) error {
		switch typ {
		case typeMehd:
			v, _, f := fullBox(data)
			m.fragments = f.duration(v == 1)
			return f.check(typ)
		case typeTrex:
			_, _, f := fullBox(data)
			id := f.u32()
			f.skip(4) // default_sample_description_index
			t := trex{duration: f.u32(), size: f.u32()}
			if err := f.check(typ); err != nil {
				return err
			}
			if _, dup := r.trex[id]; !dup {
				r.trex[id] = t
			}
		}
		return nil
	})
}

// parseTrak reads the audio trak of data b: its header, edit list, media
// header and sample table.
func (r *Reader) parseTrak(b []byte, m *movie) error {
	var mdia, elst []byte
	err := eachBox(b, func(typ uint32, data []byte) error {
		switch typ {
		case typeTkhd:
			v, _, f := fullBox(data)
			if v == 1 {
				f.skip(16)
			} else {
				f.skip(8)
			}
			r.track.ID = f.u32()
			return f.check(typ)
		case typeEdts:
			var err error
			elst, err = findBox(data, typeElst)
			return err
		case typeMdia:
			mdia = data
		}
		return nil
	})
	if err != nil {
		return err
	}

	var stbl []byte
	err = eachBox(mdia, func(typ uint32, data []byte) error {
		switch typ {
		case typeMdhd:
			v, _, f := fullBox(data)
			if v == 1 {
				f.skip(16)
			} else {
				f.skip(8)
			}
			r.track.Timescale = f.u32()
			m.media = f.duration(v == 1)
			return f.check(typ)
		case typeMinf:
			var err error
			stbl, err = findBox(data, typeStbl)
			return err
		}
		return nil
	})
	switch {
	case err != nil:
		return err
	case r.track.Timescale == 0:
		return fmt.Errorf("%w: track %d has no timescale", ErrFormat, r.track.ID)
	case stbl == nil:
		return fmt.Errorf("%w: track %d has no sample table", ErrFormat, r.track.ID)
	}
	r.scale = int64(r.track.Timescale)
	r.maxTicks = maxSeconds * r.scale

	if elst != nil {
		if err := r.parseElst(elst, m); err != nil {
			return err
		}
	}
	return r.parseStbl(stbl)
}

// parseElst reads the edit list of the track: the media time of its first
// edit that is not empty, which skips the encoder's priming, and the
// duration of the edits. A list shorter than its count says gives the
// edits it holds.
func (r *Reader) parseElst(b []byte, m *movie) error {
	v, _, f := fullBox(b)
	n := f.u32()
	found := false
	for range min(n, uint32(len(b)/12)) {
		duration := f.uint(v == 1)
		var mediaTime int64
		if v == 1 {
			mediaTime = int64(f.u64())
		} else {
			mediaTime = int64(int32(f.u32()))
		}
		f.skip(4) // media_rate
		if f.short {
			break
		}
		if mediaTime < 0 {
			continue // an empty edit, of no media
		}
		if !found {
			if mediaTime > r.maxTicks {
				return fmt.Errorf("%w: edit list starts at tick %d", ErrFormat, mediaTime)
			}
			r.delay, found = mediaTime, true
			r.track.CodecDelay = r.time(mediaTime)
		}
		var carry uint64
		if m.edits, carry = bits.Add64(m.edits, duration, 0); carry != 0 {
			m.edits = math.MaxUint64 // out of range: unknown
		}
	}
	return nil
}

// parseStbl reads the sample table of the track: its sample description
// and where its samples are.
func (r *Reader) parseStbl(b []byte) error {
	stsd, err := findBox(b, typeStsd)
	switch {
	case err != nil:
		return err
	case stsd == nil:
		return fmt.Errorf("%w: track %d has no sample description", ErrFormat, r.track.ID)
	}
	if err := r.parseStsd(stsd); err != nil {
		return err
	}
	return r.table.parse(b, r.maxTicks)
}

// parseStsd reads the first sample entry of the sample description.
func (r *Reader) parseStsd(b []byte) error {
	version, _, f := fullBox(b)
	f.skip(4) // entry_count
	if err := f.check(typeStsd); err != nil {
		return err
	}
	var typ uint32
	var entry []byte
	err := eachBox(f.b, func(t uint32, data []byte) error {
		if entry == nil {
			typ, entry = t, data
		}
		return nil
	})
	switch {
	case err != nil:
		return err
	case entry == nil:
		return fmt.Errorf("%w: track %d has no sample entry", ErrFormat, r.track.ID)
	case typ == typeEnca:
		return fmt.Errorf("mp4: %w: track %d is encrypted", errors.ErrUnsupported, r.track.ID)
	}

	// An AudioSampleEntry, as QuickTime's sound descriptions are, which
	// versions 1 and 2 extend in a sample description of version 0.
	e := fields{b: entry}
	e.skip(8) // reserved and data_reference_index
	soundVersion := e.u16()
	e.skip(6) // revision and vendor
	r.track.Channels = int(e.u16())
	e.skip(6) // sample size, compression ID and packet size
	r.track.SampleRate = int(e.u32() >> 16)
	if version == 0 {
		switch soundVersion {
		case 1:
			e.skip(16)
		case 2:
			e.skip(4) // the size of the structure
			rate := math.Float64frombits(e.u64())
			channels := e.u32()
			e.skip(20)
			if rate >= 1 && rate <= math.MaxInt32 {
				r.track.SampleRate = int(rate)
			}
			r.track.Channels = int(min(channels, math.MaxInt32))
		}
	}
	if err := e.check(typ); err != nil {
		return err
	}
	if typ != typeMp4a {
		r.track.Codec = string(binary.BigEndian.AppendUint32(nil, typ))
		return nil
	}
	r.track.Codec = "mp4a"

	esds, err := findBox(e.b, typeEsds)
	if err == nil && esds == nil {
		// In a QuickTime file, within the wave of the sound description.
		var wave []byte
		if wave, err = findBox(e.b, typeWave); wave != nil {
			esds, err = findBox(wave, typeEsds)
		}
	}
	if err != nil || esds == nil {
		return err
	}
	oti, dsi, err := parseEsds(esds)
	if err != nil {
		return err
	}
	r.track.Codec = fmt.Sprintf("mp4a.%02X", oti)
	r.track.Config = bytes.Clone(dsi)
	switch oti {
	case 0x40, 0x66, 0x67, 0x68: // MPEG-4 audio, and MPEG-2 AAC
		c, ok := parseAudioConfig(dsi)
		if !ok {
			return nil
		}
		if oti == 0x40 {
			r.track.Codec = fmt.Sprintf("mp4a.40.%d", c.objectType)
		}
		if c.rate > 0 {
			r.track.SampleRate = c.rate
		}
		if c.channels > 0 {
			r.track.Channels = c.channels
		}
	}
	return nil
}

// The tags of the descriptors of an esds, of ISO/IEC 14496-1.
const (
	tagES            = 0x03
	tagDecoderConfig = 0x04
	tagDecoderInfo   = 0x05
)

// parseEsds reads the objectTypeIndication of the DecoderConfigDescriptor
// of an esds, and its DecoderSpecificInfo, which may be missing.
func parseEsds(b []byte) (oti byte, dsi []byte, err error) {
	_, _, f := fullBox(b)
	if err := f.check(typeEsds); err != nil {
		return 0, nil, err
	}
	tag, es, _, err := descriptor(f.b)
	if err != nil {
		return 0, nil, err
	}
	config := es
	if tag == tagES {
		// The ES_ID, the flags, and the fields they say follow.
		d := fields{b: es}
		d.skip(2)
		flags := d.u8()
		if flags&0x80 != 0 {
			d.skip(2) // dependsOn_ES_ID
		}
		if flags&0x40 != 0 {
			d.skip(int(d.u8())) // the URL
		}
		if flags&0x20 != 0 {
			d.skip(2) // OCR_ES_Id
		}
		if err := d.check(typeEsds); err != nil {
			return 0, nil, err
		}
		for config = nil; len(d.b) > 0; {
			var data []byte
			if tag, data, d.b, err = descriptor(d.b); err != nil {
				return 0, nil, err
			}
			if tag == tagDecoderConfig {
				config = data
				break
			}
		}
	} else if tag != tagDecoderConfig {
		config = nil
	}
	if len(config) < 13 {
		return 0, nil, fmt.Errorf("%w: esds has no DecoderConfigDescriptor", ErrFormat)
	}
	// The object type, the stream type, the buffer size and the bitrates;
	// then the DecoderSpecificInfo among the descriptors.
	oti = config[0]
	for rest := config[13:]; len(rest) > 0; {
		var data []byte
		if tag, data, rest, err = descriptor(rest); err != nil {
			return 0, nil, err
		}
		if tag == tagDecoderInfo {
			return oti, data, nil
		}
	}
	return oti, nil, nil
}

// descriptor returns the tag and the data of the descriptor at the start
// of b, and what follows it. Its size takes 1 to 4 bytes, of 7 bits each,
// all but the last with their top bit set.
func descriptor(b []byte) (tag byte, data, rest []byte, err error) {
	errDescriptor := fmt.Errorf("%w: invalid descriptor in esds", ErrFormat)
	if len(b) < 2 {
		return 0, nil, nil, errDescriptor
	}
	size, n := 0, 1
	for {
		if n == len(b) || n == 5 {
			return 0, nil, nil, errDescriptor
		}
		c := b[n]
		n++
		size = size<<7 | int(c&0x7F)
		if c&0x80 == 0 {
			break
		}
	}
	if size > len(b)-n {
		return 0, nil, nil, errDescriptor
	}
	return b[0], b[n : n+size : n+size], b[n+size:], nil
}

// audioConfig is what the Reader reads of an AudioSpecificConfig, of
// ISO/IEC 14496-3.
type audioConfig struct {
	objectType int
	rate       int // 0 when unknown
	channels   int // 0 when the stream gives them
}

// sampleRates are the sampling frequencies by their index.
var sampleRates = [...]int{96000, 88200, 64000, 48000, 44100, 32000, 24000, 22050, 16000, 12000, 11025, 8000, 7350}

// configChannels are the channels by channelConfiguration, 0 for those
// the stream gives or that are reserved.
var configChannels = [...]int{0, 1, 2, 3, 4, 5, 6, 8, 0, 0, 0, 7, 8, 24, 8, 0}

// parseAudioConfig reads the audio object type, the sampling frequency
// and the channels of an AudioSpecificConfig. For SBR and PS signalled
// explicitly, as object types 5 and 29, the rate is the extension's, of
// the decoded audio.
func parseAudioConfig(b []byte) (audioConfig, bool) {
	br := bitReader{b: b}
	objectType := func() int {
		t := br.read(5)
		if t == 31 {
			t = 32 + br.read(6)
		}
		return t
	}
	rate := func() int {
		i := br.read(4)
		if i == 15 {
			return br.read(24)
		}
		if i < len(sampleRates) {
			return sampleRates[i]
		}
		return 0
	}
	var c audioConfig
	c.objectType = objectType()
	c.rate = rate()
	c.channels = configChannels[br.read(4)]
	if c.objectType == 5 || c.objectType == 29 {
		if ext := rate(); ext > 0 {
			c.rate = ext
		}
	}
	return c, !br.short && c.objectType != 0
}

// bitReader reads the bits of b, from the top bit of each byte down.
type bitReader struct {
	b     []byte
	n     int // the bits read
	short bool
}

// read reads n bits, at most 24, as a number: 0 past the end.
func (br *bitReader) read(n int) int {
	v := 0
	for range n {
		i := br.n >> 3
		if i >= len(br.b) {
			br.short = true
			return 0
		}
		v = v<<1 | int(br.b[i]>>(7-br.n&7)&1)
		br.n++
	}
	return v
}

// rescale converts v, in ticks of a timescale from, to ticks of a
// timescale to, rounded down, and reports whether it is at most limit.
func rescale(v uint64, from, to uint32, limit int64) (int64, bool) {
	if from == 0 {
		return 0, false
	}
	hi, lo := bits.Mul64(v, uint64(to))
	if hi >= uint64(from) {
		return 0, false
	}
	q, _ := bits.Div64(hi, lo, uint64(from))
	return int64(q), q <= uint64(limit)
}
