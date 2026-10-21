package audio

// Port of Blink's DynamicsCompressor (third_party/blink/renderer/platform/
// audio/dynamics_compressor.cc, under the BSD license of LICENSE.blink), the
// processing of DynamicsCompressorNode, for stereo at Rate. It computes in
// float as Blink does. The metering of the reduction, which the web player
// doesn't read, is left out.

import "math"

const (
	// compressorPreDelay is the compressor's look-ahead, in seconds.
	compressorPreDelay    = 0.006
	maxPreDelayFrames     = 1024
	maxPreDelayFramesMask = maxPreDelayFrames - 1
	defaultPreDelayFrames = 256
	// satReleaseTime is the release time of the detector, in seconds.
	satReleaseTime = 0.0025
	piOverTwo      = float32(math.Pi / 2)
)

// The release zones, from 0 to 1, and the coefficients of the polynomial
// through them, in float as Blink computes them.
var (
	releaseZone1 = float32(0.09)
	releaseZone2 = float32(0.16)
	releaseZone3 = float32(0.42)
	releaseZone4 = float32(0.98)

	kABase = float32(0.9999999999999998)*releaseZone1 + float32(1.8432219684323923e-16)*releaseZone2 -
		float32(1.9373394351676423e-16)*releaseZone3 + float32(8.824516011816245e-18)*releaseZone4
	kBBase = float32(-1.5788320352845888)*releaseZone1 + float32(2.3305837032074286)*releaseZone2 -
		float32(0.9141194204840429)*releaseZone3 + float32(0.1623677525612032)*releaseZone4
	kCBase = float32(0.5334142869106424)*releaseZone1 - float32(1.272736789213631)*releaseZone2 +
		float32(0.9258856042207512)*releaseZone3 - float32(0.18656310191776226)*releaseZone4
	kDBase = float32(0.08783463138207234)*releaseZone1 - float32(0.1694162967925622)*releaseZone2 +
		float32(0.08588057951595272)*releaseZone3 - float32(0.00429891410546283)*releaseZone4
	kEBase = float32(-0.042416883008123074)*releaseZone1 + float32(0.1115693827987602)*releaseZone2 -
		float32(0.09764676325265872)*releaseZone3 + float32(0.028494263462021576)*releaseZone4
)

// The settings of the web player's compressor, which levels the volume.
const (
	levelThreshold = -24  // dB
	levelKnee      = 24   // dB
	levelRatio     = 4    //
	levelAttack    = 0.01 // seconds
	levelRelease   = 0.25 // seconds
)

type compressor struct {
	// The static curve, for the parameters it was computed for.
	ratio, slope, linearThreshold, dbThreshold, dbKnee     float32
	kneeThreshold, dbKneeThreshold, dbYKneeThreshold, knee float32

	detectorAverage            float32
	compressorGain             float32
	dbMaxAttackCompressionDiff float32

	preDelay                    [2][maxPreDelayFrames]float32
	preDelayRead, preDelayWrite int
	lastPreDelayFrames          int
}

func newCompressor() *compressor {
	const uninitialized = -1
	c := &compressor{
		ratio: uninitialized, slope: uninitialized, linearThreshold: uninitialized,
		dbThreshold: uninitialized, dbKnee: uninitialized, kneeThreshold: uninitialized,
		dbKneeThreshold: uninitialized, dbYKneeThreshold: uninitialized, knee: uninitialized,
		lastPreDelayFrames: defaultPreDelayFrames,
	}
	c.reset()
	return c
}

// reset is DynamicsCompressor::Reset.
func (c *compressor) reset() {
	c.detectorAverage = 0
	c.compressorGain = 1
	c.preDelay = [2][maxPreDelayFrames]float32{}
	c.preDelayRead = 0
	c.preDelayWrite = defaultPreDelayFrames
	c.dbMaxAttackCompressionDiff = -1
}

func decibelsToLinear(db float32) float32 {
	return float32(math.Pow(10, float64(0.05*db)))
}

func linearToDecibels(linear float32) float32 {
	return 20 * float32(math.Log10(float64(linear)))
}

func ensureFinite(x, otherwise float32) float32 {
	if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
		return otherwise
	}
	return x
}

// flush32 is FlushDenormalFloatToZero.
func flush32(x float32) float32 {
	if math.Abs(float64(x)) < math.SmallestNonzeroFloat32*(1<<23) {
		return 0
	}
	return x
}

// process compresses interleaved stereo frames in place, with the
// threshold and knee in dB, the ratio, and the attack and release in
// seconds.
func (c *compressor) process(buf []float32, dbThreshold, dbKnee, ratio, attackTime, releaseTime float32) {
	frames := len(buf) / 2
	k := c.updateStaticCurveParameters(dbThreshold, dbKnee, ratio)
	// Makeup gain with empirical/perceptual tuning.
	linearPostGain := float32(math.Pow(float64(1/c.saturate(1, k)), float64(float32(0.6))))
	attackFrames := max(0.001, attackTime) * Rate
	releaseFrames := Rate * releaseTime
	satReleaseFrames := float32(satReleaseTime) * Rate
	// A smooth function through four points: y = a + b·x + c·x² + d·x³ + e·x⁴.
	a, b, cc, d, e := releaseFrames*kABase, releaseFrames*kBBase, releaseFrames*kCBase,
		releaseFrames*kDBase, releaseFrames*kEBase
	c.setPreDelayTime(compressorPreDelay)

	const divisionFrames = 32
	frame := 0
	for frame < frames {
		c.detectorAverage = ensureFinite(c.detectorAverage, 1)
		desiredGain := c.detectorAverage
		// Pre-warp so we get desiredGain after the sine warp below.
		scaledDesiredGain := float32(math.Asin(float64(desiredGain))) / piOverTwo

		// The rate the compressor slews from its current level to the
		// desired one: it depends on whether it attacks or releases, and by
		// how much.
		var envelopeRate float32
		isReleasing := scaledDesiredGain > c.compressorGain
		var dbCompressionDiff float32
		switch {
		case scaledDesiredGain == 0 && isReleasing:
			dbCompressionDiff = -1
		case scaledDesiredGain == 0:
			dbCompressionDiff = 1
		default:
			dbCompressionDiff = linearToDecibels(c.compressorGain / scaledDesiredGain)
		}
		if isReleasing {
			// Release mode: dbCompressionDiff should be negative dB.
			c.dbMaxAttackCompressionDiff = -1
			dbCompressionDiff = ensureFinite(dbCompressionDiff, -1)
			// Adaptive release: higher compression releases faster. Within
			// -12 to 0, then scaled to go from 0 to 3.
			x := min(max(dbCompressionDiff, -12), 0)
			x = 0.25 * (x + 12)
			x2 := x * x
			x3 := x2 * x
			x4 := x2 * x2
			calcReleaseFrames := a + b*x + cc*x2 + d*x3 + e*x4
			const dbSpacing = 5
			envelopeRate = decibelsToLinear(dbSpacing / calcReleaseFrames)
		} else {
			// Attack mode: dbCompressionDiff should be positive dB.
			dbCompressionDiff = ensureFinite(dbCompressionDiff, 1)
			// While attacking, the rate follows the largest difference so far.
			if c.dbMaxAttackCompressionDiff == -1 || c.dbMaxAttackCompressionDiff < dbCompressionDiff {
				c.dbMaxAttackCompressionDiff = dbCompressionDiff
			}
			dbEffAttenDiff := max(0.5, c.dbMaxAttackCompressionDiff)
			x := 0.25 / dbEffAttenDiff
			envelopeRate = 1 - float32(math.Pow(float64(x), float64(1/attackFrames)))
		}

		read, write := c.preDelayRead, c.preDelayWrite
		detectorAverage, compressorGain := c.detectorAverage, c.compressorGain
		for range min(divisionFrames, frames-frame) {
			// Delay the signal, computing the compression from the undelayed
			// one.
			var compressorInput float32
			for j := range 2 {
				undelayed := buf[2*frame+j]
				c.preDelay[j][write] = undelayed
				abs := -undelayed
				if undelayed > 0 {
					abs = undelayed
				}
				if compressorInput < abs {
					compressorInput = abs
				}
			}
			// The shaping curve is linear up to the threshold, then a knee,
			// then the ratio: each joins the next smoothly.
			absInput := compressorInput
			shapedInput := c.saturate(absInput, k)
			attenuation := float32(1)
			if !(absInput <= 0.0001) {
				attenuation = shapedInput / absInput
			}
			dbAttenuation := max(2, -linearToDecibels(attenuation))
			dbPerFrame := dbAttenuation / satReleaseFrames
			satReleaseRate := decibelsToLinear(dbPerFrame) - 1
			rate := float32(1)
			if attenuation > detectorAverage {
				rate = satReleaseRate
			}
			detectorAverage += (attenuation - detectorAverage) * rate
			detectorAverage = ensureFinite(min(1, detectorAverage), 1)

			// Exponential approach to the desired gain.
			if envelopeRate < 1 {
				// Attack: reduce the gain to the desired one.
				compressorGain += (scaledDesiredGain - compressorGain) * envelopeRate
			} else {
				// Release: increase the gain exponentially to 1.
				compressorGain = min(1, compressorGain*envelopeRate)
			}
			// Warp the gain to smooth out sharp exponential transitions.
			postWarpCompressorGain := float32(math.Sin(float64(piOverTwo * compressorGain)))
			totalGain := linearPostGain * postWarpCompressorGain
			for j := range 2 {
				buf[2*frame+j] = c.preDelay[j][read] * totalGain
			}
			frame++
			read = (read + 1) & maxPreDelayFramesMask
			write = (write + 1) & maxPreDelayFramesMask
		}
		c.preDelayRead, c.preDelayWrite = read, write
		c.detectorAverage = flush32(detectorAverage)
		c.compressorGain = flush32(compressorGain)
	}
}

// setPreDelayTime reconfigures the look-ahead when its time changes.
func (c *compressor) setPreDelayTime(seconds float32) {
	frames := min(int(seconds*Rate), maxPreDelayFrames-1)
	if c.lastPreDelayFrames != frames {
		c.lastPreDelayFrames = frames
		c.preDelay = [2][maxPreDelayFrames]float32{}
		c.preDelayRead = 0
		c.preDelayWrite = frames
	}
}

// kneeCurve is an exponential curve for the knee, 1st-derivative matched at
// the linear threshold, which approaches the threshold plus 1/k.
func (c *compressor) kneeCurve(x, k float32) float32 {
	if x < c.linearThreshold {
		return x
	}
	return c.linearThreshold + (1-float32(math.Exp(float64(-k*(x-c.linearThreshold)))))/k
}

// saturate is the full compression curve, with a constant ratio after the
// knee.
func (c *compressor) saturate(x, k float32) float32 {
	if x < c.kneeThreshold {
		return c.kneeCurve(x, k)
	}
	dbX := linearToDecibels(x)
	dbY := c.dbYKneeThreshold + c.slope*(dbX-c.dbKneeThreshold)
	return decibelsToLinear(dbY)
}

// kAtSlope finds the k of the knee whose slope, in dB, is desiredSlope.
func (c *compressor) kAtSlope(desiredSlope float32) float32 {
	dbX := c.dbThreshold + c.dbKnee
	x := decibelsToLinear(dbX)
	x2, dbX2 := float32(1), float32(0)
	if !(x < c.linearThreshold) {
		x2 = float32(float64(x) * 1.001)
		dbX2 = linearToDecibels(x2)
	}
	minK, maxK, k := float32(0.1), float32(10000), float32(5)
	slope := float32(1)
	for range 15 {
		// The 1st derivative, in dB, of the knee: the inverse of the ratio.
		if !(x < c.linearThreshold) {
			dbY := linearToDecibels(c.kneeCurve(x, k))
			dbY2 := linearToDecibels(c.kneeCurve(x2, k))
			slope = (dbY2 - dbY) / (dbX2 - dbX)
		}
		if slope < desiredSlope {
			maxK = k // too high
		} else {
			minK = k // too low
		}
		k = float32(math.Sqrt(float64(minK * maxK)))
	}
	return k
}

func (c *compressor) updateStaticCurveParameters(dbThreshold, dbKnee, ratio float32) float32 {
	if dbThreshold != c.dbThreshold || dbKnee != c.dbKnee || ratio != c.ratio {
		c.dbThreshold = dbThreshold
		c.linearThreshold = decibelsToLinear(dbThreshold)
		c.dbKnee = dbKnee
		c.ratio = ratio
		c.slope = 1 / ratio
		k := c.kAtSlope(1 / ratio)
		c.dbKneeThreshold = dbThreshold + dbKnee
		c.kneeThreshold = decibelsToLinear(c.dbKneeThreshold)
		c.dbYKneeThreshold = linearToDecibels(c.kneeCurve(c.kneeThreshold, k))
		c.knee = k
	}
	return c.knee
}
