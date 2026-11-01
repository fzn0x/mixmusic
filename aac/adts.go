package aac

import (
	"errors"
	"fmt"
)

// ADTSHeaderSize is the size of the fixed and variable headers of an ADTS
// frame, which ParseADTS reads: 7 bytes, before the CRC of a protected
// frame.
const ADTSHeaderSize = 7

// ADTSHeader is the header of a frame of ADTS, the Audio Data Transport
// Stream of ISO/IEC 13818-7, section 6.2, which ISO/IEC 14496-3, section
// 1.A.2.2, carries over to MPEG-4: the frames of a stream of AAC with no
// container, as HLS packed audio holds them.
type ADTSHeader struct {
	// Config is the stream's, of the header's profile, sampling frequency
	// and channel configuration. Its Channels is 0 for the configuration
	// 0, whose program config element is in the frame's raw data block.
	Config Config
	// Length is the frame's, in bytes, its header included.
	Length int
	// HeaderLength is the header's: 7 bytes, or 9 with the CRC of a
	// protected frame. The raw data block follows it.
	HeaderLength int
	// Blocks is the number of raw data blocks of the frame, from 1 to 4.
	Blocks int
}

// ParseADTS reads the header of an ADTS frame at the start of b, of at
// least ADTSHeaderSize bytes. It returns an error that wraps ErrFormat for
// bytes that are not one: they don't start with the syncword, or tell a
// layer other than 0, a reserved sampling frequency, or a frame too short
// for its header. It doesn't check that b holds the frame whole.
func ParseADTS(b []byte) (ADTSHeader, error) {
	if len(b) < ADTSHeaderSize {
		return ADTSHeader{}, fmt.Errorf("%w: ADTS header of %d bytes", ErrFormat, len(b))
	}
	// adts_fixed_header: syncword (12 bits), ID, layer (2), protection_absent,
	// profile_ObjectType (2), sampling_frequency_index (4), private_bit,
	// channel_configuration (3), original_copy, home.
	if b[0] != 0xFF || b[1]&0xF0 != 0xF0 {
		return ADTSHeader{}, fmt.Errorf("%w: no ADTS syncword", ErrFormat)
	}
	if b[1]&0x06 != 0 {
		return ADTSHeader{}, fmt.Errorf("%w: ADTS of layer %d", ErrFormat, b[1]>>1&3)
	}
	h := ADTSHeader{HeaderLength: 7}
	if b[1]&1 == 0 {
		h.HeaderLength = 9 // crc_check follows
	}
	rate := int(b[2] >> 2 & 15)
	if rate >= len(sampleRates) {
		return ADTSHeader{}, fmt.Errorf("%w: sampling frequency index %d", ErrFormat, rate)
	}
	h.Config = Config{
		// The profile is the audio object type less 1: 1 is LC, in MPEG-2's
		// profiles as in MPEG-4's object types.
		ObjectType: int(b[2]>>6) + 1,
		SampleRate: sampleRates[rate],
		Channels:   int(b[2]&1)<<2 | int(b[3]>>6),
	}
	// adts_variable_header: copyright_identification_bit and _start,
	// aac_frame_length (13 bits), adts_buffer_fullness (11),
	// number_of_raw_data_blocks_in_frame (2).
	h.Length = int(b[3]&3)<<11 | int(b[4])<<3 | int(b[5]>>5)
	h.Blocks = int(b[6]&3) + 1
	if h.Length <= h.HeaderLength {
		return ADTSHeader{}, fmt.Errorf("%w: ADTS frame of %d bytes", ErrFormat, h.Length)
	}
	return h, nil
}

// Decodable tells whether a Decoder decodes the frames of h as they come:
// of AAC-LC, in mono or stereo, one raw data block each. The others are
// errors.ErrUnsupported, which it returns, saying why.
func (h ADTSHeader) Decodable() error {
	switch {
	case h.Config.ObjectType != ObjectTypeLC:
		return fmt.Errorf("aac: ADTS of audio object type %d: %w", h.Config.ObjectType, errors.ErrUnsupported)
	case h.Config.Channels != 1 && h.Config.Channels != 2:
		return fmt.Errorf("aac: ADTS of channel configuration %d: %w", h.Config.Channels, errors.ErrUnsupported)
	case h.Blocks != 1:
		return fmt.Errorf("aac: ADTS frames of %d raw data blocks: %w", h.Blocks, errors.ErrUnsupported)
	}
	return nil
}
