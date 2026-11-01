package stream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// The visitor IDs fakeTube gives: from its visitor_id endpoint, and with
// its bot check, as in testdata.
const (
	testVisitor = "CgtUZXN0VmlzaXRvcijAgICABjIKCgJVUxIEGgAgAA%3D%3D"
	botVisitor  = "CgtCb3RDaGVja1Zpc2l0b3IowICAgAYyCgoCVVMSBBoAIAA%3D"
)

// The video of testdata/visionos-player.json, and the sizes of its Opus
// streams.
const (
	songID   = "3IUqoyTxEU8"
	song251  = 3474783
	songSecs = 200401 * time.Millisecond
)

// playerRequest is what a request of fakeTube's player endpoint carried.
type playerRequest struct {
	VideoID string
	Visitor string // its X-Goog-Visitor-Id
	Header  http.Header
	Body    map[string]any
}

// fakeTube is YouTube's player API and its media server. It answers with
// the answers recorded in testdata, whose stream URLs point at itself, and
// keeps what it was asked.
type fakeTube struct {
	t   *testing.T
	srv *httptest.Server

	mu sync.Mutex
	// answer is the player endpoint's answer to a request, nil for HTTP
	// 500: by default the recorded answer of the song.
	answer func(r playerRequest) []byte
	// visitorStatus, when set, is the HTTP status of the visitor_id
	// endpoint's answers.
	visitorStatus int
	// media serves the streams: by default, the bytes asked for of a
	// stream of the size its URL tells.
	media         http.HandlerFunc
	visitorAsks   int
	visitorHeader http.Header
	visitorBody   map[string]any
	requests      []playerRequest
	expires       int64 // the expire of the last URLs given
}

func newFakeTube(t *testing.T) *fakeTube {
	t.Helper()
	f := &fakeTube{t: t}
	f.answer = func(playerRequest) []byte { return f.recorded("") }
	f.media = serveClen
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

// serveClen serves the bytes asked for of a stream of the size its URL
// tells, as YouTube's media server does.
func serveClen(w http.ResponseWriter, r *http.Request) {
	size, _ := strconv.Atoi(r.URL.Query().Get("clen"))
	http.ServeContent(w, r, "", time.Time{}, strings.NewReader(strings.Repeat("a", size)))
}

// direct returns a direct route to f.
func (f *fakeTube) direct() *Direct {
	return &Direct{HTTP: f.srv.Client(), BaseURL: f.srv.URL, now: time.Now}
}

// answers has the player endpoint answer fn.
func (f *fakeTube) answers(fn func(r playerRequest) []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answer = fn
}

// answersWith has the player endpoint answer testdata's
// visionos-player<suffix>.json.
func (f *fakeTube) answersWith(suffix string) {
	f.answers(func(playerRequest) []byte { return f.recorded(suffix) })
}

// serves has the media server answer with fn.
func (f *fakeTube) serves(fn http.HandlerFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.media = fn
}

// refuses has the media server answer with status.
func (f *fakeTube) refuses(status int) {
	f.serves(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) })
}

// recorded returns an answer of testdata, visionos-player<suffix>.json,
// with stream URLs at f that expire in six hours, one second later than
// the URLs it gave before.
func (f *fakeTube) recorded(suffix string) []byte {
	f.t.Helper()
	raw, err := os.ReadFile("testdata/visionos-player" + suffix + ".json")
	if err != nil {
		f.t.Fatal(err)
	}
	f.mu.Lock()
	f.expires = max(f.expires+1, time.Now().Add(6*time.Hour).Unix())
	expires := f.expires
	f.mu.Unlock()
	s := strings.ReplaceAll(string(raw), "{{media}}", f.srv.URL+"/media")
	return []byte(strings.ReplaceAll(s, "{{expire}}", strconv.FormatInt(expires, 10)))
}

func (f *fakeTube) serve(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/youtubei/v1/visitor_id":
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.visitorAsks++
		f.visitorHeader, f.visitorBody = r.Header.Clone(), body
		status := f.visitorStatus
		f.mu.Unlock()
		if status != 0 {
			w.WriteHeader(status)
			return
		}
		fmt.Fprintf(w, `{"responseContext": {"visitorData": %q}}`, testVisitor)
	case "/youtubei/v1/player":
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		req := playerRequest{Header: r.Header.Clone(), Body: body, Visitor: r.Header.Get("X-Goog-Visitor-Id")}
		req.VideoID, _ = body["videoId"].(string)
		f.mu.Lock()
		f.requests = append(f.requests, req)
		answer := f.answer
		f.mu.Unlock()
		if a := answer(req); a != nil {
			// YouTube's answers carry back the visitor ID of the request.
			if req.Visitor != "" {
				a = []byte(strings.ReplaceAll(string(a), testVisitor, req.Visitor))
			}
			w.Write(a)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	case "/media/videoplayback":
		f.mu.Lock()
		media := f.media
		f.mu.Unlock()
		media(w, r)
	default:
		http.NotFound(w, r)
	}
}

// locked runs fn with what f keeps.
func (f *fakeTube) locked(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn()
}

// players returns the requests of the player endpoint so far.
func (f *fakeTube) players() []playerRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]playerRequest(nil), f.requests...)
}

func (f *fakeTube) visitorCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.visitorAsks
}

// dig returns the value at path in a decoded JSON object.
func dig(v any, path ...string) any {
	for _, k := range path {
		m, _ := v.(map[string]any)
		v = m[k]
	}
	return v
}

// offline is a network that is down.
type offline struct{}

func (offline) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("no network")
}

func TestDirectStream(t *testing.T) {
	f := newFakeTube(t)
	s, err := f.direct().Stream(context.Background(), songID, QualityBest)
	if err != nil {
		t.Fatal(err)
	}
	if s.MimeType != `audio/webm; codecs="opus"` || s.Codec != "opus" || s.Size != song251 || s.Duration != songSecs {
		t.Errorf("stream = %+v", s)
	}
	if !strings.Contains(s.URL, "itag=251") || s.Bitrate != 138.713 {
		t.Errorf("not the best Opus stream: itag %s at %v kbit/s", s.URL[strings.Index(s.URL, "itag="):], s.Bitrate)
	}
	var expires int64
	f.locked(func() { expires = f.expires })
	if s.Expires.Unix() != expires || !s.Available.IsZero() {
		t.Errorf("expires %v, available %v", s.Expires, s.Available)
	}
	if s.Header.Get("User-Agent") != visionAgent {
		t.Errorf("header = %v", s.Header)
	}

	saver, err := f.direct().Stream(context.Background(), songID, QualitySaver)
	if err != nil || !strings.Contains(saver.URL, "itag=249") {
		t.Errorf("saver stream %s, %v", saver.URL, err)
	}
}

func TestDirectRequests(t *testing.T) {
	f := newFakeTube(t)
	if _, err := f.direct().Stream(context.Background(), songID, QualityBest); err != nil {
		t.Fatal(err)
	}
	reqs := f.players()
	if len(reqs) != 1 {
		t.Fatalf("%d player requests", len(reqs))
	}
	var visitorHeader http.Header
	var visitorBody map[string]any
	f.locked(func() { visitorHeader, visitorBody = f.visitorHeader, f.visitorBody })
	for name, h := range map[string]http.Header{"visitor_id": visitorHeader, "player": reqs[0].Header} {
		for k, want := range map[string]string{
			"Content-Type":             "application/json",
			"User-Agent":               visionAgent,
			"Origin":                   "https://www.youtube.com",
			"X-Youtube-Client-Name":    "101",
			"X-Youtube-Client-Version": "1.02",
		} {
			if got := h.Get(k); got != want {
				t.Errorf("%s: %s = %q, want %q", name, k, got, want)
			}
		}
		for _, k := range []string{"Cookie", "Authorization", "X-Goog-Authuser", "X-Goog-Pageid", "X-Origin"} {
			if h.Get(k) != "" {
				t.Errorf("%s sent %s: the route asks signed out", name, k)
			}
		}
	}
	if reqs[0].Visitor != testVisitor || visitorHeader.Get("X-Goog-Visitor-Id") != "" {
		t.Errorf("visitor IDs sent: %q to player, %q to visitor_id", reqs[0].Visitor, visitorHeader.Get("X-Goog-Visitor-Id"))
	}
	for path, want := range map[string]any{
		"context.client.clientName":    "VISIONOS",
		"context.client.clientVersion": "1.02",
		"context.client.hl":            "en",
		"context.client.visitorData":   testVisitor,
		"videoId":                      songID,
		"playbackContext.contentPlaybackContext.html5Preference": "HTML5_PREF_WANTS",
		"contentCheckOk": true,
		"racyCheckOk":    true,
	} {
		if got := dig(reqs[0].Body, strings.Split(path, ".")...); got != want {
			t.Errorf("player %s = %v, want %v", path, got, want)
		}
	}
	if got := dig(visitorBody, "context", "client", "clientName"); got != "VISIONOS" {
		t.Errorf("visitor_id clientName = %v", got)
	}
	if user := dig(reqs[0].Body, "context", "user"); user != nil {
		t.Errorf("player context names a user: %v", user)
	}
}

func TestDirectAsksForOneVisitorID(t *testing.T) {
	f := newFakeTube(t)
	d := f.direct()
	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			if _, err := d.Stream(context.Background(), songID, QualityBest); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if n := f.visitorCount(); n != 1 {
		t.Errorf("%d visitor IDs asked for five streams", n)
	}
}

func TestDirectRenewsTheVisitorAfterABotCheck(t *testing.T) {
	// YouTube distrusts the visitor ID it gave, and trusts the one its bot
	// check carries.
	trusting := func(f *fakeTube) func(playerRequest) []byte {
		return func(r playerRequest) []byte {
			if r.Visitor != botVisitor {
				return f.recorded("-bot-check")
			}
			return f.recorded("")
		}
	}
	f := newFakeTube(t)
	f.answers(trusting(f))
	d := f.direct()
	if _, err := d.Stream(context.Background(), songID, QualityBest); err != nil {
		t.Fatal(err)
	}
	reqs := f.players()
	if len(reqs) != 2 || reqs[0].Visitor != testVisitor || reqs[1].Visitor != botVisitor {
		t.Fatalf("player requests with visitors %v", reqs)
	}
	if _, err := d.Stream(context.Background(), songID, QualityBest); err != nil {
		t.Fatal(err)
	}
	if reqs := f.players(); len(reqs) != 3 || reqs[2].Visitor != botVisitor || f.visitorCount() != 1 {
		t.Errorf("the next stream asked with %q, after %d visitor IDs", reqs[len(reqs)-1].Visitor, f.visitorCount())
	}

	// Without a visitor ID at all, the bot check's carries the route on.
	f = newFakeTube(t)
	f.locked(func() { f.visitorStatus = http.StatusInternalServerError })
	f.answers(trusting(f))
	if _, err := f.direct().Stream(context.Background(), songID, QualityBest); err != nil {
		t.Fatalf("without a visitor ID: %v", err)
	}
	if reqs := f.players(); len(reqs) != 2 || reqs[0].Visitor != "" {
		t.Errorf("player requests with visitors %v", reqs)
	}

	// A bot check that a new visitor ID doesn't pass is YouTube's answer.
	f = newFakeTube(t)
	f.answersWith("-bot-check")
	_, err := f.direct().Stream(context.Background(), songID, QualityBest)
	var refused playabilityError
	if !errors.As(err, &refused) || !refused.botCheck() || len(f.players()) != 2 {
		t.Errorf("a bot check after a new visitor ID: %v, after %d requests", err, len(f.players()))
	}
}

func TestDirectPicksTheOriginalOpusTrack(t *testing.T) {
	f := newFakeTube(t)
	var ans playerAnswer
	if err := json.Unmarshal(f.recorded("-dubbed"), &ans); err != nil {
		t.Fatal(err)
	}
	length := time.Duration(ans.VideoDetails.LengthSeconds) * time.Second
	for q, itag := range map[Quality]int{QualityBest: 251, QualitySaver: 249} {
		got, err := pickAudio(ans.StreamingData.AdaptiveFormats, q, length)
		if err != nil {
			t.Fatal(err)
		}
		// The dubs, the stable-volume twin, and the streams that need yt-dlp
		// or more than two speakers all have more bits.
		if got.Itag != itag || got.AudioTrack == nil || got.AudioTrack.DisplayName != "English (US) original" ||
			got.IsDrc || !got.direct() || got.AudioChannels != 2 {
			t.Errorf("%s: picked %d of %+v", q, got.Itag, got.AudioTrack)
		}
	}

	f.answersWith("-dubbed")
	s, err := f.direct().Stream(context.Background(), "0e3GPea1Tyg", QualityBest)
	if want := float64(ans.original(t, 251)) / 1000; err != nil || s.Bitrate != want {
		t.Errorf("stream at %v kbit/s, want %v: %v", s.Bitrate, want, err)
	}
}

// original returns the average bitrate of the format itag of a's original
// audio track.
func (a playerAnswer) original(t *testing.T, itag int) number {
	t.Helper()
	for _, f := range a.StreamingData.AdaptiveFormats {
		if f.Itag == itag && f.AudioTrack != nil && f.AudioTrack.AudioIsDefault && f.plain(0) && f.direct() {
			return f.AverageBitrate
		}
	}
	t.Fatalf("no original %d", itag)
	return 0
}

func TestPickAudioTellsWhyNone(t *testing.T) {
	heAAC := playerFormat{Itag: 139, MimeType: `audio/mp4; codecs="mp4a.40.5"`, URL: "https://media/a?itag=139"}
	ps := playerFormat{Itag: 600, MimeType: `audio/mp4; codecs="mp4a.40.29"`, URL: "https://media/a?itag=600"}
	vorbis := playerFormat{Itag: 171, MimeType: `audio/webm; codecs="vorbis"`, URL: "https://media/a?itag=171"}
	ciphered := playerFormat{Itag: 251, MimeType: `audio/webm; codecs="opus"`, SignatureCipher: "s=AAAA&sp=sig&url=x"}
	challenged := playerFormat{Itag: 251, MimeType: `audio/webm; codecs="opus"`, URL: "https://media/a?itag=251&n=abc"}
	sabr := playerFormat{Itag: 251, MimeType: `audio/webm; codecs="opus"`}
	cipheredAAC := playerFormat{Itag: 140, MimeType: `audio/mp4; codecs="mp4a.40.2"`, SignatureCipher: "s=AAAA&sp=sig&url=x"}
	// HE-AAC, and HE-AAC v2, whose codec starts as AAC-LC's does, are no
	// AAC-LC.
	if _, err := pickAudio([]playerFormat{heAAC, ps, vorbis}, QualityBest, 0); !errors.Is(err, errNoStream) {
		t.Errorf("HE-AAC alone: %v", err)
	}
	for _, f := range []playerFormat{ciphered, challenged, sabr, cipheredAAC} {
		if _, err := pickAudio([]playerFormat{heAAC, f}, QualityBest, 0); !errors.Is(err, errNoDirectURL) {
			t.Errorf("%+v: %v", f, err)
		}
	}
	short := playerFormat{Itag: 251, MimeType: `audio/webm; codecs="opus"`, URL: "https://media/a?itag=251", ApproxDurationMs: 30000}
	if _, err := pickAudio([]playerFormat{short}, QualityBest, 200*time.Second); !errors.Is(err, errNoStream) {
		t.Errorf("a stream cut short: %v", err)
	}
}

// withoutOpus returns a recorded answer whose Opus streams are Vorbis, which
// the engine doesn't play: a video YouTube has only in AAC, but for that.
func withoutOpus(answer []byte) []byte {
	return []byte(strings.ReplaceAll(string(answer), `codecs=\"opus\"`, `codecs=\"vorbis\"`))
}

func TestDirectChecksTheMediaServer(t *testing.T) {
	f := newFakeTube(t)
	f.serves(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=0-0" || r.Header.Get("User-Agent") != visionAgent {
			t.Errorf("checked with Range %q, User-Agent %q", r.Header.Get("Range"), r.Header.Get("User-Agent"))
		}
		if r.URL.Query().Get("hop") == "" {
			http.Redirect(w, r, r.URL.String()+"&hop=1", http.StatusFound)
			return
		}
		serveClen(w, r)
	})
	// A format without its size takes it from the check's answer.
	f.answers(func(playerRequest) []byte {
		return []byte(strings.ReplaceAll(string(f.recorded("")), `"contentLength": "3474783",`, ""))
	})
	s, err := f.direct().Stream(context.Background(), songID, QualityBest)
	if err != nil || s.Size != song251 {
		t.Errorf("after a redirect: size %d, %v", s.Size, err)
	}

	f.refuses(http.StatusForbidden)
	if _, err := f.direct().Stream(context.Background(), songID, QualityBest); !errors.Is(err, errRefused) {
		t.Errorf("refused: %v", err)
	}
	f.refuses(http.StatusServiceUnavailable)
	_, err = f.direct().Stream(context.Background(), songID, QualityBest)
	if status := statusError(0); !errors.As(err, &status) || status != http.StatusServiceUnavailable {
		t.Errorf("unavailable: %v", err)
	}
}

func TestDirectSaysWhyYouTubeGaveNoStream(t *testing.T) {
	f := newFakeTube(t)
	for suffix, want := range map[string]playabilityError{
		"-age":        {"LOGIN_REQUIRED", "Sign in to confirm your age"},
		"-unplayable": {"UNPLAYABLE", "This video is not available"},
		"-error":      {"ERROR", "This video is unavailable"},
		"-sign-in":    {"LOGIN_REQUIRED", "Please sign in"},
	} {
		f.answersWith(suffix)
		_, err := f.direct().Stream(context.Background(), songID, QualityBest)
		var got playabilityError
		if !errors.As(err, &got) || got != want {
			t.Errorf("%s: %v", suffix, err)
		}
	}
}

func TestDirectRejectsAnotherVideosAnswer(t *testing.T) {
	f := newFakeTube(t)
	if _, err := f.direct().Stream(context.Background(), "w3XzgjlKFjc", QualityBest); !errors.Is(err, errBadAnswer) {
		t.Errorf("the song's answer for another video: %v", err)
	}
}

func TestDirectPausesAfterRefusals(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	f := newFakeTube(t)
	d := f.direct()
	now := time.Now()
	d.now = func() time.Time { return now }
	stream := func() error {
		_, err := d.Stream(context.Background(), songID, QualityBest)
		return err
	}

	// What says a video has no stream for the route, or that the network
	// failed, never pauses it.
	for _, suffix := range []string{"-age", "-unplayable"} {
		f.answersWith(suffix)
		for range pauseAfter + 1 {
			stream()
		}
	}
	f.answers(func(playerRequest) []byte {
		return []byte(strings.ReplaceAll(string(withoutOpus(f.recorded(""))), `codecs=\"mp4a.40.2\"`, `codecs=\"mp4a.40.5\"`))
	})
	for range pauseAfter + 1 {
		if err := stream(); !errors.Is(err, errNoStream) {
			t.Fatalf("no Opus and no AAC-LC: %v", err)
		}
	}
	down := &Direct{HTTP: &http.Client{Transport: offline{}}, BaseURL: f.srv.URL, now: d.now}
	for range pauseAfter + 1 {
		if _, err := down.Stream(context.Background(), songID, QualityBest); errors.Is(err, errDirectPaused) {
			t.Fatal("paused offline")
		}
	}
	f.answersWith("")
	if err := stream(); err != nil {
		t.Fatalf("after videos without streams for the route: %v", err)
	}

	// A refusal of the media server, URLs that need yt-dlp, a bot check a
	// new visitor ID doesn't pass, and an HTTP error each count.
	for name, refuse := range map[string]func(){
		"media server": func() { f.refuses(http.StatusForbidden) },
		"signatures": func() {
			f.answers(func(playerRequest) []byte {
				return []byte(strings.ReplaceAll(string(f.recorded("")), "&expire=", "&n=AbCdEfGhIjKl&expire="))
			})
		},
		"bot check": func() { f.answersWith("-bot-check") },
		"HTTP 500":  func() { f.answers(func(playerRequest) []byte { return nil }) },
	} {
		f.answersWith("")
		f.serves(serveClen)
		refuse()
		for i := range pauseAfter {
			if err := stream(); err == nil || errors.Is(err, errDirectPaused) {
				t.Fatalf("%s: refusal %d: %v", name, i+1, err)
			}
		}
		asked := len(f.players())
		if err := stream(); !errors.Is(err, errDirectPaused) || len(f.players()) != asked {
			t.Fatalf("%s: not paused after %d refusals: %v", name, pauseAfter, err)
		}
		now = now.Add(pauseFor + time.Minute)
	}

	// A stream found sets the count back.
	f.answersWith("")
	f.refuses(http.StatusForbidden)
	stream()
	stream()
	f.serves(serveClen)
	if err := stream(); err != nil {
		t.Fatal(err)
	}
	f.refuses(http.StatusForbidden)
	stream()
	if err := stream(); !errors.Is(err, errRefused) {
		t.Errorf("after refusals that a stream found came between: %v", err)
	}

	pauses := strings.Count(logs.String(), "YouTube's player API refused 3 streams in a row; yt-dlp alone finds them for 30 minutes")
	if pauses != 4 || strings.Contains(logs.String(), "videoplayback") || strings.Contains(logs.String(), testVisitor) {
		t.Errorf("%d pauses logged, or a stream URL or the visitor ID:\n%s", pauses, logs.String())
	}
}

func TestDirectStreamsCarryTheirLoudness(t *testing.T) {
	f := newFakeTube(t)
	s, err := f.direct().Stream(context.Background(), songID, QualityBest)
	if err != nil || !s.Measured || s.Loudness != -7.93 {
		t.Errorf("the song at %v LKFS (measured %v), %v", s.Loudness, s.Measured, err)
	}
	// The stable-volume twins measure louder than the audio that plays.
	f.answersWith("-drc")
	for _, q := range []Quality{QualityBest, QualitySaver} {
		s, err := f.direct().Stream(context.Background(), "jNQXAC9IVRw", q)
		if err != nil || !s.Measured || s.Loudness != -23.67 {
			t.Errorf("%s: at %v LKFS (measured %v), %v", q, s.Loudness, s.Measured, err)
		}
	}
}

func TestDirectLoudness(t *testing.T) {
	f := newFakeTube(t)
	f.refuses(http.StatusForbidden) // the media server is never asked
	if v, err := f.direct().Loudness(context.Background(), songID); err != nil || v != -7.93 {
		t.Errorf("the song: %v, %v", v, err)
	}
	for _, suffix := range []string{"-unplayable", "-age", "-bot-check"} {
		f.answersWith(suffix)
		if _, err := f.direct().Loudness(context.Background(), songID); !errors.Is(err, ErrNoLoudness) {
			t.Errorf("%s: %v", suffix, err)
		}
	}
	f.answers(func(playerRequest) []byte { return nil })
	if _, err := f.direct().Loudness(context.Background(), songID); err == nil || errors.Is(err, ErrNoLoudness) {
		t.Errorf("HTTP 500: %v", err)
	}
}

func TestPlayerAnswerLoudness(t *testing.T) {
	for _, tc := range []struct {
		name, answer string
		picked       string // the format the stream plays, if any
		want         float64
		ok           bool
	}{
		{"the track", `{"playerConfig": {"audioConfig": {"trackAbsoluteLoudnessLkfs": -11.93, "perceptualLoudnessDb": -12}}}`, "", -11.93, true},
		{"perceptual", `{"playerConfig": {"audioConfig": {"perceptualLoudnessDb": -9}}}`, "", -9, true},
		{"above the target", `{"playerConfig": {"audioConfig": {"loudnessTargetLkfs": -14, "loudnessDb": 6.07}}}`, "", -7.93, true},
		{"above no target", `{"playerConfig": {"audioConfig": {"loudnessDb": 6.07}}}`, "", -7.93, true},
		{"the formats", `{"streamingData": {"adaptiveFormats": [
			{"mimeType": "video/webm", "trackAbsoluteLoudnessLkfs": -5},
			{"mimeType": "audio/webm", "isDrc": true, "trackAbsoluteLoudnessLkfs": -16.92},
			{"mimeType": "audio/webm", "trackAbsoluteLoudnessLkfs": -23.67}]}}`, "", -23.67, true},
		{"a format above the target", `{"streamingData": {"adaptiveFormats": [{"mimeType": "audio/webm", "loudnessDb": -9.67}]}}`, "", -23.67, true},
		{"the stream's own", `{"playerConfig": {"audioConfig": {"trackAbsoluteLoudnessLkfs": -13.01}}}`, `{"trackAbsoluteLoudnessLkfs": -13.02}`, -13.02, true},
		{"too loud", `{"playerConfig": {"audioConfig": {"trackAbsoluteLoudnessLkfs": 3}}}`, "", 0, false},
		{"too quiet", `{"playerConfig": {"audioConfig": {"trackAbsoluteLoudnessLkfs": -90}}}`, "", 0, false},
		{"none", `{"playabilityStatus": {"status": "UNPLAYABLE"}}`, "", 0, false},
	} {
		var ans playerAnswer
		if err := json.Unmarshal([]byte(tc.answer), &ans); err != nil {
			t.Fatal(err)
		}
		var picked *playerFormat
		if tc.picked != "" {
			picked = new(playerFormat)
			if err := json.Unmarshal([]byte(tc.picked), picked); err != nil {
				t.Fatal(err)
			}
		}
		got, ok := ans.loudness(picked)
		if ok != tc.ok || math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("%s: %v, %v; want %v", tc.name, got, ok, tc.want)
		}
	}
}
