package tags

import (
	"bytes"
	"strings"
)

// The content advisory, which marks a track explicit (#427): iTunes keeps
// it in an MP4 file's rtng item, and taggers write it as ITUNESADVISORY in
// other files, an ID3v2 TXXX frame or a Vorbis comment, or as EXPLICIT.

// advisoryKeys are the names of the tags of the advisory, in upper case.
var advisoryKeys = map[string]bool{"ITUNESADVISORY": true, "EXPLICIT": true}

// explicitValue reports whether the value of an advisory tag marks a track
// explicit: 1, or 4 as older programs wrote, as iTunes numbers it, where 2
// is clean and 0 none, or a word, as "yes", "true" or "explicit".
func explicitValue(v string) bool {
	switch strings.ToLower(text(v)) {
	case "1", "4", "yes", "true", "explicit":
		return true
	}
	return false
}

// setAdvisory marks t explicit when the advisory tag key, of any case,
// marks it so in one of its values.
func (t *Tags) setAdvisory(key string, values ...string) {
	if !advisoryKeys[strings.ToUpper(text(key))] {
		return
	}
	for _, v := range values {
		if explicitValue(v) {
			t.Explicit = true
		}
	}
}

// txxx reads a frame of user text of ID3v2, TXXX or 2.2's TXX: its
// description, then its values, of the encoding its first byte names.
// Only the advisory's is taken.
func txxx(data []byte, t *Tags) {
	if len(data) < 2 {
		return
	}
	enc := data[0]
	key, rest := cut(enc, data[1:])
	values := strings.Split(strings.TrimRight(decode(enc, rest), "\x00"), "\x00")
	t.setAdvisory(key, values...)
}

// rtng reads the value of an MP4 file's rtng item, a number of one to
// eight bytes: explicit for 1 or 4.
func rtng(value []byte, t *Tags) {
	if len(value) == 0 || len(value) > 8 {
		return
	}
	n := 0
	for _, b := range bytes.TrimLeft(value, "\x00") {
		n = n<<8 | int(b)
	}
	t.Explicit = t.Explicit || n == 1 || n == 4
}
