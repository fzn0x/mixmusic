package youtube

// Whether YouTube labels a video made with AI. YouTube's website tells it
// in the video's description, under "How this was made", once the
// uploader said that its sounds or visuals were made or changed with AI.
// Nothing of YouTube Music carries the label, nor does the stream: the
// website's own answer is asked.

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// labelMask keeps, of the website's answer for a video, the video it is of
// and what its description says of how it was made: about 1 KB, where the
// whole answer is about 370.
const labelMask = "currentVideoEndpoint.watchEndpoint.videoId," +
	"engagementPanels.engagementPanelSectionListRenderer.content.structuredDescriptionContentRenderer.items." +
	"howThisWasMadeSectionViewModel(bodyHeader,bodyText)"

// aiArticle is the path of the help article the label links to, in every
// language. The other notes of how a video was made, as Auto-dubbed, link
// to other articles.
const aiArticle = "/youtube/answer/15447836"

// aiHeaders are the label's headers in English, today's and an older
// one's.
var aiHeaders = []string{"Made with AI", "Altered or synthetic content"}

// MadeWithAI reports whether YouTube labels a video made with AI. It asks
// YouTube's website as a visitor, signed out, as streams are found: the
// label is public. It returns ErrNotFound for a video YouTube has none of,
// as one removed or private.
func (c *Client) MadeWithAI(ctx context.Context, videoID string) (bool, error) {
	body := map[string]any{"videoId": videoID}
	n, err := c.callWith(ctx, webClient, "next", body, nil, callOptions{
		header:    http.Header{"X-Goog-FieldMask": {labelMask}},
		anonymous: true,
	})
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusBadRequest {
		// YouTube refused the mask: the whole answer tells as well.
		n, err = c.callWith(ctx, webClient, "next", body, nil, callOptions{anonymous: true})
	}
	if err != nil {
		return false, err
	}
	return parseMadeWithAI(n, videoID)
}

// parseMadeWithAI reads whether the website's answer for videoID labels it
// made with AI, the answer masked or whole. An answer of another video, or
// of none, is ErrNotFound: YouTube answers so for a video it doesn't have.
func parseMadeWithAI(n node, videoID string) (bool, error) {
	if n.get("currentVideoEndpoint", "watchEndpoint", "videoId").str() != videoID {
		return false, ErrNotFound
	}
	for _, panel := range n.get("engagementPanels").list() {
		items := panel.get("engagementPanelSectionListRenderer", "content", "structuredDescriptionContentRenderer", "items")
		for _, it := range items.list() {
			if made := it.get("howThisWasMadeSectionViewModel"); made.exists() && aiLabel(made) {
				return true, nil
			}
		}
	}
	return false, nil
}

// aiLabel reports whether a note of how a video was made is the label of
// AI: it links to the label's article, or, should the link change, it has
// the label's header.
func aiLabel(made node) bool {
	for _, run := range made.get("bodyText", "commandRuns").list() {
		link := run.get("onTap", "innertubeCommand", "urlEndpoint", "url").str()
		if u, err := url.Parse(link); err == nil && link != "" && u.Path == aiArticle {
			return true
		}
	}
	return slices.Contains(aiHeaders, strings.TrimSpace(made.get("bodyHeader", "content").str()))
}
