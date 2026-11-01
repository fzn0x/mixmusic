package aac

// Port of Tools/MS.cs and Tools/IS.cs: the stereo of a channel pair.

// applyMS turns the mid and side of the bands M/S stereo codes into left
// and right: those whose ms_used is set, but for noise and intensity
// bands.
func (d *Decoder) applyMS() {
	if !d.commonWindow || d.msMask == 0 {
		return
	}
	left, right := &d.ch[0], &d.ch[1]
	info := &left.ics.info
	groupOff := 0
	for g := range info.groups {
		for sfb := range info.maxSFB {
			i := g*info.maxSFB + sfb
			if !d.msUsed[i] || left.ics.bandType[i] >= noiseBook || right.ics.bandType[i] >= noiseBook {
				continue
			}
			lo, hi := int(info.swb[sfb]), int(info.swb[sfb+1])
			for w := range info.groupLen[g] {
				l := left.spec[groupOff+128*w+lo : groupOff+128*w+hi]
				r := right.spec[groupOff+128*w+lo : groupOff+128*w+hi]
				for k := range l {
					l[k], r[k] = l[k]+r[k], l[k]-r[k]
				}
			}
		}
		groupOff += 128 * info.groupLen[g]
	}
}

// applyIS makes the right channel's intensity bands the left's, scaled by
// their intensity positions, and inverted for the out-of-phase codebook
// and, with an M/S mask, where ms_used is set.
func (d *Decoder) applyIS() {
	left, right := &d.ch[0], &d.ch[1]
	info := &right.ics.info
	groupOff := 0
	for g := range info.groups {
		for sfb := range info.maxSFB {
			i := g*info.maxSFB + sfb
			cb := right.ics.bandType[i]
			if cb != intensityBook && cb != intensityBook2 {
				continue
			}
			scale := pow2q[100-right.ics.sf[i]]
			if cb == intensityBook2 {
				scale = -scale
			}
			if d.msMask != 0 && d.msUsed[i] {
				scale = -scale
			}
			lo, hi := int(info.swb[sfb]), int(info.swb[sfb+1])
			for w := range info.groupLen[g] {
				l := left.spec[groupOff+128*w+lo : groupOff+128*w+hi]
				r := right.spec[groupOff+128*w+lo : groupOff+128*w+hi]
				for k := range r {
					r[k] = l[k] * scale
				}
			}
		}
		groupOff += 128 * info.groupLen[g]
	}
}
