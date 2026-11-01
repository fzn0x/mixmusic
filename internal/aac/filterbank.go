package aac

// Port of Filterbank/FilterBank.cs: the inverse MDCT of each window of a
// channel, windowed and overlapped with what the last frame left.

const (
	shortLen = FrameLength / 8
	// mid is where the first short window starts, and the start window
	// ends its flat part.
	mid = (FrameLength - shortLen) / 2
	// trans is where, in the fifth short window, the frame ends.
	trans = shortLen / 2
)

// synthesize turns the spectrum of a channel, whose windows info tells,
// into its samples of the frame, out, and keeps the second half of its
// windows for the next frame.
func (d *Decoder) synthesize(c *channel, info *icsInfo, out []float32) {
	in, buf, overlap := c.spec[:], d.buf[:], c.overlap[:]
	long, longPrev := longWindows[info.shape], longWindows[c.prevShape]
	short, shortPrev := shortWindows[info.shape], shortWindows[c.prevShape]
	switch info.seq {
	case onlyLongSequence:
		mdctLong.inverse(in, buf, d.fft[:])
		// Add the second half of the last frame's window to the first
		// half of this one's, and keep this one's second half.
		for i := range FrameLength {
			out[i] = overlap[i] + buf[i]*longPrev[i]
		}
		for i := range FrameLength {
			overlap[i] = buf[FrameLength+i] * long[FrameLength-1-i]
		}

	case longStartSequence:
		mdctLong.inverse(in, buf, d.fft[:])
		for i := range FrameLength {
			out[i] = overlap[i] + buf[i]*longPrev[i]
		}
		// The second half is flat, then falls as a short window, then is
		// zero.
		for i := range mid {
			overlap[i] = buf[FrameLength+i]
		}
		for i := range shortLen {
			overlap[mid+i] = buf[FrameLength+mid+i] * short[shortLen-1-i]
		}
		clear(overlap[mid+shortLen:])

	case eightShortSequence:
		for w := range 8 {
			mdctShort.inverse(in[w*shortLen:], buf[2*w*shortLen:], d.fft[:])
		}
		// The short windows overlap each other from mid on, past the end
		// of the frame.
		copy(out[:mid], overlap[:mid])
		for i := range shortLen {
			fall, rise := short[shortLen-1-i], short[i]
			out[mid+i] = overlap[mid+i] + buf[i]*shortPrev[i]
			out[mid+1*shortLen+i] = overlap[mid+1*shortLen+i] + buf[1*shortLen+i]*fall + buf[2*shortLen+i]*rise
			out[mid+2*shortLen+i] = overlap[mid+2*shortLen+i] + buf[3*shortLen+i]*fall + buf[4*shortLen+i]*rise
			out[mid+3*shortLen+i] = overlap[mid+3*shortLen+i] + buf[5*shortLen+i]*fall + buf[6*shortLen+i]*rise
			if i < trans {
				out[mid+4*shortLen+i] = overlap[mid+4*shortLen+i] + buf[7*shortLen+i]*fall + buf[8*shortLen+i]*rise
			}
		}
		for i := range shortLen {
			fall, rise := short[shortLen-1-i], short[i]
			if i >= trans {
				overlap[mid+4*shortLen+i-FrameLength] = buf[7*shortLen+i]*fall + buf[8*shortLen+i]*rise
			}
			overlap[mid+5*shortLen+i-FrameLength] = buf[9*shortLen+i]*fall + buf[10*shortLen+i]*rise
			overlap[mid+6*shortLen+i-FrameLength] = buf[11*shortLen+i]*fall + buf[12*shortLen+i]*rise
			overlap[mid+7*shortLen+i-FrameLength] = buf[13*shortLen+i]*fall + buf[14*shortLen+i]*rise
			overlap[mid+8*shortLen+i-FrameLength] = buf[15*shortLen+i] * fall
		}
		clear(overlap[mid+shortLen:])

	case longStopSequence:
		mdctLong.inverse(in, buf, d.fft[:])
		// The first half is zero, then rises as a short window, then is
		// flat.
		copy(out[:mid], overlap[:mid])
		for i := range shortLen {
			out[mid+i] = overlap[mid+i] + buf[mid+i]*shortPrev[i]
		}
		for i := mid + shortLen; i < FrameLength; i++ {
			out[i] = overlap[i] + buf[i]
		}
		for i := range FrameLength {
			overlap[i] = buf[FrameLength+i] * long[FrameLength-1-i]
		}
	}
	c.prevShape = info.shape
}
