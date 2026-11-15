package stream

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

// selectYTDLP picks a format of the audio formats of a yt-dlp run as yt-dlp
// does with selector, of the forms the audio selectors use: best or worst
// audio by bitrate, with the filters of an acodec, of channels, of a
// format ID and of a language preference, the first alternative that picks
// one. A format with no language preference fails a filter on it, as in
// yt-dlp.
func selectYTDLP(t *testing.T, selector string, formats []ytFormat) string {
	t.Helper()
	for _, alt := range strings.Split(selector, "/") {
		which, filters, _ := strings.Cut(alt, "[")
		if which != "bestaudio" && which != "worstaudio" {
			for _, f := range formats {
				if f.FormatID == which {
					return which
				}
			}
			continue
		}
		var picked *ytFormat
		for i, f := range formats {
			ok := true
			for _, filter := range strings.Split(strings.TrimSuffix(filters, "]"), "][") {
				switch {
				case filter == "":
				case strings.HasPrefix(filter, "acodec="):
					ok = ok && f.ACodec == strings.TrimPrefix(filter, "acodec=")
				case filter == "audio_channels<=?2":
					ok = ok && f.AudioChannels <= 2
				case strings.HasPrefix(filter, "format_id!*="):
					ok = ok && !strings.Contains(f.FormatID, strings.TrimPrefix(filter, "format_id!*="))
				case strings.HasPrefix(filter, "language_preference>="):
					n, err := strconv.Atoi(strings.TrimPrefix(filter, "language_preference>="))
					if err != nil {
						t.Fatal(err)
					}
					ok = ok && f.LanguagePreference != 0 && f.LanguagePreference >= n
				default:
					t.Fatalf("a filter this test doesn't know: %s", filter)
				}
			}
			switch {
			case !ok:
			case picked == nil, better(which, f, *picked, formats):
				picked = &formats[i]
			}
		}
		if picked != nil {
			return picked.FormatID
		}
	}
	return ""
}

func dubbedFormats(t *testing.T) []ytFormat {
	t.Helper()
	raw, err := os.ReadFile("testdata/ytdlp-dubbed.json")
	if err != nil {
		t.Fatal(err)
	}
	var in info
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}
	return in.Formats
}

// oldSaver is the selector of data saver before it knew of audio tracks.
const oldSaver = "worstaudio[acodec=opus][audio_channels<=?2][format_id!*=drc]/worstaudio[acodec=opus][audio_channels<=?2]/" +
	"worstaudio[acodec=mp4a.40.2][audio_channels<=?2][format_id!*=drc]/worstaudio[acodec=mp4a.40.2][audio_channels<=?2]/233/234/worstaudio"

// TestYTDLPPicksOriginalTrack runs the selectors over the audio formats yt-dlp
// 2026.10.26 listed of a video dubbed in 23 languages (testdata/ytdlp-dubbed.json,
// three of its tracks): the original English is format -23, at a language
// preference of 10.
func TestYTDLPPicksOriginalTrack(t *testing.T) {
	formats := dubbedFormats(t)
	// What the issue showed: the smallest Opus is a dub's.
	if got := selectYTDLP(t, oldSaver, formats); got != "249-0" {
		t.Fatalf("the old data saver selector picked %s: the fixture no longer shows the bug", got)
	}
	if got := selectYTDLP(t, formats0(QualitySaver), formats); got != "249-23" {
		t.Errorf("data saver picked %s, want the original's 249-23", got)
	}
	if got := selectYTDLP(t, formats0(QualityBest), formats); got != "251-23" {
		t.Errorf("best picked %s, want the original's 251-23", got)
	}
}

// formats0 is the selector of quality q.
func formats0(q Quality) string { return formats[q] }

// TestYTDLPTrackTiers checks the order of the tiers: the original, then
// YouTube's default, then any track; a video of one track, which has no
// language preference, still gets its audio.
func TestYTDLPTrackTiers(t *testing.T) {
	opus := func(id string, abr float64, pref int) ytFormat {
		return ytFormat{FormatID: id, ACodec: "opus", ABR: abr, AudioChannels: 2, LanguagePreference: pref}
	}
	for _, c := range []struct {
		name        string
		formats     []ytFormat
		best, saver string
	}{
		{"one track", []ytFormat{opus("249", 50, 0), opus("251", 130, 0)}, "251", "249"},
		{"original beats default", []ytFormat{opus("249-0", 50, 5), opus("249-1", 51, 10), opus("251-1", 120, 10), opus("251-0", 130, 5)}, "251-1", "249-1"},
		{"default beats a dub", []ytFormat{opus("249-0", 50, -1), opus("249-1", 51, 5), opus("251-0", 130, -1), opus("251-1", 120, 5)}, "251-1", "249-1"},
		{"dubs and a description", []ytFormat{opus("249-0", 50, -10), opus("249-1", 51, -1), opus("251-0", 100, -10), opus("251-1", 120, -1)}, "251-1", "249-0"},
		// The original in AAC beats a dub in Opus.
		{"original in AAC", []ytFormat{opus("251-0", 130, -1), {FormatID: "140-1", ACodec: "mp4a.40.2", ABR: 129, AudioChannels: 2, LanguagePreference: 10}}, "140-1", "140-1"},
	} {
		if got := selectYTDLP(t, formats[QualityBest], c.formats); got != c.best {
			t.Errorf("%s: best picked %s, want %s", c.name, got, c.best)
		}
		if got := selectYTDLP(t, formats[QualitySaver], c.formats); got != c.saver {
			t.Errorf("%s: data saver picked %s, want %s", c.name, got, c.saver)
		}
	}
}

// TestSurroundOfOriginalTrack picks the 5.1 mix of the original track,
// though a dub's is of a better codec or more bits. The 5.1 formats are
// added to the recorded run, as YouTube serves them of a dubbed video.
func TestSurroundOfOriginalTrack(t *testing.T) {
	raw, err := os.ReadFile("testdata/ytdlp-dubbed.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	mix := func(id, codec string, abr float64, pref int) map[string]any {
		return map[string]any{"format_id": id, "url": "https://rr1---sn-test.googlevideo.com/videoplayback?itag=" + id,
			"ext": "m4a", "protocol": "https", "acodec": codec, "vcodec": "none", "abr": abr, "audio_channels": 6, "language_preference": pref}
	}
	doc["formats"] = append(doc["formats"].([]any),
		mix("380-0", "ac-3", 384, -1), mix("328-1", "ec-3", 448, -1), mix("328-23", "ec-3", 384, 10), mix("380-2", "ac-3", 384, 5))
	raw, _ = json.Marshal(doc)
	s, err := parseInfo(raw)
	if err != nil || s.Surround == nil {
		t.Fatalf("%v, %+v", err, s)
	}
	if !strings.HasSuffix(s.Surround.URL, "itag=328-23") {
		t.Errorf("the 5.1 mix is %s, want the original's 328-23", s.Surround.URL)
	}
}

// better reports whether f sorts ahead of o as yt-dlp does for which: by
// quality first, which is the same in every track of a format (249 in each
// language is one quality, whatever its exact bitrate), then by language
// preference, the highest for bestaudio and the lowest for worstaudio.
func better(which string, f, o ytFormat, all []ytFormat) bool {
	quality := func(x ytFormat) float64 {
		base, _, _ := strings.Cut(x.FormatID, "-")
		var top float64
		for _, y := range all {
			if b, _, _ := strings.Cut(y.FormatID, "-"); b == base {
				top = max(top, y.ABR)
			}
		}
		return top
	}
	if qf, qo := quality(f), quality(o); qf != qo {
		return (which == "bestaudio") == (qf > qo)
	}
	if f.LanguagePreference == o.LanguagePreference {
		return false
	}
	return (which == "bestaudio") == (f.LanguagePreference > o.LanguagePreference)
}
