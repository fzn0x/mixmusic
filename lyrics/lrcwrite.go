package lyrics

import (
	"fmt"
	"strings"
)

// LRC writes the lyrics as text to edit (#598), which FromText reads back:
// synced lyrics as LRC, each line after its time, "[mm:ss.xx]", and the
// words of a line timed word by word after theirs, "<mm:ss.xx>", as
// enhanced LRC has them; plain lyrics a line each, without times.
func (l Lyrics) LRC() string {
	var b strings.Builder
	for _, line := range l.Lines {
		if l.Synced {
			b.WriteString("[" + lrcStamp(line.TimeMs) + "]")
			b.WriteString(stampedWords(line))
		} else {
			b.WriteString(line.Text)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// lrcStamp is a time of LRC, as "01:02.50", in hundredths of a second.
func lrcStamp(ms int) string {
	ms = max(ms, 0)
	return fmt.Sprintf("%02d:%02d.%02d", ms/60000, ms/1000%60, ms%1000/10)
}

// stampedWords is the text of a line with each of its words after the
// stamp of when it is sung, as enhanced LRC writes them; its text alone
// for a line of no words, or whose words its text doesn't hold in their
// order, or holds after other words.
func stampedWords(line Line) string {
	if len(line.Words) == 0 {
		return line.Text
	}
	var b strings.Builder
	text, at := line.Text, 0
	for i, w := range line.Words {
		n := strings.Index(text[at:], w.Text)
		if n < 0 || w.Text == "" || i == 0 && strings.TrimSpace(text[:n]) != "" {
			return line.Text
		}
		b.WriteString(text[at : at+n])
		b.WriteString("<" + lrcStamp(w.StartMs) + ">")
		b.WriteString(w.Text)
		at += n + len(w.Text)
	}
	b.WriteString(text[at:])
	return strings.TrimLeft(b.String(), " ")
}
