// Package stream finds the audio streams of YouTube videos, through
// YouTube's player API as YouTube's visionOS app, else with yt-dlp, and
// serves them to the player's audio element through a local scheme, with
// range requests, so it can seek. The stream of a live video is an HLS
// playlist, which OpenLive follows as the broadcast goes on. Whether a
// video has 5.1 surround audio, YouTube's player API tells YouTube's TV
// app, and yt-dlp finds that audio as it is asked for.
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
// measured is of the original audio. Both are stereo, or mono: a video's
// surround formats (AC-3 and E-AC-3 in 5.1, or Opus or AAC of more
// channels) never come before them, though the 5.1 ones are its best; a
// format whose channels yt-dlp doesn't know passes. A live video has the
// AAC-LC of formats 234 and 233 in HLS, whose codec yt-dlp doesn't tell:
// data saver takes 233 before 234, as liveFormats has it. Any other audio
// comes last, which the engine then refuses, naming its format. The 5.1
// one the engine plays comes beside the stereo one, as the Stream's
// Surround, from the formats yt-dlp lists.
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
	// AsAccount is set for a stream yt-dlp found as the user's YouTube
	// Music account (Resolver.SetAsAccount), and false for one asked for
	// signed out, as every stream was before.
	AsAccount bool
	// Route is how the stream was found, and Args, of a run of yt-dlp, the
	// arguments it ran with: they may name the user's folders and the file
	// of their session, so what shows them hides those (ShownArgs).
	Route Route
	Args  []string
	// Surround is the video's 5.1 surround audio, AC-3 or E-AC-3 in MP4,
	// beside the stereo audio of the Stream, when YouTube has one the
	// engine plays: nil otherwise, in the Surround stream itself, and in
	// a live stream. Its URL is "" when YouTube's player API told only
	// that it is there: Resolver.Surround has yt-dlp find it.
	Surround *Stream
	// later is the answer of YouTube's TV app of the video's 5.1 surround
	// audio, while Surround is nil and the answer hadn't come as the
	// direct route found the stream: the resolver keeps it with the stream
	// as it comes (Resolver.Told).
	later *toldLater
	// Videos are the streams of the video's picture, which yt-dlp and
	// YouTube's player API list beside the audio, for the now playing page.
	Videos []Video
	// Live is set for the stream of a live video, as it goes on: the
	// packed audio of the segments of an HLS media playlist, which has no
	// size, nor a length.
	Live bool
}

// Route is how a stream was found.
type Route string

const (
	// RoutePlayerAPI is YouTube's player API, asked as YouTube's visionOS
	// app, the direct route.
	RoutePlayerAPI Route = "player-api"
	// RouteQuick is yt-dlp's quick run, as the visionOS client alone.
	RouteQuick Route = "yt-dlp-quick"
	// RouteFull is yt-dlp's full run, with its default clients and the
	// JavaScript runtime.
	RouteFull Route = "yt-dlp-full"
	// RouteAccount is yt-dlp's full run with the user's YouTube Music
	// session, as their account (Resolver.SetAsAccount).
	RouteAccount Route = "yt-dlp-account"
)

// found returns s, which a run of yt-dlp with args found, or the player
// API, with how it was found: the 5.1 surround audio beside it too.
func (s Stream) found(route Route, args []string) Stream {
	s.Route, s.Args = route, append([]string(nil), args...)
	if s.Surround != nil {
		sur := *s.Surround
		sur.Route, sur.Args = s.Route, s.Args
		s.Surround = &sur
	}
	return s
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
	// Told, when set, is called with a video's ID once YouTube's TV app
	// told, after Resolve returned the video's stream, that the video has
	// 5.1 surround audio: the stream Resolve returns has its Surround from
	// then on, and the track that plays it can offer it. It is called on
	// a goroutine of its own.
	Told func(videoID string)
	// Session, when set, returns the cookies of the user's YouTube Music
	// session that yt-dlp asks for streams as, while SetAsAccount asks for
	// that: nil asks signed out.
	Session func() []*http.Cookie

	mu      sync.Mutex
	quality Quality
	// asAccount is set while the user asks for streams as their account,
	// and refusedAsAccount holds the videos whose stream found so YouTube
	// refused, which are found signed out (account.go).
	asAccount        bool
	refusedAsAccount map[string]bool
	// noLogin tells the log once of a session yt-dlp can't ask as.
	noLogin sync.Once
	calls   map[string]*resolution
	// surrounds are the 5.1 surround audio yt-dlp found of videos whose
	// stream YouTube's player API gave.
	surrounds map[string]*resolution
}

// resolution is the result of resolving a video, shared by every caller
// asking for it while it runs.
type resolution struct {
	done   chan struct{}
	stream Stream
	err    error
	// surround is the 5.1 surround audio YouTube's TV app told of after
	// the stream was found, under the resolver's mutex (hear).
	surround *Stream
}

// fresh reports whether a resolution that ended found a stream that isn't
// about to expire.
func (res *resolution) fresh() bool {
	return res.err == nil && time.Until(res.stream.Expires) >= expiryMargin
}

// NewResolver returns a resolver that runs the yt-dlp of t.
func NewResolver(t *tools.Manager, cacheDir string) *Resolver {
	r := &Resolver{Tools: t, CacheDir: cacheDir, quality: QualityBest, calls: map[string]*resolution{}, surrounds: map[string]*resolution{}}
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
	return r.share(ctx, videoID, false, r.resolve)
}

// Surround returns the 5.1 surround audio of a video, which the stream
// Resolve returns tells of: errNoSurround when YouTube has none. Of most
// videos, YouTube's player API tells only that it is there: yt-dlp finds
// it then, as it is asked for, once, and it is kept until it is about to
// expire, as Resolve's streams are.
func (r *Resolver) Surround(ctx context.Context, videoID string) (Stream, error) {
	s, err := r.Resolve(ctx, videoID)
	switch {
	case err != nil:
		return Stream{}, err
	case s.Surround == nil:
		return Stream{}, errNoSurround
	case s.Surround.URL != "":
		return *s.Surround, nil
	}
	return r.share(ctx, videoID, true, r.findSurround)
}

// share returns what find finds of a video, shared by every caller asking
// for it while it runs, and kept until it fails or is about to expire: in
// calls, or surrounds for the 5.1 surround audio.
func (r *Resolver) share(ctx context.Context, videoID string, surround bool,
	find func(ctx context.Context, videoID string, quality Quality) (Stream, error)) (Stream, error) {
	r.mu.Lock()
	calls := r.calls
	if surround {
		calls = r.surrounds
	}
	res := calls[videoID]
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
		calls[videoID] = res
		quality := r.quality
		go func() {
			// Not the caller's context: other callers may wait for this.
			ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
			s, err := find(ctx, videoID, quality)
			cancel()
			later := s.later
			s.later = nil
			res.stream, res.err = s, err
			close(res.done)
			// The TV app's answer of 5.1 audio that came after the stream.
			if later != nil {
				r.hear(videoID, res, later)
			}
		}()
	}
	r.mu.Unlock()

	select {
	case <-res.done:
		return r.result(res)
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

// Forget drops what the resolver remembers of a video, its 5.1 surround
// audio too, for example after its stream URL was refused.
func (r *Resolver) Forget(videoID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, calls := range []map[string]*resolution{r.calls, r.surrounds} {
		if res := calls[videoID]; res != nil {
			select {
			case <-res.done:
				r.refused(videoID, res)
				delete(calls, videoID)
			default:
			}
		}
	}
}

// resolve finds the stream of a video through the direct route, when the
// resolver has it, else with yt-dlp, quickly first (findWithYTDLP); as
// the user's account first, while they ask for that (accountStream). The
// log tells when it took a while, and why and how yt-dlp found it, never
// the stream's URL, which carries the user's IP address.
func (r *Resolver) resolve(ctx context.Context, videoID string, quality Quality) (Stream, error) {
	if s, ok := r.accountStream(ctx, videoID, quality); ok {
		return s, nil
	}
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
			s = s.found(RoutePlayerAPI, nil)
			if s.Surround != nil && s.Surround.URL == "" && !r.findsSurround() {
				log.Printf("mixmusic: YouTube has 5.1 surround audio of %s, which takes yt-dlp and a JavaScript runtime to play", videoID)
				s.Surround = nil
			}
			return s, nil
		}
		direct = err
	}
	s, run, err := r.findWithYTDLP(ctx, videoID, quality, direct)
	if err != nil {
		// Without yt-dlp, why YouTube gave the direct route no stream is
		// the better answer, when it is a plain one.
		var refused playabilityError
		if errors.Is(err, tools.ErrNoYTDLP) && errors.As(direct, &refused) {
			if plain, kind := plainError(refused.Reason); plain != "" {
				return Stream{}, failure{plain, kind}
			}
		}
		if direct != nil {
			return Stream{}, bothFailed{direct, err}
		}
		return Stream{}, err
	}
	took := time.Since(start).Round(10 * time.Millisecond)
	switch {
	case direct != nil:
		log.Printf("mixmusic: found the stream of %s in %s with yt-dlp's %s run in %s (%s)", videoID, took, run.name, run.took,
			strings.TrimPrefix(direct.Error(), "stream: "))
	case took > slowResolve:
		log.Printf("mixmusic: found the stream of %s in %s with yt-dlp's %s run in %s", videoID, took, run.name, run.took)
	}
	return s, nil
}

// findsSurround reports whether yt-dlp can find the 5.1 surround audio
// YouTube's player API tells only of: its URL plays once yt-dlp solved a
// challenge, which takes a JavaScript runtime. A resolver without tools,
// as in tests, runs a yt-dlp of its own.
func (r *Resolver) findsSurround() bool {
	if r.Tools == nil {
		return true
	}
	_, err := r.Tools.YTDLP()
	_, _, ok := r.Tools.JSRuntime()
	return err == nil && ok
}

// findSurround has yt-dlp find the 5.1 surround audio of a video, which
// YouTube's player API told only of.
func (r *Resolver) findSurround(ctx context.Context, videoID string, quality Quality) (Stream, error) {
	start := time.Now()
	s, err := r.ytdlp(ctx, videoID, quality)
	if err != nil {
		return Stream{}, err
	}
	if s.Surround == nil {
		return Stream{}, errNoSurround
	}
	log.Printf("mixmusic: found the 5.1 surround audio of %s in %s with yt-dlp", videoID, time.Since(start).Round(10*time.Millisecond))
	return *s.Surround, nil
}

// ytdlp asks yt-dlp for the stream of a video the full way: with its
// default clients and the JavaScript runtime, which find what the quick
// run can't, and the 5.1 surround audio, as the stream's Surround. It asks
// signed out, also when the user signed in to YouTube Music: with an
// account's cookies, YouTube answers yt-dlp with fewer streams, often none
// it can play, and it may flag the account for using yt-dlp. Only the
// user's own choice asks as their account, before it (accountStream).
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
	s, err := parseInfo(out)
	if err != nil {
		return Stream{}, err
	}
	return s.found(RouteFull, args), nil
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
		if f := ytdlpFailure(stderr.String()); f != nil {
			return nil, f
		}
		return nil, fmt.Errorf("yt-dlp: %w", err)
	}
	return out, nil
}

// ytdlpFailure is the error yt-dlp printed last, in plain words when it is
// one of plainErrors, with its kind; nil when it printed none.
func ytdlpFailure(stderr string) error {
	msg := ytdlpError(stderr)
	if msg == "" {
		return nil
	}
	if plain, kind := plainError(msg); plain != "" {
		log.Printf("mixmusic: %s", msg)
		return failure{plain, kind}
	}
	return failure{msg, networkKind(msg)}
}

// ytdlpError returns the last error line yt-dlp printed, without its
// prefixes.
func ytdlpError(stderr string) string {
	if line := tools.ErrorLine(stderr); line != "" {
		return "yt-dlp: " + line
	}
	return ""
}

// plainErrors say in plain words why yt-dlp found no stream, for the
// errors it reports most, by a part of yt-dlp's message, with their kind:
// the first that matches applies.
var plainErrors = []struct {
	match, plain string
	kind         error
}{
	{"not a bot", "YouTube asks to confirm this is not a bot; try again in a while", ErrThrottled},
	{"try again later", "YouTube is turning down requests from this network for now; try again in a while", ErrThrottled},
	{"confirm your age", "YouTube plays this track only to signed-in adults, on its own website", ErrUnavailable},
	{"age-restricted", "YouTube plays this track only to signed-in adults, on its own website", ErrUnavailable},
	{"inappropriate for some users", "YouTube plays this track only to signed-in adults, on its own website", ErrUnavailable},
	{"in your country", "this track is not available in your country", ErrUnavailable},
	{"private video", "this track is private", ErrUnavailable},
	{"members-only", "only the members of its channel can play this track", ErrUnavailable},
	{"join this channel", "only the members of its channel can play this track", ErrUnavailable},
	{"premium", "only YouTube Music Premium members can play this track", ErrUnavailable},
	{"requested format is not available", "YouTube gave no audio for this track; updating yt-dlp in Settings › Playback often fixes it", nil},
	{"video unavailable", "this track is no longer on YouTube Music", ErrUnavailable},
	{"this video is unavailable", "this track is no longer on YouTube Music", ErrUnavailable},
}

// plainError returns msg, an error of yt-dlp, in plain words, with its
// kind, or "" when it is not one of plainErrors.
func plainError(msg string) (string, error) {
	lower := strings.ToLower(msg)
	for _, e := range plainErrors {
		if strings.Contains(lower, e.match) {
			return e.plain, e.kind
		}
	}
	return "", nil
}

// networkErrors are parts of yt-dlp's messages when it couldn't reach
// YouTube, or YouTube didn't answer it.
var networkErrors = []string{"unable to download", "urlopen error", "timed out", "connection reset", "connection refused",
	"getaddrinfo", "name resolution", "network is unreachable"}

// networkKind is ErrNetwork when msg, an error of yt-dlp, tells that it
// couldn't reach YouTube, else nil.
func networkKind(msg string) error {
	lower := strings.ToLower(msg)
	for _, m := range networkErrors {
		if strings.Contains(lower, m) {
			return ErrNetwork
		}
	}
	return nil
}

// failure is why no stream was found, in the words the player shows, and
// its kind, if known.
type failure struct {
	msg  string
	kind error
}

func (f failure) Error() string { return f.msg }
func (f failure) Unwrap() error { return f.kind }

// info is what yt-dlp prints of a video with one format selected: the
// fields of that format, and the formats of the video.
type info struct {
	ytFormat
	Duration float64 `json:"duration"`
	// LiveStatus is "is_live" for a live video, as it goes on, whose
	// stream comes over Protocol "m3u8_native": an HLS media playlist.
	LiveStatus string     `json:"live_status"`
	Formats    []ytFormat `json:"formats"`
}

// ytFormat is what yt-dlp prints of a format.
type ytFormat struct {
	FormatID       string            `json:"format_id"`
	URL            string            `json:"url"`
	Ext            string            `json:"ext"`
	Protocol       string            `json:"protocol"`
	ACodec         string            `json:"acodec"`
	VCodec         string            `json:"vcodec"`
	ABR            float64           `json:"abr"`
	AudioChannels  int               `json:"audio_channels"`
	Filesize       int64             `json:"filesize"`
	FilesizeApprox int64             `json:"filesize_approx"`
	Headers        map[string]string `json:"http_headers"`
	AvailableAt    float64           `json:"available_at"` // Unix seconds
	// HasDRM is true, false or "maybe".
	HasDRM any `json:"has_drm"`
	// Width, Height, FPS and TBR are a video format's, which the now
	// playing page shows the picture of (ytdlpVideos).
	Width  int     `json:"width"`
	Height int     `json:"height"`
	FPS    float64 `json:"fps"`
	TBR    float64 `json:"tbr"`
}

// surroundRank ranks the codecs of the 5.1 surround formats the engine
// plays, as they are named: AC-3 first, whose decoding the tests check
// whole, then E-AC-3; 0 for any other.
func surroundRank(codec string) int {
	switch strings.ToLower(codec) {
	case "ac-3", "ac3":
		return 2
	case "ec-3", "eac3":
		return 1
	}
	return 0
}

// surround reports whether f is a 5.1 surround format the engine plays:
// AC-3 or E-AC-3 in MP4, of six channels, of the original dynamics,
// unencrypted, whole, over plain HTTPS.
func (f ytFormat) surround() bool {
	return surroundRank(f.ACodec) > 0 && f.AudioChannels == 6 && (f.VCodec == "" || f.VCodec == "none") &&
		(f.Ext == "m4a" || f.Ext == "mp4") && (f.Protocol == "" || f.Protocol == "https") &&
		f.URL != "" && f.HasDRM != true && f.HasDRM != "maybe" && !strings.Contains(f.FormatID, "drc")
}

// stream returns the Stream of f, of a video of duration seconds.
func (f ytFormat) stream(duration float64) Stream {
	s := Stream{
		URL:      f.URL,
		Header:   http.Header{},
		MimeType: mimeType(f.Ext, f.ACodec),
		Size:     f.Filesize,
		Duration: time.Duration(duration * float64(time.Second)),
		Codec:    f.ACodec,
		Bitrate:  f.ABR,
		Expires:  expiry(f.URL),
	}
	for k, v := range f.Headers {
		s.Header.Set(k, v)
	}
	if f.AvailableAt > 0 {
		s.Available = time.UnixMilli(int64(math.Round(f.AvailableAt * 1000)))
	}
	return s
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
	s := in.ytFormat.stream(in.Duration)
	s.Videos = ytdlpVideos(in.Formats)
	// The 5.1 surround beside it, of the best codec, then the most bits.
	var best *ytFormat
	for i, f := range in.Formats {
		if !f.surround() {
			continue
		}
		if best == nil || surroundRank(f.ACodec) > surroundRank(best.ACodec) ||
			surroundRank(f.ACodec) == surroundRank(best.ACodec) && f.ABR > best.ABR {
			best = &in.Formats[i]
		}
	}
	if best != nil {
		sur := best.stream(in.Duration)
		s.Surround = &sur
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
