package native

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"golang.org/x/text/unicode/norm"

	"github.com/fzn0x/mixmusic/internal/media"
)

// trackSorts are the orders a list of tracks shows in, the list's own
// first, and their labels after it.
var trackSorts = []string{"order", "title", "artist", "length"}
var trackSortLabels = []string{"", "Title", "Artist", "Length"}

// trackTool is how the user sorted a list of tracks, and what they search
// in it.
type trackTool struct {
	sort  string
	query string
}

// trackView is a list of tracks as the user sorted and searched it: its
// tracks, and where each is in the list.
type trackView struct {
	tracks  []media.Track
	indices []int
}

// trackTools is the search box and the sort of a list of tracks, which
// noun names, as the user left them in the list of key. It returns the
// view of tracks they make, whole until the list has two tracks.
func (s *shell) trackTools(c *ui.Context, key, noun string, tracks []media.Track) trackView {
	k := &s.lk
	tool := s.tools[key]
	if tool == nil {
		tool = &trackTool{sort: "order"}
		s.tools[key] = tool
	}
	if len(tracks) < 2 {
		return viewTracks(tracks, "order", "")
	}
	view := viewTracks(tracks, tool.sort, tool.query)
	ui.Row(c).Gap(12).Margin(0, 0, 16).Children(func() {
		box := ui.Row(c).Height(36).Width(320).Shrink(1).MinWidth(160).Gap(8).PaddingX(12).Radius(k.radiusSm).Cursor(ui.CursorText)
		var search ui.Element
		box.Children(func() {
			ui.Icon(c, icon("search")).FontSize(16).TextColor(k.muted)
			search = ui.TextInputBase(c.Key(key+"-search"), &tool.query).Placeholder("search this " + noun).Grow(1).MinWidth(0).
				Label("Search this " + noun).FontSize(k.bodySize())
			if tool.query != "" {
				// Escape clears the search, and goes on once it's clear.
				search.OnShortcut(0, ui.KeyEscape, func() { tool.query = "" })
			}
		})
		if search.Focused() {
			box.Border(2, k.accent)
		} else {
			box.Border(2, k.outline)
		}
		labels := slices.Clone(trackSortLabels)
		labels[0] = strings.ToUpper(noun[:1]) + noun[1:] + " order"
		chosen := labels[max(slices.Index(trackSorts, tool.sort), 0)]
		if ui.Select(c.Key(key+"-sort"), &chosen, labels).Width(180).Label("Sort by").Changed() {
			if i := slices.Index(labels, chosen); i >= 0 {
				tool.sort = trackSorts[i]
			}
		}
		if strings.TrimSpace(tool.query) != "" {
			ui.Text(c, fmt.Sprintf("%d of %d", len(view.tracks), len(tracks))).FontSize(k.smallSize()).TextColor(k.muted).
				FontFeatures("tnum")
		}
	})
	return view
}

// searching is what the user searches in the list of key, if anything.
func (s *shell) searching(key string) string {
	if tool := s.tools[key]; tool != nil {
		return strings.TrimSpace(tool.query)
	}
	return ""
}

// viewTracks keeps the tracks whose title, artists or album hold every
// word of query, in the order sort says, as the web interface's track
// tools do: tracks that sort the same keep the list's order.
func viewTracks(tracks []media.Track, sort, query string) trackView {
	words := strings.Fields(fold(query))
	type row struct {
		track media.Track
		index int
	}
	var rows []row
	for i, t := range tracks {
		if len(words) > 0 {
			album := ""
			if t.Album != nil {
				album = t.Album.Name
			}
			text := fold(t.Title + " " + names(t.Artists) + " " + album)
			if !allIn(words, text) {
				continue
			}
		}
		rows = append(rows, row{t, i})
	}
	var by func(a, b media.Track) int
	switch sort {
	case "title":
		by = func(a, b media.Track) int { return compareText(a.Title, b.Title) }
	case "artist":
		by = func(a, b media.Track) int {
			return cmp.Or(compareText(names(a.Artists), names(b.Artists)), compareText(a.Title, b.Title))
		}
	case "length":
		by = func(a, b media.Track) int { return cmp.Compare(a.DurationMs, b.DurationMs) }
	}
	if by != nil {
		slices.SortStableFunc(rows, func(a, b row) int { return by(a.track, b.track) })
	}
	v := trackView{tracks: make([]media.Track, len(rows)), indices: make([]int, len(rows))}
	for i, r := range rows {
		v.tracks[i], v.indices[i] = r.track, r.index
	}
	return v
}

func allIn(words []string, text string) bool {
	for _, w := range words {
		if !strings.Contains(text, w) {
			return false
		}
	}
	return true
}

// fold makes text compare as a reader reads it: lowercase, and without
// its accents.
func fold(s string) string {
	var b strings.Builder
	for _, r := range norm.NFKD.String(s) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// compareText compares texts as the web interface's collator does, more
// or less: regardless of case and accents, and their numbers by value, so
// that "Track 2" comes before "Track 10".
func compareText(a, b string) int {
	a, b = fold(a), fold(b)
	for a != "" && b != "" {
		ra, na := utf8.DecodeRuneInString(a)
		rb, nb := utf8.DecodeRuneInString(b)
		if isDigit(ra) && isDigit(rb) {
			da, db := leadingDigits(a), leadingDigits(b)
			ta, tb := strings.TrimLeft(da, "0"), strings.TrimLeft(db, "0")
			if c := cmp.Or(cmp.Compare(len(ta), len(tb)), strings.Compare(ta, tb)); c != 0 {
				return c
			}
			a, b = a[len(da):], b[len(db):]
			continue
		}
		if ra != rb {
			return cmp.Compare(ra, rb)
		}
		a, b = a[na:], b[nb:]
	}
	return cmp.Compare(len(a), len(b))
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func leadingDigits(s string) string {
	i := 0
	for i < len(s) && isDigit(rune(s[i])) {
		i++
	}
	return s[:i]
}
