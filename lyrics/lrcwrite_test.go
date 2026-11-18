package lyrics

import "testing"

// TestLRCWrite writes synced lyrics as LRC, a line of words timed with
// the stamps of enhanced LRC, a line of no words with its time alone, and
// reads them back as they were.
func TestLRCWrite(t *testing.T) {
	l := Lyrics{Synced: true, Source: "Unison", Lines: []Line{
		{TimeMs: 1500, Text: "Paper lanterns", Words: []Word{{"Paper", 1500, 2000}, {"lanterns", 2000, 2000}}},
		{TimeMs: 3500, Text: ""},
		{TimeMs: 61230, Text: "Over the bay"},
	}}
	text := l.LRC()
	want := "[00:01.50]<00:01.50>Paper <00:02.00>lanterns\n[00:03.50]\n[01:01.23]Over the bay\n"
	if text != want {
		t.Fatalf("LRC %q, want %q", text, want)
	}
	got, ok := FromText(text, "Unison")
	checkText(t, got, ok, l)
}

// TestLRCWritePlain writes plain lyrics a line each, the gaps between
// verses kept, and reads them back as they were.
func TestLRCWritePlain(t *testing.T) {
	l := Plain("Paper kites\n\nOver the river", "LRCLIB")
	if text := l.LRC(); text != "Paper kites\n\nOver the river\n" {
		t.Fatalf("LRC %q", text)
	}
	got, ok := FromText(l.LRC(), "LRCLIB")
	checkText(t, got, ok, l)
}

// TestLRCWriteWordsOutOfText times a line whole whose words its text
// doesn't hold in their order: stamps would move its words.
func TestLRCWriteWordsOutOfText(t *testing.T) {
	l := Lyrics{Synced: true, Lines: []Line{
		{TimeMs: 1000, Text: "Hello world", Words: []Word{{"world", 1000, 1500}, {"Hello", 1500, 2000}}},
		{TimeMs: 2000, Text: "Next"},
	}}
	if text := l.LRC(); text != "[00:01.00]Hello world\n[00:02.00]Next\n" {
		t.Errorf("LRC %q", text)
	}
}
