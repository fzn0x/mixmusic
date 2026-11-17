package tags

import "testing"

// Which lyrics a file shows when it holds several (#554): the place
// decides, not the order in the file.

// syltOf is a SYLT frame of ID3v2.4 of content, timed in milliseconds,
// that sings one line, text, at one second.
func syltOf(content byte, text string) []byte {
	return frame4("SYLT", 0, syltFrame(0, 2, content, z8, cat([]byte(text), z8), 1000))
}

// TestLyricsSYLTContentOrder prefers a SYLT frame of lyrics, then of a
// transcription, then of other text, in whatever order they come, and
// never reads one of chords, events or links.
func TestLyricsSYLTContentOrder(t *testing.T) {
	lyrics, transcription, other := syltOf(1, "Lyrics"), syltOf(2, "Transcription"), syltOf(0, "Other")
	chords, events, links := syltOf(5, "Chords"), syltOf(4, "Events"), syltOf(8, "Links")
	for name, tc := range map[string]struct {
		frames [][]byte
		want   string
	}{
		"other first":          {[][]byte{other, transcription, lyrics}, "[00:01.000]Lyrics"},
		"lyrics first":         {[][]byte{lyrics, transcription, other}, "[00:01.000]Lyrics"},
		"lyrics in the middle": {[][]byte{transcription, lyrics, other}, "[00:01.000]Lyrics"},
		"no lyrics":            {[][]byte{other, transcription}, "[00:01.000]Transcription"},
		"only other":           {[][]byte{chords, other, events}, "[00:01.000]Other"},
		"none sung":            {[][]byte{chords, events, links}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			body := []byte{}
			for _, f := range tc.frames {
				body = cat(body, f)
			}
			checkLyrics(t, read(t, cat(id3(4, 0, body), mp3)), "", tc.want)
		})
	}
}

// TestLyricsPlainOrderID3 prefers a USLT frame to the TXXX tags, and
// LYRICS to UNSYNCEDLYRICS, wherever they sit in the tag.
func TestLyricsPlainOrderID3(t *testing.T) {
	txxx := func(key, value string) []byte {
		return frame4("TXXX", 0, cat([]byte{3}, []byte(key), z8, []byte(value)))
	}
	uslt := frame4("USLT", 0, cat([]byte{3}, []byte("eng"), z8, []byte("From USLT")))
	tag := func(frames ...[]byte) Tags {
		body := []byte{}
		for _, f := range frames {
			body = cat(body, f)
		}
		return read(t, cat(id3(4, 0, body), mp3))
	}
	checkLyrics(t, tag(txxx("UNSYNCEDLYRICS", "Unsynced"), txxx("LYRICS", "Plain"), uslt), "From USLT", "")
	checkLyrics(t, tag(txxx("UNSYNCEDLYRICS", "Unsynced"), txxx("LYRICS", "Plain")), "Plain", "")
	checkLyrics(t, tag(txxx("LYRICS", "Plain"), txxx("UNSYNCEDLYRICS", "Unsynced")), "Plain", "")
	checkLyrics(t, tag(txxx("UNSYNCEDLYRICS", "Unsynced")), "Unsynced", "")
	// An empty USLT frame doesn't hide the tags.
	empty := frame4("USLT", 0, cat([]byte{3}, []byte("eng"), z8))
	checkLyrics(t, tag(empty, txxx("LYRICS", "Plain")), "Plain", "")
	// Synced: SYLT, then SYNCEDLYRICS, then SYNCLYRICS.
	checkLyrics(t, tag(txxx("SYNCLYRICS", "[00:01.00]Sync"), txxx("SYNCEDLYRICS", "[00:01.00]Synced")), "", "[00:01.00]Synced")
	checkLyrics(t, tag(txxx("SYNCEDLYRICS", "[00:01.00]Synced"), syltOf(2, "Frame")), "", "[00:01.000]Frame")
}

// TestLyricsPlainOrderVorbis prefers LYRICS to UNSYNCEDLYRICS to
// "UNSYNCED LYRICS", and SYNCEDLYRICS to SYNCLYRICS, in any order.
func TestLyricsPlainOrderVorbis(t *testing.T) {
	for _, order := range [][]string{
		{"UNSYNCED LYRICS=Third", "UNSYNCEDLYRICS=Second", "LYRICS=First"},
		{"LYRICS=First", "UNSYNCEDLYRICS=Second", "UNSYNCED LYRICS=Third"},
		{"UNSYNCEDLYRICS=Second", "LYRICS=First", "UNSYNCED LYRICS=Third"},
	} {
		checkLyrics(t, read(t, flacFile([]byte{4}, vcomments(order...))), "First", "")
	}
	checkLyrics(t, read(t, flacFile([]byte{4}, vcomments("UNSYNCED LYRICS=Third", "UNSYNCEDLYRICS=Second"))), "Second", "")
	checkLyrics(t, read(t, flacFile([]byte{4}, vcomments("SYNCLYRICS=[00:01.00]B", "SYNCEDLYRICS=[00:01.00]A"))), "", "[00:01.00]A")
	checkLyrics(t, read(t, flacFile([]byte{4}, vcomments("SYNCEDLYRICS=[00:01.00]A", "SYNCLYRICS=[00:01.00]B"))), "", "[00:01.00]A")
}

// TestLyricsPlainOrderMP4 prefers the (c)lyr item to iTunes' LYRICS item,
// in either order.
func TestLyricsPlainOrderMP4(t *testing.T) {
	m4a := func(items ...[]byte) []byte {
		meta := mbox("meta", make([]byte, 4), mbox("hdlr", make([]byte, 25)), mbox("ilst", items...))
		return cat(mbox("ftyp", []byte("M4A "), be32(0)), mbox("moov", mbox("udta", meta)), mbox("mdat", make([]byte, 30)))
	}
	itunes := func(name, value string) []byte {
		return mbox("----", mbox("mean", be32(0), []byte("com.apple.iTunes")), mbox("name", be32(0), []byte(name)),
			mbox("data", be32(1), be32(0), []byte(value)))
	}
	lyr := item("\xa9lyr", 1, []byte("From lyr"))
	checkLyrics(t, read(t, m4a(itunes("LYRICS", "From iTunes"), lyr)), "From lyr", "")
	checkLyrics(t, read(t, m4a(lyr, itunes("LYRICS", "From iTunes"))), "From lyr", "")
	checkLyrics(t, read(t, m4a(itunes("UNSYNCEDLYRICS", "Second"), itunes("LYRICS", "First"))), "First", "")
}
