package tags

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"unicode"
)

// The lyrics a file holds (#554). Taggers keep them as text in an ID3v2
// USLT frame, a Vorbis comment, an MP4 file's ©lyr item or an item of
// iTunes' own, and timed in an ID3v2 SYLT frame or a Vorbis comment of
// LRC. Some write LRC where the text goes, which the app tells apart
// (lyrics.FromText).

const (
	// maxLyrics bounds the bytes of the lyrics of a tag that are kept.
	maxLyrics = 1 << 20
	// maxSynced bounds the timed pieces of a SYLT frame that are read: a
	// song has a few thousand syllables at most.
	maxSynced = 1 << 16
)

// lyricsKeys are the names of the tags of lyrics, as Vorbis comments, the
// TXXX frames of ID3v2 and the items of iTunes in MP4 files have them, in
// upper case, each with whether it holds synced lyrics, in LRC, and how it
// ranks among the places of its kind in a file that holds several: the
// lower, the better.
var lyricsKeys = map[string]lyricsKey{
	"SYNCEDLYRICS": {true, rankSynced}, "SYNCLYRICS": {true, rankSynced + 1},
	"LYRICS": {false, rankPlain}, "UNSYNCEDLYRICS": {false, rankPlain + 1}, "UNSYNCED LYRICS": {false, rankPlain + 2},
}

// lyricsKey is what a tag's name says of its lyrics.
type lyricsKey struct {
	synced bool
	rank   int
}

// The ranks of the places that hold lyrics. A format's standard place
// ranks first: a USLT frame, an MP4 file's (c)lyr item. The plain tags
// follow, as lyricsKeys orders them. Synced ranks count apart: a SYLT
// frame's is syltRank's, below rankSynced, then the tags'.
const (
	rankStandard = 0
	rankPlain    = 1
	rankSynced   = 3
)

// syltRank is the rank of a SYLT frame of content type content: 1 is
// lyrics, 2 a transcription and 0 other text. The others, as events,
// chords and links, are not lyrics: ok is false.
func syltRank(content byte) (rank int, ok bool) {
	switch content {
	case 1:
		return 0, true
	case 2:
		return 1, true
	case 0:
		return 2, true
	}
	return 0, false
}

// isLyricsKey reports whether a tag's name, in upper case, is that of
// lyrics.
func isLyricsKey(key string) bool {
	_, ok := lyricsKeys[key]
	return ok
}

// setLyrics keeps the first of values that holds lyrics as those of the
// tag key, of any case, unless the file gave better ones: synced or
// plain, as lyricsKeys says. A key that names no lyrics changes nothing.
func (t *Tags) setLyrics(key string, values ...string) {
	k, ok := lyricsKeys[strings.ToUpper(text(key))]
	if !ok {
		return
	}
	for _, v := range values {
		if t.keepLyrics(k.synced, k.rank, v) {
			return
		}
	}
}

// keepLyrics keeps the lyrics s, synced or plain, when the tag holds none
// of that kind or a worse rank, so that the place a file keeps them in,
// not their order in it, decides. It reports whether s holds lyrics.
func (t *Tags) keepLyrics(synced bool, rank int, s string) bool {
	s = lyricsText(s)
	if s == "" {
		return false
	}
	field, held := &t.Lyrics, &t.lyricsRank
	if synced {
		field, held = &t.SyncedLyrics, &t.syncedRank
	}
	if *field == "" || rank < *held {
		*field, *held = s, rank
	}
	return true
}

// lyricsText is s as lyrics: valid UTF-8, cut at maxLyrics bytes, its
// lines ending in "\n", without a byte order mark nor other control
// characters, and trimmed.
func lyricsText(s string) string {
	if len(s) > maxLyrics {
		s = s[:maxLyrics]
	}
	s = strings.ToValidUTF8(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '\r':
			return '\n'
		case r == '\n' || r == '\t':
			return r
		case r == 0xFEFF || unicode.IsControl(r):
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

// uslt reads a frame of unsynchronised lyrics of ID3v2, USLT or 2.2's
// ULT: their encoding, language and description, then the lyrics.
func uslt(data []byte) string {
	if len(data) < 5 {
		return ""
	}
	enc := data[0]
	_, rest := cut(enc, data[4:])
	return decode(enc, rest)
}

// syncedText is a piece of text of a SYLT frame, a line or a syllable,
// and the time it's sung at, in milliseconds.
type syncedText struct {
	text string
	ms   int
}

// sylt reads a frame of synchronised lyrics of ID3v2, SYLT or 2.2's SLT,
// as LRC (lrcOf): its encoding, language, the unit of its times, the type
// of its content and its description, then pieces of text, each followed
// by the time it's sung at, in milliseconds or in frames of the MPEG audio,
// which last frameMs each. It is "" for times in frames when frameMs is
// 0, as in a WAV file, and for content that isn't sung, as chords or
// events.
func sylt(data []byte, frameMs float64) (lrc string, rank int) {
	if len(data) < 6 {
		return "", 0
	}
	enc, unit, content := data[0], data[4], data[5]
	// Only 0, 1 and 2 are lyrics; the others are movements, events,
	// chords, trivia and links.
	rank, ok := syltRank(content)
	if !ok {
		return "", 0
	}
	scale := 1.0
	switch unit {
	case 1:
		scale = frameMs
	case 2:
	default:
		return "", 0
	}
	if scale <= 0 {
		return "", 0
	}
	_, b := cut(enc, data[6:])
	var pieces []syncedText
	for len(b) > 0 && len(pieces) < maxSynced {
		var s string
		s, b = cut(enc, b)
		if len(b) < 4 {
			break
		}
		ms := math.Round(float64(binary.BigEndian.Uint32(b)) * scale)
		b = b[4:]
		if ms > math.MaxInt32 {
			continue
		}
		pieces = append(pieces, syncedText{s, int(ms)})
	}
	return lrcOf(pieces), rank
}

// lrcOf writes timed pieces of text as LRC, a line of each. Where some
// start with a line break, as taggers mark the first syllable of a line,
// or end with one, the pieces between the breaks are a line, its time the
// first's, and each of a line of several is timed by a stamp of enhanced
// LRC, as "<00:01.250>", before it. It stops past maxLyrics bytes.
func lrcOf(pieces []syncedText) string {
	breaks := false
	for _, p := range pieces {
		if strings.ContainsAny(p.text, "\r\n") {
			breaks = true
			break
		}
	}
	var lines [][]syncedText
	start := true
	for _, p := range pieces {
		text := strings.TrimLeft(p.text, "\r\n")
		if breaks && text != p.text {
			start = true
		}
		next := strings.TrimRight(text, "\r\n")
		p.text = strings.Join(strings.FieldsFunc(next, func(r rune) bool { return r == '\r' || r == '\n' }), " ")
		if start || !breaks {
			lines = append(lines, nil)
		}
		lines[len(lines)-1] = append(lines[len(lines)-1], p)
		start = next != text
	}
	var b strings.Builder
	for _, line := range lines {
		if b.Len() > maxLyrics {
			break
		}
		b.WriteString(lrcStamp('[', line[0].ms))
		if len(line) == 1 {
			b.WriteString(line[0].text)
		} else {
			for _, p := range line {
				b.WriteString(lrcStamp('<', p.ms))
				b.WriteString(p.text)
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// lrcStamp writes a time of ms milliseconds as LRC does, in brackets, as
// "[01:02.345]", or in angle brackets, as enhanced LRC times words.
func lrcStamp(open rune, ms int) string {
	end := ']'
	if open == '<' {
		end = '>'
	}
	return fmt.Sprintf("%c%02d:%02d.%03d%c", open, ms/60000, ms/1000%60, ms%1000, end)
}

// mpegFrameMs returns how long a frame of the MPEG audio whose first
// header is at offset at lasts, in milliseconds, as SYLT frames may count
// time in frames: 0 when no frame's header is there.
func (f *file) mpegFrameMs(at int64) float64 {
	var h [4]byte
	if _, err := f.readAt(at, h[:]); err != nil || h[0] != 0xFF || h[1]&0xE0 != 0xE0 {
		return 0
	}
	// The version: 0 is MPEG 2.5, 2 MPEG-2 and 3 MPEG-1; the layer: 1 is
	// Layer III, 2 Layer II and 3 Layer I.
	version, layer, rate := h[1]>>3&3, h[1]>>1&3, h[2]>>2&3
	if version == 1 || layer == 0 || rate == 3 {
		return 0
	}
	rates := [4][3]float64{{11025, 12000, 8000}, {}, {22050, 24000, 16000}, {44100, 48000, 32000}}
	samples := 1152.0
	switch {
	case layer == 3:
		samples = 384
	case layer == 1 && version != 3:
		samples = 576
	}
	return samples * 1000 / rates[version][rate]
}
