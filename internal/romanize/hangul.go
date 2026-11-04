package romanize

// Korean is written by the Revised Romanization of Korean (Ministry of
// Culture and Tourism, notice 2000-8), as it sounds: its special
// provisions write the sound changes within a word, and so does korean,
// syllable by syllable:
//
//   - liaison: a final goes on to a syllable that starts with a vowel
//     (한국어, hangugeo; 읽어, ilgeo; 좋아, joa);
//   - nasalization: k, t and p before n or m are ng, n and m (국물,
//     gungmul; 있는, inneun), and r after a consonant but l is n (종로,
//     jongno; 백리, baengni);
//   - liquidization: n beside l is l (신라, silla; 설날, seollal);
//   - aspiration: g, d, b and j beside h are k, t, p and ch (좋다, jota;
//     축하, chuka; 잡혀, japyeo), though the provisions keep it for nouns,
//     which korean can't tell;
//   - palatalization: d and t before i are j and ch (같이, gachi; 굳히다,
//     guchida).
//
// Tensing isn't written, as the provisions have it. Each word, as the
// spaces of the lyrics part them, is romanized apart: the syllables of
// Hangul are decomposed by arithmetic, with no data.

// The parts of a syllable of Hangul, as Unicode orders them: 19 initials,
// 21 vowels and 28 finals, the first of which is none.
const (
	hangulFirst = 0xAC00
	hangulLast  = 0xD7A3
	vowelCount  = 21
	finalCount  = 28
)

// initials are the romanizations of the initials, before a vowel.
var initials = [19]string{"g", "kk", "n", "d", "tt", "r", "m", "b", "pp", "s", "ss", "", "j", "jj", "ch", "k", "t", "p", "h"}

// The initials korean changes.
const (
	iG, iKK, iN, iD, iTT, iR, iM, iB, iPP, iS, iSS, iNone, iJ, iJJ, iCh, iK, iT, iP, iH = 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18
)

// vowels are the romanizations of the vowels.
var vowels = [vowelCount]string{"a", "ae", "ya", "yae", "eo", "e", "yeo", "ye", "o", "wa", "wae", "oe", "yo", "u", "wo", "we", "wi", "yu", "eu", "ui", "i"}

// vowelI is the vowel ㅣ, which palatalizes; and those that start with
// the sound i, ㅑ ㅕ ㅛ ㅠ ㅒ ㅖ, which do too.
const vowelI = 20

// finals are what each final is made of: the consonant that stays, and,
// for a pair, the one that goes on to a vowel after it, as initials;
// -1 for none.
var finals = [finalCount][2]int{
	{-1, -1}, {iG, -1}, {iKK, -1}, {iG, iS}, {iN, -1}, {iN, iJ}, {iN, iH}, {iD, -1},
	{iR, -1}, {iR, iG}, {iR, iM}, {iR, iB}, {iR, iS}, {iR, iT}, {iR, iP}, {iR, iH},
	{iM, -1}, {iB, -1}, {iB, iS}, {iS, -1}, {iSS, -1}, {-1, -1}, {iJ, -1}, {iCh, -1},
	{iK, -1}, {iT, -1}, {iP, -1}, {iH, -1},
}

// The finals by their sound before a consonant, or at the end: the seven
// a final can be.
const (
	fNone, fK, fN, fT, fL, fM, fP, fNg = 0, 1, 2, 3, 4, 5, 6, 7
)

var finalSounds = [8]string{"", "k", "n", "t", "l", "m", "p", "ng"}

// soundOf is the sound of a final consonant written as the initial c,
// before a consonant: ㄱ ㄲ ㅋ are k, ㄷ ㅅ ㅆ ㅈ ㅊ ㅌ ㅎ are t, and so on.
func soundOf(c int) int {
	switch c {
	case iG, iKK, iK:
		return fK
	case iN:
		return fN
	case iD, iS, iSS, iJ, iCh, iT, iH:
		return fT
	case iR:
		return fL
	case iM:
		return fM
	case iB, iP:
		return fP
	}
	return fNone
}

// syllable is a syllable of a word under way: its initial, its vowel,
// and its final's sound, which the syllables around it change.
type syllable struct {
	initial, vowel, final int
	// tail is the final as written, a pair or one, for what the next
	// syllable does to it.
	tail int
	// end is where the syllable ends in its run.
	end int
	// jamo is set for a letter written alone, as ㅋ, which is written as
	// itself.
	jamo string
}

func init() { register(hangul, korean, "ko") }

// korean romanizes a run of Hangul, word by word as spaces part them:
// the run holds no spaces, so it is one word.
func korean(run, _ string) []piece {
	var word []syllable
	for i, r := range run {
		end := i + len(string(r))
		if r < hangulFirst || r > hangulLast {
			word = append(word, syllable{initial: -1, end: end, jamo: jamoOf(r)})
			continue
		}
		n := int(r - hangulFirst)
		t := n % finalCount
		word = append(word, syllable{initial: n / (vowelCount * finalCount), vowel: n % (vowelCount * finalCount) / finalCount, tail: t, end: end})
	}
	soundChanges(word)
	pieces := make([]piece, len(word))
	for i, s := range word {
		text := s.jamo
		if s.initial >= 0 {
			text = initials[s.initial] + vowels[s.vowel] + finalSounds[s.final]
			// ㄹ is r before a vowel, and l after another, as in 빨리,
			// ppalli.
			if s.initial == iR && i > 0 && word[i-1].final == fL {
				text = "l" + text[1:]
			}
		}
		pieces[i] = piece{s.end, text}
	}
	return pieces
}

// soundChanges sets the initial and the final of each syllable of a word
// as the sounds around them change them.
func soundChanges(w []syllable) {
	for i := range w {
		s := &w[i]
		if s.initial < 0 {
			continue
		}
		f := finals[s.tail]
		var next *syllable
		if i+1 < len(w) && w[i+1].initial >= 0 {
			next = &w[i+1]
		}
		// The final as it sounds before a consonant, or at the end: of a
		// pair, the one that stays, but ㄺ ㄻ ㄿ, whose second does.
		s.final = fNone
		switch {
		case s.tail == 21:
			s.final = fNg
		case f[0] >= 0:
			s.final = soundOf(f[0])
			switch s.tail {
			case 9: // ㄺ: k, but l before ㄱ, as in 읽고, ilgo.
				s.final = fK
				if next != nil && next.initial == iG {
					s.final = fL
				}
			case 10: // ㄻ
				s.final = fM
			case 14: // ㄿ
				s.final = fP
			}
		}
		if next == nil {
			continue
		}
		stays, goes := f[0], f[1]
		switch {
		case next.initial == iNone && s.tail != 0 && s.tail != 21:
			// Liaison: the final goes on to the vowel, ㅎ is silent, and
			// ㄷ and ㅌ before ㅣ are ㅈ and ㅊ.
			// Of a pair, the second goes on, but a silent ㅎ, which lets
			// the first go: 싫어, sireo.
			moved := stays
			s.final = fNone
			if goes >= 0 && goes != iH {
				s.final = soundOf(stays)
				moved = goes
			}
			if moved == iH {
				moved = iNone
			}
			if next.vowel == vowelI {
				switch moved {
				case iD:
					moved = iJ
				case iT:
					moved = iCh
				}
			}
			if moved != iNone {
				next.initial = moved
			}
		case next.initial == iH && (stays == iH || goes == iH):
			// ㅎ before ㅎ is one.
			s.final = fNone
			if goes == iH {
				s.final = soundOf(stays)
			}
		case next.initial == iH:
			// Aspiration: k, t, p and ㅈ before ㅎ are ㅋ, ㅌ, ㅍ, ㅊ, and
			// ㄷ before 히 is 치.
			last := stays
			if goes >= 0 {
				last = goes
			}
			aspirated := -1
			switch soundOf(last) {
			case fK:
				aspirated = iK
			case fP:
				aspirated = iP
			case fT:
				aspirated = iT
				if last == iJ || last == iCh || next.vowel == vowelI {
					aspirated = iCh
				}
			}
			if aspirated >= 0 {
				next.initial = aspirated
				s.final = fNone
				if goes >= 0 {
					s.final = soundOf(stays)
					if stays == iR {
						s.final = fL
					}
				}
			}
		case s.tail == 27 || s.tail == 6 || s.tail == 15:
			// ㅎ, ㄶ and ㅀ before a consonant: g, d, j after it are k, t,
			// ch, and n after it is n (놓는, nonneun), or l after ㅀ.
			h := map[int]int{iG: iK, iD: iT, iJ: iCh}
			if a, ok := h[next.initial]; ok {
				next.initial = a
				s.final = fNone
				if s.tail != 27 {
					s.final = soundOf(stays)
				}
			} else if next.initial == iN {
				s.final = fN
				if s.tail == 15 {
					s.final, next.initial = fL, iR
				}
			} else if next.initial == iS {
				s.final = soundOf(stays)
				if s.tail == 27 {
					s.final = fNone
				}
			}
		}
		// Nasalization and liquidization, on the sounds as they are now.
		switch {
		case next.initial == iR && s.final == fN:
			s.final = fL
		case next.initial == iN && s.final == fL:
			next.initial = iR
		case next.initial == iR && s.final != fNone && s.final != fL:
			next.initial = iN
		}
		if next.initial == iN || next.initial == iM {
			switch s.final {
			case fK:
				s.final = fNg
			case fT:
				s.final = fN
			case fP:
				s.final = fM
			}
		}
	}
}

// jamoOf writes a letter of Hangul written alone, as in ㅋㅋ or ㅠㅠ: a
// consonant as an initial, a vowel as itself.
func jamoOf(r rune) string {
	const consonants = "ㄱㄲㄳㄴㄵㄶㄷㄸㄹㄺㄻㄼㄽㄾㄿㅀㅁㅂㅃㅄㅅㅆㅇㅈㅉㅊㅋㅌㅍㅎ"
	latin := []string{"g", "kk", "gs", "n", "nj", "nh", "d", "tt", "r", "lg", "lm", "lb", "ls", "lt", "lp", "lh", "m", "b", "pp", "bs", "s", "ss", "ng", "j", "jj", "ch", "k", "t", "p", "h"}
	i := 0
	for _, c := range consonants {
		if c == r {
			return latin[i]
		}
		i++
	}
	if r >= 0x314F && r <= 0x3163 {
		return vowels[r-0x314F]
	}
	return plain(r)
}
