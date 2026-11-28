package lyrics

import (
	"bytes"
	"encoding/binary"
	"regexp"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Lyrics kept as text, in a music file's tags or in a .lrc file beside it
// (#554), come as LRC, enhanced LRC whose words are timed too, TTML, or
// plain text.

var (
	// wordTagRe is a stamp of enhanced LRC, "<mm:ss.xx>", in a line's text,
	// of the groups of timeTagRe.
	wordTagRe = regexp.MustCompile(`<(\d+):(\d{1,2})(?:[.:](\d{1,3}))?>`)
	// voiceRe is what names the voice that sings a line in enhanced LRC,
	// as "v1:", before its text.
	voiceRe = regexp.MustCompile(`^v\d+:\s*`)
	// lrcTagRe is a line of LRC's metadata, as "[ar:Ada Field]".
	lrcTagRe = regexp.MustCompile(`^\[[A-Za-z#]+:[^\]]*\]$`)
)

// FromText reads lyrics kept as text: LRC, synced by the rule of
// timedLyrics, their words timed where stamps of enhanced LRC time them
// (enhancedWords); else TTML; else plain text, a line each (detecting
// embedded bilingual sections or explicit "--- Romanized ---" /
// "--- Translation ---" blocks when present). ok is false for text of no
// words, as LRC of only its metadata.
func FromText(text, source string) (l Lyrics, ok bool) {
	text = strings.TrimSpace(strings.TrimPrefix(text, "\uFEFF"))
	primary, roman, trans, transTo, hasMarkers := splitMarkedTranslation(text)
	parseBody := primary
	if !hasMarkers {
		parseBody = text
	}
	switch lines := parseLRC(parseBody, true); {
	case len(lines) > 0:
		l = timedLyrics(lines, source)
	case strings.HasPrefix(parseBody, "<") && strings.Contains(parseBody, "<tt"):
		var err error
		if l, err = ParseTTML(parseBody); err != nil {
			return Lyrics{}, false
		}
		l.Source = source
	default:
		// Lines of LRC's metadata aren't sung.
		var sung []string
		for line := range strings.SplitSeq(parseBody, "\n") {
			if !lrcTagRe.MatchString(strings.TrimSpace(line)) {
				sung = append(sung, line)
			}
		}
		joined := strings.TrimSpace(strings.Join(sung, "\n"))
		if !hasMarkers {
			l = PlainBilingual(joined, source)
		} else {
			l = Plain(joined, source)
		}
	}
	if hasMarkers {
		attachMarkedTranslation(&l, roman, trans, transTo)
	}
	for _, line := range l.Lines {
		if line.Text != "" {
			return l, true
		}
	}
	return Lyrics{}, false
}

// enhancedWords reads the words of a line of enhanced LRC, text after the
// line's times: each word after the stamp of when it is sung, as
// "<00:12.50>", which a stamp after it ends. It returns the text without
// its stamps, nor the "v1:" or "v2:" before it that names its voice, and
// its words; nil for a line of no stamps, or text before its first stamp,
// which lights as a whole. offset is that of the lyrics, as ParseLRC
// reads it.
func enhancedWords(text string, offset int) (string, []Word) {
	text = voiceRe.ReplaceAllString(text, "")
	stamps := wordTagRe.FindAllStringSubmatchIndex(text, -1)
	if len(stamps) == 0 {
		return text, nil
	}
	plain := wordTagRe.ReplaceAllString(text, "")
	if strings.TrimSpace(text[:stamps[0][0]]) != "" {
		return strings.TrimSpace(plain), nil
	}
	var words []Word
	for i, s := range stamps {
		ms := max(tagTime(text, s)-offset, 0)
		if n := len(words); n > 0 && words[n-1].EndMs < 0 {
			words[n-1].EndMs = max(ms, words[n-1].StartMs)
		}
		end := len(text)
		if i+1 < len(stamps) {
			end = stamps[i+1][0]
		}
		if word := strings.TrimSpace(text[s[1]:end]); word != "" {
			words = append(words, Word{Text: word, StartMs: ms, EndMs: -1})
		}
	}
	// The last word, of no stamp after it, lights as it starts.
	if n := len(words); n > 0 && words[n-1].EndMs < 0 {
		words[n-1].EndMs = words[n-1].StartMs
	}
	return strings.TrimSpace(plain), words
}

// shiftWords returns words d milliseconds later, for a line of enhanced
// LRC sung again, at another of its times: nil for none.
func shiftWords(words []Word, d int) []Word {
	if words == nil {
		return nil
	}
	out := make([]Word, len(words))
	for i, w := range words {
		out[i] = Word{Text: w.Text, StartMs: max(w.StartMs+d, 0), EndMs: max(w.EndMs+d, 0)}
	}
	return out
}

// DecodeText decodes a text file of lyrics, as a .lrc file: UTF-8, with a
// byte order mark or without; UTF-16 by its byte order mark; else
// ISO-8859-1, as older programs wrote.
func DecodeText(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}):
		return strings.ToValidUTF8(string(b[3:]), "\uFFFD")
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE}):
		return decodeUTF16(b[2:], binary.LittleEndian)
	case bytes.HasPrefix(b, []byte{0xFE, 0xFF}):
		return decodeUTF16(b[2:], binary.BigEndian)
	case utf8.Valid(b):
		return string(b)
	}
	rs := make([]rune, len(b))
	for i, c := range b {
		rs[i] = rune(c)
	}
	return string(rs)
}

// decodeUTF16 decodes UTF-16 of the byte order order, an odd last byte
// left out.
func decodeUTF16(b []byte, order binary.ByteOrder) string {
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = order.Uint16(b[2*i:])
	}
	return string(utf16.Decode(u))
}
