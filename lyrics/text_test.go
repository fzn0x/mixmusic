package lyrics

import (
	"reflect"
	"testing"
)

// The lyrics here are made up, as are the songs they're of.

func checkText(t *testing.T, got Lyrics, ok bool, want Lyrics) {
	t.Helper()
	if !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("lyrics %+v, %v\nwant %+v", got, ok, want)
	}
}

// TestFromTextLRC reads LRC kept as text: its metadata left out, a line of
// several times at each, the times moved by the offset, and line breaks
// of Windows.
func TestFromTextLRC(t *testing.T) {
	lrc := "\xef\xbb\xbf[ar:Ada Field]\r\n[ti:Glass Rivers]\r\n[offset:+250]\r\n" +
		"[00:01.00][00:21.00]Glass rivers\r\n[00:04.5]Run clear\r\n\r\n[00:11.000]\r\n[00:12.00]Down to the sea\r\n"
	got, ok := FromText(lrc, "Glass Rivers.lrc")
	checkText(t, got, ok, Lyrics{Synced: true, Source: "Glass Rivers.lrc", Lines: []Line{
		{TimeMs: 750, Text: "Glass rivers"},
		{TimeMs: 4250, Text: "Run clear"},
		{TimeMs: 10750, Text: ""},
		{TimeMs: 11750, Text: "Down to the sea"},
		{TimeMs: 20750, Text: "Glass rivers"},
	}})
}

// TestFromTextEnhancedLRC reads the words of enhanced LRC: each from its
// stamp to the next, the last of a line of no stamp after it lighting as
// it starts, the voice before them dropped, and a line sung twice timed
// twice. The offset moves the words too.
func TestFromTextEnhancedLRC(t *testing.T) {
	lrc := "[offset:-500]\n" +
		"[00:01.00]v1:<00:01.00>Paper <00:01.50>lan<00:01.80>terns<00:02.40>\n" +
		"[00:03.00]v2: <00:03.00>Over <00:03.40>the bay\n" +
		"[00:05.00][00:15.00]<00:05.00>Again <00:05.50>and again\n"
	got, ok := FromText(lrc, "tags")
	checkText(t, got, ok, Lyrics{Synced: true, Source: "tags", Lines: []Line{
		{TimeMs: 1500, Text: "Paper lanterns", Words: []Word{{"Paper", 1500, 2000}, {"lan", 2000, 2300}, {"terns", 2300, 2900}}},
		{TimeMs: 3500, Text: "Over the bay", Words: []Word{{"Over", 3500, 3900}, {"the bay", 3900, 3900}}},
		{TimeMs: 5500, Text: "Again and again", Words: []Word{{"Again", 5500, 6000}, {"and again", 6000, 6000}}},
		{TimeMs: 15500, Text: "Again and again", Words: []Word{{"Again", 15500, 16000}, {"and again", 16000, 16000}}},
	}})
	if !got.WordTimed() {
		t.Error("enhanced LRC isn't timed word by word")
	}
}

// TestFromTextEnhancedLRCWordless times a line whole whose text starts
// before its first stamp, as a word of no time would light out of turn.
func TestFromTextEnhancedLRCWordless(t *testing.T) {
	got, ok := FromText("[00:01.00]Hello <00:01.50>world\n[00:02.00]Next", "")
	checkText(t, got, ok, Lyrics{Synced: true, Lines: []Line{{TimeMs: 1000, Text: "Hello world"}, {TimeMs: 2000, Text: "Next"}}})
}

// TestFromTextOneTime reads LRC of one time as plain, by the rule of
// #277, without its time.
func TestFromTextOneTime(t *testing.T) {
	got, ok := FromText("[00:00.00]probe", "x")
	checkText(t, got, ok, Lyrics{Source: "x", Lines: []Line{{Text: "probe"}}})
}

// TestFromTextTTML reads TTML kept as text.
func TestFromTextTTML(t *testing.T) {
	ttml := `<tt><body><div><p begin="0:01.000">From the file</p><p begin="0:04.000">Second line</p></div></body></tt>`
	got, ok := FromText(ttml, "Song.m4a")
	checkText(t, got, ok, Lyrics{Synced: true, Source: "Song.m4a", Lines: []Line{{TimeMs: 1000, Text: "From the file"}, {TimeMs: 4000, Text: "Second line"}}})
}

// TestFromTextPlain reads plain text a line each, the empty line between
// verses kept, and text that only looks like TTML or LRC as plain.
func TestFromTextPlain(t *testing.T) {
	got, ok := FromText("  Glass rivers\nRun clear\n\n[Chorus]\nDown to the sea  \n", "Song.flac")
	checkText(t, got, ok, Lyrics{Source: "Song.flac", Lines: []Line{
		{Text: "Glass rivers"}, {Text: "Run clear"}, {Text: ""}, {Text: "[Chorus]"}, {Text: "Down to the sea"},
	}})
	got, ok = FromText("<3 you\nmore than words", "")
	checkText(t, got, ok, Lyrics{Lines: []Line{{Text: "<3 you"}, {Text: "more than words"}}})
	// LRC of no times, its metadata left out.
	got, ok = FromText("[ar:Ada Field]\r\n[ti:Glass Rivers]\r\n\r\nGlass rivers\r\n[Verse 2: Ada]\r\nRun clear", "")
	checkText(t, got, ok, Lyrics{Lines: []Line{{Text: "Glass rivers"}, {Text: "[Verse 2: Ada]"}, {Text: "Run clear"}}})
}

// TestFromTextNone has no lyrics of text of no words, as LRC of only
// its metadata.
func TestFromTextNone(t *testing.T) {
	for _, text := range []string{"", " \n\t", "\xef\xbb\xbf", "[00:01.00]\n[00:02.00]", "[ar:Ada Field]\n[ti:Glass Rivers]\n\n[length: 03:20]", "<tt><body></body></tt>"} {
		if got, ok := FromText(text, "x"); ok {
			t.Errorf("%q: %+v", text, got)
		}
	}
}

// TestParseLRCKeepsStamps keeps ParseLRC as LRCLIB's lyrics read it: the
// stamps of enhanced LRC stay in the text.
func TestParseLRCKeepsStamps(t *testing.T) {
	got := ParseLRC("[00:01.00]v1:<00:01.00>a <00:02.00>b")
	if want := []Line{{TimeMs: 1000, Text: "v1:<00:01.00>a <00:02.00>b"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("lines %+v, want %+v", got, want)
	}
}

// TestDecodeText decodes a .lrc file of UTF-8, with a byte order mark or
// without, of UTF-16 by its byte order mark, either way round, and of
// ISO-8859-1.
func TestDecodeText(t *testing.T) {
	for _, c := range []struct {
		name string
		b    string
		want string
	}{
		{"UTF-8", "[00:01.00]Caf\xc3\xa9", "[00:01.00]Café"},
		{"UTF-8 with a byte order mark", "\xef\xbb\xbf[00:01.00]Caf\xc3\xa9", "[00:01.00]Café"},
		{"UTF-16, little-endian", "\xff\xfe[\x000\x00]\x00C\x00\xe9\x00\x3d\xd8\xb5\xdc", "[0]Cé\U0001F4B5"},
		{"UTF-16, big-endian, of an odd byte", "\xfe\xff\x00[\x000\x00]\x00C\x00\xe9\x00", "[0]Cé"},
		{"ISO-8859-1", "[00:01.00]Caf\xe9 cr\xe8me", "[00:01.00]Café crème"},
	} {
		if got := DecodeText([]byte(c.b)); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
