package aac

import (
	"errors"
	"fmt"
)

// readADTS splits an ADTS stream, such as ffmpeg -f adts writes, into the
// raw data blocks of its frames, and returns the config its first header
// gives. A stream of channel configuration 0 holds the program config
// element in its first frame: the config then has no channels.
func readADTS(b []byte) (Config, [][]byte, error) {
	var cfg Config
	var frames [][]byte
	for len(b) > 0 {
		if len(b) < 7 || b[0] != 0xFF || b[1]&0xF6 != 0xF0 {
			return cfg, nil, errors.New("adts: no frame header")
		}
		header := 7
		if b[1]&1 == 0 { // protection_absent
			header = 9
		}
		size := int(b[3]&3)<<11 | int(b[4])<<3 | int(b[5]>>5)
		if b[6]&3 != 0 {
			return cfg, nil, errors.New("adts: frames of more than one raw data block")
		}
		if size < header || size > len(b) {
			return cfg, nil, fmt.Errorf("adts: frame of %d bytes", size)
		}
		if frames == nil {
			rate := int(b[2] >> 2 & 15)
			if rate >= len(sampleRates) {
				return cfg, nil, fmt.Errorf("adts: sampling frequency index %d", rate)
			}
			cfg = Config{
				ObjectType: int(b[2]>>6) + 1,
				SampleRate: sampleRates[rate],
				Channels:   int(b[2]&1)<<2 | int(b[3]>>6),
			}
		}
		frames = append(frames, b[header:size])
		b = b[size:]
	}
	return cfg, frames, nil
}
