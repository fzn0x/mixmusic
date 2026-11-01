// Package stream finds the audio streams of YouTube videos, through
// YouTube's player API as YouTube's visionOS app, else with yt-dlp, and
// serves them to the player's audio element through a local scheme, with
// range requests, so it can seek. The stream of a live video is an HLS
// playlist, which OpenLive follows as the broadcast goes on.
package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/proc"
	"github.com/fzn0x/mixmusic/internal/tools"
)

// Quality chooses between the best audio and smaller streams.
type Quality string

const (
	QualityBest  Quality = "best"
	QualitySaver Quality = "saver"
)

// formats are yt-dlp's format selectors for each quality: Opus in WebM
// first, else AAC-LC in MP4, of itag 140, for a video YouTube has only in
// AAC, never the HE-AAC of itag 139, which the engine doesn't play. Data
// saver takes the smallest Opus, else AAC-LC too. YouTube's stable-volume
// formats, whose IDs end in -drc, come only when a video has no other of
// their codec: their dynamics are squeezed, and the loudness YouTube
// measured is of the original audio. Both are stereo, or mono: the
// engine plays two channels, so a video's surround formats (AC-3 and
// E-AC-3 in 5.1, or Opus or AAC of more channels) never come before them,
// though the 5.1 ones are its best; a format whose channels yt-dlp doesn't
// know passes. A live video has the AAC-LC of formats 234 and 233 in HLS,
// whose codec yt-dlp doesn't tell: data saver takes 233 before 234, as
// liveFormats has it. Any other audio comes last, which the engine then
// refuses, naming its format.
var formats = map[Quality]string{
	QualityBest: "bestaudio[acodec=opus][audio_channels<=?2][format_id!*=drc]/bestaudio[acodec=opus][audio_channels<=?2]/" +
		"bestaudio[acodec=mp4a.40.2][audio_channels<=?2][format_id!*=drc]/bestaudio[acodec=mp4a.40.2][audio_channels<=?2]/" +
		"234/233/bestaudio",
	QualitySaver: "worstaudio[acodec=opus][audio_channels<=?2][format_id!*=drc]/worstaudio[acodec=opus][audio_channels<=?2]/" +
		"worstaudio[acodec=mp4a.40.2][audio_channels<=?2][format_id!*=drc]/worstaudio[acodec=mp4a.40.2][audio_channels<=?2]/" +
		"233/234/worstaudio",
}

// liveFormats are the itags of the audio of a live video, in the order
// each quality takes them: AAC-LC at 44.1 kHz in stereo, of about 128
// kbit/s (234), or, with data saver, at 22.05 kHz (233), which is HE-AAC,
// whose AAC-LC core the engine plays. YouTube serves both in HLS, as
// packed audio.
var liveFormats = map[Quality][]string{
	QualityBest:  {"234", "233"},
	QualitySaver: {"233", "234"},
}

// resolveTimeout bounds one run of yt-dlp.
const resolveTimeout = 90 * time.Second

// expiryMargin is how long before a stream URL expires it is resolved
// again, so that a track started near the expiry plays to its end.
const expiryMargin = 30 * time.Minute

var videoIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// ErrBadVideoID is returned for strings that are not YouTube video IDs.
var ErrBadVideoID = errors.New("stream: not a YouTube video id")

// ValidVideoID reports whether id is a YouTube video ID.
func ValidVideoID(id string) bool { return videoIDRe.MatchString(id) }

// Stream is the audio stream of a video.
type Stream struct {
	// URL is the stream's: of its file, or of the HLS media playlist of a
	// live stream.
	URL      string
	Header   http.Header
	MimeType string
	// Size is the length of the stream in bytes, 0 when unknown.
	Size     int64
	Duration time.Duration
	Codec    string
	// Bitrate is in kbit/s.
	Bitrate float64
	Expires time.Time
	// Available is when YouTube starts serving the stream, a few seconds
	// after yt-dlp found it: it refuses the URL until then. It is zero
	// when yt-dlp did not tell.
	Available time.Time
	// Loudness is the integrated loudness of the stream's audio, in LKFS,
	// as YouTube measured it, when Measured: YouTube's player API tells
	// it, and yt-dlp doesn't.
	Loudness float64
	Measured bool
	// Live is set for the stream of a live video, as it goes on: the
	// packed audio of the segments of an HLS media playlist, which has no
	// size, nor a length.
	Live bool
}

// slowResolve is how long finding a stream takes before the log tells.
const slowResolve = time.Second

// Resolver finds streams and remembers them until they expire.
type Resolver struct {
	Tools *tools.Manager
	// CacheDir is where yt-dlp caches what it learns of YouTube's player.
	CacheDir string
	// Direct, when set, finds streams first, in a fraction of the time
	// yt-dlp takes; yt-dlp finds those it can't.
	Direct *Direct
	// Run runs yt-dlp with args and returns what it printed; tests replace
	// it.
	Run func(ctx context.Context, args ...string) ([]byte, error)

	mu      sync.Mutex
	quality Quality
	calls   map[string]*resolution
}

// resolution is the result of resolving a video, shared by every caller
// asking for it while it runs.
type resolution struct {
	done   chan struct{}
	stream Stream
	err    error
}

// fresh reports whether a resolution that ended found a stream that isn't
// about to expire.
func (res *resolution) fresh() bool {
	return res.err == nil && time.Until(res.stream.Expires) >= expiryMargin
}

// NewResolver returns a resolver that runs the yt-dlp of t.
func NewResolver(t *tools.Manager, cacheDir string) *Resolver {
	r := &Resolver{Tools: t, CacheDir: cacheDir, quality: QualityBest, calls: map[string]*resolution{}}
	r.Run = r.runYTDLP
	return r
}

// SetQuality changes the quality of the streams resolved from now on.
func (r *Resolver) SetQuality(q Quality) {
	if _, ok := formats[q]; !ok {
		q = QualityBest
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.quality != q {
		r.quality = q
		r.calls = map[string]*resolution{}
	}
}

// Resolve returns the audio stream of a video.
func (r *Resolver) Resolve(ctx context.Context, videoID string) (Stream, error) {
	if !ValidVideoID(videoID) {
		return Stream{}, ErrBadVideoID
	}
	r.mu.Lock()
	res := r.calls[videoID]
	if res != nil {
		select {
		case <-res.done:
			if !res.fresh() {
				res = nil // failed or about to expire: resolve again
			}
		default: // running: wait for it below
		}
	}
	if res == nil {
		res = &resolution{done: make(chan struct{})}
		r.calls[videoID] = res
		quality := r.quality
		go func() {
			defer close(res.done)
			// Not the caller's context: other callers may wait for this.
			ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
			defer cancel()
			res.stream, res.err = r.resolve(ctx, videoID, quality)
		}()
	}
	r.mu.Unlock()

	select {
	case <-res.done:
		return res.stream, res.err
	case <-ctx.Done():
		return Stream{}, ctx.Err()
	}
}

// Fresh reports whether the resolver holds a stream of a video that isn't
// about to expire, which Resolve returns without running yt-dlp.
func (r *Resolver) Fresh(videoID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	res := r.calls[videoID]
	if res == nil {
		return false
	}
	select {
	case <-res.done:
		return res.fresh()
	default:
		return false
	}
}

// Live reports whether the resolver holds a stream of a video that is a
// live one.
func (r *Resolver) Live(videoID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	res := r.calls[videoID]
	if res == nil {
		return false
	}
	select {
	case <-res.done:
		return res.err == nil && res.stream.Live
	default:
		return false
	}
}

// Forget drops what the resolver remembers of a video, for example after
// its stream URL was refused.
func (r *Resolver) Forget(videoID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if res := r.calls[videoID]; res != nil {
		select {
		case <-res.done:
			delete(r.calls, videoID)
		default:
		}
	}
}

// resolve finds the stream of a video through the direct route, when the
// resolver has it, else with yt-dlp. The log tells when it took a while,
// and why yt-dlp found it, never the stream's URL, which carries the
// user's IP address.
func (r *Resolver) resolve(ctx context.Context, videoID string, quality Quality) (Stream, error) {
	start := time.Now()
	var direct error
	if r.Direct != nil {
		dctx, cancel := context.WithTimeout(ctx, directTimeout)
		s, err := r.Direct.Stream(dctx, videoID, quality)
		cancel()
		if err == nil {
			if took := time.Since(start); took > slowResolve {
				log.Printf("mixmusic: found the stream of %s in %s, through YouTube's player API", videoID, took.Round(10*time.Millisecond))
			}
			return s, nil
		}
		direct = err
	}
	s, err := r.ytdlp(ctx, videoID, quality)
	if err != nil {
		// Without yt-dlp, why YouTube gave the direct route no stream is
		// the better answer, when it is a plain one.
		var refused playabilityError
		if errors.Is(err, tools.ErrNoYTDLP) && errors.As(direct, &refused) {
			if plain := plainError(refused.Reason); plain != "" {
				return Stream{}, errors.New(plain)
			}
		}
		return Stream{}, err
	}
	took := time.Since(start).Round(10 * time.Millisecond)
	switch {
	case direct != nil:
		log.Printf("mixmusic: found the stream of %s in %s with yt-dlp (%s)", videoID, took, strings.TrimPrefix(direct.Error(), "stream: "))
	case took > slowResolve:
		log.Printf("mixmusic: found the stream of %s in %s with yt-dlp", videoID, took)
	}
	return s, nil
}

// ytdlp asks yt-dlp for the stream of a video. It asks signed out, also
// when the user signed in to YouTube Music: with an account's cookies,
// YouTube answers yt-dlp with fewer streams, often none it can play, and
// it may flag the account for using yt-dlp.
func (r *Resolver) ytdlp(ctx context.Context, videoID string, quality Quality) (Stream, error) {
	args := []string{"-J", "--no-playlist", "-f", formats[quality]}
	if r.Tools != nil {
		args = append(args, r.Tools.YTDLPArgs(r.CacheDir)...)
	}
	args = append(args, "--", "https://music.youtube.com/watch?v="+videoID)
	out, err := r.Run(ctx, args...)
	if err != nil {
		return Stream{}, err
	}
	return parseInfo(out)
}

// runYTDLP runs yt-dlp, turning its error output into the error.
func (r *Resolver) runYTDLP(ctx context.Context, args ...string) ([]byte, error) {
	path, err := r.Tools.YTDLP()
	if err != nil {
		return nil, err
	}
	var stderr strings.Builder
	cmd := proc.Command(ctx, path, args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := ytdlpError(stderr.String()); msg != "" {
			if plain := plainError(msg); plain != "" {
				log.Printf("mixmusic: %s", msg)
				return nil, errors.New(plain)
			}
			return nil, errors.New(msg)
		}
		return nil, fmt.Errorf("yt-dlp: %w", err)
	}
	return out, nil
}

// ytdlpError returns the last error line yt-dlp printed, without its
// prefixes.
func ytdlpError(stderr string) string {
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); strings.HasPrefix(line, "ERROR:") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "ERROR:"))
			if _, rest, ok := strings.Cut(line, "] "); ok && strings.HasPrefix(line, "[") {
				line = rest
			}
			return "yt-dlp: " + line
		}
	}
	return ""
}

// plainErrors say in plain words why yt-dlp found no stream, for the
// errors it reports most, by a part of yt-dlp's message: the first that
// matches applies.
var plainErrors = []struct{ match, plain string }{
	{"not a bot", "YouTube asks to confirm this is not a bot; try again in a while"},
	{"try again later", "YouTube is turning down requests from this network for now; try again in a while"},
	{"confirm your age", "YouTube plays this track only to signed-in adults, on its own website"},
	{"age-restricted", "YouTube plays this track only to signed-in adults, on its own website"},
	{"inappropriate for some users", "YouTube plays this track only to signed-in adults, on its own website"},
	{"in your country", "this track is not available in your country"},
	{"private video", "this track is private"},
	{"members-only", "only the members of its channel can play this track"},
	{"join this channel", "only the members of its channel can play this track"},
	{"premium", "only YouTube Music Premium members can play this track"},
	{"requested format is not available", "YouTube gave no audio for this track; updating yt-dlp in Settings › Playback often fixes it"},
	{"video unavailable", "this track is no longer on YouTube Music"},
	{"this video is unavailable", "this track is no longer on YouTube Music"},
}

// plainError returns msg, an error of yt-dlp, in plain words, or "" when it
// is not one of plainErrors.
func plainError(msg string) string {
	lower := strings.ToLower(msg)
	for _, e := range plainErrors {
		if strings.Contains(lower, e.match) {
			return e.plain
		}
	}
	return ""
}

// info is what yt-dlp prints of a video with one format selected.
type info struct {
	URL            string            `json:"url"`
	Ext            string            `json:"ext"`
	ACodec         string            `json:"acodec"`
	ABR            float64           `json:"abr"`
	Filesize       int64             `json:"filesize"`
	FilesizeApprox int64             `json:"filesize_approx"`
	Duration       float64           `json:"duration"`
	Headers        map[string]string `json:"http_headers"`
	AvailableAt    float64           `json:"available_at"` // Unix seconds
	// LiveStatus is "is_live" for a live video, as it goes on, whose
	// stream comes over Protocol "m3u8_native": an HLS media playlist.
	LiveStatus string `json:"live_status"`
	Protocol   string `json:"protocol"`
}

// parseInfo reads the stream of yt-dlp's -J output.
func parseInfo(out []byte) (Stream, error) {
	var in info
	if err := json.Unmarshal(out, &in); err != nil {
		return Stream{}, fmt.Errorf("yt-dlp: unexpected output: %w", err)
	}
	if in.URL == "" {
		return Stream{}, errors.New("yt-dlp found no audio stream")
	}
	if strings.HasPrefix(in.Protocol, "m3u8") {
		// The engine plays HLS only as a live stream goes on.
		if in.LiveStatus != "is_live" {
			return Stream{}, errors.New("YouTube gave no audio for this track that MixMusic plays")
		}
		s := Stream{URL: in.URL, Header: http.Header{}, MimeType: "audio/aac", Codec: "mp4a.40.2", Expires: expiry(in.URL), Live: true}
		for k, v := range in.Headers {
			s.Header.Set(k, v)
		}
		return s, nil
	}
	s := Stream{
		URL:      in.URL,
		Header:   http.Header{},
		MimeType: mimeType(in.Ext, in.ACodec),
		Size:     in.Filesize,
		Duration: time.Duration(in.Duration * float64(time.Second)),
		Codec:    in.ACodec,
		Bitrate:  in.ABR,
		Expires:  expiry(in.URL),
	}
	for k, v := range in.Headers {
		s.Header.Set(k, v)
	}
	if in.AvailableAt > 0 {
		s.Available = time.UnixMilli(int64(math.Round(in.AvailableAt * 1000)))
	}
	return s, nil
}

// mimeType returns the content type of a stream of a container and codec.
func mimeType(ext, codec string) string {
	switch {
	case ext == "webm" && codec != "":
		return `audio/webm; codecs="` + codec + `"`
	case ext == "webm":
		return "audio/webm"
	case ext == "m4a" || ext == "mp4":
		return "audio/mp4"
	case ext == "mp3":
		return "audio/mpeg"
	}
	return "application/octet-stream"
}

// expiry reads when a googlevideo URL expires, from its expire parameter,
// or, in the URL of an HLS playlist, its path's expire segment; in six
// hours when it has neither.
func expiry(raw string) time.Time {
	if u, err := url.Parse(raw); err == nil {
		if sec, err := strconv.ParseInt(u.Query().Get("expire"), 10, 64); err == nil {
			return time.Unix(sec, 0)
		}
		if _, rest, ok := strings.Cut(u.Path, "/expire/"); ok {
			v, _, _ := strings.Cut(rest, "/")
			if sec, err := strconv.ParseInt(v, 10, 64); err == nil {
				return time.Unix(sec, 0)
			}
		}
	}
	return time.Now().Add(6 * time.Hour)
}
