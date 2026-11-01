package hct

import "math"

// The solver finds the sRGB colour of a hue, a chroma and a tone, as
// Material Color Utilities' HctSolver does: by Newton's method on CAM16's
// lightness J, and, where the chroma is out of sRGB's gamut, by bisecting
// the edges of the sRGB cube at the tone's luminance for the hue.

var (
	scaledDiscountFromLinRGB = [3][3]float64{
		{0.001200833568784504, 0.002389694492170889, 0.0002795742885861124},
		{0.0005891086651375999, 0.0029785502573438758, 0.0003270666104008398},
		{0.00010146692491640572, 0.0005364214359186694, 0.0032979401770712076},
	}
	linRGBFromScaledDiscount = [3][3]float64{
		{1373.2198709594231, -1100.4251190754821, -7.278681089101213},
		{-271.815969077903, 559.6580465940733, -32.46047482791194},
		{1.9622899599665666, -57.173814538844006, 308.7233197812385},
	}
	yFromLinRGB = [3]float64{0.2126, 0.7152, 0.0722}
	// criticalPlanes are the linear values, from 0 to 100, halfway
	// between neighbouring components of sRGB: where a component rounds
	// to the next.
	criticalPlanes = func() (planes [255]float64) {
		for i := range planes {
			n := (float64(i) + 0.5) / 255
			if n <= 0.040449936 {
				planes[i] = n / 12.92 * 100
			} else {
				planes[i] = math.Pow((n+0.055)/1.055, 2.4) * 100
			}
		}
		return planes
	}()
)

// solve returns the sRGB colour of hue, chroma and tone, or of the most
// chroma sRGB has at them.
func solve(hue, chroma, tone float64) ARGB {
	if chroma < 0.0001 || tone < 0.0001 || tone > 99.9999 {
		return gray(tone)
	}
	hueRadians := sanitizeDegrees(hue) / 180 * math.Pi
	y := yFromLstar(tone)
	if exact := findByJ(hueRadians, chroma, y); exact != 0 {
		return exact
	}
	return fromLinRGB(bisectToLimit(y, hueRadians))
}

func sanitizeRadians(a float64) float64 { return math.Mod(a+math.Pi*8, math.Pi*2) }

func chromaticAdaptation(c float64) float64 {
	af := math.Pow(math.Abs(c), 0.42)
	return signum(c) * 400 * af / (af + 27.13)
}

// hueOf is the hue, in radians, of a linear sRGB colour.
func hueOf(linrgb [3]float64) float64 {
	d := mul(linrgb, scaledDiscountFromLinRGB)
	rA, gA, bA := chromaticAdaptation(d[0]), chromaticAdaptation(d[1]), chromaticAdaptation(d[2])
	a := (11*rA + -12*gA + bA) / 11
	b := (rA + gA - 2*bA) / 9
	return math.Atan2(b, a)
}

func inCyclicOrder(a, b, c float64) bool { return sanitizeRadians(b-a) < sanitizeRadians(c-a) }

func lerpPoint(s [3]float64, t float64, d [3]float64) [3]float64 {
	return [3]float64{s[0] + (d[0]-s[0])*t, s[1] + (d[1]-s[1])*t, s[2] + (d[2]-s[2])*t}
}

func setCoordinate(s [3]float64, coordinate float64, d [3]float64, axis int) [3]float64 {
	return lerpPoint(s, (coordinate-s[axis])/(d[axis]-s[axis]), d)
}

func bounded(x float64) bool { return 0 <= x && x <= 100 }

// nthVertex is the nth of the 12 points where the plane of luminance y
// crosses the edges of the cube of linear sRGB, or {-1, -1, -1} where it
// misses that edge.
func nthVertex(y float64, n int) [3]float64 {
	kR, kG, kB := yFromLinRGB[0], yFromLinRGB[1], yFromLinRGB[2]
	coordA, coordB := 0.0, 0.0
	if n%4 > 1 {
		coordA = 100
	}
	if n%2 != 0 {
		coordB = 100
	}
	none := [3]float64{-1, -1, -1}
	switch {
	case n < 4:
		g, b := coordA, coordB
		if r := (y - g*kG - b*kB) / kR; bounded(r) {
			return [3]float64{r, g, b}
		}
	case n < 8:
		b, r := coordA, coordB
		if g := (y - r*kR - b*kB) / kG; bounded(g) {
			return [3]float64{r, g, b}
		}
	default:
		r, g := coordA, coordB
		if b := (y - r*kR - g*kG) / kB; bounded(b) {
			return [3]float64{r, g, b}
		}
	}
	return none
}

// bisectToSegment returns the two vertices of the polygon of luminance y
// whose hues are on either side of targetHue.
func bisectToSegment(y, targetHue float64) (left, right [3]float64) {
	left = [3]float64{-1, -1, -1}
	right = left
	var leftHue, rightHue float64
	initialized, uncut := false, true
	for n := range 12 {
		mid := nthVertex(y, n)
		if mid[0] < 0 {
			continue
		}
		midHue := hueOf(mid)
		if !initialized {
			left, right, leftHue, rightHue, initialized = mid, mid, midHue, midHue, true
			continue
		}
		if uncut || inCyclicOrder(leftHue, midHue, rightHue) {
			uncut = false
			if inCyclicOrder(leftHue, targetHue, midHue) {
				right, rightHue = mid, midHue
			} else {
				left, leftHue = mid, midHue
			}
		}
	}
	return left, right
}

func midpoint(a, b [3]float64) [3]float64 {
	return [3]float64{(a[0] + b[0]) / 2, (a[1] + b[1]) / 2, (a[2] + b[2]) / 2}
}

func criticalPlaneBelow(x float64) int { return int(math.Floor(x - 0.5)) }
func criticalPlaneAbove(x float64) int { return int(math.Ceil(x - 0.5)) }

// bisectToLimit returns the linear sRGB colour of luminance y and of
// targetHue at the gamut's edge: the most chroma sRGB has there.
func bisectToLimit(y, targetHue float64) [3]float64 {
	left, right := bisectToSegment(y, targetHue)
	leftHue := hueOf(left)
	for axis := range 3 {
		if left[axis] == right[axis] {
			continue
		}
		var lPlane, rPlane int
		if left[axis] < right[axis] {
			lPlane = criticalPlaneBelow(trueDelinearized(left[axis]))
			rPlane = criticalPlaneAbove(trueDelinearized(right[axis]))
		} else {
			lPlane = criticalPlaneAbove(trueDelinearized(left[axis]))
			rPlane = criticalPlaneBelow(trueDelinearized(right[axis]))
		}
		for range 8 {
			if abs(rPlane-lPlane) <= 1 {
				break
			}
			mPlane := int(math.Floor(float64(lPlane+rPlane) / 2))
			mid := setCoordinate(left, criticalPlanes[mPlane], right, axis)
			midHue := hueOf(mid)
			if inCyclicOrder(leftHue, targetHue, midHue) {
				right, rPlane = mid, mPlane
			} else {
				left, leftHue, lPlane = mid, midHue, mPlane
			}
		}
	}
	return midpoint(left, right)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// findByJ finds the colour of the hue, in radians, the chroma and the
// luminance y by Newton's method on J, or 0 where sRGB has none.
func findByJ(hueRadians, chroma, y float64) ARGB {
	j := math.Sqrt(y) * 11
	tInner := 1 / math.Pow(1.64-math.Pow(0.29, vc.n), 0.73)
	eHue := 0.25 * (math.Cos(hueRadians+2) + 3.8)
	p1 := eHue * (50000.0 / 13.0) * vc.nc * vc.ncb
	hSin, hCos := math.Sin(hueRadians), math.Cos(hueRadians)
	for round := range 5 {
		jn := j / 100
		alpha := 0.0
		if chroma != 0 && j != 0 {
			alpha = chroma / math.Sqrt(jn)
		}
		t := math.Pow(alpha*tInner, 1/0.9)
		ac := vc.aw * math.Pow(jn, 1/vc.c/vc.z)
		p2 := ac / vc.nbb
		gamma := 23 * (p2 + 0.305) * t / (23*p1 + 11*t*hCos + 108*t*hSin)
		a, b := gamma*hCos, gamma*hSin
		rA := (460*p2 + 451*a + 288*b) / 1403
		gA := (460*p2 - 891*a - 261*b) / 1403
		bA := (460*p2 - 220*a - 6300*b) / 1403
		inverse := func(adapted float64) float64 {
			abs := math.Abs(adapted)
			base := max(0, 27.13*abs/(400-abs))
			return signum(adapted) * math.Pow(base, 1/0.42)
		}
		linrgb := mul([3]float64{inverse(rA), inverse(gA), inverse(bA)}, linRGBFromScaledDiscount)
		if linrgb[0] < 0 || linrgb[1] < 0 || linrgb[2] < 0 {
			return 0
		}
		fnj := yFromLinRGB[0]*linrgb[0] + yFromLinRGB[1]*linrgb[1] + yFromLinRGB[2]*linrgb[2]
		if fnj <= 0 {
			return 0
		}
		if round == 4 || math.Abs(fnj-y) < 0.002 {
			if linrgb[0] > 100.01 || linrgb[1] > 100.01 || linrgb[2] > 100.01 {
				return 0
			}
			return fromLinRGB(linrgb)
		}
		j -= (fnj - y) * j / (2 * fnj)
	}
	return 0
}
