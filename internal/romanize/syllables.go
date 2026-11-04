package romanize

// syllables are the pieces of a run of a script romanized a syllable at a
// time, each a space apart, and digits and punctuation as they go.
type syllables struct {
	pieces []piece
	last   string
	// word is set when the piece before is a syllable or a digit.
	word bool
}

// add adds the syllable latin, the run's bytes up to end.
func (b *syllables) add(end int, latin string) {
	if latin != "" {
		b.last = latin
		if len(b.pieces) > 0 {
			latin = " " + latin
		}
		b.word = true
	}
	b.pieces = append(b.pieces, piece{end, latin})
}

// repeat says the syllable before again, for a mark of repetition, the
// run's bytes up to end.
func (b *syllables) repeat(end int) { b.add(end, b.last) }

// plain adds text that isn't a syllable, the run's bytes up to end:
// digits, a space from a syllable, and punctuation.
func (b *syllables) plain(end int, text string) {
	digit := text != "" && text[0] >= '0' && text[0] <= '9'
	if digit && b.word && !b.digit() {
		text = " " + text
	}
	b.word = digit
	b.pieces = append(b.pieces, piece{end, text})
}

// digit tells whether the piece before ends with a digit.
func (b *syllables) digit() bool {
	if len(b.pieces) == 0 {
		return false
	}
	last := b.pieces[len(b.pieces)-1].text
	return last != "" && last[len(last)-1] >= '0' && last[len(last)-1] <= '9'
}

// bytesOf counts the bytes of the runes rs.
func bytesOf(rs []rune) int {
	n := 0
	for _, r := range rs {
		n += len(string(r))
	}
	return n
}
