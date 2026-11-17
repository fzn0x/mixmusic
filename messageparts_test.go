package main

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestMessageParts splits a message said with marked parts back into its
// own words and its parts, in the order the message puts them.
func TestMessageParts(t *testing.T) {
	for _, tc := range []struct {
		msg  string
		want string
	}{
		{"You played " + mark(0, "3 tracks") + " for " + mark(1, "12 min") + ".", `[{You played  -1} {3 tracks 0} { for  -1} {12 min 1} {. -1}]`},
		// A translation may put the parts first, last or the other way
		// round.
		{mark(1, "12 min") + "、" + mark(0, "3 tracks"), `[{12 min 1} {、 -1} {3 tracks 0}]`},
		{"plain", `[{plain -1}]`},
		// A value that holds markers can't start a part of its own.
		{"x " + mark(0, "a"+string(rune(markStart+1))+"b"+string(rune(markEnd))), `[{x  -1} {ab 0}]`},
	} {
		if got := fmt.Sprint(messageParts(tc.msg)); got != tc.want {
			t.Errorf("messageParts(%q) = %s, want %s", tc.msg, got, tc.want)
		}
	}
}

// TestSpansOf gives the message's own words the base span and each part
// its own, with the same size and weight.
func TestSpansOf(t *testing.T) {
	base, bold := ui.Span{Size: 20, Weight: 300}, ui.Span{Size: 20, Weight: 400}
	got := spansOf("In 2025 you played "+mark(0, "3 tracks")+" for "+mark(1, "12 minutes")+".", base, bold, bold)
	want := []ui.Span{{Text: "In 2025 you played ", Size: 20, Weight: 300}, {Text: "3 tracks", Size: 20, Weight: 400},
		{Text: " for ", Size: 20, Weight: 300}, {Text: "12 minutes", Size: 20, Weight: 400}, {Text: ".", Size: 20, Weight: 300}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("spans %v, want %v", got, want)
	}
}
