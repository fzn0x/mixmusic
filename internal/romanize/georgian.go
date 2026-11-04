package romanize

// Georgian is written by its national system (2002), which the State
// Department of Geodesy and Cartography and the Institute of Linguistics
// adopted and the President approved by decree 109 of 2011: its ejectives
// take an apostrophe, as კ k’, apart from the aspirates, as ქ k.

var georgianLetters = map[rune]string{
	'ა': "a", 'ბ': "b", 'გ': "g", 'დ': "d", 'ე': "e", 'ვ': "v", 'ზ': "z", 'თ': "t", 'ი': "i",
	'კ': "k’", 'ლ': "l", 'მ': "m", 'ნ': "n", 'ო': "o", 'პ': "p’", 'ჟ': "zh", 'რ': "r", 'ს': "s",
	'ტ': "t’", 'უ': "u", 'ფ': "p", 'ქ': "k", 'ღ': "gh", 'ყ': "q’", 'შ': "sh", 'ჩ': "ch", 'ც': "ts",
	'ძ': "dz", 'წ': "ts’", 'ჭ': "ch’", 'ხ': "kh", 'ჯ': "j", 'ჰ': "h",
	// The letters Georgian no longer writes.
	'ჱ': "e", 'ჲ': "y", 'ჳ': "w", 'ჴ': "q", 'ჵ': "o", 'ჶ': "f",
}

func init() { register(georgian, georgianRun, "ka") }

// georgianRun romanizes a run of Georgian: Mkhedruli, and its capitals,
// Mtavruli, which titles write.
func georgianRun(run, _ string) []piece {
	var pieces []piece
	for i, r := range run {
		lower, upper := r, false
		if r >= 0x1C90 && r <= 0x1CBF {
			// Mtavruli: the capitals, a block apart.
			lower, upper = r-0x1C90+0x10D0, true
		}
		latin, ok := georgianLetters[lower]
		if !ok {
			latin = plain(r)
		}
		if upper {
			latin = capital(latin, 'A')
		}
		pieces = append(pieces, piece{i + len(string(r)), latin})
	}
	return pieces
}
