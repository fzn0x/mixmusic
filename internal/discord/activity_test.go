package discord

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestWireButtons gives Discord two buttons at most, of labels it takes,
// each opening a web page: a button that would open anything else, or
// with no label, is left out.
func TestWireButtons(t *testing.T) {
	cases := []struct {
		name string
		in   []Button
		want string
	}{
		{"none", nil, `{"type":2}`},
		{"one", []Button{{"Listen on Spotify", "https://open.spotify.com/track/1"}},
			`{"type":2,"buttons":[{"label":"Listen on Spotify","url":"https://open.spotify.com/track/1"}]}`},
		{"the first two", []Button{{"A", "https://a.example/"}, {"B", "http://b.example/"}, {"C", "https://c.example/"}},
			`{"type":2,"buttons":[{"label":"A","url":"https://a.example/"},{"label":"B","url":"http://b.example/"}]}`},
		{"long label", []Button{{strings.Repeat("l", 40), "https://a.example/"}},
			`{"type":2,"buttons":[{"label":"` + strings.Repeat("l", 31) + `…","url":"https://a.example/"}]}`},
		{"not the web", []Button{{"Open", "file:///C:/song.mp3"}, {"Listen", " https://a.example/ "}},
			`{"type":2,"buttons":[{"label":"Listen","url":"https://a.example/"}]}`},
		{"no label", []Button{{"  ", "https://a.example/"}}, `{"type":2}`},
		{"long URL", []Button{{"Listen", "https://a.example/" + strings.Repeat("a", 500)}}, `{"type":2}`},
	}
	for _, c := range cases {
		b, err := json.Marshal(Activity{Type: Listening, Buttons: c.in}.wire())
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != c.want {
			t.Errorf("%s: %s\nwant %s", c.name, b, c.want)
		}
	}
}

func TestFit(t *testing.T) {
	a127 := strings.Repeat("a", 127)
	cases := []struct{ in, want string }{
		{"", ""},
		{"   ", ""},
		{"A", "A "},
		{" A ", "A "},
		{"Ab", "Ab"},
		{"界", "界 "},
		{"🎵", "🎵"}, // two UTF-16 code units, as Discord counts
		{strings.Repeat("a", 128), strings.Repeat("a", 128)},
		{strings.Repeat("a", 129), a127 + "…"},
		{strings.Repeat("界", 200), strings.Repeat("界", 127) + "…"},
		{strings.Repeat("🎵", 64), strings.Repeat("🎵", 64)},
		{strings.Repeat("🎵", 65), strings.Repeat("🎵", 63) + "…"},
		{strings.Repeat("a", 126) + " bcd", strings.Repeat("a", 126) + "…"},
	}
	for _, c := range cases {
		got := fit(c.in)
		if got != c.want {
			t.Errorf("fit(%q) = %q, want %q", c.in, got, c.want)
		}
		if n := utf16Len(got); got != "" && (n < minText || n > maxText) {
			t.Errorf("fit(%q) is %d long", c.in, n)
		}
	}
}

func TestWire(t *testing.T) {
	end := time.UnixMilli(1_700_000_180_000)
	cases := []struct {
		name string
		in   Activity
		want string
	}{
		{"empty", Activity{}, `{"type":0}`},
		{"short and long", Activity{Type: Listening, Details: "X", State: strings.Repeat("b", 200)},
			`{"type":2,"details":"X ","state":"` + strings.Repeat("b", 127) + `…"}`},
		{"end only", Activity{Type: Listening, Details: "Song", End: end},
			`{"type":2,"details":"Song","timestamps":{"end":1700000180000}}`},
		{"image only", Activity{Type: Listening, LargeImage: "https://example.com/a.jpg"},
			`{"type":2,"assets":{"large_image":"https://example.com/a.jpg"}}`},
		{"image too long", Activity{Type: Listening, LargeImage: "https://example.com/" + strings.Repeat("a", 300)},
			`{"type":2}`},
		{"album fitted", Activity{Type: Listening, LargeText: "Z"},
			`{"type":2,"assets":{"large_text":"Z "}}`},
		{"artists in the member list", Activity{Type: Listening, State: "Tame Impala", StatusDisplay: ShowState},
			`{"type":2,"status_display_type":1,"state":"Tame Impala"}`},
		{"nothing to show in the member list", Activity{Type: Listening, Details: "Song", StatusDisplay: ShowState},
			`{"type":2,"details":"Song"}`},
		{"badge", Activity{Type: Listening, LargeImage: "https://example.com/a.jpg", SmallImage: " https://example.com/b.png ", SmallText: "Spotify"},
			`{"type":2,"assets":{"large_image":"https://example.com/a.jpg","small_image":"https://example.com/b.png","small_text":"Spotify"}}`},
		{"badge too long", Activity{Type: Listening, SmallImage: "https://example.com/" + strings.Repeat("a", 300), SmallText: "P"},
			`{"type":2,"assets":{"small_text":"P "}}`},
	}
	for _, c := range cases {
		b, err := json.Marshal(c.in.wire())
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != c.want {
			t.Errorf("%s: %s\nwant %s", c.name, b, c.want)
		}
	}
}
