package youtube

import (
	"context"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
)

// watchNextTabs returns the tabs of a "next" response: up next, lyrics,
// related.
func watchNextTabs(n node) []node {
	return n.get("contents", "singleColumnMusicWatchNextResultsRenderer", "tabbedRenderer", "watchNextTabbedResultsRenderer", "tabs").list()
}

// Radio returns the start of a radio: a mix of tracks like the one given,
// which YouTube Music plays after it. The first track is the one given.
func (c *Client) Radio(ctx context.Context, videoID string) ([]media.Track, error) {
	return c.RadioOf(ctx, videoID, "RDAMVM"+videoID)
}

// RadioOf returns the start of the radio playlistID (RD…), as a link to
// it plays it from videoID: that track first, then the radio's.
func (c *Client) RadioOf(ctx context.Context, videoID, playlistID string) ([]media.Track, error) {
	n, err := c.call(ctx, "next", map[string]any{
		"videoId":                       videoID,
		"playlistId":                    playlistID,
		"isAudioOnly":                   true,
		"enablePersistentPlaylistPanel": true,
		"tunerSettingValue":             "AUTOMIX_SETTING_NORMAL",
	}, nil)
	if err != nil {
		return nil, err
	}
	return parseQueue(n), nil
}

// parseQueue reads the tracks of the queue of a "next" response.
func parseQueue(n node) []media.Track {
	tracks := []media.Track{}
	tabs := watchNextTabs(n)
	if len(tabs) == 0 {
		return tracks
	}
	panel := tabs[0].get("tabRenderer", "content", "musicQueueRenderer", "content", "playlistPanelRenderer")
	for _, item := range panel.get("contents").list() {
		v := item.get("playlistPanelVideoRenderer")
		if !v.exists() {
			v = item.get("playlistPanelVideoWrapperRenderer", "primaryRenderer", "playlistPanelVideoRenderer")
		}
		videoID := v.get("videoId").str()
		if videoID == "" {
			continue
		}
		b := parseByline(v.get("longBylineText").runs())
		if b.duration == 0 {
			b.duration = parseDuration(v.get("lengthText").text())
		}
		watch := v.get("navigationEndpoint", "watchEndpoint")
		tracks = append(tracks, *newTrack(videoID, v.get("title").text(), watch, b, artwork(thumbnailList(v)), isExplicit(v)))
	}
	return tracks
}

// Lyrics returns the plain lyrics of a track and where they come from,
// such as "Source: Musixmatch", or ErrNotFound when it has none.
func (c *Client) Lyrics(ctx context.Context, videoID string) (text, source string, err error) {
	n, err := c.call(ctx, "next", map[string]any{"videoId": videoID, "isAudioOnly": true}, nil)
	if err != nil {
		return "", "", err
	}
	browseID := lyricsBrowseID(n)
	if browseID == "" {
		return "", "", ErrNotFound
	}
	page, err := c.browse(ctx, browseID)
	if err != nil {
		return "", "", err
	}
	text, source = parseLyrics(page)
	if text == "" {
		return "", "", ErrNotFound
	}
	return text, source, nil
}

// lyricsBrowseID returns the browse ID of a track's lyrics tab, if it has
// lyrics.
func lyricsBrowseID(n node) string {
	for _, tab := range watchNextTabs(n) {
		if id := tab.get("tabRenderer", "endpoint", "browseEndpoint", "browseId").str(); strings.HasPrefix(id, "MPLYt") {
			return id
		}
	}
	return ""
}

func parseLyrics(n node) (text, source string) {
	shelf := n.get("contents", "sectionListRenderer", "contents", 0, "musicDescriptionShelfRenderer")
	return strings.TrimSpace(shelf.get("description").text()), strings.TrimSpace(shelf.get("footer").text())
}
