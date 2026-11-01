package lyrics

import "testing"

// richsync is made-up lyrics synced word by word, in the shape Unison's
// TTML has: the words in timed spans, the space between them outside,
// and a background vocal in a span of its own.
const richsync = `<?xml version="1.0" encoding="UTF-8"?>
<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata" xmlns:ttp="http://www.w3.org/ns/ttml#parameter" ttp:timeBase="media" xml:lang="en">
  <head>
    <metadata>
      <ttm:title>A made-up song</ttm:title>
      <ttm:agent xml:id="v1" type="person"><ttm:name>Lead</ttm:name></ttm:agent>
    </metadata>
  </head>
  <body dur="1:05.000">
    <div>
      <p begin="0:01.250" end="0:03.000" ttm:agent="v1"><span begin="0:01.250" end="0:01.700">Paper</span> <span begin="0:01.700" end="0:03.000">kites</span></p>
      <p begin="0:03.500" end="0:06.000" ttm:agent="v1"><span begin="0:03.500" end="0:04.000">over</span> <span begin="0:04.000" end="0:04.400">the</span> <span begin="0:04.400" end="0:05.000">bay</span><span ttm:role="x-bg"><span begin="0:05.000" end="0:06.000">(the bay)</span></span></p>
      <p begin="1:02.000" end="1:04.000" ttm:agent="v1"><span begin="1:02.000" end="1:02.500">Tom</span> <span begin="1:02.500" end="1:03.000">&amp;</span> <span begin="1:03.000" end="1:04.000">Jo</span><span ttm:role="x-bg"><span begin="1:03.000" end="1:04.000">hey</span></span></p>
    </div>
  </body>
</tt>`

func TestParseTTMLWordSynced(t *testing.T) {
	l, err := ParseTTML(richsync)
	if err != nil {
		t.Fatal(err)
	}
	want := []Line{{1250, "Paper kites"}, {3500, "over the bay (the bay)"}, {62000, "Tom & Jo (hey)"}}
	if !l.Synced || len(l.Lines) != len(want) {
		t.Fatalf("lyrics = %+v", l)
	}
	for i := range want {
		if l.Lines[i] != want[i] {
			t.Errorf("line %d = %+v, want %+v", i, l.Lines[i], want[i])
		}
	}
}

// TestParseTTMLLineSynced reads lyrics synced line by line, whose
// paragraphs hold their text, Japanese without spaces among them, in
// times of several shapes.
func TestParseTTMLLineSynced(t *testing.T) {
	ttml := `<tt xmlns="http://www.w3.org/ns/ttml"><body><div>
		<p begin="00:00:10.5">紙の<span>飛行機</span></p>
		<p begin="12s">Second
			line</p>
		<p begin="13500ms">Third<br/>line</p>
		<p begin="0.25m">Fourth</p>
		<p begin="9.000">Earlier</p>
	</div></body></tt>`
	l, err := ParseTTML(ttml)
	if err != nil {
		t.Fatal(err)
	}
	want := []Line{{9000, "Earlier"}, {10500, "紙の飛行機"}, {12000, "Second line"}, {13500, "Third line"}, {15000, "Fourth"}}
	if !l.Synced || len(l.Lines) != len(want) {
		t.Fatalf("lyrics = %+v", l)
	}
	for i := range want {
		if l.Lines[i] != want[i] {
			t.Errorf("line %d = %+v, want %+v", i, l.Lines[i], want[i])
		}
	}
}

// TestParseTTMLLeavesOut leaves out what isn't sung, and lines without a
// time among timed ones; a paragraph without a time of its own is sung at
// its first word's.
func TestParseTTMLLeavesOut(t *testing.T) {
	ttml := `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata"><body><div>
		<p begin="0:01.000">Sung<span ttm:role="x-translation">Gesungen</span><span ttm:role="x-roman">sangu</span></p>
		<p>No time</p>
		<p begin="later">Bad time</p>
		<p begin="0:00:01:05">Frames</p>
		<p><span begin="0:02.000">First</span> <span begin="0:02.500">word</span></p>
	</div></body></tt>`
	l, err := ParseTTML(ttml)
	if err != nil {
		t.Fatal(err)
	}
	want := []Line{{1000, "Sung"}, {2000, "First word"}}
	if !l.Synced || len(l.Lines) != len(want) || l.Lines[0] != want[0] || l.Lines[1] != want[1] {
		t.Errorf("lyrics = %+v", l)
	}
}

// Lyrics without times are plain.
func TestParseTTMLPlain(t *testing.T) {
	l, err := ParseTTML(`<tt><body><div><p>One</p><p></p><p>Two &nbsp;words</p></div></body></tt>`)
	if err != nil {
		t.Fatal(err)
	}
	if l.Synced || len(l.Lines) != 3 || l.Lines[0].Text != "One" || l.Lines[1].Text != "" || l.Lines[2].Text != "Two words" {
		t.Errorf("lyrics = %+v", l)
	}
	if l, err := ParseTTML(`<tt><body></body></tt>`); err != nil || l.Lines == nil || len(l.Lines) != 0 {
		t.Errorf("no lines = %+v, %v", l, err)
	}
}

// TestParseTTMLTyped reads lyrics typed by hand, with a bare ampersand,
// and refuses what isn't XML at all.
func TestParseTTMLTyped(t *testing.T) {
	l, err := ParseTTML(`<tt><body><div><p begin="1s">Salt & pepper</p></div></body></tt>`)
	if err != nil || len(l.Lines) != 1 || l.Lines[0].Text != "Salt & pepper" {
		t.Errorf("a bare ampersand: %+v, %v", l, err)
	}
	if _, err := ParseTTML(`<tt><body><p begin="1s">Cut off`); err == nil {
		t.Error("TTML cut off reads")
	}
}

func TestClockTime(t *testing.T) {
	for in, want := range map[string]int{
		"0:01.5": 1500, "1:02.345": 62345, "01:00:00": 3_600_000, "2.5": 2500,
		"2.5s": 2500, "250ms": 250, "1.5m": 90_000, "1h": 3_600_000, " 3s ": 3000,
	} {
		if got, ok := clockTime(in); !ok || got != want {
			t.Errorf("clockTime(%q) = %d, %v, want %d", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "NaN", "Inf", "-1s", "1e3", "0x10", "1:2:3:4", "1.5:00", "10f", "99999h", "s"} {
		if got, ok := clockTime(in); ok {
			t.Errorf("clockTime(%q) = %d, want none", in, got)
		}
	}
}
