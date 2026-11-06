package output

import "unsafe"

// audioBuffer is Core Audio's AudioBuffer: the channels its frames
// interleave, its size in bytes, and its data.
type audioBuffer struct {
	channels, size uint32
	data           unsafe.Pointer
}

// audioBufferList is the head of Core Audio's AudioBufferList, as 64-bit
// systems lay it out: the count of its buffers, then the first of them,
// 8 bytes in, which the others follow.
type audioBufferList struct {
	buffers uint32
	_       uint32
	first   audioBuffer
}

// appendStereo appends the float32 frames of l, a buffer of interleaved
// channels or a buffer for each, to out as interleaved stereo: the first
// two channels, or the only one on both sides. It appends no more than out
// has room for, so that Core Audio's I/O thread, which calls it, allocates
// nothing.
func appendStereo(out []float32, l *audioBufferList) []float32 {
	type channel struct {
		samples []float32
		stride  int
	}
	var chs [2]channel
	n, frames := 0, (cap(out)-len(out))/2
	for _, b := range unsafe.Slice(&l.first, l.buffers) {
		if b.data == nil || b.channels == 0 {
			continue
		}
		stride := int(b.channels)
		samples := unsafe.Slice((*float32)(b.data), b.size/4)
		frames = min(frames, len(samples)/stride)
		for c := 0; c < stride && n < len(chs); c++ {
			chs[n] = channel{samples[c:], stride}
			n++
		}
	}
	switch n {
	case 0:
		return out
	case 1:
		chs[1] = chs[0]
	}
	for i := range frames {
		out = append(out, chs[0].samples[i*chs[0].stride], chs[1].samples[i*chs[1].stride])
	}
	return out
}
