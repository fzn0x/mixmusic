package listenbrainz

import (
	"context"
	"net/http"
	"net/url"
)

// Recording names a recording as ListenBrainz knows it: by the MessyBrainz
// ID that a listen of it got, or by its MusicBrainz ID.
type Recording struct {
	MSID string `json:"recording_msid,omitempty"`
	MBID string `json:"recording_mbid,omitempty"`
}

// Love loves the recording r for the user of token, or takes the love
// back: recording feedback of score 1, or 0.
func (c *Client) Love(ctx context.Context, token string, r Recording, loved bool) error {
	body := struct {
		Recording
		Score int `json:"score"`
	}{Recording: r}
	if loved {
		body.Score = 1
	}
	return c.call(ctx, http.MethodPost, "feedback/recording-feedback", token, nil, body, nil)
}

// Lookup returns the MusicBrainz ID of the recording that t is, as the
// server's metadata lookup finds it by its artist, title and album, or ""
// when it finds none.
func (c *Client) Lookup(ctx context.Context, token string, t Track) (string, error) {
	q := url.Values{"artist_name": {t.Artist}, "recording_name": {t.Title}}
	if t.Album != "" {
		q.Set("release_name", t.Album)
	}
	var answer struct {
		RecordingMBID string `json:"recording_mbid"`
	}
	if err := c.call(ctx, http.MethodGet, "metadata/lookup/", token, q, nil, &answer); err != nil {
		return "", err
	}
	return answer.RecordingMBID, nil
}
