package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// The captions of a video, as YouTube's player API lists them to the
// visionOS client the direct route asks as: their addresses, at
// www.youtube.com/api/timedtext, answer without the proof-of-origin token
// those of the website's own client need, which MixMusic doesn't make.

// ErrNoCaptions is returned for a video YouTube tells of no captions of,
// or gives the direct route nothing of, as one made for children.
var ErrNoCaptions = errors.New("stream: YouTube tells of no captions of this video")

// captionList is the captions part of the player API's answer.
type captionList struct {
	Renderer struct {
		Tracks []struct {
			BaseURL      string `json:"baseUrl"`
			LanguageCode string `json:"languageCode"`
			// Kind is "asr" for captions YouTube made from speech.
			Kind string `json:"kind"`
			Name struct {
				SimpleText string `json:"simpleText"`
				Runs       []struct {
					Text string `json:"text"`
				} `json:"runs"`
			} `json:"name"`
		} `json:"captionTracks"`
		// AudioTracks tell, by the video's audio, the captions YouTube
		// shows of it: DefaultAudioTrackIndex's.
		AudioTracks []struct {
			DefaultCaptionTrackIndex *int `json:"defaultCaptionTrackIndex"`
		} `json:"audioTracks"`
		DefaultAudioTrackIndex int `json:"defaultAudioTrackIndex"`
	} `json:"playerCaptionsTracklistRenderer"`
}

// CaptionTrack is a video's captions in a language.
type CaptionTrack struct {
	// Language is their language, as YouTube tells it: "en", "pt-BR".
	Language string
	// Name names them, as "English (auto-generated)".
	Name string
	// Speech tells captions YouTube made from what the video says.
	Speech bool
	// Default tells the captions YouTube shows of the video's audio.
	Default bool
	// url is where they are.
	url string
}

// Captions are the captions of a video, and its length.
type Captions struct {
	Tracks []CaptionTrack
	Length time.Duration
}

// Caption is a caption of a video: its text, and the times it shows from
// and to, in milliseconds.
type Caption struct {
	StartMs, EndMs int
	Text           string
}

// Captions returns the captions of a video, as YouTube's player API lists
// them, signed out; ErrNoCaptions when it lists none.
func (d *Direct) Captions(ctx context.Context, videoID string) (Captions, error) {
	ans, err := d.answer(ctx, videoID)
	var refused playabilityError
	if errors.As(err, &refused) {
		return Captions{}, fmt.Errorf("%w: %w", ErrNoCaptions, err)
	}
	if err != nil {
		return Captions{}, err
	}
	r := ans.Captions.Renderer
	def := -1
	if i := r.DefaultAudioTrackIndex; i >= 0 && i < len(r.AudioTracks) && r.AudioTracks[i].DefaultCaptionTrackIndex != nil {
		def = *r.AudioTracks[i].DefaultCaptionTrackIndex
	}
	caps := Captions{Length: time.Duration(float64(ans.VideoDetails.LengthSeconds) * float64(time.Second))}
	for i, t := range r.Tracks {
		u, err := url.Parse(t.BaseURL)
		if err != nil || u.Host == "" || t.LanguageCode == "" {
			continue
		}
		name := t.Name.SimpleText
		for _, run := range t.Name.Runs {
			name += run.Text
		}
		caps.Tracks = append(caps.Tracks, CaptionTrack{Language: t.LanguageCode, Name: name, Speech: t.Kind == "asr", Default: i == def, url: t.BaseURL})
	}
	if len(caps.Tracks) == 0 {
		return Captions{}, ErrNoCaptions
	}
	return caps, nil
}

// json3 is the format of captions YouTube's players read, as timedtext
// answers it for fmt=json3: events, each a caption from its start for its
// duration, of segments of text. Those YouTube made from speech have a
// segment a word, and events that only add a line break.
type json3 struct {
	Events []struct {
		StartMs    int `json:"tStartMs"`
		DurationMs int `json:"dDurationMs"`
		Segs       []struct {
			UTF8 string `json:"utf8"`
		} `json:"segs"`
	} `json:"events"`
}

// CaptionText returns the captions of a track, in their order of time,
// each on one line.
func (d *Direct) CaptionText(ctx context.Context, t CaptionTrack) ([]Caption, error) {
	u, err := url.Parse(t.url)
	if err != nil {
		return nil, fmt.Errorf("%w: captions at an address that doesn't parse", errBadAnswer)
	}
	q := u.Query()
	q.Set("fmt", "json3")
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", visionAgent)
	res, err := d.HTTP.Do(req)
	if err != nil {
		// The error quotes the address, which carries the user's IP
		// address.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, fmt.Errorf("stream: asking for captions: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stream: captions answered HTTP %d", res.StatusCode)
	}
	var ans json3
	if err := json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(&ans); err != nil {
		return nil, fmt.Errorf("%w: captions: %v", errBadAnswer, err)
	}
	caps := []Caption{}
	for _, e := range ans.Events {
		var b strings.Builder
		for _, s := range e.Segs {
			b.WriteString(s.UTF8)
		}
		text := strings.Join(strings.Fields(b.String()), " ")
		if text == "" {
			continue
		}
		caps = append(caps, Caption{StartMs: e.StartMs, EndMs: e.StartMs + max(e.DurationMs, 0), Text: text})
	}
	return caps, nil
}
