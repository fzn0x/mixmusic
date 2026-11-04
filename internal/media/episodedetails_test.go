package media

import (
	"reflect"
	"testing"
)

// TestParseDescription reads a plain description, made up, line by line:
// its web addresses are links, without the stop after one, and its times
// seek, but not the numbers that only look like times.
func TestParseDescription(t *testing.T) {
	got := ParseDescription("Tonight: why owls stay up.\r\n\n\n" +
		"Notes at https://example.com/owls?ep=4. Mail (https://example.org/mail)\n" +
		"0:00 Hello\n12:34 - The barn\n1:02:03 | Goodbye\n" +
		"Taped 2026:10:30, at 10:30pm, v1.12:30, ftp://example.net/x and javascript:alert(1)\n")
	want := [][]TextRun{
		{{Text: "Tonight: why owls stay up."}},
		nil,
		{{Text: "Notes at "}, {Text: "https://example.com/owls?ep=4", Link: "https://example.com/owls?ep=4"},
			{Text: ". Mail ("}, {Text: "https://example.org/mail", Link: "https://example.org/mail"}, {Text: ")"}},
		{{Text: "0:00", Seek: true}, {Text: " Hello"}},
		{{Text: "12:34", Seek: true, AtMs: 754000}, {Text: " - The barn"}},
		{{Text: "1:02:03", Seek: true, AtMs: 3723000}, {Text: " | Goodbye"}},
		{{Text: "Taped 2026:10:30, at 10:30pm, v1.12:30, ftp://example.net/x and javascript:alert(1)"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseDescription =\n%+v\nwant\n%+v", got, want)
	}
}

// TestChaptersOf finds the chapters of the lines that start or end with a
// time, in the order of their times, and none of a lone time.
func TestChaptersOf(t *testing.T) {
	lines := ParseDescription("A walk through the night.\n" +
		"(0:00) Dusk\n" +
		"- Midnight snacks – 25:00\n" +
		"12:30 — The owl's (long) song\n" +
		"Recorded at 9:15 in the barn\n" +
		"40:00\n")
	want := []Chapter{
		{AtMs: 0, Title: "Dusk", Line: 1},
		{AtMs: 750000, Title: "The owl's (long) song", Line: 3},
		{AtMs: 1500000, Title: "Midnight snacks", Line: 2},
	}
	if got := ChaptersOf(lines); !reflect.DeepEqual(got, want) {
		t.Errorf("ChaptersOf = %+v, want %+v", got, want)
	}
	if got := ChaptersOf(ParseDescription("Intro 0:00\nThe rest.")); got != nil {
		t.Errorf("one chapter alone = %+v", got)
	}
}

// TestWebLink keeps only the addresses of web pages.
func TestWebLink(t *testing.T) {
	for in, want := range map[string]string{
		"https://example.com/a?b=c": "https://example.com/a?b=c", " HTTP://example.com ": "http://example.com",
		"javascript:alert(1)": "", "file:///etc/passwd": "", "mailto:someone@example.com": "", "/watch?v=x": "", "https://": "",
	} {
		if got := WebLink(in); got != want {
			t.Errorf("WebLink(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestPlainText reads HTML as plain text: lines at its paragraphs and
// breaks, no tags, and its entities as characters.
func TestPlainText(t *testing.T) {
	got := PlainText(`<p>Owls &amp; bats</p><p>Part one<br/>at <a href="https://example.com">our site</a> &lt;3</p>`)
	if want := "Owls & bats\nPart one\nat our site <3"; got != want {
		t.Errorf("PlainText = %q, want %q", got, want)
	}
}
