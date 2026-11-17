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
// upper case: true for those of synced lyrics, in LRC.
var lyricsKeys = map[string]bool{
	"SYNCEDLYRICS": true, "SYNCLYRICS": true,
	"LYRICS": false, "UNSYNCEDLYRICS": false, "UNSYNCED LYRICS": false,
}

// isLyricsKey reports whether a tag's name, in upper case, is that of
// lyrics.
func isLyricsKey(key string) bool {
	_, ok := lyricsKeys[key]
	return ok
}

// setLyrics keeps the first of values that holds lyrics as those of the
// tag key, of any case, unless the file gave them before: synced or
// plain, as lyricsKeys says. A key that names no lyrics changes nothing.
func (t *Tags) setLyrics(key string, values ...string) {
	synced, ok := lyricsKeys[strings.ToUpper(text(key))]
	if !ok {
		return
	}
	field := &t.Lyrics
	if synced {
		field = &t.SyncedLyrics
	}
	for _, v := range values {
		keepLyrics(field, v)
	}
}

// keepLyrics sets *field to the lyrics s, unless it holds some already.
func keepLyrics(field *string, s string) {
	if *field == "" {
		*field = lyricsText(s)
	}
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
func sylt(data []byte, frameMs float64) string {
	if len(data) < 6 {
		return ""
	}
	enc, unit, content := data[0], data[4], data[5]
	// 0 is other text, 1 lyrics and 2 a transcription; the others are
	// movements, events, chords, trivia and links.
	if content > 2 {
		return ""
	}
	scale := 1.0
	switch unit {
	case 1:
		scale = frameMs
	case 2:
	default:
		return ""
	}
	if scale <= 0 {
		return ""
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
	return lrcOf(pieces)
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
