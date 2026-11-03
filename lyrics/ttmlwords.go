package lyrics

import (
	"encoding/xml"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The words of TTML synced word by word are its spans with a begin time:
// a word, or a syllable of one, with the space between words outside of
// them. ParseTTML keeps, with each rune of a paragraph's text, the span
// it comes from, and makes words of those once the text is whole.

// lineText is the text of a paragraph as ParseTTML reads it, each rune
// with the word it belongs to: the index of its timed span among the
// paragraph's, or -1.
type lineText struct {
	runes  []rune
	owners []int
}

func (t *lineText) reset() { t.runes, t.owners = t.runes[:0], t.owners[:0] }

// write adds s, of the word owner.
func (t *lineText) write(s string, owner int) {
	for _, r := range s {
		t.runes = append(t.runes, r)
		t.owners = append(t.owners, owner)
	}
}

// add adds the text u.
func (t *lineText) add(u lineText) {
	t.runes = append(t.runes, u.runes...)
	t.owners = append(t.owners, u.owners...)
}

// collapsed returns the text with each run of white space made one space,
// and trimmed, as XML reads text: the space belongs to the word the run's
// first does.
func (t *lineText) collapsed() lineText {
	var out lineText
	space, owner := false, -1
	for i, r := range t.runes {
		if unicode.IsSpace(r) {
			if !space {
				space, owner = true, t.owners[i]
			}
			continue
		}
		if space && len(out.runes) > 0 {
			out.runes = append(out.runes, ' ')
			out.owners = append(out.owners, owner)
		}
		space = false
		out.runes = append(out.runes, r)
		out.owners = append(out.owners, t.owners[i])
	}
	return out
}

// parenthesized returns a background vocal in parentheses, unless it is
// already: they belong to its first word and its last.
func (t lineText) parenthesized() lineText {
	if len(t.runes) == 0 || t.runes[0] == '(' {
		return t
	}
	var out lineText
	out.write("(", t.owners[0])
	out.add(t)
	out.write(")", t.owners[len(t.owners)-1])
	return out
}

// span is a timed span of a paragraph: when its word is sung, in
// milliseconds, end -1 where the span doesn't tell.
type span struct{ begin, end int }

// wordTimes are the timed spans of the paragraph read, and the word each
// element open in it is in, -1 outside of them.
type wordTimes struct {
	spans []span
	open  []int
}

func (w *wordTimes) reset() { w.spans, w.open = w.spans[:0], w.open[:0] }

// enter takes in element e opening in the paragraph: a span with a time
// of its own is a word, and what is in it is of that word, unless timed
// itself.
func (w *wordTimes) enter(e xml.StartElement) {
	owner := w.at()
	if begin, ok := clockTime(attr(e, "begin")); ok {
		owner = len(w.spans)
		w.spans = append(w.spans, span{begin, endOf(e, begin)})
	}
	w.open = append(w.open, owner)
}

// leave takes in the element entered last closing.
func (w *wordTimes) leave() {
	if len(w.open) > 0 {
		w.open = w.open[:len(w.open)-1]
	}
}

// at returns the word of the element open last, -1 for none.
func (w *wordTimes) at() int {
	if len(w.open) == 0 {
		return -1
	}
	return w.open[len(w.open)-1]
}

// endOf returns when element e, sung from begin, ends: at its end, or its
// begin and its dur; -1 when it doesn't tell.
func endOf(e xml.StartElement, begin int) int {
	if end, ok := clockTime(attr(e, "end")); ok {
		return end
	}
	if dur, ok := clockTime(attr(e, "dur")); ok && begin >= 0 {
		return begin + dur
	}
	return -1
}

// words returns the words of the line text t, which their spans time, in
// the order they come in it; the paragraph ends at end, -1 when it
// doesn't tell. A line with words that have no time, or whose words a
// search from each to the next wouldn't find, as Word.Text promises, has
// none: it lights as a whole.
func (w *wordTimes) words(t lineText, end int) []Word {
	if len(w.spans) == 0 {
		return nil
	}
	first, last := make([]int, len(w.spans)), make([]int, len(w.spans))
	for i := range first {
		first[i] = -1
	}
	for i, r := range t.runes {
		o := t.owners[i]
		switch {
		case unicode.IsSpace(r):
		case o < 0:
			// Punctuation between words lights with the word before.
			if unicode.IsLetter(r) || unicode.IsNumber(r) {
				return nil
			}
		default:
			if first[o] < 0 {
				first[o] = i
			}
			last[o] = i
		}
	}
	var order []int
	for o := range w.spans {
		if first[o] >= 0 {
			order = append(order, o)
		}
	}
	if len(order) == 0 {
		return nil
	}
	slices.SortFunc(order, func(a, b int) int { return first[a] - first[b] })
	// The byte each rune starts at.
	at := make([]int, len(t.runes)+1)
	for i, r := range t.runes {
		at[i+1] = at[i] + utf8.RuneLen(r)
	}
	text := string(t.runes)
	words := make([]Word, 0, len(order))
	from := 0
	for _, o := range order {
		a, b := first[o], last[o]+1
		for i := a; i < b; i++ {
			if t.owners[i] != o && !unicode.IsSpace(t.runes[i]) {
				return nil // a word inside another
			}
		}
		word := text[at[a]:at[b]]
		if strings.Index(text[from:], word) != at[a]-from {
			return nil
		}
		from = at[b]
		words = append(words, Word{Text: word, StartMs: w.spans[o].begin, EndMs: w.spans[o].end})
	}
	// A word without an end of its own lasts until the next starts, or
	// the line ends.
	for i := range words {
		wd := &words[i]
		if wd.EndMs >= 0 {
			wd.EndMs = max(wd.EndMs, wd.StartMs)
			continue
		}
		wd.EndMs = wd.StartMs
		switch {
		case i+1 < len(words) && words[i+1].StartMs > wd.StartMs:
			wd.EndMs = words[i+1].StartMs
		case end > wd.StartMs:
			wd.EndMs = end
		}
	}
	return words
}
