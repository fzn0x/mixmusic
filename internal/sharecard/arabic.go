package sharecard

import "strings"

// Arabic letters change shape by the letters beside them: joined to the
// letter before, to the one after, to both or to neither. The fonts have
// each shape as a letter of its own, in the block of presentation forms,
// and shapeArabic puts them where the shapes go. It is what a card's text
// needs, and not all a text engine does.

// dualJoining are the letters that join on both sides, by the code of their
// isolated form: the initial, final and medial forms follow it, as isolated,
// final, initial, medial.
var dualJoining = map[rune]rune{
	0x626: 0xFE89, 0x628: 0xFE8F, 0x62A: 0xFE95, 0x62B: 0xFE99, 0x62C: 0xFE9D, 0x62D: 0xFEA1,
	0x62E: 0xFEA5, 0x633: 0xFEB1, 0x634: 0xFEB5, 0x635: 0xFEB9, 0x636: 0xFEBD, 0x637: 0xFEC1,
	0x638: 0xFEC5, 0x639: 0xFEC9, 0x63A: 0xFECD, 0x641: 0xFED1, 0x642: 0xFED5, 0x643: 0xFED9,
	0x644: 0xFEDD, 0x645: 0xFEE1, 0x646: 0xFEE5, 0x647: 0xFEE9, 0x64A: 0xFEF1,
}

// rightJoining are the letters that join only to the letter before them,
// by the code of their isolated form: the final form follows it.
var rightJoining = map[rune]rune{
	0x622: 0xFE81, 0x623: 0xFE83, 0x624: 0xFE85, 0x625: 0xFE87, 0x627: 0xFE8D, 0x629: 0xFE93,
	0x62F: 0xFEA9, 0x630: 0xFEAB, 0x631: 0xFEAD, 0x632: 0xFEAF, 0x648: 0xFEED, 0x649: 0xFEEF,
}

// lamAlef are the ligatures of lam with an alef, by the alef: the isolated
// form, and the final one after it.
var lamAlef = map[rune]rune{0x622: 0xFEF5, 0x623: 0xFEF7, 0x625: 0xFEF9, 0x627: 0xFEFB}

// transparent reports whether r is a mark that joining passes over.
func transparent(r rune) bool { return r >= 0x64B && r <= 0x652 }

// joinsNext reports whether r joins to the letter after it.
func joinsNext(r rune) bool { _, ok := dualJoining[r]; return ok }

// joinsPrev reports whether r joins to the letter before it.
func joinsPrev(r rune) bool {
	if _, ok := dualJoining[r]; ok {
		return true
	}
	_, ok := rightJoining[r]
	return ok
}

// shapeArabic returns s with its Arabic letters in the shapes they take
// where they stand.
func shapeArabic(s string) string {
	if !strings.ContainsFunc(s, func(r rune) bool { return r >= 0x621 && r <= 0x64A }) {
		return s
	}
	rs := []rune(s)
	out := make([]rune, 0, len(rs))
	// before is the letter before, past marks, or 0.
	var before rune
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if transparent(r) {
			out = append(out, r)
			continue
		}
		if !joinsPrev(r) {
			before = 0
			if r < 0x621 || r > 0x64A {
				out = append(out, r)
				continue
			}
			// Hamza has one shape.
			out = append(out, 0xFE80)
			continue
		}
		joined := before != 0 && joinsNext(before)
		// Lam and alef make one letter.
		if r == 0x644 {
			j := i + 1
			for j < len(rs) && transparent(rs[j]) {
				j++
			}
			if j < len(rs) {
				if lig, ok := lamAlef[rs[j]]; ok {
					if joined {
						lig++
					}
					out = append(out, lig)
					out = append(out, rs[i+1:j]...)
					i = j
					before = 0
					continue
				}
			}
		}
		next := nextJoins(rs, i)
		if iso, ok := dualJoining[r]; ok {
			switch {
			case joined && next:
				out = append(out, iso+3)
			case joined:
				out = append(out, iso+1)
			case next:
				out = append(out, iso+2)
			default:
				out = append(out, iso)
			}
		} else {
			iso := rightJoining[r]
			if joined {
				iso++
			}
			out = append(out, iso)
		}
		before = r
	}
	return string(out)
}

// nextJoins reports whether the letter at i joins to the letter after it:
// it can, and the one after, past marks, can join to it.
func nextJoins(rs []rune, i int) bool {
	if !joinsNext(rs[i]) {
		return false
	}
	for j := i + 1; j < len(rs); j++ {
		if transparent(rs[j]) {
			continue
		}
		return joinsPrev(rs[j])
	}
	return false
}
