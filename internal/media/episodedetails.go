package media

import (
	"html"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// EpisodeDetails are what an episode's page tells besides its row: what
// the player shows of an episode where it shows a song's lyrics.
type EpisodeDetails struct {
	// Show is the podcast the episode is of, with its ID when the service
	// tells it.
	Show Ref `json:"show"`
	// Released is the day the episode came out, as Episode.Released.
	Released string `json:"released,omitempty"`
	// Plays is how many times the episode was played, as YouTube Music
	// tells its views: 0 where the service doesn't tell.
	Plays int64 `json:"plays,omitempty"`
	// DurationMs is the episode's length, 0 when not told.
	DurationMs int `json:"durationMs,omitempty"`
	// Description is the episode's description, line by line: each line
	// is runs of plain text, links and times of the episode.
	Description [][]TextRun `json:"description"`
	// Chapters are the parts of the episode its description tells, in
	// the order of their times.
	Chapters []Chapter `json:"chapters,omitempty"`
}

// TextRun is a piece of a line of a description: plain text, a link to a
// web page, or a time of the episode.
type TextRun struct {
	Text string `json:"text"`
	// Link is the web page the run opens: an http or https address, or ""
	// for a run that isn't a link.
	Link string `json:"link,omitempty"`
	// Seek tells that the run is a time of the episode, AtMs in, which
	// plays the episode from there.
	Seek bool `json:"seek,omitempty"`
	// AtMs is the time of the episode a run that seeks tells.
	AtMs int `json:"atMs,omitempty"`
}

// Chapter is a part of an episode, as a line of its description tells it:
// the time it starts at, and its title.
type Chapter struct {
	AtMs  int    `json:"atMs"`
	Title string `json:"title"`
	// Line is the line of the description that tells it.
	Line int `json:"line"`
}

// WebLink returns raw when it is the address of a web page, http or https
// with a host, and "" otherwise: a description's links open nothing else.
func WebLink(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.String()
}

// Lines splits runs into lines at their newlines; a run across a newline
// is a run on each line, which keeps its link or time. Blank lines at the
// start and the end go, and blank lines in a row are one.
func Lines(runs []TextRun) [][]TextRun {
	lines := [][]TextRun{nil}
	for _, r := range runs {
		for i, part := range strings.Split(strings.ReplaceAll(r.Text, "\r", ""), "\n") {
			if i > 0 {
				lines = append(lines, nil)
			}
			if part != "" {
				p := r
				p.Text = part
				lines[len(lines)-1] = append(lines[len(lines)-1], p)
			}
		}
	}
	out := make([][]TextRun, 0, len(lines))
	for _, line := range lines {
		if blank(line) && (len(out) == 0 || blank(out[len(out)-1])) {
			continue
		}
		out = append(out, line)
	}
	if len(out) > 0 && blank(out[len(out)-1]) {
		out = out[:len(out)-1]
	}
	return out
}

// blank reports whether a line holds nothing but spaces.
func blank(line []TextRun) bool {
	for _, r := range line {
		if strings.TrimSpace(r.Text) != "" {
			return false
		}
	}
	return true
}

var (
	// webRe finds the web addresses of plain text; what ends a sentence
	// after one is trimmed by trimLink.
	webRe = regexp.MustCompile("https?://[^\\s<>\"'`]+")
	// timeRe finds the times of plain text, as 1:02:03, 12:34 or 0:05.
	timeRe = regexp.MustCompile(`\b(?:(\d{1,2}):([0-5]\d):([0-5]\d)|(\d{1,3}):([0-5]\d))\b`)
	// tagRe finds HTML's tags, and breakRe those that end a line.
	tagRe   = regexp.MustCompile(`<\s*/?\s*[a-zA-Z][^>]*>`)
	breakRe = regexp.MustCompile(`(?i)<\s*(?:br\s*/?|/p|/li|/div|/h[1-6])\s*>`)
)

// ParseDescription reads a description that is plain text, as Spotify
// gives it, line by line: its web addresses become links, and its times,
// as 12:34 or 1:02:03, times of the episode.
func ParseDescription(text string) [][]TextRun {
	var runs []TextRun
	plain := func(s string) {
		for s != "" {
			at := nextTime(s)
			if at == nil {
				runs = append(runs, TextRun{Text: s})
				return
			}
			if at[0] > 0 {
				runs = append(runs, TextRun{Text: s[:at[0]]})
			}
			runs = append(runs, TextRun{Text: s[at[0]:at[1]], Seek: true, AtMs: timeMs(timeRe.FindStringSubmatch(s[at[0]:at[1]]))})
			s = s[at[1]:]
		}
	}
	for text != "" {
		at := webRe.FindStringIndex(text)
		if at == nil {
			plain(text)
			break
		}
		link := trimLink(text[at[0]:at[1]])
		plain(text[:at[0]])
		if u := WebLink(link); u != "" {
			runs = append(runs, TextRun{Text: link, Link: u})
		} else {
			runs = append(runs, TextRun{Text: link})
		}
		text = text[at[0]+len(link):]
	}
	return Lines(runs)
}

// nextTime returns where the first time of s is, or nil: one that isn't a
// part of a longer run of numbers, as 2026:10:30 or 1.12:30.
func nextTime(s string) []int {
	from := 0
	for {
		at := timeRe.FindStringIndex(s[from:])
		if at == nil {
			return nil
		}
		at[0], at[1] = at[0]+from, at[1]+from
		before := at[0] > 0 && strings.ContainsRune(":.,", rune(s[at[0]-1]))
		after := at[1] < len(s) && s[at[1]] == ':' && at[1]+1 < len(s) && s[at[1]+1] >= '0' && s[at[1]+1] <= '9'
		if !before && !after {
			return at
		}
		from = at[1]
	}
}

// timeMs reads the time of timeRe's submatches m into milliseconds.
func timeMs(m []string) int {
	n := func(s string) int { v, _ := strconv.Atoi(s); return v }
	if m[1] != "" {
		return ((n(m[1])*60+n(m[2]))*60 + n(m[3])) * 1000
	}
	return (n(m[4])*60 + n(m[5])) * 1000
}

// trimLink trims what ends a sentence from the end of a web address found
// in text, and a bracket that closes around it.
func trimLink(u string) string {
	for u != "" {
		last := u[len(u)-1]
		switch {
		case strings.IndexByte(".,;:!?*", last) >= 0:
		case last == ')' && strings.Count(u, "(") < strings.Count(u, ")"):
		case last == ']' && strings.Count(u, "[") < strings.Count(u, "]"):
		default:
			return u
		}
		u = u[:len(u)-1]
	}
	return u
}

// PlainText returns the text of HTML, as Spotify's html_description: its
// paragraphs and breaks are lines, its other tags go, and its entities are
// the characters they stand for.
func PlainText(s string) string {
	s = breakRe.ReplaceAllString(s, "\n")
	s = tagRe.ReplaceAllString(s, "")
	return strings.TrimSpace(html.UnescapeString(s))
}

// ChaptersOf returns the chapters a description tells: its lines that
// start with a time, as "12:34 - Their title", or end with one, as "Their
// title (12:34)", in the order of their times. One such line alone tells
// none, nor does a line that is only a time.
func ChaptersOf(lines [][]TextRun) []Chapter {
	var out []Chapter
	for i, line := range lines {
		if c, ok := chapterOf(line); ok {
			c.Line = i
			out = append(out, c)
		}
	}
	if len(out) < 2 {
		return nil
	}
	slices.SortStableFunc(out, func(a, b Chapter) int { return a.AtMs - b.AtMs })
	return out
}

// chapterOf reads a line that starts or ends with a time as a chapter.
func chapterOf(line []TextRun) (Chapter, bool) {
	first, last := -1, -1
	for i, r := range line {
		if strings.TrimFunc(r.Text, isMark) != "" {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	var title strings.Builder
	switch {
	case first < 0:
		return Chapter{}, false
	case line[first].Seek:
		for _, r := range line[first+1:] {
			title.WriteString(r.Text)
		}
		c := Chapter{AtMs: line[first].AtMs, Title: strings.TrimSpace(strings.TrimLeftFunc(title.String(), isMark))}
		return c, c.Title != ""
	case line[last].Seek:
		for _, r := range line[:last] {
			title.WriteString(r.Text)
		}
		t := strings.TrimRightFunc(title.String(), isMark)
		c := Chapter{AtMs: line[last].AtMs, Title: strings.TrimSpace(strings.TrimLeftFunc(t, isBullet))}
		return c, c.Title != ""
	}
	return Chapter{}, false
}

// isMark reports whether r is a space or a mark between a chapter's time
// and its title, as " - ", " | " or brackets.
func isMark(r rune) bool { return isBullet(r) || strings.ContainsRune(":|()[]", r) }

// isBullet reports whether r is a space or a mark a line of a list starts
// with.
func isBullet(r rune) bool { return unicode.IsSpace(r) || strings.ContainsRune("-–—•·*>", r) }
