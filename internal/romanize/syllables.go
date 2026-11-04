package romanize

// syllables are the pieces of a run of a script romanized a syllable at a
// time, each a space apart, and digits and punctuation as they go.
type syllables struct {
	pieces []piece
	at     int
	last   string
	// word is set when the piece before is a syllable or a digit.
	word bool
}

// add adds the syllable latin, made of the runes rs.
func (b *syllables) add(rs []rune, latin string) {
	if latin != "" {
		b.last = latin
		if len(b.pieces) > 0 {
			latin = " " + latin
		}
		b.word = true
	}
	b.put(rs, latin)
}

// repeat says the syllable before again, for the repeat mark rs.
func (b *syllables) repeat(rs []rune) { b.add(rs, b.last) }

// plain adds text, made of rs, that isn't a syllable: digits, a space
// from a syllable, and punctuation.
func (b *syllables) plain(rs []rune, text string) {
	digit := text != "" && text[0] >= '0' && text[0] <= '9'
	if digit && b.word && !b.digit() {
		text = " " + text
	}
	b.word = digit
	b.put(rs, text)
}

func (b *syllables) digit() bool {
	if len(b.pieces) == 0 {
		return false
	}
	last := b.pieces[len(b.pieces)-1].text
	return last != "" && last[len(last)-1] >= '0' && last[len(last)-1] <= '9'
}

func (b *syllables) put(rs []rune, text string) {
	for _, r := range rs {
		b.at += len(string(r))
	}
	b.pieces = append(b.pieces, piece{b.at, text})
}
