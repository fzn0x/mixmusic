package stream

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

// livePlaylist is a media playlist laid out as YouTube's of a live stream
// are, of n segments from seq: 5 seconds each, a date and time on the
// first, a discontinuity before the second, and the cues of an ad, which a
// player can pass over. Its URIs are of host.
func livePlaylist(host string, seq int64, n int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:5\n#EXT-X-MEDIA-SEQUENCE:%d\n#EXT-X-DISCONTINUITY-SEQUENCE:307\n", seq)
	b.WriteString("#EXT-X-PROGRAM-DATE-TIME:2026-11-01T20:39:55.047+00:00\n")
	for i := range int64(n) {
		if i == 1 {
			b.WriteString("#EXT-X-DISCONTINUITY\n")
			b.WriteString(`#EXT-X-DATERANGE:CLASS="CUEPOINT-AD",DURATION=60.0,ID="1-AD-START",START-DATE="2026-11-01T20:41:10.047+00:00",X-YTCP-DURATION=5` + "\n")
			b.WriteString(`#EXT-X-CUEPOINT:CONTEXT=Kg0KBw,DURATION=60.0,EVENT=START,IDENTIFIER="SJ4g",TIME_OFFSET=0.0,TYPE=AD` + "\n")
		}
		fmt.Fprintf(&b, "#EXTINF:5.0,\n%s/sq/%d/segment.ts\n", host, seq+i)
	}
	return b.String()
}

func TestParseMediaPlaylist(t *testing.T) {
	base, _ := url.Parse("https://media.test/api/manifest/hls_playlist/expire/1793600000/playlist/index.m3u8")
	m, err := parseMediaPlaylist(livePlaylist("https://media.test/videoplayback", 1287944, 3), base)
	if err != nil {
		t.Fatal(err)
	}
	want := mediaPlaylist{target: 5 * time.Second, sequence: 1287944, segments: []segment{
		{seq: 1287944, uri: "https://media.test/videoplayback/sq/1287944/segment.ts", duration: 5 * time.Second},
		{seq: 1287945, uri: "https://media.test/videoplayback/sq/1287945/segment.ts", duration: 5 * time.Second, discontinuity: true},
		{seq: 1287946, uri: "https://media.test/videoplayback/sq/1287946/segment.ts", duration: 5 * time.Second},
	}}
	if !reflect.DeepEqual(m, want) || m.end() != 1287947 {
		t.Errorf("playlist %+v\nwant %+v", m, want)
	}

	// Relative URIs, CRLFs, a BOM, comments, a media sequence after the
	// segments, unencrypted segments, and the end.
	m, err = parseMediaPlaylist("\xef\xbb\xbf#EXTM3U\r\n#EXT-X-TARGETDURATION:6\r\n# a comment\r\n#EXT-X-KEY:METHOD=NONE\r\n"+
		"#EXTINF:5.5,a title, with a comma\r\nseg/1.ts\r\n\r\n#EXTINF:4\r\n/abs/2.ts\r\n#EXT-X-MEDIA-SEQUENCE:7\r\n#EXT-X-ENDLIST\r\n", base)
	want = mediaPlaylist{target: 6 * time.Second, sequence: 7, ended: true, segments: []segment{
		{seq: 7, uri: "https://media.test/api/manifest/hls_playlist/expire/1793600000/playlist/seg/1.ts", duration: 5500 * time.Millisecond},
		{seq: 8, uri: "https://media.test/abs/2.ts", duration: 4 * time.Second},
	}}
	if err != nil || !reflect.DeepEqual(m, want) {
		t.Errorf("playlist %+v, %v\nwant %+v", m, err, want)
	}

	// An empty live playlist, as a stream that just started has.
	if m, err := parseMediaPlaylist("#EXTM3U\n#EXT-X-TARGETDURATION:5\n#EXT-X-MEDIA-SEQUENCE:0\n", base); err != nil || len(m.segments) != 0 || m.ended {
		t.Errorf("an empty playlist: %+v, %v", m, err)
	}

	for _, c := range []struct{ name, text, says string }{
		{"no header", "#EXT-X-TARGETDURATION:5\n#EXTINF:5,\na.ts\n", "#EXTM3U"},
		{"no target duration", "#EXTM3U\n#EXTINF:5,\na.ts\n", "no target duration"},
		{"a target duration of words", "#EXTM3U\n#EXT-X-TARGETDURATION:five\n", "target duration"},
		{"a media sequence below 0", "#EXTM3U\n#EXT-X-TARGETDURATION:5\n#EXT-X-MEDIA-SEQUENCE:-1\n", "media sequence"},
		{"a segment of NaN seconds", "#EXTM3U\n#EXT-X-TARGETDURATION:5\n#EXTINF:NaN,\na.ts\n", "seconds"},
		{"encrypted", "#EXTM3U\n#EXT-X-TARGETDURATION:5\n#EXT-X-KEY:METHOD=AES-128,URI=\"k\"\n#EXTINF:5,\na.ts\n", "AES-128"},
		{"fragmented MP4", "#EXTM3U\n#EXT-X-TARGETDURATION:5\n#EXT-X-MAP:URI=\"init.mp4\"\n", "fragmented MP4"},
		{"byte ranges", "#EXTM3U\n#EXT-X-TARGETDURATION:5\n#EXTINF:5,\n#EXT-X-BYTERANGE:1000@0\na.ts\n", "byte ranges"},
		{"a master playlist", "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1\nv.m3u8\n", "master"},
		{"a URI that doesn't parse", "#EXTM3U\n#EXT-X-TARGETDURATION:5\n#EXTINF:5,\nhttp//a b%zz/secret\n", "doesn't parse"},
	} {
		_, err := parseMediaPlaylist(c.text, base)
		if !errors.Is(err, errPlaylist) || !strings.Contains(err.Error(), c.says) || strings.Contains(err.Error(), "secret") {
			t.Errorf("%s: %v, want one saying %q", c.name, err, c.says)
		}
	}
}

// masterPlaylist is a master playlist laid out as YouTube's of a live
// stream are, whose audio renditions are of host.
func masterPlaylist(host string) string {
	return "#EXTM3U\n#EXT-X-INDEPENDENT-SEGMENTS\n" +
		`#EXT-X-MEDIA:URI="` + host + `/itag/233/index.m3u8",TYPE=AUDIO,GROUP-ID="233",NAME="Default",DEFAULT=YES,AUTOSELECT=YES` + "\n" +
		`#EXT-X-MEDIA:URI="` + host + `/itag/234/index.m3u8",TYPE=AUDIO,GROUP-ID="234",NAME="Default",DEFAULT=YES,AUTOSELECT=YES` + "\n" +
		`#EXT-X-STREAM-INF:BANDWIDTH=507418,CODECS="avc1.4D4015,mp4a.40.5",RESOLUTION=426x240,FRAME-RATE=30,VIDEO-RANGE=SDR,AUDIO="233",CLOSED-CAPTIONS=NONE` + "\n" +
		host + "/itag/229/index.m3u8\n" +
		`#EXT-X-STREAM-INF:BANDWIDTH=591418,CODECS="avc1.4D4015,mp4a.40.2",RESOLUTION=426x240,FRAME-RATE=30,VIDEO-RANGE=SDR,AUDIO="234",CLOSED-CAPTIONS=NONE` + "\n" +
		host + "/itag/230/index.m3u8\n"
}

func TestAudioRenditions(t *testing.T) {
	base, _ := url.Parse("https://media.test/hls_variant/master.m3u8")
	got, err := audioRenditions(masterPlaylist("https://media.test/hls_playlist"), base)
	want := map[string]string{
		"233": "https://media.test/hls_playlist/itag/233/index.m3u8",
		"234": "https://media.test/hls_playlist/itag/234/index.m3u8",
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("renditions %v, %v; want %v", got, err, want)
	}
	// Relative URIs, and renditions of subtitles, which aren't audio.
	got, err = audioRenditions("#EXTM3U\n"+`#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",URI="subs.m3u8"`+"\n"+
		`#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="a",NAME="English, original",URI="a/index.m3u8"`+"\n", base)
	if want := map[string]string{"a": "https://media.test/hls_variant/a/index.m3u8"}; err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("renditions %v, %v; want %v", got, err, want)
	}
	if _, err := audioRenditions("<html>", base); !errors.Is(err, errPlaylist) {
		t.Errorf("not a playlist: %v", err)
	}
}

func TestAttributes(t *testing.T) {
	got := attributes(`TYPE=AUDIO,GROUP-ID="234",NAME="Default, of a comma",DEFAULT=YES, AUTOSELECT=YES,BANDWIDTH=591418,RESOLUTION=426x240`)
	want := map[string]string{"TYPE": "AUDIO", "GROUP-ID": "234", "NAME": "Default, of a comma", "DEFAULT": "YES",
		"AUTOSELECT": "YES", "BANDWIDTH": "591418", "RESOLUTION": "426x240"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("attributes %v, want %v", got, want)
	}
	for _, list := range []string{"", "NAME", `URI="unterminated`, `A="x"junk,B=1`, ",,="} {
		attributes(list) // never panics
	}
	if got := attributes(`A="x"junk,B=1`); got["A"] != "x" || got["B"] != "1" {
		t.Errorf("attributes after a quoted value: %v", got)
	}
}
