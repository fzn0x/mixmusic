package main

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// A sentence whose parts look apart, as a headline's bold counts or a link
// inside a note, is one message all the same, so that a translation orders
// its words as its language does (#775). Its parts are placeholders, each
// given wrapped in the markers of its index (mark); the message said is
// then split at the markers (messageParts) into its plain text and its
// parts, each drawn as it looks.

// The markers are code points of the Supplementary Private Use Area-A,
// which no message, name or title has: a part starts with markStart plus
// its index, and ends with markEnd.
const (
	markStart = 0xF0000
	markEnd   = 0xF00FF
)

// marker tells whether r is one of the markers.
func marker(r rune) bool { return r >= markStart && r <= markEnd }

// mark wraps v, the value of the placeholder of a part, in the markers of
// the part's index i. Markers v has itself go, so that a title can't start
// a part.
func mark(i int, v string) string {
	if strings.IndexFunc(v, marker) >= 0 {
		v = strings.Map(func(r rune) rune {
			if marker(r) {
				return -1
			}
			return r
		}, v)
	}
	return string(rune(markStart+i)) + v + string(rune(markEnd))
}

// messagePart is a piece of a message said with marked parts: its text,
// and the index of its part, or -1 for the message's own words.
type messagePart struct {
	text string
	part int
}

// messageParts splits msg, said with marked parts, into its pieces in
// order, the empty ones left out. A part's markers nest in no other's;
// a part a translation left open runs to the end.
func messageParts(msg string) []messagePart {
	var out []messagePart
	part, from := -1, 0
	add := func(to int) {
		if to > from {
			out = append(out, messagePart{msg[from:to], part})
		}
	}
	for i, r := range msg {
		if !marker(r) {
			continue
		}
		add(i)
		from = i + utf8.RuneLen(r)
		if r == markEnd {
			part = -1
		} else {
			part = int(r - markStart)
		}
	}
	add(len(msg))
	return out
}

// spansOf makes the spans of a rich text of msg, said with marked parts:
// its own words in base, and the part of index i in parts[i]. A part with
// no span of its own shows in base.
func spansOf(msg string, base ui.Span, parts ...ui.Span) []ui.Span {
	pieces := messageParts(msg)
	out := make([]ui.Span, len(pieces))
	for i, p := range pieces {
		out[i] = base
		if p.part >= 0 && p.part < len(parts) {
			out[i] = parts[p.part]
		}
		out[i].Text = p.text
	}
	return out
}

// inlineMessage writes msg, said with marked parts, into the children of
// a rich text: its own words as text, and the part of index i by
// parts[i], as a link. A part with no function shows as text.
func inlineMessage(c *ui.Context, msg string, parts ...func(text string)) {
	for _, p := range messageParts(msg) {
		if p.part >= 0 && p.part < len(parts) {
			parts[p.part](p.text)
			continue
		}
		ui.Text(c, p.text)
	}
}
