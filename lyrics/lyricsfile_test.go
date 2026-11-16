package lyrics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

// wordLyricsfile is a made-up Lyricsfile as LRCLIB writes them: the
// words of its lines timed, quoted as YAML quotes them, the last word of a
// line without an end of its own.
const wordLyricsfile = `version: '1.0'
metadata:
  title: Paper Kites
  artist: Nobody
  duration_ms: 176000
  instrumental: false
  language: en
lines:
- text: Paper kites over the bay
  start_ms: 1000
  end_ms: 3000
  words:
  - text: 'Paper '
    start_ms: 1000
    end_ms: 1400
  - text: 'kites '
    start_ms: 1400
  - text: 'over '
    start_ms: 1900
  - text: 'the '
    start_ms: 2300
  - text: bay
    start_ms: 2500
- text: 'I''ll wait for you'
  start_ms: 4000
  end_ms: 6000
  words:
  - text: "I'll "
    start_ms: 4000
  - text: 'wait '
    start_ms: 4500
  - text: 'for '
    start_ms: 5000
  - text: you
    start_ms: 5500
plain: |
  Paper kites over the bay
  I'll wait for you
`

// TestLyricsfileWords reads the words of a Lyricsfile's lines with their
// times: a word without an end lasts until the next starts, or the line
// ends.
func TestLyricsfileWords(t *testing.T) {
	l, ok := wordSynced(wordLyricsfile, "LRCLIB")
	if !ok || !l.Synced || len(l.Lines) != 2 {
		t.Fatalf("read %+v, %v", l, ok)
	}
	first := l.Lines[0]
	want := []Word{{"Paper", 1000, 1400}, {"kites", 1400, 1900}, {"over", 1900, 2300}, {"the", 2300, 2500}, {"bay", 2500, 3000}}
	if first.TimeMs != 1000 || first.Text != "Paper kites over the bay" || !slices.Equal(first.Words, want) {
		t.Errorf("first line %+v", first)
	}
	if second := l.Lines[1]; second.Text != "I'll wait for you" || len(second.Words) != 4 || second.Words[0].Text != "I'll" || second.Words[3].EndMs != 6000 {
		t.Errorf("second line %+v", second)
	}
}

// TestLyricsfileWordsNotInText has a line whose text doesn't hold its
// words read as its words do, which then light in it.
func TestLyricsfileWordsNotInText(t *testing.T) {
	lines, ok := readLyricsfile("lines:\n- text: Paper kites\n  start_ms: 1000\n  end_ms: 2000\n  words:\n  - text: 'Papier '\n    start_ms: 1000\n  - text: kites\n    start_ms: 1500\n")
	if !ok || len(lines) != 1 || lines[0].Text != "Papier kites" || !wordsIn(lines[0].Text, lines[0].Words) {
		t.Errorf("read %+v, %v", lines, ok)
	}
}

// TestLyricsfileUnread tells a document it can't read, and one with no
// word timed, apart from lyrics timed word by word.
func TestLyricsfileUnread(t *testing.T) {
	for name, doc := range map[string]string{
		"flow":            "lines: [{text: a, start_ms: 1}]",
		"open quote":      "lines:\n- text: 'Paper\n  kites'\n  start_ms: 1000\n",
		"no words":        "lines:\n- text: One\n  start_ms: 1000\n- text: Two\n  start_ms: 2000\n",
		"untimed word":    "lines:\n- text: One two\n  start_ms: 1000\n  words:\n  - text: 'One '\n  - text: two\n    start_ms: 1500\n- text: Three\n  start_ms: 3000\n",
		"no lines at all": "version: '1.0'\nmetadata:\n  title: Paper Kites\n",
	} {
		if l, ok := wordSynced(doc, "LRCLIB"); ok {
			t.Errorf("%s: read %+v", name, l)
		}
	}
}

// TestLRCLIBWordSync lights LRCLIB's words one by one when hasWordSync
// says its Lyricsfile has them, and keeps its synced lyrics when it can't
// read them, or LRCLIB says they have none.
func TestLRCLIBWordSync(t *testing.T) {
	answer := func(hasWordSync bool, lyricsfile string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"trackName": "Paper Kites", "artistName": "Nobody", "duration": 176,
				"syncedLyrics": "[00:01.00]Paper kites over the bay\n[00:04.00]I'll wait for you",
				"hasWordSync":  hasWordSync, "lyricsfile": lyricsfile,
			})
		}
	}
	get := func(h http.HandlerFunc) Lyrics {
		srv := httptest.NewServer(h)
		defer srv.Close()
		c := New("test")
		c.BaseURL = srv.URL
		l, err := c.Get(context.Background(), Query{Title: "Paper Kites", Artist: "Nobody"})
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	if l := get(answer(true, wordLyricsfile)); !l.Synced || l.Source != "LRCLIB" || len(l.Lines[0].Words) != 5 {
		t.Errorf("word-synced: %+v", l)
	}
	if l := get(answer(true, "lines: [oops")); !l.Synced || len(l.Lines) != 2 || l.Lines[0].Words != nil {
		t.Errorf("a Lyricsfile it can't read: %+v", l)
	}
	if l := get(answer(false, wordLyricsfile)); !l.Synced || l.Lines[0].Words != nil {
		t.Errorf("hasWordSync false: %+v", l)
	}
}
