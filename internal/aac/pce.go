package aac

// Port of Syntax/PCE.cs, which keeps what this decoder only skips.

import "fmt"

// programConfig is program_config_element: the channels of a program, as
// an AudioSpecificConfig of channel configuration 0 holds it, or a raw
// data block.
type programConfig struct {
	// The elements of the front, the sides, the back, and the LFEs.
	front, side, back, lfe int
	// channels are the channels of the front, side and back elements.
	channels int
}

func (p *programConfig) parse(r *bitReader) error {
	r.skip(4 + 2 + 4) // element_instance_tag, object_type, sampling_frequency_index
	p.front, p.side, p.back = r.read(4), r.read(4), r.read(4)
	p.lfe = r.read(2)
	assoc, cc := r.read(3), r.read(4)
	if r.flag() { // mono_mixdown_present
		r.skip(4)
	}
	if r.flag() { // stereo_mixdown_present
		r.skip(4)
	}
	if r.flag() { // matrix_mixdown_idx_present
		r.skip(3)
	}
	p.channels = 0
	for range p.front + p.side + p.back {
		if r.flag() { // is_cpe
			p.channels += 2
		} else {
			p.channels++
		}
		r.skip(4) // element_tag_select
	}
	r.skip(4*p.lfe + 4*assoc + 5*cc)
	// The comment field starts on a byte, of the config or of the raw data
	// block, which start on one too.
	r.align()
	r.skip(8 * r.read(8))
	if r.over() {
		return fmt.Errorf("%w: program config element past its data", ErrFormat)
	}
	return nil
}
