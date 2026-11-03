package lyrics

import (
	"slices"
	"testing"
)

// TestParseTTMLWords keeps the times of the words of lyrics synced word by
// word, in Unison's shape: background vocals' too, with the parentheses
// they are put in.
func TestParseTTMLWords(t *testing.T) {
	l, err := ParseTTML(richsync)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]Word{
		{{"Paper", 1250, 1700}, {"kites", 1700, 3000}},
		{{"over", 3500, 4000}, {"the", 4000, 4400}, {"bay", 4400, 5000}, {"(the bay)", 5000, 6000}},
		{{"Tom", 62000, 62500}, {"&", 62500, 63000}, {"Jo", 63000, 64000}, {"(hey)", 63000, 64000}},
	}
	if len(l.Lines) != len(want) {
		t.Fatalf("lyrics = %+v", l)
	}
	for i, line := range l.Lines {
		if !slices.Equal(line.Words, want[i]) {
			t.Errorf("words of %q = %+v, want %+v", line.Text, line.Words, want[i])
		}
	}
}

// TestParseTTMLWordTimes reads the words of made-up lyrics in other shapes:
// syllables, a word timed by its duration, words that end as the next
// starts or as the line does, punctuation between words, and background
// vocals timed as a whole besides their words.
func TestParseTTMLWordTimes(t *testing.T) {
	ttml := `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata"><body><div>
		<p begin="0:10.000" end="0:12.000"><span begin="0:10.000">Pa</span><span begin="0:10.400">per</span> <span begin="0:11.000" dur="0.5s">boats,</span>
			<span begin="0:11.500">down</span></p>
		<p begin="0:20.000"><span begin="0:20.000" end="0:20.500">Row</span>, <span begin="0:20.500" end="0:21.000">row</span>!<span ttm:role="x-translation">Rudern</span></p>
		<p begin="0:40.000"><span begin="0:40.000" end="0:41.000">Rain</span><span ttm:role="x-bg" begin="0:40.500" end="0:41.500"><span begin="0:40.500" end="0:41.000">(soft</span> <span begin="0:41.000" end="0:41.500">rain)</span></span></p>
		<p begin="0:50.000"><span begin="0:50.000" end="0:50.900">Late</span><span begin="0:50.800" end="0:50.700">ly</span></p>
	</div></body></tt>`
	l, err := ParseTTML(ttml)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		text  string
		words []Word
	}{
		{"Paper boats, down", []Word{{"Pa", 10000, 10400}, {"per", 10400, 11000}, {"boats,", 11000, 11500}, {"down", 11500, 12000}}},
		{"Row, row!", []Word{{"Row", 20000, 20500}, {"row", 20500, 21000}}},
		{"Rain (soft rain)", []Word{{"Rain", 40000, 41000}, {"(soft", 40500, 41000}, {"rain)", 41000, 41500}}},
		// A word can't end before it starts.
		{"Lately", []Word{{"Late", 50000, 50900}, {"ly", 50800, 50800}}},
	}
	if len(l.Lines) != len(want) {
		t.Fatalf("lyrics = %+v", l)
	}
	for i, line := range l.Lines {
		if line.Text != want[i].text || !slices.Equal(line.Words, want[i].words) {
			t.Errorf("line %d = %q %+v, want %q %+v", i, line.Text, line.Words, want[i].text, want[i].words)
		}
	}
}

// TestParseTTMLNoWords leaves the words out of lines that aren't wholly
// timed word by word: those light as a whole.
func TestParseTTMLNoWords(t *testing.T) {
	ttml := `<tt xmlns="http://www.w3.org/ns/ttml"><body><div>
		<p begin="0:01.000"><span begin="0:01.000" end="0:01.500">Half</span> timed</p>
		<p begin="0:02.000">Plain <span>text</span></p>
		<p begin="0:03.000"><span begin="0:03.000" end="0:04.000">Out<span begin="0:03.500" end="0:04.000">in</span>side</span></p>
	</div></body></tt>`
	l, err := ParseTTML(ttml)
	if err != nil {
		t.Fatal(err)
	}
	if got := saidOf(l.Lines); !slices.Equal(got, []said{{1000, "Half timed"}, {2000, "Plain text"}, {3000, "Outinside"}}) {
		t.Fatalf("lines = %+v", got)
	}
	for _, line := range l.Lines {
		if line.Words != nil {
			t.Errorf("words of %q = %+v", line.Text, line.Words)
		}
	}
}
