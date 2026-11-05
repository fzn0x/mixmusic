package stream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/tools"
)

// surroundInfo is what yt-dlp prints of a video of stereo and 5.1 audio,
// with the stereo Opus selected, its formats those of formats, a JSON
// list's items.
func surroundInfo(formats ...string) []byte {
	return fmt.Appendf(nil, `{"format_id": "251", "url": "https://example.com/opus?expire=%d", "ext": "webm", "acodec": "opus",
		"abr": 138, "audio_channels": 2, "filesize": 1000, "duration": 207.5, "formats": [%s]}`,
		time.Now().Add(6*time.Hour).Unix(), strings.Join(formats, ", "))
}

// ytSurround is a 5.1 format, as yt-dlp lists it, of id and codec, with
// fields that replace its own.
func ytSurround(id, codec string, abr float64, fields string) string {
	f := fmt.Sprintf(`"format_id": "%s", "url": "https://example.com/%s", "ext": "m4a", "protocol": "https", "acodec": "%s",
		"vcodec": "none", "abr": %v, "audio_channels": 6, "filesize": 9000, "has_drm": false, "http_headers": {"User-Agent": "test"}`,
		id, id, codec, abr)
	if fields != "" {
		f += ", " + fields
	}
	return "{" + f + "}"
}

func TestYTDLPKeepsSurroundBesideStereo(t *testing.T) {
	stereo := `{"format_id": "251", "url": "https://example.com/251", "ext": "webm", "acodec": "opus", "abr": 138, "audio_channels": 2}`
	for _, c := range []struct {
		name    string
		formats []string
		want    string // the surround format's ID, "" for none
	}{
		{"stereo only", []string{stereo}, ""},
		{"AC-3 and E-AC-3", []string{stereo, ytSurround("328", "ec-3", 384, ""), ytSurround("380", "ac-3", 384, "")}, "380"},
		{"E-AC-3 alone", []string{stereo, ytSurround("328", "ec-3", 384, "")}, "328"},
		{"of the most bits", []string{ytSurround("380", "ac-3", 256, ""), ytSurround("380b", "ac-3", 384, "")}, "380b"},
		{"stable volume", []string{ytSurround("380-drc", "ac-3", 384, "")}, ""},
		{"encrypted", []string{ytSurround("380", "ac-3", 384, `"has_drm": true`), ytSurround("381", "ac-3", 384, `"has_drm": "maybe"`)}, ""},
		{"of four channels", []string{ytSurround("380", "ac-3", 384, `"audio_channels": 4`)}, ""},
		{"of video", []string{ytSurround("380", "ac-3", 384, `"vcodec": "avc1"`)}, ""},
		{"in fragments", []string{ytSurround("380", "ac-3", 384, `"protocol": "m3u8_native"`)}, ""},
		{"surround Opus", []string{ytSurround("338", "opus", 480, `"ext": "webm"`)}, ""},
		{"without a URL", []string{ytSurround("380", "ac-3", 384, `"url": ""`)}, ""},
	} {
		s, err := parseInfo(surroundInfo(c.formats...))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if s.URL == "" || s.Codec != "opus" {
			t.Errorf("%s: the stereo stream is %+v", c.name, s)
		}
		switch {
		case c.want == "" && s.Surround != nil:
			t.Errorf("%s: surround %+v", c.name, s.Surround)
		case c.want == "":
		case s.Surround == nil:
			t.Errorf("%s: no surround", c.name)
		case s.Surround.URL != "https://example.com/"+c.want || s.Surround.MimeType != "audio/mp4" ||
			s.Surround.Size != 9000 || s.Surround.Duration != 207500*time.Millisecond ||
			s.Surround.Header.Get("User-Agent") != "test" || s.Surround.Surround != nil:
			t.Errorf("%s: surround %+v", c.name, s.Surround)
		}
	}
}

func TestPickSurround(t *testing.T) {
	ac3 := playerFormat{Itag: 380, MimeType: `audio/mp4; codecs="ac-3"`, URL: "https://media/a?itag=380", AudioChannels: 6, Bitrate: 384000}
	eac3 := playerFormat{Itag: 328, MimeType: `audio/mp4; codecs="ec-3"`, URL: "https://media/a?itag=328", AudioChannels: 6, Bitrate: 384000}
	opus := playerFormat{Itag: 251, MimeType: `audio/webm; codecs="opus"`, URL: "https://media/a?itag=251", AudioChannels: 2, Bitrate: 150000}
	opus51 := playerFormat{Itag: 338, MimeType: `audio/webm; codecs="opus"`, URL: "https://media/a?itag=338", AudioChannels: 6, Bitrate: 480000}
	ciphered := ac3
	ciphered.URL, ciphered.SignatureCipher = "", "s=AAAA&sp=sig&url=x"
	drc := ac3
	drc.IsDrc = true
	dub, original := ac3, eac3
	dub.AudioTrack = &struct {
		DisplayName    string `json:"displayName"`
		AudioIsDefault bool   `json:"audioIsDefault"`
	}{"French", false}
	original.AudioTrack = &struct {
		DisplayName    string `json:"displayName"`
		AudioIsDefault bool   `json:"audioIsDefault"`
	}{"English original", true}
	short := ac3
	short.ApproxDurationMs = 30000
	challenged := eac3
	challenged.URL += "&n=Qm7xKz3vLpW9cT"
	for _, c := range []struct {
		name    string
		formats []playerFormat
		want    int // the itag, 0 for none
		listed  int // the itag of any URL, 0 for none
	}{
		{"stereo only", []playerFormat{opus}, 0, 0},
		{"AC-3 before E-AC-3", []playerFormat{opus, eac3, ac3}, 380, 380},
		{"E-AC-3", []playerFormat{opus, eac3}, 328, 328},
		{"surround Opus", []playerFormat{opus51}, 0, 0},
		{"needs yt-dlp", []playerFormat{ciphered}, 0, 380},
		{"a challenge to solve", []playerFormat{opus, challenged}, 0, 328},
		{"stable volume", []playerFormat{drc}, 0, 0},
		{"the original track before a dub", []playerFormat{dub, original}, 328, 328},
		{"cut short", []playerFormat{short}, 0, 0},
	} {
		got, ok := pickSurround(c.formats, 200*time.Second, true)
		if ok != (c.want != 0) || ok && got.Itag != c.want {
			t.Errorf("%s: %d, %v", c.name, got.Itag, ok)
		}
		got, ok = pickSurround(c.formats, 200*time.Second, false)
		if ok != (c.listed != 0) || ok && got.Itag != c.listed {
			t.Errorf("%s, of any URL: %d, %v", c.name, got.Itag, ok)
		}
	}
	// Stereo stays what pickAudio picks.
	if got, err := pickAudio([]playerFormat{ac3, eac3, opus}, QualityBest, 0); err != nil || got.Itag != 251 {
		t.Errorf("pickAudio picked %d, %v", got.Itag, err)
	}
}

func TestDirectFindsSurroundBesideStereo(t *testing.T) {
	f := newFakeTube(t)
	// The recorded answer, with a 5.1 AC-3 stream of the media server's.
	f.answers(func(playerRequest) []byte {
		b := f.recorded("")
		ac3 := fmt.Sprintf(`{"itag": 380, "url": "%s/media/videoplayback?itag=380&clen=9000", "mimeType": "audio/mp4; codecs=\"ac-3\"", "bitrate": 384000,
			"audioChannels": 6, "contentLength": "9000", "approxDurationMs": "0"}, `, f.srv.URL)
		return bytes.Replace(b, []byte(`"adaptiveFormats": [`), []byte(`"adaptiveFormats": [`+ac3), 1)
	})
	s, err := f.direct().Stream(context.Background(), songID, QualityBest)
	if err != nil {
		t.Fatal(err)
	}
	if s.Codec != "opus" || s.Surround == nil || s.Surround.Codec != "ac-3" || s.Surround.Size != 9000 ||
		!strings.Contains(s.Surround.URL, "itag=380") || s.Surround.Header.Get("User-Agent") == "" {
		t.Errorf("stream %+v, surround %+v", s, s.Surround)
	}
	// Without, none.
	f.answers(func(playerRequest) []byte { return f.recorded("") })
	if s, err := f.direct().Stream(context.Background(), songID, QualityBest); err != nil || s.Surround != nil {
		t.Errorf("without 5.1: %+v, %v", s.Surround, err)
	}
}

// The video of testdata's visionos-player-surround.json and
// tv-player-surround.json, "5.1 Dolby Atmos Surround Experience", and the
// size and length of its AC-3 stream.
const (
	surroundID   = "vrssjWvQMEc"
	surround380  = 11462153
	surroundSecs = 238720 * time.Millisecond
)

// TestDirectAsksTheTVClientOfSurround finds the stereo audio of a video
// through the visionOS client, whose answer lists no 5.1 audio, and learns
// from the TV client's, asked beside it, that the video has some: its
// AC-3 stream, without its URL, whose challenge only yt-dlp solves.
func TestDirectAsksTheTVClientOfSurround(t *testing.T) {
	f := newFakeTube(t)
	f.answersWith("-surround")
	f.answersTVWith("-surround")
	d := f.direct()
	s, err := d.Stream(context.Background(), surroundID, QualityBest)
	if err != nil {
		t.Fatal(err)
	}
	if s.Codec != "opus" || !strings.Contains(s.URL, "c=VISIONOS") || !strings.Contains(s.URL, "itag=251") {
		t.Errorf("stereo %+v", s)
	}
	if sur := s.Surround; sur == nil || sur.URL != "" || sur.Codec != "ac-3" || sur.MimeType != `audio/mp4; codecs="ac-3"` ||
		sur.Size != surround380 || sur.Duration != surroundSecs || !sur.Measured || sur.Loudness != -26.720001 || sur.Surround != nil {
		t.Errorf("surround %+v", s.Surround)
	}

	// It asks as YouTube's app for Samsung's TVs, signed out, with the
	// route's visitor ID, and the version of a player of a week ago.
	tv, vision := f.tvPlayers(), f.players()
	if len(tv) != 1 || len(vision) != 1 {
		t.Fatalf("%d requests of the TV client, %d of the visionOS one", len(tv), len(vision))
	}
	for k, want := range map[string]string{
		"Content-Type":             "application/json",
		"User-Agent":               tvAgent,
		"Origin":                   "https://www.youtube.com",
		"X-Youtube-Client-Name":    "7",
		"X-Youtube-Client-Version": "5.20260707",
		"X-Goog-Visitor-Id":        testVisitor,
	} {
		if got := tv[0].Header.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	for _, k := range []string{"Cookie", "Authorization", "X-Goog-Authuser"} {
		if tv[0].Header.Get(k) != "" {
			t.Errorf("the TV client sent %s: the route asks signed out", k)
		}
	}
	for path, want := range map[string]any{
		"context.client.clientName":                              "TVHTML5",
		"context.client.clientVersion":                           "5.20260707",
		"context.client.deviceMake":                              "Samsung",
		"context.client.deviceModel":                             "UKS9800",
		"context.client.osName":                                  "Tizen",
		"context.client.visitorData":                             testVisitor,
		"videoId":                                                surroundID,
		"playbackContext.contentPlaybackContext.html5Preference": "HTML5_PREF_WANTS",
		"contentCheckOk":                                         true,
		"racyCheckOk":                                            true,
	} {
		if got := dig(tv[0].Body, strings.Split(path, ".")...); got != want {
			t.Errorf("%s = %v, want %v", path, got, want)
		}
	}
	sts, _ := dig(tv[0].Body, "playbackContext", "contentPlaybackContext", "signatureTimestamp").(float64)
	if today := float64(signatureTimestamp(time.Now())); sts != today && sts != today-1 {
		t.Errorf("signatureTimestamp = %v, want %v", sts, today)
	}
	if got := dig(vision[0].Body, "playbackContext", "contentPlaybackContext", "signatureTimestamp"); got != nil {
		t.Errorf("the visionOS client told a player's version: %v", got)
	}

	// A video of no 5.1 audio has none.
	f.answersWith("")
	f.answersTVWith("")
	if s, err := d.Stream(context.Background(), songID, QualityBest); err != nil || s.Surround != nil {
		t.Errorf("without 5.1: %+v, %v", s.Surround, err)
	}
}

// TestSignatureTimestamp tells the version of YouTube's player of a week
// before: of 2026-11-02, whose code told 20759 to yt-dlp, on 2026-11-09.
func TestSignatureTimestamp(t *testing.T) {
	if got := signatureTimestamp(time.Date(2026, 11, 9, 23, 59, 0, 0, time.UTC)); got != 20759 {
		t.Errorf("signatureTimestamp = %d", got)
	}
}

// TestDirectWaitsLittleForTheTVClient finds the stereo audio of a video
// while the TV client's answer doesn't come, surroundGrace after its check
// at most: of no 5.1 audio, then.
func TestDirectWaitsLittleForTheTVClient(t *testing.T) {
	f := newFakeTube(t)
	f.answersWith("-surround")
	released := make(chan struct{})
	t.Cleanup(func() { close(released) })
	f.answersTV(func(playerRequest) []byte {
		<-released
		return nil
	})
	start := time.Now()
	s, err := f.direct().Stream(context.Background(), surroundID, QualityBest)
	if took := time.Since(start); err != nil || s.Surround != nil || took > surroundGrace+time.Second {
		t.Errorf("stream %+v, surround %+v, %v, after %v", s, s.Surround, err, took)
	}
}

// TestDirectRestsTheTVClientAfterRefusals asks the TV client nothing for
// pauseFor once it refused pauseAfter videos in a row, as when YouTube no
// longer takes the version of the player it tells: the stereo audio comes
// on as before.
func TestDirectRestsTheTVClientAfterRefusals(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	f := newFakeTube(t)
	f.answersTVWith("-reload")
	d := f.direct()
	now := time.Now()
	d.now = func() time.Time { return now }
	for i := range pauseAfter + 2 {
		if s, err := d.Stream(context.Background(), songID, QualityBest); err != nil || s.Surround != nil {
			t.Fatalf("stream %d: %+v, %v", i+1, s.Surround, err)
		}
	}
	if n := len(f.tvPlayers()); n != pauseAfter {
		t.Errorf("the TV client was asked %d times, refusing %d in a row", n, pauseAfter)
	}
	now = now.Add(pauseFor + time.Minute)
	f.answersTVWith("")
	if _, err := d.Stream(context.Background(), songID, QualityBest); err != nil || len(f.tvPlayers()) != pauseAfter+1 {
		t.Errorf("after the rest: %v, %d requests of the TV client", err, len(f.tvPlayers()))
	}
	if n := strings.Count(logs.String(), "refused the TV app 3 times in a row (YouTube's player API answered UNPLAYABLE: The page needs to be reloaded.)"); n != 1 {
		t.Errorf("%d rests logged:\n%s", n, logs.String())
	}
}

// TestResolverFindsSurroundWithYTDLP finds the stereo audio of a video
// through the direct route, which tells of its 5.1 audio, and has yt-dlp
// find that audio only as it is asked for: once, for all who ask at once,
// until the resolver forgets the video.
func TestResolverFindsSurroundWithYTDLP(t *testing.T) {
	f := newFakeTube(t)
	f.answersWith("-surround")
	f.answersTVWith("-surround")
	ctx := context.Background()
	var runs atomic.Int32
	available := time.Now().Add(time.Hour).Unix()
	r := NewResolver(nil, "")
	r.Direct = f.direct()
	r.Run = func(_ context.Context, args ...string) ([]byte, error) {
		runs.Add(1)
		if url := args[len(args)-1]; url != "https://music.youtube.com/watch?v="+surroundID {
			t.Errorf("yt-dlp asked for %s", url)
		}
		return surroundInfo(ytSurround("380", "ac-3", 384, fmt.Sprintf(`"available_at": %d`, available))), nil
	}
	s, err := r.Resolve(ctx, surroundID)
	if err != nil || s.Surround == nil || s.Surround.URL != "" || runs.Load() != 0 {
		t.Fatalf("stream %+v, %v, after %d runs of yt-dlp", s.Surround, err, runs.Load())
	}
	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			sur, err := r.Surround(ctx, surroundID)
			if err != nil || sur.URL != "https://example.com/380" || sur.Codec != "ac-3" || sur.Size != 9000 ||
				sur.Header.Get("User-Agent") != "test" || !sur.Available.Equal(time.Unix(available, 0)) {
				t.Errorf("surround %+v, %v", sur, err)
			}
		})
	}
	wg.Wait()
	if runs.Load() != 1 {
		t.Errorf("yt-dlp ran %d times for one video", runs.Load())
	}
	// The stereo audio stays the direct route's.
	if s, err := r.Resolve(ctx, surroundID); err != nil || !strings.Contains(s.URL, "c=VISIONOS") {
		t.Errorf("stereo %s, %v", s.URL, err)
	}
	r.Forget(surroundID)
	if _, err := r.Surround(ctx, surroundID); err != nil || runs.Load() != 2 {
		t.Errorf("a video forgotten: %v, after %d runs of yt-dlp", err, runs.Load())
	}

	// yt-dlp finding none, there is none.
	r.Forget(surroundID)
	r.Run = func(context.Context, ...string) ([]byte, error) { return surroundInfo(), nil }
	if _, err := r.Surround(ctx, surroundID); !errors.Is(err, errNoSurround) {
		t.Errorf("none that yt-dlp found: %v", err)
	}
	// Nor of a video YouTube has none of, without yt-dlp.
	f.answersWith("")
	f.answersTVWith("")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		t.Error("yt-dlp ran for a video of no 5.1 audio")
		return nil, errors.New("no yt-dlp here")
	}
	if _, err := r.Surround(ctx, songID); !errors.Is(err, errNoSurround) {
		t.Errorf("of a video of none: %v", err)
	}
}

// TestResolverOffersNoSurroundYTDLPCantFind leaves out the 5.1 audio the
// direct route tells of when yt-dlp couldn't find it: without a
// JavaScript runtime, which solves its URL's challenge, or without
// yt-dlp.
func TestResolverOffersNoSurroundYTDLPCantFind(t *testing.T) {
	f := newFakeTube(t)
	f.answersWith("-surround")
	f.answersTVWith("-surround")
	dir := t.TempDir()
	ytdlp := filepath.Join(dir, "yt-dlp")
	if err := os.WriteFile(ytdlp, []byte("yt-dlp"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, configure := range map[string]func(*tools.Manager){
		// A runtime of a name yt-dlp knows none of.
		"no JavaScript runtime": func(m *tools.Manager) { m.Configure(ytdlp, "none") },
		"no yt-dlp":             func(m *tools.Manager) { m.Configure(filepath.Join(dir, "missing"), "") },
	} {
		m := tools.New(t.TempDir())
		configure(m)
		r := NewResolver(m, "")
		r.Direct = f.direct()
		r.Run = func(context.Context, ...string) ([]byte, error) {
			t.Errorf("%s: yt-dlp ran", name)
			return nil, errors.New("no yt-dlp here")
		}
		if s, err := r.Resolve(context.Background(), surroundID); err != nil || s.Codec != "opus" || s.Surround != nil {
			t.Errorf("%s: stream %+v, surround %+v, %v", name, s, s.Surround, err)
		}
	}
}

// TestProxyServesSurroundYTDLPFinds serves the 5.1 audio of a video whose
// stereo audio the direct route found: yt-dlp finds it as the proxy is
// asked for it, which waits for YouTube to serve it, and the stereo audio
// comes without yt-dlp.
func TestProxyServesSurroundYTDLPFinds(t *testing.T) {
	f := newFakeTube(t)
	f.answersWith("-surround")
	f.answersTVWith("-surround")
	var minGen atomic.Int32
	srv5, data5 := upstream(t, 5000, &minGen)
	var runs atomic.Int32
	r := NewResolver(nil, "")
	r.Direct = f.direct()
	const wait = 300 * time.Millisecond
	r.Run = func(context.Context, ...string) ([]byte, error) {
		runs.Add(1)
		return surroundInfo(fmt.Sprintf(`{"format_id": "380", "url": "%s/a?expire=%d", "ext": "m4a", "protocol": "https", "acodec": "ac-3",
			"vcodec": "none", "abr": 384, "audio_channels": 6, "filesize": %d, "available_at": %.3f}`,
			srv5.URL, time.Now().Add(6*time.Hour).Unix(), len(data5), float64(time.Now().Add(wait).UnixMilli())/1000)), nil
	}
	p := NewProxy(r)
	get := func(path, header string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		if header != "" {
			req.Header.Set("Range", header)
		}
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, req)
		return rec
	}
	if rec := get(Path(surroundID), "bytes=0-99"); rec.Code != 206 || rec.Body.Len() != 100 || runs.Load() != 0 {
		t.Fatalf("stereo: %d, %d bytes, after %d runs of yt-dlp", rec.Code, rec.Body.Len(), runs.Load())
	}
	start := time.Now()
	if rec := get(SurroundPath(surroundID), ""); rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), data5) ||
		rec.Header().Get("Content-Type") != "audio/mp4" {
		t.Fatalf("5.1: %d, %d bytes of %q", rec.Code, rec.Body.Len(), rec.Header().Get("Content-Type"))
	}
	if took := time.Since(start); runs.Load() != 1 || took < wait-50*time.Millisecond {
		t.Errorf("5.1 after %d runs of yt-dlp, in %v", runs.Load(), took)
	}
	if rec := get(SurroundPath(surroundID), "bytes=10-19"); rec.Code != 206 || !bytes.Equal(rec.Body.Bytes(), data5[10:20]) || runs.Load() != 1 {
		t.Errorf("5.1 again: %d, %d bytes, after %d runs of yt-dlp", rec.Code, rec.Body.Len(), runs.Load())
	}
}

// surroundProxy returns a proxy whose yt-dlp resolves to a video of stereo
// audio of srv and, when surround, 5.1 audio of srv5.
func surroundProxy(srv, srv5 *httptest.Server, size, size5 int, surround bool) (*Proxy, *atomic.Int32) {
	var runs atomic.Int32
	r := NewResolver(nil, "")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		runs.Add(1)
		expire := time.Now().Add(6 * time.Hour).Unix()
		formats := ""
		if surround {
			formats = fmt.Sprintf(`{"format_id": "380", "url": "%s/a?expire=%d", "ext": "m4a", "protocol": "https", "acodec": "ac-3",
				"vcodec": "none", "abr": 384, "audio_channels": 6, "filesize": %d}`, srv5.URL, expire, size5)
		}
		return fmt.Appendf(nil, `{"url": "%s/a?expire=%d", "ext": "webm", "acodec": "opus", "filesize": %d, "formats": [%s]}`,
			srv.URL, expire, size, formats), nil
	}
	return NewProxy(r), &runs
}

func TestProxyServesSurround(t *testing.T) {
	var minGen atomic.Int32
	srv, data := upstream(t, 3000, &minGen)
	srv5, data5 := upstream(t, 5000, &minGen)
	for i := range data5 {
		data5[i] ^= 0x55 // not the stereo audio
	}
	p, runs := surroundProxy(srv, srv5, len(data), len(data5), true)
	dir := t.TempDir()
	p.Cache = NewCache(dir, 1<<20)
	get := func(path, header string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		if header != "" {
			req.Header.Set("Range", header)
		}
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, req)
		return rec
	}
	if rec := get(SurroundPath(testID), ""); rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), data5) || rec.Header().Get("Content-Type") != "audio/mp4" {
		t.Fatalf("5.1: %d, %d bytes of %q", rec.Code, rec.Body.Len(), rec.Header().Get("Content-Type"))
	}
	if rec := get(Path(testID), "bytes=0-"); rec.Code != 206 || !bytes.Equal(rec.Body.Bytes(), data) {
		t.Fatalf("stereo: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	if runs.Load() != 1 {
		t.Errorf("yt-dlp ran %d times for one video", runs.Load())
	}
	// Both are cached, each by its name.
	if _, err := os.Stat(filepath.Join(dir, SurroundKey(testID)+".m4a")); err != nil || !p.Cache.Has(SurroundKey(testID)) || !p.Cache.Has(testID) {
		t.Fatalf("5.1 not cached beside stereo: %v", err)
	}
	srv.Close()
	srv5.Close() // from now on only the cache can answer
	if rec := get(SurroundPath(testID), "bytes=10-19"); rec.Code != 206 || !bytes.Equal(rec.Body.Bytes(), data5[10:20]) {
		t.Errorf("5.1 from the cache: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	if rec := get(Path(testID), "bytes=10-19"); rec.Code != 206 || !bytes.Equal(rec.Body.Bytes(), data[10:20]) {
		t.Errorf("stereo from the cache: %d, %d bytes", rec.Code, rec.Body.Len())
	}
}

// TestProxyKeepsStereoOnly serves the stereo audio of a video kept, as one
// downloaded, from what is kept, and its 5.1 surround audio, which no
// download holds, from YouTube: once the network is gone, that fails, for
// the player to play the stereo audio kept instead.
func TestProxyKeepsStereoOnly(t *testing.T) {
	var minGen atomic.Int32
	srv, data := upstream(t, 3000, &minGen)
	srv5, data5 := upstream(t, 5000, &minGen)
	for i := range data5 {
		data5[i] ^= 0x55 // not the stereo audio
	}
	p, _ := surroundProxy(srv, srv5, len(data), len(data5), true)
	kept := []byte("the stereo audio downloaded")
	p.Kept = keptStore{testID: string(kept)}
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec
	}
	if rec := get(Path(testID)); rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), kept) {
		t.Fatalf("stereo: %d, %q", rec.Code, rec.Body.Bytes())
	}
	if rec := get(SurroundPath(testID)); rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), data5) {
		t.Fatalf("5.1: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	srv.Close()
	srv5.Close() // the network is gone
	if rec := get(SurroundPath(testID)); rec.Code == 200 {
		t.Errorf("5.1 without the network: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	if rec := get(Path(testID)); rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), kept) {
		t.Errorf("stereo without the network: %d, %q", rec.Code, rec.Body.Bytes())
	}
}

func TestProxySaysThereIsNoSurround(t *testing.T) {
	var minGen atomic.Int32
	srv, _ := upstream(t, 3000, &minGen)
	p, _ := surroundProxy(srv, nil, 3000, 0, false)
	s, err := p.OpenSurround(context.Background(), testID, nil)
	if err == nil {
		s.Close()
		t.Fatal("5.1 of a video without any")
	}
	if err.Error() != errNoSurround.Error() || p.Failure(testID) != errNoSurround.Error() {
		t.Errorf("failed with %q, the proxy tells %q", err, p.Failure(testID))
	}
	// The stereo audio plays.
	st, err := p.Open(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if b, err := io.ReadAll(st); err != nil || len(b) != 3000 {
		t.Errorf("stereo: %d bytes, %v", len(b), err)
	}
}

func TestSourceOpensSurround(t *testing.T) {
	var minGen atomic.Int32
	srv, _ := upstream(t, 3000, &minGen)
	srv5, data5 := upstream(t, chunkSize+777, &minGen)
	p, _ := surroundProxy(srv, srv5, 3000, len(data5), true)
	s, err := p.OpenSurround(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Seek(int64(chunkSize), io.SeekStart); err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(s)
	if err != nil || !bytes.Equal(b, data5[chunkSize:]) {
		t.Errorf("%d bytes from a seek, %v", len(b), err)
	}
}

// TestLiveHasNoSurround reads what yt-dlp prints of a live video whose
// formats list a 5.1 one: the live stream plays in stereo, with no 5.1
// audio beside it, which would have a length and seeking it hasn't.
func TestLiveHasNoSurround(t *testing.T) {
	out := fmt.Appendf(nil, `{"format_id": "234", "url": "https://example.com/hls_playlist/expire/%d/itag/234/index.m3u8",
		"ext": "mp4", "acodec": null, "protocol": "m3u8_native", "live_status": "is_live", "duration": null, "formats": [%s]}`,
		time.Now().Add(6*time.Hour).Unix(), ytSurround("380", "ac-3", 384, ""))
	s, err := parseInfo(out)
	if err != nil || !s.Live || s.Surround != nil {
		t.Errorf("stream %+v, %v", s, err)
	}
}
