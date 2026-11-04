package romanize

import "strings"

// Ethiopic is written by BGN/PCGN (1967) for Amharic: each syllable is a
// consonant and one of seven vowels, by its order in its row, e, u, i, a,
// ē, i and o, and the eighth of some rows is wa; the sixth, which Amharic
// often doesn't say, is a consonant alone at the end of a word, as in
// "ሰላም", "selam". The rows of the glottal stops and of h say their first
// order as a; the ejectives take an apostrophe, as ጠ t'.

// ethiopicRows are the consonants of the rows of Ethiopic, eight syllables
// each, from ሀ; "" for a row of a vowel alone, and "-" for none.
var ethiopicRows = strings.Fields("h l h m s r s sh k' kw' q qw' b v t ch h hw n ny ' k kw h hw w ' z zh y d d j g gw ng t' ch' p' ts' ts' f p")

// ethiopicOrders are the vowels of the orders.
var ethiopicOrders = [8]string{"e", "u", "i", "a", "ē", "i", "o", "wa"}

func init() { register(ethiopic, ethiopicRun, "am ti") }

// ethiopicRun romanizes a run of Ethiopic.
func ethiopicRun(run, _ string) []piece {
	var pieces []piece
	runes := []rune(run)
	at := 0
	for i, r := range runes {
		at += len(string(r))
		row, order := int(r-0x1200)/8, int(r-0x1200)%8
		if r < 0x1200 || r > 0x135A || row >= len(ethiopicRows) || ethiopicRows[row] == "-" {
			pieces = append(pieces, piece{at, plain(r)})
			continue
		}
		c := ethiopicRows[row]
		if c == "'" {
			c = ""
		}
		v := ethiopicOrders[order]
		labial := strings.HasSuffix(c, "w") || strings.HasSuffix(c, "w'")
		switch {
		case labial:
			// The rows of labialized consonants: kwe, kwi, kwa, kwē, kwi.
			c = strings.Replace(c, "w'", "'w", 1)
			v = [8]string{"e", "", "i", "a", "ē", "i", "", ""}[order]
		case order == 0 && strings.ContainsRune("hʾ", firstRuneOf(c)) || order == 0 && c == "":
			v = "a"
		case order == 5:
			// The sixth order is said only within a word, where two
			// consonants meet.
			end := i+1 == len(runes) || runes[i+1] < 0x1200 || runes[i+1] > 0x135A
			if end || c == "" {
				v = ""
			}
		case order == 7:
			v = "wa"
		}
		if c == "" && v == "" {
			v = "i"
		}
		pieces = append(pieces, piece{at, c + v})
	}
	return pieces
}

func firstRuneOf(s string) rune {
	for _, r := range s {
		return r
	}
	return 0
}
