package youtube

import "strings"

// shelfKey names a shelf of YouTube Music by the page its header opens:
// the browse ID of its title's link, or else of its More button. Unlike
// its title, it stays the same in every language and from day to day, so
// that Home keeps the shelf where the user put it. A page of YouTube
// Music's own, whose ID starts with "FE", keeps the parameters that pick
// a part of it, as one mood of Moods & genres; an artist's, an album's or
// a playlist's ID names it alone. It is "" for a shelf whose header opens
// no page, as Quick picks, which Home names by its title.
func shelfKey(header node) string {
	for _, nav := range []node{
		header.get("title", "runs", 0, "navigationEndpoint", "browseEndpoint"),
		header.get("moreContentButton", "buttonRenderer", "navigationEndpoint", "browseEndpoint"),
	} {
		id := nav.get("browseId").str()
		if id == "" {
			continue
		}
		if params := nav.get("params").str(); params != "" && strings.HasPrefix(id, "FE") {
			return id + "/" + params
		}
		return id
	}
	return ""
}
