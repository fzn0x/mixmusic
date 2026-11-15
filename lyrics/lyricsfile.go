package lyrics

import (
	"math"
	"slices"
	"strconv"
	"strings"
)

// LRCLIB keeps every record as a Lyricsfile too, a YAML document whose
// lines may have their words timed, which syncedLyrics, in LRC, can't
// tell: hasWordSync says they have. Its lines are a list of text, start_ms
// and end_ms, each with words, a list of text, start_ms and, sometimes,
// end_ms:
//
//	lines:
//	- text: This was a triumph.
//	  start_ms: 70
//	  end_ms: 1070
//	  words:
//	  - text: 'This '
//	    start_ms: 70
//
// readLyricsfile reads what LRCLIB writes of YAML: block mappings and
// sequences of plain, quoted and block scalars, without anchors or flow
// collections but the empty "[]".

// lyricsfileItem is an item of a Lyricsfile's lines, or of a line's words:
// its text and times, -1 where it doesn't tell them.
type lyricsfileItem struct {
	text       string
	start, end int
	words      []lyricsfileItem
}

// readLyricsfile returns the lines of a Lyricsfile, in their order of
// time, the words of each where they have times; ok is false for a
// document it can't read.
func readLyricsfile(doc string) (lines []Line, ok bool) {
	rows := strings.Split(strings.ReplaceAll(doc, "\r\n", "\n"), "\n")
	var items []lyricsfileItem
	in := false
	// lineAt is the indent of the lines' dashes; list is the key of the
	// line's list read now, whose items are words when it is "words".
	lineAt, list := -1, ""
	var word *lyricsfileItem
	wordAt := -1
	for i := 0; i < len(rows); i++ {
		raw := strings.TrimRight(rows[i], " \t")
		body := strings.TrimLeft(raw, " ")
		if body == "" || body[0] == '#' {
			continue
		}
		at := len(raw) - len(body)
		if strings.HasPrefix(body, "\t") {
			return nil, false // YAML indents with spaces only
		}
		if at == 0 && body[0] != '-' {
			key, value, _ := strings.Cut(body, ":")
			in = key == "lines"
			if in && strings.TrimSpace(value) != "" && strings.TrimSpace(value) != "[]" {
				return nil, false
			}
			word, list = nil, ""
			continue
		}
		if !in {
			continue
		}
		dash := body == "-" || strings.HasPrefix(body, "- ")
		if dash {
			rest := strings.TrimLeft(body[1:], " ")
			keyAt := at + len(body) - len(rest)
			if lineAt < 0 {
				lineAt = at
			}
			switch {
			case at == lineAt:
				items = append(items, lyricsfileItem{start: -1, end: -1})
				word, list = nil, ""
			case at > lineAt && list == "words" && len(items) > 0:
				cur := &items[len(items)-1]
				cur.words = append(cur.words, lyricsfileItem{start: -1, end: -1})
				word, wordAt = &cur.words[len(cur.words)-1], at
			default:
				// An item of a list MixMusic doesn't read.
				word, wordAt = &lyricsfileItem{}, at
			}
			if rest == "" {
				continue
			}
			body, at = rest, keyAt
		}
		if len(items) == 0 || at < lineAt {
			return nil, false
		}
		key, value, found := strings.Cut(body, ":")
		if !found {
			return nil, false
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if value == "|" || value == "|-" || value == "|+" || value == ">" || value == ">-" || value == ">+" {
			var block []string
			for i+1 < len(rows) {
				next := rows[i+1]
				if strings.TrimSpace(next) != "" && len(next)-len(strings.TrimLeft(next, " ")) <= at {
					break
				}
				block = append(block, strings.TrimSpace(next))
				i++
			}
			sep := "\n"
			if value[0] == '>' {
				sep = " "
			}
			value = strings.Join(block, sep)
		} else if v, ok := yamlScalar(value); ok {
			value = v
		} else {
			return nil, false
		}
		var item *lyricsfileItem
		switch {
		case word != nil && at == wordAt+2:
			item = word
		case at == lineAt+2:
			item, word = &items[len(items)-1], nil
			if value == "" || value == "[]" {
				list = key
			} else {
				list = ""
			}
		default:
			continue // a key of a mapping MixMusic doesn't read
		}
		switch key {
		case "text":
			item.text = value
		case "start_ms":
			item.start = msOf(value)
		case "end_ms":
			item.end = msOf(value)
		}
	}
	lines = []Line{}
	for _, it := range items {
		if it.start < 0 {
			continue
		}
		lines = append(lines, Line{TimeMs: it.start, Text: strings.TrimSpace(it.text), Words: it.timedWords()})
		if l := &lines[len(lines)-1]; l.Words != nil && !wordsIn(l.Text, l.Words) {
			// A line whose words its text doesn't hold, in their order, reads
			// as its words do.
			var b strings.Builder
			for _, w := range it.words {
				b.WriteString(w.text)
			}
			l.Text = strings.TrimSpace(b.String())
		}
	}
	slices.SortStableFunc(lines, func(a, b Line) int { return a.TimeMs - b.TimeMs })
	return lines, true
}

// timedWords returns the words of the line it with their times, trimmed: a
// word without an end lasts until the next starts, or the line ends. A line
// with a word of no time has none: it lights as a whole.
func (it lyricsfileItem) timedWords() []Word {
	var words []Word
	for _, w := range it.words {
		text := strings.TrimSpace(w.text)
		if text == "" {
			continue
		}
		if w.start < 0 {
			return nil
		}
		words = append(words, Word{Text: text, StartMs: w.start, EndMs: w.end})
	}
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
		case it.end > wd.StartMs:
			wd.EndMs = it.end
		}
	}
	return words
}

// wordsIn reports whether text holds the words, each the first found after
// the word before, as Word.Text promises.
func wordsIn(text string, words []Word) bool {
	from := 0
	for _, w := range words {
		i := strings.Index(text[from:], w.Text)
		if i < 0 {
			return false
		}
		from += i + len(w.Text)
	}
	return true
}

// yamlScalar returns the value of a scalar of YAML written on one line:
// plain, in single quotes or in double quotes; ok is false for one it
// can't read, as a quote that goes on to another line.
func yamlScalar(v string) (string, bool) {
	switch {
	case v == "":
		return "", true
	case v[0] == '\'':
		if len(v) < 2 || v[len(v)-1] != '\'' {
			return "", false
		}
		return strings.ReplaceAll(v[1:len(v)-1], "''", "'"), true
	case v[0] == '"':
		s, err := strconv.Unquote(v)
		return s, err == nil
	case v[0] == '{' || v[0] == '&' || v[0] == '*' || v[0] == '!':
		return "", false
	}
	if i := strings.Index(v, " #"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	if v == "~" || v == "null" {
		return "", true
	}
	return v, true
}

// msOf returns the milliseconds a time of a Lyricsfile tells, -1 for one
// that isn't a time.
func msOf(v string) int {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < 0 || f > math.MaxInt32 {
		return -1
	}
	return int(math.Round(f))
}

// wordSynced returns lyrics of the Lyricsfile doc timed word by word, by
// the rule of timedLyrics; ok is false when it can't be read, or none of
// its lines has its words timed.
func wordSynced(doc, source string) (Lyrics, bool) {
	lines, ok := readLyricsfile(doc)
	if !ok {
		return Lyrics{}, false
	}
	l := timedLyrics(lines, source)
	return l, l.WordTimed()
}
