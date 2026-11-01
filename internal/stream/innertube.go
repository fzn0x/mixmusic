package stream

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The client the direct route asks YouTube's player API as: YouTube's app
// for visionOS. YouTube answers it, signed out, with Opus streams that play
// at once: their URLs need no challenge solved, and its answers carry no
// pre-roll ad, for which YouTube holds the streams of other clients back.
// When YouTube changes what it asks of this client, these change with it.
const (
	visionName    = "VISIONOS"
	visionVersion = "1.02"
	// visionNumber is visionName's number, for the headers.
	visionNumber = "101"
	visionAgent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 15_7_3) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Safari/605.1.15"
	visionMake   = "Apple"
	visionModel  = "RealityDevice17,1"
	visionOS     = "visionOS"
	visionOSVer  = "26.5.23O471"
)

// innertubeURL is the origin of YouTube's player API.
const innertubeURL = "https://www.youtube.com"

const (
	// visitorTTL is how long the direct route goes on with one visitor ID.
	visitorTTL = 6 * time.Hour
	// directTimeout bounds the direct route, so that an answer that doesn't
	// come leaves yt-dlp the time to find the stream.
	directTimeout = 4 * time.Second
	// After pauseAfter refusals in a row, the direct route rests for
	// pauseFor: yt-dlp alone finds streams meanwhile.
	pauseAfter = 3
	pauseFor   = 30 * time.Minute
)

var (
	// errDirectPaused is returned while the direct route rests.
	errDirectPaused = errors.New("stream: YouTube's player API refused the last streams; resting")
	// errRefused is returned when the media server refuses a stream URL
	// YouTube's player API just gave.
	errRefused = errors.New("stream: YouTube's media server refused the stream")
	// errNoStream is returned for a video with no Opus or AAC-LC stream
	// the engine plays, and errNoDirectURL for one whose streams of them
	// all need what only yt-dlp does: a signature or a challenge solved.
	errNoStream    = errors.New("stream: YouTube's player API gave no Opus or AAC-LC stream")
	errNoDirectURL = errors.New("stream: YouTube's player API gave no Opus or AAC-LC stream that plays as given")
	// errBadAnswer is returned for an answer the direct route can't read.
	errBadAnswer = errors.New("stream: unexpected answer of YouTube's player API")
	// ErrNoLoudness is returned when YouTube tells no loudness of a video.
	ErrNoLoudness = errors.New("stream: YouTube tells no loudness of this video")
)

// apiStatusError is returned when YouTube's player API answers with an
// HTTP status other than 200.
type apiStatusError int

func (e apiStatusError) Error() string {
	return fmt.Sprintf("stream: YouTube's player API answered HTTP %d", int(e))
}

// playabilityError is why YouTube's player API gave no streams of a video,
// as it said.
type playabilityError struct{ Status, Reason string }

func (e playabilityError) Error() string {
	if e.Reason == "" {
		return "stream: YouTube's player API answered " + e.Status
	}
	return "stream: YouTube's player API answered " + e.Status + ": " + e.Reason
}

// botCheck tells whether YouTube asks to confirm this is not a bot, which it
// asks of a visitor ID it does not trust.
func (e playabilityError) botCheck() bool {
	return e.Status == "LOGIN_REQUIRED" && strings.Contains(strings.ToLower(e.Reason), "not a bot")
}

// refusal tells whether err says that YouTube no longer gives the direct
// route streams, rather than that a video has none for it, or that the
// network failed: refusals in a row pause the route.
func refusal(err error) bool {
	var status apiStatusError
	var playability playabilityError
	return errors.Is(err, errRefused) || errors.Is(err, errNoDirectURL) || errors.Is(err, errBadAnswer) ||
		errors.As(err, &status) || errors.As(err, &playability) && playability.botCheck()
}

// Direct finds the audio streams of videos by asking YouTube's player API
// itself, as YouTube's visionOS app, signed out: in a fraction of the time
// yt-dlp takes, and with streams YouTube serves at once. YouTube gives it
// no streams of some videos, such as those made for children, which yt-dlp
// still finds. Its methods are safe for concurrent use.
type Direct struct {
	// HTTP asks the API, and the media server whether it serves a stream.
	HTTP *http.Client
	// BaseURL is the API's origin, replaced in tests.
	BaseURL string
	// now tells the time; tests replace it.
	now func() time.Time

	mu sync.Mutex
	// visitor is the visitor ID YouTube gave the route, since visitorAt.
	// It lives in memory only, and is never the signed-in session's, so
	// that what the route asks can't be tied to the account, nor to other
	// runs.
	visitor   string
	visitorAt time.Time
	asking    *visitorCall // the request of a visitor ID under way, or nil
	// refused counts refusals in a row; the route rests until pausedUntil.
	refused     int
	pausedUntil time.Time
}

// visitorCall is a request of a visitor ID, shared by every stream that
// waits for it.
type visitorCall struct {
	done chan struct{}
	id   string
	err  error
}

// NewDirect returns the direct route to YouTube's player API.
func NewDirect() *Direct {
	// The default transport, which the proxy's client uses too, keeps the
	// connection the check opened to the media server for its requests.
	return &Direct{HTTP: &http.Client{Timeout: 8 * time.Second}, BaseURL: innertubeURL, now: time.Now}
}

// Stream returns the audio stream of a video in quality q, once the media
// server answered for it.
func (d *Direct) Stream(ctx context.Context, videoID string, q Quality) (Stream, error) {
	d.mu.Lock()
	resting := d.now().Before(d.pausedUntil)
	d.mu.Unlock()
	if resting {
		return Stream{}, errDirectPaused
	}
	s, err := d.stream(ctx, videoID, q)
	d.count(err)
	return s, err
}

// count takes in how finding a stream went, and pauses the route after
// pauseAfter refusals in a row.
func (d *Direct) count(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch {
	case err == nil:
		d.refused = 0
	case refusal(err):
		d.refused++
		if d.refused >= pauseAfter {
			d.refused = 0
			d.pausedUntil = d.now().Add(pauseFor)
			log.Printf("mixmusic: YouTube's player API refused %d streams in a row; yt-dlp alone finds them for %d minutes",
				pauseAfter, int(pauseFor/time.Minute))
		}
	}
}

func (d *Direct) stream(ctx context.Context, videoID string, q Quality) (Stream, error) {
	ans, err := d.answer(ctx, videoID)
	if err != nil {
		return Stream{}, err
	}
	length := time.Duration(ans.VideoDetails.LengthSeconds) * time.Second
	f, err := pickAudio(ans.StreamingData.AdaptiveFormats, q, length)
	if err != nil {
		return Stream{}, err
	}
	st := f.stream()
	st.Loudness, st.Measured = ans.loudness(&f)
	st.Videos = directVideos(ans.StreamingData.AdaptiveFormats, length)
	if err := d.check(ctx, &st); err != nil {
		return Stream{}, err
	}
	return st, nil
}

// Loudness asks YouTube how loud a video is: the integrated loudness of its
// audio, in LKFS, as YouTube measured it. Streams of the direct route carry
// it already; this is for those yt-dlp found. It returns ErrNoLoudness when
// YouTube gave no measure, as of videos it gives the route no streams of.
func (d *Direct) Loudness(ctx context.Context, videoID string) (float64, error) {
	ans, err := d.answer(ctx, videoID)
	var refused playabilityError
	if errors.As(err, &refused) {
		return 0, ErrNoLoudness
	}
	if err != nil {
		return 0, err
	}
	if v, ok := ans.loudness(nil); ok {
		return v, nil
	}
	return 0, ErrNoLoudness
}

// answer returns the answer of YouTube's player API about a video, when it
// gave the video's streams.
func (d *Direct) answer(ctx context.Context, videoID string) (playerAnswer, error) {
	visitor, err := d.visitorID(ctx)
	if err != nil && ctx.Err() != nil {
		return playerAnswer{}, err
	}
	// Without a visitor ID it trusts, YouTube asks to confirm this is not
	// a bot. Its answer carries a new one, which passes; else the route
	// asks for one again.
	ans, err := d.player(ctx, videoID, visitor)
	if err == nil && ans.status().botCheck() {
		d.dropVisitor(visitor)
		fresh := ans.ResponseContext.VisitorData
		if fresh == "" || fresh == visitor {
			fresh, err = d.visitorID(ctx)
		}
		visitor = fresh
		if err == nil {
			ans, err = d.player(ctx, videoID, visitor)
		}
	}
	if err != nil {
		return playerAnswer{}, err
	}
	s := ans.status()
	if !s.botCheck() {
		d.keepVisitor(cmp.Or(ans.ResponseContext.VisitorData, visitor))
	}
	if s.Status != "OK" {
		return playerAnswer{}, s
	}
	if ans.VideoDetails.VideoID != videoID {
		return playerAnswer{}, fmt.Errorf("%w: the streams of another video", errBadAnswer)
	}
	return ans, nil
}

// visitorID returns the route's visitor ID, asking YouTube for one when it
// has none, or an old one. Streams found at once share one request.
func (d *Direct) visitorID(ctx context.Context) (string, error) {
	d.mu.Lock()
	if d.visitor != "" && d.now().Sub(d.visitorAt) < visitorTTL {
		v := d.visitor
		d.mu.Unlock()
		return v, nil
	}
	call := d.asking
	if call == nil {
		call = &visitorCall{done: make(chan struct{})}
		d.asking = call
		go func() {
			// Not the caller's context: other streams may wait for it.
			ctx, cancel := context.WithTimeout(context.Background(), directTimeout)
			defer cancel()
			id, err := d.askVisitor(ctx)
			d.mu.Lock()
			call.id, call.err = id, err
			if err == nil {
				d.visitor, d.visitorAt = id, d.now()
			}
			d.asking = nil
			d.mu.Unlock()
			close(call.done)
		}()
	}
	d.mu.Unlock()
	select {
	case <-call.done:
		return call.id, call.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// keepVisitor keeps the visitor ID of an answer YouTube played.
func (d *Direct) keepVisitor(id string) {
	if id == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.visitor != id {
		d.visitor, d.visitorAt = id, d.now()
	}
}

// dropVisitor forgets a visitor ID YouTube no longer trusts.
func (d *Direct) dropVisitor(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.visitor == id {
		d.visitor = ""
	}
}

// askVisitor asks YouTube for a visitor ID.
func (d *Direct) askVisitor(ctx context.Context) (string, error) {
	var ans struct {
		ResponseContext struct {
			VisitorData string `json:"visitorData"`
		} `json:"responseContext"`
	}
	if err := d.call(ctx, "visitor_id", nil, "", &ans); err != nil {
		return "", err
	}
	if ans.ResponseContext.VisitorData == "" {
		return "", fmt.Errorf("%w: no visitor ID", errBadAnswer)
	}
	return ans.ResponseContext.VisitorData, nil
}

// player asks YouTube's player API for the streams of a video.
func (d *Direct) player(ctx context.Context, videoID, visitor string) (playerAnswer, error) {
	var ans playerAnswer
	err := d.call(ctx, "player", map[string]any{
		"videoId": videoID,
		"playbackContext": map[string]any{
			"contentPlaybackContext": map[string]any{"html5Preference": "HTML5_PREF_WANTS"},
		},
		"contentCheckOk": true,
		"racyCheckOk":    true,
	}, visitor, &ans)
	return ans, err
}

// call posts body, with the context of the visionOS client, to an endpoint
// of YouTube's API, and reads its answer into out. It sends no cookie and
// no account: the route asks signed out.
func (d *Direct) call(ctx context.Context, endpoint string, body map[string]any, visitor string, out any) error {
	client := map[string]any{
		"clientName":       visionName,
		"clientVersion":    visionVersion,
		"deviceMake":       visionMake,
		"deviceModel":      visionModel,
		"osName":           visionOS,
		"osVersion":        visionOSVer,
		"userAgent":        visionAgent,
		"hl":               "en",
		"timeZone":         "UTC",
		"utcOffsetMinutes": 0,
	}
	if visitor != "" {
		client["visitorData"] = visitor
	}
	payload := map[string]any{"context": map[string]any{"client": client}}
	for k, v := range body {
		payload[k] = v
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	u := strings.TrimRight(d.BaseURL, "/") + "/youtubei/v1/" + endpoint + "?prettyPrint=false"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", visionAgent)
	req.Header.Set("Origin", innertubeURL)
	req.Header.Set("X-YouTube-Client-Name", visionNumber)
	req.Header.Set("X-YouTube-Client-Version", visionVersion)
	if visitor != "" {
		req.Header.Set("X-Goog-Visitor-Id", visitor)
	}
	res, err := d.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("stream: asking YouTube's player API: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return apiStatusError(res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return fmt.Errorf("stream: reading YouTube's player API: %w", err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: %v", errBadAnswer, err)
	}
	return nil
}

// check asks the media server for the first byte of a stream, so that a
// URL it refuses fails here, where yt-dlp can still find the stream, and
// not in the player. It reads the stream's size when its format didn't
// tell it.
func (d *Direct) check(ctx context.Context, s *Stream) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return fmt.Errorf("%w: a stream URL that doesn't parse", errBadAnswer)
	}
	for k, v := range s.Header {
		req.Header[k] = v
	}
	req.Header.Set("Range", "bytes=0-0")
	res, err := d.HTTP.Do(req)
	if err != nil {
		// The error quotes the URL, which carries the user's IP address.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("stream: checking the stream: %w", err)
	}
	// Read to its end, the connection stays open for the proxy's requests.
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	res.Body.Close()
	switch res.StatusCode {
	case http.StatusPartialContent, http.StatusOK:
	case http.StatusForbidden, http.StatusGone:
		return errRefused
	default:
		return statusError(res.StatusCode)
	}
	if s.Size == 0 {
		_, total, _ := strings.Cut(res.Header.Get("Content-Range"), "/")
		if size, err := strconv.ParseInt(total, 10, 64); err == nil && size > 0 {
			s.Size = size
		}
	}
	return nil
}

// playerAnswer is what the direct route reads of YouTube's player API's
// answer.
type playerAnswer struct {
	ResponseContext struct {
		VisitorData string `json:"visitorData"`
	} `json:"responseContext"`
	PlayabilityStatus struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	} `json:"playabilityStatus"`
	VideoDetails struct {
		VideoID       string `json:"videoId"`
		LengthSeconds number `json:"lengthSeconds"`
	} `json:"videoDetails"`
	PlayerConfig struct {
		// AudioConfig tells how loud the video is, as YouTube measured it
		// with ITU-R BS.1770, and the level its apps play it at.
		AudioConfig struct {
			TrackAbsoluteLoudnessLkfs *float64 `json:"trackAbsoluteLoudnessLkfs"`
			PerceptualLoudnessDb      *float64 `json:"perceptualLoudnessDb"`
			LoudnessTargetLkfs        *float64 `json:"loudnessTargetLkfs"`
			// LoudnessDb is how far above the target the video is.
			LoudnessDb *float64 `json:"loudnessDb"`
		} `json:"audioConfig"`
	} `json:"playerConfig"`
	StreamingData struct {
		AdaptiveFormats []playerFormat `json:"adaptiveFormats"`
	} `json:"streamingData"`
}

// loudness returns the integrated loudness of a video's audio, in LKFS, as
// YouTube measured it: of format f, when it is not nil and YouTube measured
// it, else of the video, else of its first audio format of the original
// dynamics. A measure is taken only in [-70, 0]: anything else is no
// loudness a track can have.
func (a playerAnswer) loudness(f *playerFormat) (float64, bool) {
	ac := a.PlayerConfig.AudioConfig
	target := -14.0
	if ac.LoudnessTargetLkfs != nil {
		target = *ac.LoudnessTargetLkfs
	}
	above := func(db *float64) *float64 {
		if db == nil {
			return nil
		}
		v := target + *db
		return &v
	}
	var measures []*float64
	if f != nil {
		measures = append(measures, f.TrackAbsoluteLoudnessLkfs, above(f.LoudnessDb))
	}
	measures = append(measures, ac.TrackAbsoluteLoudnessLkfs, ac.PerceptualLoudnessDb, above(ac.LoudnessDb))
	for _, af := range a.StreamingData.AdaptiveFormats {
		if strings.HasPrefix(af.MimeType, "audio/") && !af.IsDrc {
			measures = append(measures, af.TrackAbsoluteLoudnessLkfs, above(af.LoudnessDb))
			break
		}
	}
	for _, v := range measures {
		if v != nil && *v >= -70 && *v <= 0 {
			return *v, true
		}
	}
	return 0, false
}

func (a playerAnswer) status() playabilityError {
	return playabilityError{a.PlayabilityStatus.Status, a.PlayabilityStatus.Reason}
}

// playerFormat is a stream of a video, as YouTube's player API lists it.
type playerFormat struct {
	Itag              int               `json:"itag"`
	URL               string            `json:"url"`
	SignatureCipher   string            `json:"signatureCipher"`
	MimeType          string            `json:"mimeType"`
	Width             number            `json:"width"`
	Height            number            `json:"height"`
	FPS               number            `json:"fps"`
	Bitrate           number            `json:"bitrate"`
	AverageBitrate    number            `json:"averageBitrate"`
	ContentLength     number            `json:"contentLength"`
	ApproxDurationMs  number            `json:"approxDurationMs"`
	AudioChannels     number            `json:"audioChannels"`
	SpatialAudioType  string            `json:"spatialAudioType"`
	IsDrc             bool              `json:"isDrc"`
	Type              string            `json:"type"`
	TargetDurationSec number            `json:"targetDurationSec"`
	DrmFamilies       []json.RawMessage `json:"drmFamilies"`
	// TrackAbsoluteLoudnessLkfs is how loud the format's audio is, and
	// LoudnessDb how far above the video's loudness target.
	TrackAbsoluteLoudnessLkfs *float64 `json:"trackAbsoluteLoudnessLkfs"`
	LoudnessDb                *float64 `json:"loudnessDb"`
	AudioTrack                *struct {
		DisplayName    string `json:"displayName"`
		AudioIsDefault bool   `json:"audioIsDefault"`
	} `json:"audioTrack"`
}

// number is a number YouTube's API sends as a JSON number, or as a string
// of one, as it does sizes and durations.
type number float64

func (n *number) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*n = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*n = number(v)
	return nil
}

// codec returns the codec of f's stream, as its MIME type's codecs
// parameter names it: "opus", or "mp4a.40.2" for AAC-LC.
func (f playerFormat) codec() string {
	_, params, err := mime.ParseMediaType(f.MimeType)
	if err != nil {
		return ""
	}
	return params["codecs"]
}

func (f playerFormat) opus() bool {
	return strings.HasPrefix(f.MimeType, "audio/webm") && strings.Contains(f.MimeType, "opus")
}

// aacLC tells whether f is AAC-LC in MP4, as itag 140 is, which the engine
// plays when a video has no Opus. It doesn't play HE-AAC, mp4a.40.5, as
// itag 139 is.
func (f playerFormat) aacLC() bool {
	return strings.HasPrefix(f.MimeType, "audio/mp4") && f.codec() == "mp4a.40.2"
}

// plain tells whether f is a stream the engine plays as YouTube's apps
// play it: mono or stereo, of the original dynamics rather than YouTube's
// stable volume, unencrypted, and whole, not cut short. length is the
// video's, 0 when unknown.
func (f playerFormat) plain(length time.Duration) bool {
	dur := time.Duration(f.ApproxDurationMs) * time.Millisecond
	return !f.IsDrc && len(f.DrmFamilies) == 0 && f.Type != "FORMAT_STREAM_TYPE_OTF" && f.TargetDurationSec == 0 &&
		f.AudioChannels <= 2 && f.SpatialAudioType == "" && !(length > 0 && dur > 0 && dur < length/2)
}

// direct tells whether f's URL plays as given: YouTube sends some only
// with a signature, or with a challenge to solve first, as yt-dlp does.
func (f playerFormat) direct() bool {
	if f.URL == "" {
		return false
	}
	u, err := url.Parse(f.URL)
	return err == nil && !u.Query().Has("n")
}

// trackRank ranks the audio track of f: a video's only one first, then its
// original, then the one YouTube plays by default, then any other dub, and
// audio descriptions last. The names are in English, the language the
// route asks in.
func (f playerFormat) trackRank() int {
	t := f.AudioTrack
	if t == nil {
		return 3
	}
	name := strings.ToLower(t.DisplayName)
	switch {
	case strings.Contains(name, "descriptive"):
		return -1
	case strings.Contains(name, "original"):
		return 2
	case t.AudioIsDefault:
		return 1
	}
	return 0
}

// stream is the Stream of f.
func (f playerFormat) stream() Stream {
	bitrate := f.AverageBitrate
	if bitrate == 0 {
		bitrate = f.Bitrate
	}
	s := Stream{
		URL:      f.URL,
		Header:   http.Header{},
		MimeType: f.MimeType,
		Size:     int64(f.ContentLength),
		Duration: time.Duration(f.ApproxDurationMs) * time.Millisecond,
		Codec:    f.codec(),
		Bitrate:  float64(bitrate) / 1000,
		Expires:  expiry(f.URL),
	}
	s.Header.Set("User-Agent", visionAgent)
	return s
}

// pickAudio picks the stream of a video in quality q that the engine plays:
// the best or the smallest Opus stream of its original audio track, else,
// for a video YouTube has only in AAC, its AAC-LC stream. A video whose
// Opus streams need yt-dlp goes to yt-dlp, which finds them. length is the
// video's, 0 when unknown.
func pickAudio(formats []playerFormat, q Quality, length time.Duration) (playerFormat, error) {
	best, found, ciphered := pickOf(formats, q, length, playerFormat.opus)
	if !found && !ciphered {
		best, found, ciphered = pickOf(formats, q, length, playerFormat.aacLC)
	}
	switch {
	case found:
		return best, nil
	case ciphered:
		return playerFormat{}, errNoDirectURL
	}
	return playerFormat{}, errNoStream
}

// pickOf picks, of the formats of a codec, which of tells, the best or the
// smallest stream in quality q of the original audio track, and tells
// whether it passed over one that needs yt-dlp.
func pickOf(formats []playerFormat, q Quality, length time.Duration, of func(playerFormat) bool) (best playerFormat, found, ciphered bool) {
	rank := 0
	for _, f := range formats {
		if !of(f) || !f.plain(length) {
			continue
		}
		if !f.direct() {
			ciphered = true
			continue
		}
		r := f.trackRank()
		switch {
		case !found, r > rank:
		case r < rank:
			continue
		case q == QualitySaver && f.Bitrate >= best.Bitrate, q != QualitySaver && f.Bitrate <= best.Bitrate:
			continue
		}
		best, rank, found = f, r, true
	}
	return best, found, ciphered
}
