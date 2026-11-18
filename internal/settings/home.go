package settings

import "unicode/utf8"

// Home is what the home page keeps: what it told the user, and how the
// user arranged its shelves.
type Home struct {
	// SkippedYTDLP is the release of yt-dlp the home page told of, which
	// the user put off: it doesn't tell of it again.
	SkippedYTDLP string `json:"skippedYtdlp"`
	// HomeShelves are Home's shelves as the user put them in order, hid
	// and added, by their keys; none leaves Home as it comes. The shelves
	// it doesn't name show after those it does, in their own order, as a
	// service's new shelves do.
	HomeShelves []HomeShelf `json:"homeShelves"`
}

// HomeShelf is a shelf of Home, where the user put it. Its Key names the
// shelf in words that stay the same in every language and from day to
// day, after where it comes from: "local:recent" for Recently played,
// "youtube:FEmusic_listen_again" for the shelf of YouTube Music whose
// title opens that page, "youtube:title:Quick picks" for one that opens
// none, "playlist:yt:PL…" for a playlist the user added.
type HomeShelf = Arranged

// Arranged is an item of a list the user arranges, where they put it: a
// shelf of Home, or a tab of the Library.
type Arranged struct {
	// Key names the item in words that stay the same in every language.
	Key string `json:"key"`
	// Title is the item's title when the user last arranged the list,
	// which names it while it's hidden or still loads.
	Title string `json:"title,omitempty"`
	// Hidden keeps the item out of its list, until the user brings it
	// back.
	Hidden bool `json:"hidden,omitempty"`
}

// ArrangeKey is the key the item moves by, in Edit home or Edit tabs.
func (a Arranged) ArrangeKey() string { return a.Key }

// These bound what HomeShelves holds: how many shelves, and how long a
// key and a title may be, in bytes.
const (
	maxHomeShelves = 200
	maxHomeKey     = 512
	maxHomeTitle   = 300
)

// defaultHome is the home page of a new install: its shelves in their own
// order.
func defaultHome() Home {
	var h Home
	h.HomeShelves = []HomeShelf{}
	return h
}

// clean is h with each shelf named once, by a key of its own, without
// those past maxHomeShelves, and titles cut to maxHomeTitle.
func (h Home) clean() Home {
	h.HomeShelves = cleanArranged(h.HomeShelves, maxHomeShelves)
	return h
}

// cleanArranged is list with each item named once, by a key of its own,
// without those past most, and titles cut to maxHomeTitle.
func cleanArranged(list []Arranged, most int) []Arranged {
	out := make([]Arranged, 0, len(list))
	seen := map[string]bool{}
	for _, a := range list {
		if a.Key == "" || len(a.Key) > maxHomeKey || seen[a.Key] || len(out) == most {
			continue
		}
		seen[a.Key] = true
		if len(a.Title) > maxHomeTitle {
			cut := maxHomeTitle
			for cut > 0 && !utf8.RuneStart(a.Title[cut]) {
				cut--
			}
			a.Title = a.Title[:cut]
		}
		out = append(out, a)
	}
	return out
}
