package match

import (
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Verdict is how sure a match of a song an import names is (#613).
type Verdict int

// The verdicts of Judge.
const (
	// NotFound: no candidate is close enough.
	NotFound Verdict = iota
	// ToCheck: a candidate is close, which the user checks.
	ToCheck
	// Sure: the candidate is the song, by its title, its artists, its
	// version and its length.
	Sure
)

// checkFloor is the least Score a candidate the user checks has.
const checkFloor = 0.45

// versionWords are the words of titles that tell a version of a song from
// another: a live take, a remix, an acoustic one. A candidate is sure
// only when its title has the same of them as the song's.
var versionWords = []string{"live", "remix", "acoustic", "instrumental", "karaoke", "cover", "demo", "extended",
	"unplugged", "orchestral", "piano", "slowed", "sped", "nightcore", "acapella", "cappella", "reprise", "rework",
	"bootleg", "symphonic"}

// featuring cuts the featured artists off a title: "Song feat. Name",
// "Song (with Name)".
var featuring = regexp.MustCompile(`(?i)\s+(?:[(\[]\s*(?:feat\.?|ft\.?|featuring|with)\s.*|(?:feat\.?|ft\.?|featuring)\s.*)$`)

// bracketed are the parts of a title in brackets, which name a version
// or an upload rather than the song.
var bracketed = regexp.MustCompile(`\s*[(\[][^)\]]*[)\]]`)

// Judge tells how sure it is that candidate is the recording of t: Sure
// when their titles, their credited artists and the words of their
// versions agree, and their lengths are within 3 seconds or 2%, whichever
// is more, or one is unknown; else ToCheck when it scores close enough,
// else NotFound.
func Judge(t, candidate media.Track) Verdict {
	if Score(t, candidate) < checkFloor {
		return NotFound
	}
	if TitleKey(t.Title) == TitleKey(candidate.Title) && artistsAgree(t, candidate) &&
		slices.Equal(versionsOf(t.Title), versionsOf(candidate.Title)) && lengthsAgree(t.DurationMs, candidate.DurationMs) {
		return Sure
	}
	return ToCheck
}

// TitleKey is what a title names of the song: its words, without what
// names a release, a version or a featured artist, sorted. Two titles of
// the same song have the same key.
func TitleKey(title string) string {
	core := featuring.ReplaceAllString(CleanTitle(title), "")
	core = bracketed.ReplaceAllString(core, "")
	if a, _, ok := strings.Cut(core, " - "); ok && strings.TrimSpace(a) != "" {
		core = a
	}
	ws := slices.DeleteFunc(words(core), func(w string) bool { return slices.Contains(versionWords, w) })
	if len(ws) == 0 {
		ws = words(title)
	}
	slices.Sort(ws)
	return strings.Join(slices.Compact(ws), " ")
}

// versionsOf are the version words a title has, sorted.
func versionsOf(title string) []string {
	var out []string
	for _, w := range words(title) {
		if slices.Contains(versionWords, w) && !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	slices.Sort(out)
	return out
}

// artistsAgree reports whether the candidate credits t's artists: its
// first by name, and every word of their names, among the candidate's
// artists or in its title, as featured artists are.
func artistsAgree(t, candidate media.Track) bool {
	if len(t.Artists) == 0 || t.Artists[0].Name == "" {
		return false
	}
	theirs := normalize(candidate.ArtistNames() + " " + candidate.Title)
	names := map[string]bool{}
	for _, a := range candidate.Artists {
		for _, n := range artistNames(a.Name) {
			names[n] = true
		}
	}
	if first := artistNames(t.Artists[0].Name); !slices.ContainsFunc(first, func(n string) bool { return names[n] }) &&
		!strings.Contains(theirs, normalize(t.Artists[0].Name)) {
		return false
	}
	have := map[string]bool{}
	for _, w := range strings.Fields(theirs) {
		have[w] = true
	}
	for _, a := range t.Artists {
		for _, w := range words(a.Name) {
			if !have[w] {
				return false
			}
		}
	}
	return true
}

// lengthsAgree reports whether two lengths in milliseconds are within 3
// seconds or 2% of the longer, whichever is more; an unknown one agrees.
func lengthsAgree(a, b int) bool {
	if a <= 0 || b <= 0 {
		return true
	}
	d := time.Duration(max(a-b, b-a)) * time.Millisecond
	return d <= max(3*time.Second, time.Duration(max(a, b))*time.Millisecond/50)
}

// FirstArtist is the first of the artists a byline names, as "A feat. B"
// or "A, B" join them.
func FirstArtist(byline string) string {
	parts := joiners.Split(byline, 2)
	return strings.TrimSpace(parts[0])
}
