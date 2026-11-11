package media

import (
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The versions of an album: its explicit and clean ones, and its editions,
// as a deluxe or a remastered one, which the services list as albums of
// their own, alike but for a mark in their titles or a badge. Lists of
// albums show one tile for each album, whose page switches between its
// versions (#345).
//
// Two albums are versions of one when they come from the same service,
// are of the same kind (album, single or EP), have the same artists, and
// the same title once the marks of versions are set aside: a last part in
// brackets, or after " - " or ": ", made of the words of versions only, as
// "(Clean)", "[Explicit]", "(Super Deluxe Edition)", "(Remastered 2011)" or
// " - 2011 Remaster", or naming an edition with words of its own beside a
// word only editions have, as ": Expanded Lantern Edition" (#459).
// "(Live)", "(Taylor's Version)" or "(Acoustic)" are no such marks, nor is
// ": A New Dawn": those are albums of their own. Of the albums with a
// title in common, those of the same edition are the same release, as an
// explicit one and its clean one, when their years and their numbers of
// tracks agree, where both tell them. Two releases of the album as first
// put out (without an edition's mark) are two albums that share a title,
// as an artist's self-titled album and a live one of the same name from
// another year: each stays an album of its own, and an edition joins the
// one of its year, if one alone, or stays apart too.

// Version is what an album's title says of its version.
type Version struct {
	// Base is the title without its marks, folded: the key of the album,
	// whichever its version.
	Base string
	// Edition names the edition as the title writes it, as "Super Deluxe
	// Edition" or "Remastered 2009", or is "" for the album as first put
	// out, as a "Standard Edition" is.
	Edition string
	// Rating is the word of the title that marks a clean or an explicit
	// version, as written, capitalised: "Clean", "Edited" or "Explicit".
	Rating string
	// Clean and Explicit are set for a title marked clean or explicit.
	Clean, Explicit bool
	// edition is Edition's key: its words, without those that tell
	// nothing, in order, so that "Deluxe" and "Deluxe Edition" match.
	edition string
}

// The kinds of the words of a version's mark.
const (
	notAVersionWord = iota
	// fillerWord: words that go with others: "version", "the", "special".
	fillerWord
	// editionWord: words that make a mark of an edition: "deluxe", "remastered".
	editionWord
	// namingWord: words of an edition that make a mark of it with words of
	// its own: "deluxe", "edition", as "Lantern Keeper's Edition".
	namingWord
	// cleanWord and explicitWord: words that mark the rating of a version.
	cleanWord
	explicitWord
)

// versionWords are the words a version's mark is made of, in lower case.
var versionWords = map[string]int{
	"clean": cleanWord, "edited": cleanWord, "censored": cleanWord,
	"explicit": explicitWord,

	"deluxe": namingWord, "remaster": namingWord, "remastered": namingWord, "remasters": namingWord,
	"expanded": namingWord, "anniversary": namingWord, "edition": namingWord, "reissue": namingWord,
	"bonus": editionWord, "mono": editionWord, "stereo": editionWord,
	"édition": namingWord, "edición": namingWord, "edicion": namingWord, "edizione": namingWord,

	"version": fillerWord, "versión": fillerWord, "the": fillerWord, "and": fillerWord, "with": fillerWord, "of": fillerWord,
	"super": fillerWord, "special": fillerWord, "collector's": fillerWord, "collectors": fillerWord, "collector": fillerWord,
	"limited": fillerWord, "standard": fillerWord, "original": fillerWord, "complete": fillerWord, "legacy": fillerWord,
	"platinum": fillerWord, "gold": fillerWord, "tour": fillerWord, "digital": fillerWord, "track": fillerWord, "tracks": fillerWord,
	"mix": fillerWord, "international": fillerWord, "japan": fillerWord, "japanese": fillerWord,
}

// unkeyedWords are the words an edition's key leaves out, which name no
// edition: "Deluxe Edition" is "Deluxe".
var unkeyedWords = map[string]bool{
	"edition": true, "version": true, "versión": true, "the": true, "and": true, "with": true, "of": true,
	"édition": true, "edición": true, "edicion": true, "edizione": true,
}

// keyWords are the words an edition's key writes as another.
var keyWords = map[string]string{
	"remastered": "remaster", "remasters": "remaster", "collectors": "collector", "collector's": "collector",
}

// VersionOf reads the marks of versions of an album's title.
func VersionOf(title string) Version {
	rest := strings.TrimSpace(title)
	var marks []string
	for {
		r, m, ok := cutVersionMark(rest)
		if !ok || r == "" || !isVersionMark(m) {
			break
		}
		marks, rest = append(marks, m), r
	}
	slices.Reverse(marks)
	v := Version{Base: foldTitle(rest)}
	var editions, keys []string
	for _, m := range marks {
		var kept []string
		named := false
		for _, w := range strings.Fields(m) {
			lw := versionWord(w)
			switch versionWords[lw] {
			case cleanWord:
				v.Clean, v.Rating = true, capitalised(strings.Trim(w, markPunctuation))
				continue
			case explicitWord:
				v.Explicit, v.Rating = true, capitalised(strings.Trim(w, markPunctuation))
				continue
			}
			if lw == "" {
				continue
			}
			kept = append(kept, w)
			if !unkeyedWords[lw] {
				named = true
				if s, ok := keyWords[lw]; ok {
					lw = s
				}
				keys = append(keys, lw)
			}
		}
		if named {
			editions = append(editions, strings.Join(kept, " "))
		}
	}
	slices.Sort(keys)
	if keys = slices.Compact(keys); len(keys) == 1 && (keys[0] == "standard" || keys[0] == "original") {
		// A standard edition is the album as first put out.
		return v
	}
	v.Edition, v.edition = strings.Join(editions, " · "), strings.Join(keys, " ")
	return v
}

// markPunctuation is what a word of a mark may start or end with.
const markPunctuation = ".,:;!?\"“”()[]-–—/&+·"

// versionWord is a word of a mark as versionWords has it, or "" for one of
// markPunctuation alone, as "-" or "&".
func versionWord(w string) string {
	return strings.ToLower(strings.Trim(strings.ReplaceAll(w, "’", "'"), markPunctuation))
}

// capitalised is w with its first letter in upper case.
func capitalised(w string) string {
	if w == "" {
		return w
	}
	r, n := utf8.DecodeRuneInString(w)
	return string(unicode.ToUpper(r)) + w[n:]
}

// cutVersionMark cuts the last part of a title that may be a version's mark: in
// brackets at its end, or after a dash or a colon, whichever comes last.
func cutVersionMark(t string) (rest, mark string, ok bool) {
	for _, b := range [...][2]string{{"(", ")"}, {"[", "]"}, {"（", "）"}} {
		if !strings.HasSuffix(t, b[1]) {
			continue
		}
		i := strings.LastIndex(t, b[0])
		if i <= 0 {
			return t, "", false
		}
		return strings.TrimSpace(t[:i]), t[i+len(b[0]) : len(t)-len(b[1])], true
	}
	at, sep := -1, ""
	for _, s := range []string{" - ", " – ", " — ", ": ", "：", "： "} {
		if i := strings.LastIndex(t, s); i > at {
			at, sep = i, s
		}
	}
	if at > 0 {
		return strings.TrimSpace(t[:at]), strings.TrimSpace(t[at+len(sep):]), true
	}
	return t, "", false
}

// isVersionMark reports whether m names an edition or a rating: made of
// the words of versions' marks alone, years and ordinals, as of "50th
// Anniversary", going with them; or with words of its own beside a word
// only an edition's name has, as "Expanded Lantern Edition".
func isVersionMark(m string) bool {
	marks, named, own := false, false, false
	for _, w := range strings.Fields(m) {
		lw := versionWord(w)
		switch kind := versionWords[lw]; {
		case lw == "":
		case kind == notAVersionWord && !versionNumber(lw):
			own = true
		case kind == namingWord:
			marks, named = true, true
		case kind != fillerWord && kind != notAVersionWord:
			marks = true
		}
	}
	if own {
		return named
	}
	return marks
}

// versionNumber reports whether w is a number, as a year, or an ordinal, as
// "50th".
func versionNumber(w string) bool {
	for _, suffix := range []string{"", "st", "nd", "rd", "th"} {
		if n, ok := strings.CutSuffix(w, suffix); ok && n != "" && strings.Trim(n, "0123456789") == "" {
			return true
		}
	}
	return false
}

// foldTitle is a title as titles compare: in lower case, its quotes and spaces
// made alike.
func foldTitle(s string) string {
	s = strings.NewReplacer("’", "'", "‘", "'", "“", `"`, "”", `"`).Replace(strings.ToLower(s))
	return strings.Join(strings.Fields(s), " ")
}

// GroupVersions groups albums by album: each group holds the versions of
// one album, as this file's comment tells, the album as first put out
// first, then its editions, in the order of the list, each edition's
// explicit version first (explicitFirst). The groups come in the order of
// their first albums in the list. Albums of this computer and of MixMusic
// itself are each a group of their own.
func GroupVersions(albums []Album) [][]Album {
	type entry struct {
		at int
		v  Version
	}
	var (
		keys    []string
		buckets = map[string][]entry{}
		groups  [][]int
	)
	for i, a := range albums {
		if a.Source != SourceYouTube && a.Source != SourceSpotify {
			groups = append(groups, []int{i})
			continue
		}
		v := VersionOf(a.Title)
		kind := a.Kind
		if kind == "" {
			kind = AlbumKindAlbum
		}
		names := make([]string, len(a.Artists))
		for j, r := range a.Artists {
			names[j] = foldTitle(r.Name)
		}
		key := string(a.Source) + "\x00" + string(kind) + "\x00" + strings.Join(names, "\x00") + "\x00\x00" + v.Base
		if _, ok := buckets[key]; !ok {
			keys = append(keys, key)
		}
		buckets[key] = append(buckets[key], entry{i, v})
	}
	for _, key := range keys {
		entries := buckets[key]
		// The releases of the title: the albums of an edition whose years
		// and counts of tracks agree.
		type release struct {
			edition string
			at      []int
		}
		var releases []release
	place:
		for _, e := range entries {
			for r := range releases {
				if releases[r].edition == e.v.edition &&
					!slices.ContainsFunc(releases[r].at, func(j int) bool { return !oneRelease(albums[j], albums[e.at]) }) {
					releases[r].at = append(releases[r].at, e.at)
					continue place
				}
			}
			releases = append(releases, release{e.v.edition, []int{e.at}})
		}
		var firsts, editions []release
		for _, r := range releases {
			if r.edition == "" {
				firsts = append(firsts, r)
			} else {
				editions = append(editions, r)
			}
		}
		if len(firsts) <= 1 {
			// One album, in all its versions.
			var all []int
			for _, r := range append(firsts, editions...) {
				all = append(all, r.at...)
			}
			groups = append(groups, all)
			continue
		}
		// Albums that share a title: an edition joins the one of its year,
		// if only one is of its year.
		at := len(groups)
		for _, r := range firsts {
			groups = append(groups, r.at)
		}
		for _, ed := range editions {
			year, joins := albums[ed.at[0]].Year, -1
			for i, r := range firsts {
				if year != "" && albums[r.at[0]].Year == year {
					if joins >= 0 {
						joins = -1
						break
					}
					joins = i
				}
			}
			if joins < 0 {
				groups = append(groups, ed.at)
				continue
			}
			groups[at+joins] = append(groups[at+joins], ed.at...)
		}
	}
	// Each group goes where its first album was.
	slices.SortStableFunc(groups, func(a, b []int) int { return slices.Min(a) - slices.Min(b) })
	out := make([][]Album, len(groups))
	for i, g := range groups {
		out[i] = make([]Album, len(g))
		for j, at := range g {
			out[i][j] = albums[at]
		}
		explicitFirst(out[i])
	}
	return out
}

// explicitFirst puts the explicit version of each edition of a group
// first, then those not marked, then those marked clean, as their list
// tells: the editions stay in their order.
func explicitFirst(versions []Album) {
	rank := map[string]int{}
	for _, a := range versions {
		e := VersionOf(a.Title).edition
		if _, ok := rank[e]; !ok {
			rank[e] = len(rank)
		}
	}
	rating := func(a Album) int {
		switch v := VersionOf(a.Title); {
		case a.Explicit || v.Explicit:
			return 0
		case v.Clean:
			return 2
		}
		return 1
	}
	slices.SortStableFunc(versions, func(a, b Album) int {
		if ra, rb := rank[VersionOf(a.Title).edition], rank[VersionOf(b.Title).edition]; ra != rb {
			return ra - rb
		}
		return rating(a) - rating(b)
	})
}

// oneRelease reports whether two albums of one title and edition are one
// release, as an explicit one and its clean one: of the same year and the
// same number of tracks, where both tell them.
func oneRelease(a, b Album) bool {
	return (a.Year == "" || b.Year == "" || a.Year == b.Year) &&
		(a.TrackCount == 0 || b.TrackCount == 0 || a.TrackCount == b.TrackCount)
}

// versionRatings returns what the titles of versions say of them, and which of
// them are explicit: those marked so, by their service, their title or
// one of their tracks, and, of an edition of which a version is marked
// clean and none explicit, the versions not marked.
func versionRatings(versions []Album) (vs []Version, explicit []bool) {
	n := len(versions)
	vs, explicit = make([]Version, n), make([]bool, n)
	for i, a := range versions {
		vs[i] = VersionOf(a.Title)
		explicit[i] = a.Explicit || vs[i].Explicit || slices.ContainsFunc(a.Tracks, func(t Track) bool { return t.Explicit })
	}
	marked := slices.Clone(explicit)
	for i := range versions {
		if explicit[i] || vs[i].Clean {
			continue
		}
		cleanOne, explicitOne := false, false
		for j := range versions {
			if j != i && vs[j].edition == vs[i].edition {
				cleanOne = cleanOne || vs[j].Clean
				explicitOne = explicitOne || marked[j]
			}
		}
		explicit[i] = cleanOne && !explicitOne
	}
	return vs, explicit
}

// VersionsTold reports whether versions tell, as their list gives them,
// which of them is explicit: when one of them is marked explicit or clean,
// or no edition of them has more than one version. Else, which version to
// open waits for the pages of the versions, which tell it by their tracks,
// as Spotify's lists of albums do.
func VersionsTold(versions []Album) bool {
	vs, explicit := versionRatings(versions)
	editions := map[string]int{}
	for i, v := range vs {
		if explicit[i] || v.Clean {
			return true
		}
		editions[v.edition]++
	}
	for _, n := range editions {
		if n > 1 {
			return false
		}
	}
	return true
}

// PreferredVersion returns which of versions, GroupVersions' group, to
// open: of the versions of the first one's edition, the explicit one, or
// the clean one when clean is set. It returns the first one when versions
// don't tell which is explicit, with told false.
func PreferredVersion(versions []Album, clean bool) (i int, told bool) {
	if len(versions) < 2 {
		return 0, true
	}
	vs, explicit := versionRatings(versions)
	var same []int
	for j, v := range vs {
		if v.edition == vs[0].edition {
			same = append(same, j)
		}
	}
	if len(same) == 1 {
		return 0, true
	}
	if !VersionsTold(versions) {
		return 0, false
	}
	for _, j := range same {
		if explicit[j] != clean {
			return j, true
		}
	}
	return 0, true
}

// VersionLabels names each of versions, GroupVersions' group, as the
// switch between them shows it: its edition, as its title writes it, and,
// of an edition that comes explicit and clean, which it is, as its title
// writes it, or else "Explicit" or "Clean"; "Standard" for the album as
// first put out. Versions that would share a name take their years, their
// numbers of tracks, or a number, after it.
func VersionLabels(versions []Album) []string {
	n := len(versions)
	vs, explicit := versionRatings(versions)
	labels := make([]string, n)
	for i := range versions {
		var parts []string
		if vs[i].Edition != "" {
			parts = append(parts, vs[i].Edition)
		}
		both := false
		for j := range versions {
			if vs[j].edition == vs[i].edition && (explicit[j] != explicit[i] || vs[j].Rating != "") {
				both = true
			}
		}
		switch {
		case vs[i].Rating != "":
			parts = append(parts, vs[i].Rating)
		case both && explicit[i]:
			parts = append(parts, "Explicit")
		case both:
			parts = append(parts, "Clean")
		}
		if len(parts) == 0 {
			parts = append(parts, "Standard")
		}
		labels[i] = strings.Join(parts, " · ")
	}
	// Versions that share a name.
	for i := range labels {
		var same []int
		for j := i; j < n; j++ {
			if labels[j] == labels[i] {
				same = append(same, j)
			}
		}
		if len(same) < 2 {
			continue
		}
		name := labels[i]
		years, counts := map[string]bool{}, map[int]bool{}
		for _, j := range same {
			years[versions[j].Year] = true
			counts[versions[j].TrackCount] = true
		}
		for k, j := range same {
			switch {
			case len(years) == len(same) && !years[""]:
				labels[j] = name + " · " + versions[j].Year
			case len(counts) == len(same) && !counts[0]:
				labels[j] = name + " · " + strconv.Itoa(versions[j].TrackCount) + " tracks"
			default:
				labels[j] = name + " · " + strconv.Itoa(k+1)
			}
		}
	}
	return labels
}
