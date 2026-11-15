package settings

import "slices"

// The sources of lyrics, by the keys LyricsSources names them by.
const (
	// LyricsBetterLyrics is Better Lyrics, for lyrics timed word by word.
	LyricsBetterLyrics = "betterlyrics"
	// LyricsUnison is Unison, the crowdsourced database of lyrics.
	LyricsUnison = "unison"
	// LyricsLRCLIB is LRCLIB, the open database of synced lyrics.
	LyricsLRCLIB = "lrclib"
	// LyricsYouTubeMusic is YouTube Music's own lyrics.
	LyricsYouTubeMusic = "youtube"
	// LyricsCaptions is the captions of the video that plays, on YouTube.
	LyricsCaptions = "captions"
)

// LyricsSourceKeys are the sources of lyrics in their own order, which a
// new install asks them in, every one on. A source a later version adds
// goes after the one it follows here.
var LyricsSourceKeys = []string{LyricsBetterLyrics, LyricsUnison, LyricsLRCLIB, LyricsYouTubeMusic, LyricsCaptions}

// LyricsSource is a source of lyrics, where the user put it.
type LyricsSource struct {
	// Key names the source, one of LyricsSourceKeys, or of a later
	// version's, which this one keeps but doesn't ask.
	Key string `json:"key"`
	// Off keeps the source from being asked, until the user turns it on.
	Off bool `json:"off,omitempty"`
}

// These bound what LyricsSources holds: how many sources, and how long a
// key may be, in bytes.
const (
	maxLyricsSources = 32
	maxLyricsKey     = 64
)

// ArrangeKey is the key the source moves by, in Settings.
func (s LyricsSource) ArrangeKey() string { return s.Key }

// SourceOrder returns the sources of lyrics this version knows, each once,
// in the order the user put them, and whether each is off. Those
// LyricsSources doesn't name take their place after the one before them
// in LyricsSourceKeys, or first. One source is always on: the first, when
// the user switched every one off.
func (l Lyrics) SourceOrder() []LyricsSource {
	at := func(order []LyricsSource, key string) int {
		return slices.IndexFunc(order, func(s LyricsSource) bool { return s.Key == key })
	}
	order := make([]LyricsSource, 0, len(LyricsSourceKeys))
	for _, s := range l.LyricsSources {
		if slices.Contains(LyricsSourceKeys, s.Key) && at(order, s.Key) < 0 {
			order = append(order, s)
		}
	}
	for i, key := range LyricsSourceKeys {
		if at(order, key) >= 0 {
			continue
		}
		after := 0
		if i > 0 {
			after = at(order, LyricsSourceKeys[i-1]) + 1
		}
		order = slices.Insert(order, after, LyricsSource{Key: key})
	}
	if !slices.ContainsFunc(order, func(s LyricsSource) bool { return !s.Off }) {
		order[0].Off = false
	}
	return order
}

// cleanSources is sources with each named once, by a key of its own,
// without those past maxLyricsSources.
func cleanSources(sources []LyricsSource) []LyricsSource {
	out := make([]LyricsSource, 0, len(sources))
	for _, s := range sources {
		if s.Key == "" || len(s.Key) > maxLyricsKey || len(out) == maxLyricsSources ||
			slices.ContainsFunc(out, func(o LyricsSource) bool { return o.Key == s.Key }) {
			continue
		}
		out = append(out, s)
	}
	return out
}
