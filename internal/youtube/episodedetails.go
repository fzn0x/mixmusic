package youtube

import (
	"context"
	"net/url"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
)

// detailsPrefix browses an episode's details by its video ID: the page of
// the Details tab YouTube Music's player has for an episode where a song
// has Lyrics, of the type MUSIC_PAGE_TYPE_NON_MUSIC_AUDIO_DETAILS. The
// episode's own page, "MPED", has the same description under its head;
// the player's answer has none.
const detailsPrefix = "MPPE"

// EpisodeDetails returns the details of the episode of videoID: its show,
// views, day and length, and its description, whose links and times
// YouTube Music marks.
func (c *Client) EpisodeDetails(ctx context.Context, videoID string) (media.EpisodeDetails, error) {
	n, err := c.browse(ctx, detailsPrefix+videoID)
	if err != nil {
		return media.EpisodeDetails{}, err
	}
	return parseEpisodeDetails(n, videoID)
}

// parseEpisodeDetails reads the details of the episode of videoID: a
// musicDescriptionShelfRenderer whose strapline is "941K views • Jan 31,
// 2026 • 4 hr 25 min", whose subheader links the show, and whose
// description is runs.
func parseEpisodeDetails(n node, videoID string) (media.EpisodeDetails, error) {
	var shelf node
	for _, s := range n.get("contents", "sectionListRenderer", "contents").list() {
		if shelf = s.get("musicDescriptionShelfRenderer"); shelf.exists() {
			break
		}
	}
	if !shelf.exists() {
		return media.EpisodeDetails{}, ErrNotFound
	}
	line := parseEpisodeRuns(append(shelf.get("strapline").runs(), shelf.get("subheader").runs()...))
	return media.EpisodeDetails{
		Show:        line.show,
		Released:    line.released,
		Plays:       line.plays,
		DurationMs:  line.duration,
		Description: media.Lines(descriptionRuns(shelf.get("description").runs(), videoID)),
	}, nil
}

// descriptionRuns reads the runs of an episode's description: a link to a
// web page opens it, one to a time of the episode itself seeks there, and
// one to another video opens that video. Links to YouTube's own pages, as
// a hashtag's, are plain text.
func descriptionRuns(runs []node, videoID string) []media.TextRun {
	out := make([]media.TextRun, 0, len(runs))
	for _, r := range runs {
		run := media.TextRun{Text: r.get("text").str()}
		if run.Text == "" {
			continue
		}
		nav := r.get("navigationEndpoint")
		if u := nav.get("urlEndpoint", "url").str(); u != "" {
			run.Link = descriptionLink(u)
		} else if w := nav.get("watchEndpoint"); w.exists() {
			id, at := w.get("videoId").str(), w.get("startTimeSeconds")
			switch {
			case id == videoID && at.exists():
				run.Seek, run.AtMs = true, at.number()*1000
			case id != "" && id != videoID:
				run.Link = "https://music.youtube.com/watch?v=" + url.QueryEscape(id)
			}
		}
		out = append(out, run)
	}
	return out
}

// descriptionLink returns the web page a link of a description opens, or
// "": YouTube wraps the addresses of other sites in its redirect, which
// gives them as q.
func descriptionLink(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Path == "/redirect" &&
		(u.Hostname() == "youtube.com" || strings.HasSuffix(u.Hostname(), ".youtube.com")) {
		return media.WebLink(u.Query().Get("q"))
	}
	return media.WebLink(raw)
}
