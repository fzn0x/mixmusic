// Package sponsorblock asks SponsorBlock, the open, crowdsourced database
// of the parts of YouTube videos, which parts of a video it marks: sponsor
// reads, intros, the parts of music videos without music and the like, and
// its highlight (#548). MixMusic only reads it: it never sends parts or
// votes.
//
// A lookup never names the video: it sends the first 4 hex characters of
// the SHA-256 of the video's ID, and SponsorBlock answers with the parts of
// every video whose hash starts so, of which the client keeps the video's.
package sponsorblock

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

// ServerURL is SponsorBlock's own server, which a mirror may stand in for.
const ServerURL = "https://sponsor.ajay.app"

// Credit is the line SponsorBlock's attribution template asks of the apps
// that use its data, which is under CC BY-NC-SA 4.0.
const Credit = "Uses SponsorBlock data licensed under CC BY-NC-SA 4.0 from https://sponsor.ajay.app/."

// License is the notice of SponsorBlock's data, for the licenses MixMusic
// lists.
const License = `The parts of videos MixMusic skips, and their highlights, come from
SponsorBlock (https://sponsor.ajay.app/), the crowdsourced database of
Ajay Ramachandran and its contributors.

Uses SponsorBlock data licensed under CC BY-NC-SA 4.0 from
https://sponsor.ajay.app/.

SponsorBlock's API and database are under the Creative Commons
Attribution-NonCommercial-ShareAlike 4.0 International license:
https://creativecommons.org/licenses/by-nc-sa/4.0/

MixMusic doesn't change the data it reads: it only leaves out parts that
don't fit the audio that plays, and joins parts that overlap.
`

// These bound a lookup, as the settings promise; tests shorten them.
var (
	// timeout bounds a lookup, its answer read.
	timeout = 8 * time.Second
	// maxAnswer is the most of an answer read: the parts of all the videos
	// of a hash prefix take a few dozen KB.
	maxAnswer int64 = 2 << 20
)

// prefixLength is how many hex characters of a video ID's hash a lookup
// sends: 4 of them, which some 65,000 videos share, among which
// SponsorBlock can't tell the one asked for.
const prefixLength = 4

// Kind is a kind of part SponsorBlock marks, by its name for it, its
// category.
type Kind string

const (
	NonMusic    Kind = "music_offtopic"
	Sponsor     Kind = "sponsor"
	SelfPromo   Kind = "selfpromo"
	Interaction Kind = "interaction"
	Intro       Kind = "intro"
	Outro       Kind = "outro"
	Preview     Kind = "preview"
	Hook        Kind = "hook"
	Filler      Kind = "filler"
	// Highlight is the moment a video is best seen from, a point rather
	// than a part: the player jumps to it when asked, never by itself.
	Highlight Kind = "poi_highlight"
)

// Kinds are the kinds of parts MixMusic skips, in the order the settings
// list them.
var Kinds = []Kind{NonMusic, Sponsor, SelfPromo, Interaction, Intro, Outro, Preview, Hook, Filler}

// DefaultKinds are the kinds skipped at first: Filler never is, as it
// skips a lot.
var DefaultKinds = []Kind{NonMusic, Sponsor, SelfPromo}

// kindWords are a kind's name in the settings, and how a notice names a
// part of it skipped: "Skipped the intro".
var kindWords = map[Kind][2]string{
	NonMusic:    {"Non-music parts", "a part without music"},
	Sponsor:     {"Sponsor", "a sponsor read"},
	SelfPromo:   {"Self-promotion", "self-promotion"},
	Interaction: {"Reminders to like or subscribe", "a reminder to like or subscribe"},
	Intro:       {"Intro", "the intro"},
	Outro:       {"Outro", "the outro"},
	Preview:     {"Preview or recap", "a preview or recap"},
	Hook:        {"Hook or greeting", "a hook or greeting"},
	Filler:      {"Filler", "filler"},
	Highlight:   {"Highlight", "the highlight"},
}

// Name is the kind's name, as the settings and the seek bar show it.
func (k Kind) Name() string {
	if w, ok := kindWords[k]; ok {
		return w[0]
	}
	return string(k)
}

// Part is how a notice names a part of the kind: "the intro".
func (k Kind) Part() string {
	if w, ok := kindWords[k]; ok {
		return w[1]
	}
	return "a part"
}

// Known reports whether k is a kind MixMusic skips.
func (k Kind) Known() bool { return slices.Contains(Kinds, k) }

// Segment is a part of a video SponsorBlock marks, or its highlight, whose
// From and To are the same.
type Segment struct {
	Kind     Kind
	From, To time.Duration
	// Of is the length of the video the part was marked on, or 0 when
	// SponsorBlock doesn't know it: a part marked on a video since cut
	// falls elsewhere in it.
	Of time.Duration
}

// Client asks a SponsorBlock server.
type Client struct {
	HTTP      *http.Client
	UserAgent string
}

// New returns a client introducing itself as userAgent, through the
// system's proxy, as MixMusic's other requests go.
func New(userAgent string) *Client {
	return &Client{HTTP: &http.Client{Timeout: timeout}, UserAgent: userAgent}
}

// answer is what SponsorBlock answers for a hash prefix: the videos of it
// with parts, each with its parts.
type answer []struct {
	VideoID  string `json:"videoID"`
	Segments []struct {
		Category      Kind      `json:"category"`
		ActionType    string    `json:"actionType"`
		Segment       []float64 `json:"segment"`
		VideoDuration float64   `json:"videoDuration"`
	} `json:"segments"`
}

// Prefix is the part of a video ID's hash a lookup sends.
func Prefix(videoID string) string {
	sum := sha256.Sum256([]byte(videoID))
	return hex.EncodeToString(sum[:])[:prefixLength]
}

// Segments asks server, SponsorBlock's own when "", which parts of the
// video videoID it marks, of the kinds MixMusic skips, and its highlight,
// in their order: none for a video with none, which isn't an error. The
// answer holds the parts of every video of the hash prefix: only the
// video's are kept.
func (c *Client) Segments(ctx context.Context, server, videoID string) ([]Segment, error) {
	if server == "" {
		server = ServerURL
	}
	q := url.Values{}
	kinds := append(slices.Clone(Kinds), Highlight)
	names, _ := json.Marshal(kinds)
	q.Set("categories", string(names))
	q.Set("actionTypes", `["skip","poi"]`)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	address := strings.TrimSuffix(server, "/") + "/api/skipSegments/" + Prefix(videoID) + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		// No video of the prefix has parts.
		return nil, nil
	default:
		return nil, fmt.Errorf("SponsorBlock answered HTTP %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxAnswer+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxAnswer {
		return nil, errors.New("SponsorBlock's answer is too long")
	}
	return parse(body, videoID)
}

// parse returns the parts of videoID that an answer of its hash prefix
// holds, in their order: those of the kinds MixMusic skips, and its
// highlight, that make sense.
func parse(body []byte, videoID string) ([]Segment, error) {
	var ans answer
	if err := json.Unmarshal(body, &ans); err != nil {
		return nil, fmt.Errorf("reading SponsorBlock's answer: %w", err)
	}
	var out []Segment
	for _, v := range ans {
		if v.VideoID != videoID {
			continue
		}
		for _, s := range v.Segments {
			if len(s.Segment) != 2 {
				continue
			}
			seg := Segment{Kind: s.Category, From: seconds(s.Segment[0]), To: seconds(s.Segment[1]), Of: seconds(s.VideoDuration)}
			switch {
			case seg.From < 0 || seg.Of < 0:
			case s.ActionType == "poi" && seg.Kind == Highlight:
				seg.To = seg.From
				out = append(out, seg)
			case s.ActionType == "skip" && seg.Kind.Known() && seg.To > seg.From:
				out = append(out, seg)
			}
		}
	}
	slices.SortStableFunc(out, func(a, b Segment) int { return cmp.Compare(a.From, b.From) })
	return out, nil
}

// seconds is a time SponsorBlock tells, in seconds, to the millisecond: -1
// for one that isn't a number.
func seconds(s float64) time.Duration {
	if math.IsNaN(s) || math.IsInf(s, 0) || s > 1e7 {
		return -1
	}
	return time.Duration(math.Round(s*1000)) * time.Millisecond
}

// Server returns the address of a server as the settings take it: an
// https address of a host, without its last slash, else "".
func Server(address string) string {
	address = strings.TrimSpace(address)
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	return strings.TrimSuffix(u.String(), "/")
}
