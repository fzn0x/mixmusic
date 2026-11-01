// Package opus decodes Opus audio (RFC 6716 and RFC 8251): a pure Go port
// of the decoder of libopus 1.6.1, float build, under the BSD license of
// LICENSE.libopus.
package opus

// Port of the packet functions of src/opus.c and src/opus_decoder.c.

import (
	_ "embed"
	"errors"
)

// License is the license of libopus, whose binary copies must reproduce it.
//
//go:embed LICENSE.libopus
var License string

// ErrInvalidPacket is OPUS_INVALID_PACKET.
var ErrInvalidPacket = errors.New("opus: invalid packet")

// Modes of a frame.
const (
	ModeSILKOnly = 1000
	ModeHybrid   = 1001
	ModeCELTOnly = 1002
)

// Bandwidths of a frame.
const (
	BandwidthNarrowband    = 1101
	BandwidthMediumband    = 1102
	BandwidthWideband      = 1103
	BandwidthSuperwideband = 1104
	BandwidthFullband      = 1105
)

// PacketMode is the mode of a packet's frames, from its TOC byte.
func PacketMode(toc byte) int {
	switch {
	case toc&0x80 != 0:
		return ModeCELTOnly
	case toc&0x60 == 0x60:
		return ModeHybrid
	}
	return ModeSILKOnly
}

// PacketBandwidth is the bandwidth of a packet's frames.
func PacketBandwidth(toc byte) int {
	switch {
	case toc&0x80 != 0:
		bw := BandwidthMediumband + int((toc>>5)&0x3)
		if bw == BandwidthMediumband {
			bw = BandwidthNarrowband
		}
		return bw
	case toc&0x60 == 0x60:
		if toc&0x10 != 0 {
			return BandwidthFullband
		}
		return BandwidthSuperwideband
	}
	return BandwidthNarrowband + int((toc>>5)&0x3)
}

// PacketChannels is the number of channels a packet codes.
func PacketChannels(toc byte) int {
	if toc&0x4 != 0 {
		return 2
	}
	return 1
}

// SamplesPerFrame is the number of samples per channel of each frame of a
// packet, at the rate fs.
func SamplesPerFrame(toc byte, fs int) int {
	switch {
	case toc&0x80 != 0:
		size := int((toc >> 3) & 0x3)
		return (fs << size) / 400
	case toc&0x60 == 0x60:
		if toc&0x08 != 0 {
			return fs / 50
		}
		return fs / 100
	}
	size := int((toc >> 3) & 0x3)
	if size == 3 {
		return fs * 60 / 1000
	}
	return (fs << size) / 100
}

// parseSize reads the size of a frame: its value and the bytes it took, or
// -1.
func parseSize(data []byte) (size, n int) {
	switch {
	case len(data) < 1:
		return -1, -1
	case data[0] < 252:
		return int(data[0]), 1
	case len(data) < 2:
		return -1, -1
	}
	return 4*int(data[1]) + int(data[0]), 2
}

// ParsePacket splits a packet into its frames, as opus_packet_parse_impl
// does for packets that are not self-delimited. The frames share the
// packet's memory.
func ParsePacket(data []byte) (toc byte, frames [][]byte, err error) {
	var all [48][]byte
	toc, count, err := parsePacket(data, &all)
	if err != nil {
		return 0, nil, err
	}
	return toc, append([][]byte(nil), all[:count]...), nil
}

// parsePacket is ParsePacket into frames, at most 48 of 2.5 ms, without
// allocating; it returns how many there are.
func parsePacket(data []byte, frames *[48][]byte) (toc byte, count int, err error) {
	if len(data) == 0 {
		return 0, 0, ErrInvalidPacket
	}
	framesize := SamplesPerFrame(data[0], 48000)
	toc = data[0]
	data = data[1:]
	n := len(data)
	lastSize := n
	var size [48]int
	switch toc & 0x3 {
	case 0:
		// One frame.
		count = 1
	case 1:
		// Two frames of the same size.
		count = 2
		if n&1 != 0 {
			return 0, 0, ErrInvalidPacket
		}
		lastSize = n / 2
		size[0] = lastSize
	case 2:
		// Two frames of different sizes.
		count = 2
		s, b := parseSize(data)
		n -= b
		if s < 0 || s > n {
			return 0, 0, ErrInvalidPacket
		}
		size[0] = s
		data = data[b:]
		lastSize = n - s
	default:
		// Any number of frames, up to 120 ms.
		if n < 1 {
			return 0, 0, ErrInvalidPacket
		}
		ch := data[0]
		data = data[1:]
		count = int(ch & 0x3F)
		if count <= 0 || framesize*count > 5760 {
			return 0, 0, ErrInvalidPacket
		}
		n--
		// Padding.
		if ch&0x40 != 0 {
			for {
				if n <= 0 {
					return 0, 0, ErrInvalidPacket
				}
				p := int(data[0])
				data = data[1:]
				n--
				tmp := p
				if p == 255 {
					tmp = 254
				}
				n -= tmp
				if p != 255 {
					break
				}
			}
		}
		if n < 0 {
			return 0, 0, ErrInvalidPacket
		}
		if ch&0x80 != 0 {
			// Variable sizes.
			lastSize = n
			for i := 0; i < count-1; i++ {
				s, b := parseSize(data[:min(len(data), n)])
				n -= b
				if s < 0 || s > n {
					return 0, 0, ErrInvalidPacket
				}
				size[i] = s
				data = data[b:]
				lastSize -= b + s
			}
			if lastSize < 0 {
				return 0, 0, ErrInvalidPacket
			}
		} else {
			// The same size.
			lastSize = n / count
			if lastSize*count != n {
				return 0, 0, ErrInvalidPacket
			}
			for i := 0; i < count-1; i++ {
				size[i] = lastSize
			}
		}
	}
	// The last frame's size is implied; reject one above 1275 bytes.
	if lastSize > 1275 {
		return 0, 0, ErrInvalidPacket
	}
	size[count-1] = lastSize
	for i := 0; i < count; i++ {
		frames[i] = data[:size[i]:size[i]]
		data = data[size[i]:]
	}
	return toc, count, nil
}

// PacketSamples is the number of samples per channel of a packet at fs,
// opus_packet_get_nb_samples.
func PacketSamples(packet []byte, fs int) (int, error) {
	if len(packet) < 1 {
		return 0, ErrInvalidPacket
	}
	var count int
	switch packet[0] & 0x3 {
	case 0:
		count = 1
	case 1, 2:
		count = 2
	default:
		if len(packet) < 2 {
			return 0, ErrInvalidPacket
		}
		count = int(packet[1] & 0x3F)
	}
	samples := count * SamplesPerFrame(packet[0], fs)
	// No more than 120 ms.
	if samples*25 > fs*3 {
		return 0, ErrInvalidPacket
	}
	return samples, nil
}
