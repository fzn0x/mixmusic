package lyrics

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
)

// ParseTTML reads lyrics in TTML, the timed text XML that lyrics
// synchronized word by word, or line by line, come in. Each paragraph
// (<p>) is a line, sung at its begin time; the words of a line synced word
// by word are read as its text, and their own times are left out.
// Background vocals (spans of the role x-bg) follow the line's words, in
// parentheses.
//
// As ParseLRC, it leaves out lines without times in lyrics that have some.
// Lyrics without any times are plain. The Source of the lyrics is left
// for the caller to set.
func ParseTTML(ttml string) (Lyrics, error) {
	d := xml.NewDecoder(strings.NewReader(ttml))
	// Lyrics are typed by people: an ampersand or an entity of HTML, as
	// &nbsp;, shouldn't lose the whole song.
	d.Strict = false
	d.AutoClose = xml.HTMLAutoClose
	d.Entity = xml.HTMLEntity

	var (
		timed, untimed []Line
		in             bool // in a paragraph
		begin          string
		text, bg       strings.Builder
		// skip is the depth of an element whose text isn't sung, as a
		// translation, and bgAt the depth of a background vocal, 0
		// outside of them; depth counts the elements open in the
		// paragraph.
		depth, skip, bgAt int
	)
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Lyrics{}, fmt.Errorf("lyrics: reading TTML: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if !in {
				if t.Name.Local == "p" {
					in, depth, skip, bgAt = true, 0, 0, 0
					begin = attr(t, "begin")
					text.Reset()
					bg.Reset()
				}
				continue
			}
			depth++
			switch role := attr(t, "role"); {
			case skip > 0:
				continue
			case t.Name.Local == "metadata" || unsung[role]:
				skip = depth
				continue
			case role == "x-bg" && bgAt == 0:
				bgAt = depth
				bg.Reset()
			case t.Name.Local == "br":
				write(&text, &bg, bgAt, " ")
			}
			// A paragraph without a time of its own is sung at its
			// first word's.
			if begin == "" {
				begin = attr(t, "begin")
			}
		case xml.EndElement:
			if !in {
				continue
			}
			if depth == 0 { // the paragraph's own end
				in = false
				line := Line{Text: collapse(text.String())}
				ms, ok := clockTime(begin)
				if !ok {
					untimed = append(untimed, line)
					continue
				}
				line.TimeMs = ms
				timed = append(timed, line)
				continue
			}
			switch depth {
			case skip:
				skip = 0
			case bgAt:
				bgAt = 0
				if vocal := collapse(bg.String()); vocal != "" {
					if !strings.HasPrefix(vocal, "(") {
						vocal = "(" + vocal + ")"
					}
					text.WriteString(" " + vocal)
				}
			}
			depth--
		case xml.CharData:
			if in && skip == 0 {
				write(&text, &bg, bgAt, string(t))
			}
		}
	}
	if len(timed) > 0 {
		slices.SortStableFunc(timed, func(a, b Line) int { return a.TimeMs - b.TimeMs })
		return Lyrics{Synced: true, Lines: timed}, nil
	}
	if untimed == nil {
		untimed = []Line{}
	}
	return Lyrics{Lines: untimed}, nil
}

// unsung are the roles of spans that hold what isn't sung, as the
// translation or the romanization of a line.
var unsung = map[string]bool{"x-translation": true, "x-roman": true, "x-transliteration": true}

// write adds s to the line's text, or to its background vocal when in one.
func write(text, bg *strings.Builder, bgAt int, s string) {
	if bgAt > 0 {
		bg.WriteString(s)
	} else {
		text.WriteString(s)
	}
}

// attr returns the attribute of e named local, in any namespace.
func attr(e xml.StartElement, local string) string {
	for _, a := range e.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

// collapse makes each run of white space in s one space, as XML reads
// text, and trims it.
func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// clockTime reads a time of TTML in milliseconds: a clock time, as
// "1:02.5" or "00:01:02.500", or an offset with its unit, as "62.5s",
// "62500ms", "1.5m" or "1h". A number without a unit is in seconds.
func clockTime(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if strings.Contains(s, ":") {
		parts := strings.Split(s, ":")
		if len(parts) > 3 {
			return 0, false // frames, which need the frame rate
		}
		secs := 0.0
		for i, p := range parts {
			n, ok := number(p)
			// Only the seconds have a fraction.
			if !ok || i < len(parts)-1 && strings.Contains(p, ".") {
				return 0, false
			}
			secs = secs*60 + n
		}
		return millis(secs * 1000)
	}
	unit := 1000.0
	for _, u := range []struct {
		suffix string
		ms     float64
	}{{"ms", 1}, {"s", 1000}, {"m", 60_000}, {"h", 3_600_000}} {
		if rest, ok := strings.CutSuffix(s, u.suffix); ok {
			s, unit = rest, u.ms
			break
		}
	}
	n, ok := number(s)
	if !ok {
		return 0, false
	}
	return millis(n * unit)
}

// number reads a number of a time: decimal digits, with a fraction.
func number(s string) (float64, bool) {
	if s == "" || strings.Trim(s, "0123456789.") != "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(s, 64)
	return n, err == nil
}

// millis rounds ms to an int, refusing times past a day, which no song
// lasts.
func millis(ms float64) (int, bool) {
	if ms > 24*3_600_000 {
		return 0, false
	}
	return int(math.Round(ms)), true
}
