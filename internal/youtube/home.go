package youtube

import (
	"context"

	"github.com/fzn0x/mixmusic/internal/media"
)

// maxHomePages bounds how many pages of shelves of home are read: YouTube
// sends its home a few shelves at a time.
const maxHomePages = 6

// Home returns the shelves of YouTube Music's home page: quick picks,
// mixes, albums and playlists. Signed in, they are the user's own.
func (c *Client) Home(ctx context.Context) ([]media.Shelf, error) {
	n, err := c.browse(ctx, "FEmusic_home")
	if err != nil {
		return nil, err
	}
	shelves := parseShelves(n)
	list := n.get("contents", "singleColumnBrowseResultsRenderer", "tabs", 0, "tabRenderer", "content", "sectionListRenderer")
	token := list.get("continuations", 0, "nextContinuationData", "continuation").str()
	for page := 1; token != "" && page < maxHomePages; page++ {
		more, err := c.call(ctx, "browse", map[string]any{"continuation": token}, nil)
		if err != nil {
			break // keep the shelves read so far
		}
		next := more.get("continuationContents", "sectionListContinuation")
		shelves = append(shelves, sectionShelves(next.get("contents"))...)
		token = next.get("continuations", 0, "nextContinuationData", "continuation").str()
	}
	return shelves, nil
}

// parseShelves reads the carousels and grids of a browse page.
func parseShelves(n node) []media.Shelf {
	return sectionShelves(n.get("contents", "singleColumnBrowseResultsRenderer", "tabs", 0, "tabRenderer", "content", "sectionListRenderer", "contents"))
}

// sectionShelves reads the shelves among the sections of a page: its
// carousels, and its grids of cards. Sections of other kinds, and shelves
// with no item MixMusic can show, are left out.
func sectionShelves(sections node) []media.Shelf {
	shelves := []media.Shelf{}
	for _, s := range sections.list() {
		if inner := s.get("itemSectionRenderer", "contents"); inner.exists() {
			shelves = append(shelves, sectionShelves(inner)...)
			continue
		}
		var (
			title, subtitle, key string
			contents             []node
		)
		if carousel := s.get("musicCarouselShelfRenderer"); carousel.exists() {
			header := carousel.get("header", "musicCarouselShelfBasicHeaderRenderer")
			title, subtitle, key = header.get("title").text(), header.get("strapline").text(), shelfKey(header)
			contents = carousel.get("contents").list()
		} else if grid := s.get("gridRenderer"); grid.exists() {
			header := grid.get("header", "gridHeaderRenderer")
			title, key = header.get("title").text(), shelfKey(header)
			contents = grid.get("items").list()
		} else if list := s.get("musicShelfRenderer"); list.exists() {
			title, key = list.get("title").text(), shelfKey(list)
			contents = list.get("contents").list()
		} else {
			continue
		}
		items := parseItems(contents)
		if len(items) == 0 {
			continue
		}
		shelves = append(shelves, media.Shelf{Title: title, Subtitle: subtitle, Source: media.SourceYouTube, Items: items, Key: key})
	}
	return shelves
}
