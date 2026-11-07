package youtube

import (
	"context"
	"errors"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
)

// librarySongs is the browse ID of the library's songs: those the user
// saved there by a song's "Save to library", which liking leaves apart.
const librarySongs = "FEmusic_liked_videos"

// errNoSaveToggle is returned for a song whose menu offers no way to save
// it to the library, as for a visitor, or for a video YouTube Music keeps
// out of it.
var errNoSaveToggle = errors.New("YouTube Music offers no way to save this track to your library")

// LibrarySongs returns the songs the user saved to their library on
// YouTube Music, the last saved first.
func (c *Client) LibrarySongs(ctx context.Context) ([]media.Track, error) {
	items, err := c.libraryItems(ctx, librarySongs)
	tracks := []media.Track{}
	for _, it := range items {
		if it.Kind == media.ItemTrack {
			tracks = append(tracks, *it.Track)
		}
	}
	return tracks, err
}

// SongSaved reports whether a song, by its video ID, is in the user's
// library, as its menu tells.
func (c *Client) SongSaved(ctx context.Context, videoID string) (bool, error) {
	t, err := c.libraryToggle(ctx, videoID)
	return t.saved, err
}

// SetSongSaved saves a song, by its video ID, to the user's library, or
// takes it out, without liking it, with the token its menu holds for
// that. A song already as asked stays as it is.
func (c *Client) SetSongSaved(ctx context.Context, videoID string, saved bool) error {
	t, err := c.libraryToggle(ctx, videoID)
	if err != nil || t.saved == saved {
		return err
	}
	token := t.add
	if !saved {
		token = t.remove
	}
	n, err := c.call(ctx, "feedback", map[string]any{"feedbackTokens": []string{token}}, nil)
	if err != nil {
		return err
	}
	if done := n.get("feedbackResponses", 0, "isProcessed"); done.exists() && !done.boolean() {
		return errors.New("YouTube Music didn't take the change to your library")
	}
	return nil
}

// libraryToggle reads the save toggle of a song's menu, as the queue of its
// "next" holds it.
func (c *Client) libraryToggle(ctx context.Context, videoID string) (songToggle, error) {
	if err := c.session(); err != nil {
		return songToggle{}, err
	}
	n, err := c.call(ctx, "next", map[string]any{"videoId": videoID, "isAudioOnly": true}, nil)
	if err != nil {
		return songToggle{}, err
	}
	if t, ok := queuedToggle(n, videoID); ok {
		return t, nil
	}
	return songToggle{}, errNoSaveToggle
}

// queuedToggle returns the save toggle of the entry of videoID in the
// queue of a "next" response.
func queuedToggle(n node, videoID string) (songToggle, bool) {
	tabs := watchNextTabs(n)
	if len(tabs) == 0 {
		return songToggle{}, false
	}
	panel := tabs[0].get("tabRenderer", "content", "musicQueueRenderer", "content", "playlistPanelRenderer")
	for _, item := range panel.get("contents").list() {
		v := item.get("playlistPanelVideoRenderer")
		if !v.exists() {
			v = item.get("playlistPanelVideoWrapperRenderer", "primaryRenderer", "playlistPanelVideoRenderer")
		}
		if v.get("videoId").str() == videoID {
			return songSaveToggle(v.get("menu"))
		}
	}
	return songToggle{}, false
}

// songToggle is what a song's menu holds to save it to the library and to
// take it out: a feedback token for each, and whether it is saved.
type songToggle struct {
	add, remove string
	saved       bool
}

// songSaveToggle reads the save toggle of a song's menu, which YouTube
// Music shows to a signed-in user only. Its default side is what a click
// does: for a song saved, its icon says so, and it takes the song out.
func songSaveToggle(menu node) (songToggle, bool) {
	for _, it := range menu.get("menuRenderer", "items").list() {
		t := it.get("toggleMenuServiceItemRenderer")
		first := t.get("defaultServiceEndpoint", "feedbackEndpoint", "feedbackToken").str()
		then := t.get("toggledServiceEndpoint", "feedbackEndpoint", "feedbackToken").str()
		icon := t.get("defaultIcon", "iconType").str()
		if first == "" || then == "" || !strings.Contains(icon, "LIBRARY") && !strings.HasPrefix(icon, "BOOKMARK") {
			continue
		}
		if icon == "LIBRARY_SAVED" || icon == "LIBRARY_REMOVE" || icon == "BOOKMARK" {
			return songToggle{add: then, remove: first, saved: true}, true
		}
		return songToggle{add: first, remove: then}, true
	}
	return songToggle{}, false
}
