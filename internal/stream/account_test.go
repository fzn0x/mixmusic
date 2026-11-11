package stream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/tools"
)

// testSession is a made-up signed-in session of YouTube Music, as the app
// keeps one: LOGIN_INFO and SAPISID, which yt-dlp needs to ask as the
// account, a cookie of the browser's session, one that expired, and one
// whose value would break the file's line.
func testSession() []*http.Cookie {
	return []*http.Cookie{
		{Name: "LOGIN_INFO", Value: "madeup-login", Domain: ".youtube.com", Path: "/", Secure: true, HttpOnly: true,
			Expires: time.Unix(1893456000, 0)},
		{Name: "SAPISID", Value: "madeup-sapisid/AbC", Domain: ".youtube.com", Path: "/", Secure: true,
			Expires: time.Unix(1893456000, 0)},
		{Name: "YSC", Value: "madeup-ysc", Domain: "youtube.com"},
		{Name: "OLD", Value: "gone", Expires: time.Unix(1000, 0)},
		{Name: "BROKEN", Value: "a\tb"},
		{Name: "SAPISID", Value: "a second one"},
	}
}

// premiumAnswer is testdata's ytdlp-premium.json, yt-dlp's answer for a
// Premium account, whose URLs expire in six hours.
func premiumAnswer(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/ytdlp-premium.json")
	if err != nil {
		t.Fatal(err)
	}
	return bytes.ReplaceAll(raw, []byte("expire=1793591621"), fmt.Appendf(nil, "expire=%d", time.Now().Add(6*time.Hour).Unix()))
}

// accountYTDLP is a fake yt-dlp that answers a run with --cookies as
// asAccount does, and one without as signedOut does, keeping each run's
// arguments, and, of a run as the account, the file of cookies it read
// and whether that file was readable by the user alone.
type accountYTDLP struct {
	asAccount, signedOut func(ctx context.Context) ([]byte, error)

	mu      sync.Mutex
	calls   [][]string
	session string
	file    string
	private bool
}

func (y *accountYTDLP) run(ctx context.Context, args ...string) ([]byte, error) {
	y.mu.Lock()
	y.calls = append(y.calls, args)
	y.mu.Unlock()
	i := slices.Index(args, "--cookies")
	if i < 0 {
		return y.signedOut(ctx)
	}
	file := args[i+1]
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(file)
	if err != nil {
		return nil, err
	}
	y.mu.Lock()
	y.session, y.file = string(data), file
	y.private = runtime.GOOS == "windows" || info.Mode().Perm() == 0o600
	y.mu.Unlock()
	return y.asAccount(ctx)
}

func (y *accountYTDLP) runs() [][]string {
	y.mu.Lock()
	defer y.mu.Unlock()
	return slices.Clone(y.calls)
}

// accountResolver is a resolver whose yt-dlp, with a JavaScript runtime, is
// y, asking as the account of testSession.
func accountResolver(t *testing.T, y *accountYTDLP) *Resolver {
	t.Helper()
	m := toolsWithRuntime(t)
	m.Configure(yes(t), "deno")
	r := NewResolver(m, "")
	r.Run = y.run
	r.Session = testSession
	r.SetAsAccount(true)
	return r
}

// TestAccountStreamAsksWithTheSession has yt-dlp find the stream as the
// account: the full way, with the JavaScript runtime and its default
// clients, and the session in a file of the Netscape format, readable by
// the user alone and gone once yt-dlp ran. The stream is Premium's Opus
// of format 774, marked as found as the account, and the log tells how it
// went, never the session.
func TestAccountStreamAsksWithTheSession(t *testing.T) {
	logs := captureLog(t)
	y := &accountYTDLP{
		asAccount: func(ctx context.Context) ([]byte, error) {
			if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > accountTimeout {
				t.Errorf("the run as the account may take until %v", deadline)
			}
			return premiumAnswer(t), nil
		},
		signedOut: func(context.Context) ([]byte, error) { return nil, errors.New("asked signed out") },
	}
	r := accountResolver(t, y)
	s, err := r.Resolve(context.Background(), songID)
	if err != nil {
		t.Fatal(err)
	}
	if !s.AsAccount || s.Codec != "opus" || s.Itag() != 774 || s.Bitrate < 250 || s.MimeType != `audio/webm; codecs="opus"` {
		t.Errorf("stream %+v", s)
	}
	runs := y.runs()
	if len(runs) != 1 {
		t.Fatalf("yt-dlp ran %d times: %q", len(runs), runs)
	}
	args := runs[0]
	if i := slices.Index(args, "-f"); i < 0 || args[i+1] != formats[QualityBest] || !slices.Contains(args, "-J") ||
		!slices.Contains(args, "--js-runtimes") || slices.Contains(args, "--extractor-args") {
		t.Errorf("the run as the account: %q", args)
	}
	if n := len(args); n < 2 || args[n-2] != "--" || args[n-1] != "https://music.youtube.com/watch?v="+songID {
		t.Errorf("the run as the account's URL: %q", args)
	}

	y.mu.Lock()
	session, file, private := y.session, y.file, y.private
	y.mu.Unlock()
	want := tools.NetscapeHeader +
		".youtube.com\tTRUE\t/\tTRUE\t1893456000\tLOGIN_INFO\tmadeup-login\n" +
		".youtube.com\tTRUE\t/\tTRUE\t1893456000\tSAPISID\tmadeup-sapisid/AbC\n" +
		".youtube.com\tTRUE\t/\tTRUE\t0\tYSC\tmadeup-ysc\n"
	if session != want {
		t.Errorf("yt-dlp read the session\n%q\nwant\n%q", session, want)
	}
	if !strings.HasPrefix(filepath.Base(file), SessionFilePrefix) || !private {
		t.Errorf("the session was in %s, private %v", file, private)
	}
	if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the session's file stays after the run: %v", err)
	}

	if got := logs.String(); !strings.Contains(got, "found the stream of "+songID+" as your YouTube Music account in ") ||
		!strings.Contains(got, "opus at 256 kbit/s") || strings.Contains(got, "madeup") {
		t.Errorf("the log:\n%s", got)
	}
}

// TestAccountStreamFallsBack finds the stream signed out, as without the
// switch, when the run as the account fails, takes too long, or finds
// nothing the engine plays: the direct route first, then yt-dlp signed
// out, without the session.
func TestAccountStreamFallsBack(t *testing.T) {
	old := accountTimeout
	accountTimeout = 200 * time.Millisecond
	t.Cleanup(func() { accountTimeout = old })
	expires := time.Now().Add(6 * time.Hour).Unix()
	for name, answer := range map[string]func(ctx context.Context) ([]byte, error){
		"a bot check": func(context.Context) ([]byte, error) {
			return nil, failure{"YouTube asks to confirm this is not a bot; try again in a while", ErrThrottled}
		},
		"Premium only": func(context.Context) ([]byte, error) {
			return nil, failure{"only YouTube Music Premium members can play this track", ErrUnavailable}
		},
		"no audio": func(context.Context) ([]byte, error) { return []byte(`{"formats": []}`), nil },
		"not JSON": func(context.Context) ([]byte, error) { return []byte("<html>"), nil },
		"HLS": func(context.Context) ([]byte, error) {
			return []byte(`{"url": "https://example.com/a.m3u8", "protocol": "m3u8_native"}`), nil
		},
		"too long":   func(ctx context.Context) ([]byte, error) { <-ctx.Done(); return nil, ctx.Err() },
		"no yt-dlp":  func(context.Context) ([]byte, error) { return nil, tools.ErrNoYTDLP },
		"live":       func(context.Context) ([]byte, error) { return liveAnswer(expires), nil },
		"HE-AAC":     func(context.Context) ([]byte, error) { return codecAnswer("m4a", "mp4a.40.5", expires), nil },
		"its own":    func(context.Context) ([]byte, error) { return codecAnswer("webm", "vorbis", expires), nil },
		"a crash":    func(context.Context) ([]byte, error) { return nil, errors.New("yt-dlp: exit status 1") },
		"no session": nil,
	} {
		logs := captureLog(t)
		var runs atomic.Int32
		signedOut := fakeYTDLP("https://example.com/audio", &runs)
		y := &accountYTDLP{asAccount: answer, signedOut: func(ctx context.Context) ([]byte, error) { return signedOut(ctx, "--") }}
		r := accountResolver(t, y)
		if answer == nil {
			r.Session = func() []*http.Cookie { return nil }
		}
		r.Direct = refusedDirect(t)
		s, err := r.Resolve(context.Background(), songID)
		if err != nil || s.AsAccount || s.Codec != "opus" || s.Header.Get("User-Agent") != "test" {
			t.Errorf("%s: stream %+v, %v", name, s, err)
			continue
		}
		all := y.runs()
		asked := 0
		if answer != nil {
			asked = 1
			if len(all) == 0 || !slices.Contains(all[0], "--cookies") {
				t.Errorf("%s: yt-dlp ran %q", name, all)
				continue
			}
		}
		for _, args := range all[asked:] {
			if slices.Contains(args, "--cookies") {
				t.Errorf("%s: a run after the one as the account had the session: %q", name, args)
			}
		}
		if len(all) <= asked {
			t.Errorf("%s: nothing asked signed out: %q", name, all)
		}
		if answer != nil && !strings.Contains(logs.String(), "yt-dlp found no stream of "+songID+" as your YouTube Music account in ") {
			t.Errorf("%s: the log doesn't tell:\n%s", name, logs.String())
		}
	}
}

// liveAnswer is yt-dlp's answer of a live video, as it goes on.
func liveAnswer(expires int64) []byte {
	return fmt.Appendf(nil, `{"url": "https://example.com/live.m3u8?expire=%d", "protocol": "m3u8_native", "live_status": "is_live"}`, expires)
}

// codecAnswer is yt-dlp's answer of a stream of a codec.
func codecAnswer(ext, codec string, expires int64) []byte {
	return fmt.Appendf(nil, `{"url": "https://example.com/audio?expire=%d", "ext": %q, "acodec": %q, "abr": 48, "filesize": 1000}`,
		expires, ext, codec)
}

// TestAccountStreamFallsBackToTheDirectRoute finds the stream through
// YouTube's player API, as every stream is found signed out, when the run
// as the account fails.
func TestAccountStreamFallsBackToTheDirectRoute(t *testing.T) {
	y := &accountYTDLP{
		asAccount: func(context.Context) ([]byte, error) { return nil, errors.New("yt-dlp: exit status 1") },
		signedOut: func(context.Context) ([]byte, error) { return nil, errors.New("yt-dlp ran signed out") },
	}
	r := accountResolver(t, y)
	r.Direct = newFakeTube(t).direct()
	s, err := r.Resolve(context.Background(), songID)
	if err != nil || s.AsAccount || !s.Measured {
		t.Fatalf("stream %+v, %v", s, err)
	}
	if runs := y.runs(); len(runs) != 1 || !slices.Contains(runs[0], "--cookies") {
		t.Errorf("yt-dlp ran %q", runs)
	}
}

// TestAccountStreamOnlyWhenAsked asks signed out, without running yt-dlp
// as the account, while the switch is off, with data saver, which gains
// nothing from Premium, without a session, or one without LOGIN_INFO,
// which yt-dlp wouldn't ask as, and without a JavaScript runtime, which
// the clients it asks as an account need.
func TestAccountStreamOnlyWhenAsked(t *testing.T) {
	for name, set := range map[string]func(r *Resolver){
		"off":        func(r *Resolver) { r.SetAsAccount(false) },
		"data saver": func(r *Resolver) { r.SetQuality(QualitySaver) },
		"no Session": func(r *Resolver) { r.Session = nil },
		"signed out": func(r *Resolver) { r.Session = func() []*http.Cookie { return nil } },
		"no runtime": func(r *Resolver) { r.Tools.Configure(yes(t), "none-such") },
		"no yt-dlp":  func(r *Resolver) { r.Tools.Configure(filepath.Join(t.TempDir(), "yt-dlp"), "deno") },
		"no LOGIN_INFO": func(r *Resolver) {
			r.Session = func() []*http.Cookie { return []*http.Cookie{{Name: "SAPISID", Value: "madeup"}} }
		},
		"no SAPISID": func(r *Resolver) {
			r.Session = func() []*http.Cookie { return []*http.Cookie{{Name: "LOGIN_INFO", Value: "madeup"}} }
		},
	} {
		logs := captureLog(t)
		var runs atomic.Int32
		signedOut := fakeYTDLP("https://example.com/audio", &runs)
		y := &accountYTDLP{
			asAccount: func(context.Context) ([]byte, error) { return premiumAnswer(t), nil },
			signedOut: func(ctx context.Context) ([]byte, error) { return signedOut(ctx, "--") },
		}
		r := accountResolver(t, y)
		set(r)
		for _, id := range []string{songID, testID} {
			s, err := r.Resolve(context.Background(), id)
			if err != nil || s.AsAccount {
				t.Errorf("%s: stream %+v, %v", name, s, err)
			}
		}
		for _, args := range y.runs() {
			if slices.Contains(args, "--cookies") {
				t.Errorf("%s: yt-dlp ran as the account: %q", name, args)
			}
		}
		told := strings.Count(logs.String(), "lacks its LOGIN_INFO or SAPISID cookie")
		if want := map[bool]int{true: 1, false: 0}[strings.HasPrefix(name, "no LOGIN_INFO") || name == "no SAPISID"]; told != want {
			t.Errorf("%s: the log tells of the missing cookie %d times:\n%s", name, told, logs.String())
		}
	}
}

// yes is a program that is there, for a yt-dlp the tools find: the test
// binary itself.
func yes(t *testing.T) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return self
}

// TestAccountStreamRefusedGoesSignedOut finds the stream of a video
// signed out once YouTube refused the one found as the account, so that
// the track plays: others are still found as the account.
func TestAccountStreamRefusedGoesSignedOut(t *testing.T) {
	var runs atomic.Int32
	signedOut := fakeYTDLP("https://example.com/audio", &runs)
	y := &accountYTDLP{
		asAccount: func(context.Context) ([]byte, error) { return premiumAnswer(t), nil },
		signedOut: func(ctx context.Context) ([]byte, error) { return signedOut(ctx, "--") },
	}
	r := accountResolver(t, y)
	if s, err := r.Resolve(context.Background(), songID); err != nil || !s.AsAccount {
		t.Fatalf("first: %+v, %v", s, err)
	}
	r.Forget(songID) // as after YouTube refused its URL
	if s, err := r.Resolve(context.Background(), songID); err != nil || s.AsAccount {
		t.Errorf("after a refusal: %+v, %v", s, err)
	}
	if s, err := r.Resolve(context.Background(), testID); err != nil || !s.AsAccount {
		t.Errorf("another video: %+v, %v", s, err)
	}
	// Its stream found signed out, refused too, is found signed out again.
	r.Forget(songID)
	if s, err := r.Resolve(context.Background(), songID); err != nil || s.AsAccount {
		t.Errorf("after a second refusal: %+v, %v", s, err)
	}
}

// TestSetAsAccountFindsStreamsAgain finds again, as the account, a stream
// found signed out before the switch was turned on, and signed out once
// it is off again.
func TestSetAsAccountFindsStreamsAgain(t *testing.T) {
	var runs atomic.Int32
	signedOut := fakeYTDLP("https://example.com/audio", &runs)
	y := &accountYTDLP{
		asAccount: func(context.Context) ([]byte, error) { return premiumAnswer(t), nil },
		signedOut: func(ctx context.Context) ([]byte, error) { return signedOut(ctx, "--") },
	}
	r := accountResolver(t, y)
	r.SetAsAccount(false)
	for i, on := range []bool{false, true, true, false} {
		r.SetAsAccount(on)
		s, err := r.Resolve(context.Background(), songID)
		if err != nil || s.AsAccount != on {
			t.Errorf("%d, as the account %v: %+v, %v", i, on, s, err)
		}
	}
	if n := len(y.runs()); n != 3 {
		t.Errorf("yt-dlp ran %d times, want 3: the same switch keeps what was found", n)
	}
}

// TestAccountStreamThroughTheProxy plays a stream found as the account, and,
// once YouTube refuses its URL, the stream found signed out in its place.
func TestAccountStreamThroughTheProxy(t *testing.T) {
	var minGen atomic.Int32
	srv, data := upstream(t, 1000, &minGen)
	var gen atomic.Int32
	answer := func(asAccount bool) func(context.Context) ([]byte, error) {
		return func(context.Context) ([]byte, error) {
			g := gen.Add(1)
			itag := 251
			if asAccount {
				itag = 774
			}
			return fmt.Appendf(nil, `{"url": "%s/a?gen=%d&itag=%d&expire=%d", "ext": "webm", "acodec": "opus", "filesize": 1000}`,
				srv.URL, g, itag, time.Now().Add(6*time.Hour).Unix()), nil
		}
	}
	y := &accountYTDLP{asAccount: answer(true), signedOut: answer(false)}
	r := accountResolver(t, y)
	p := NewProxy(r)
	if rec := get(t, p, ""); rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), data) {
		t.Fatalf("as the account: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	if s, ok := r.Known(testID); !ok || !s.AsAccount || s.Itag() != 774 {
		t.Errorf("as the account, the resolver knows %+v", s)
	}
	minGen.Store(2) // YouTube refuses the URL found as the account
	if rec := get(t, p, "bytes=0-99"); rec.Code != 206 || !bytes.Equal(rec.Body.Bytes(), data[:100]) {
		t.Fatalf("after the refusal: %d %q", rec.Code, rec.Body.String())
	}
	if s, ok := r.Known(testID); !ok || s.AsAccount || s.Itag() != 251 {
		t.Errorf("after the refusal, the resolver knows %+v", s)
	}
}

// TestWriteSession writes the session in the Netscape format yt-dlp reads,
// each cookie for every host of youtube.com, without those that expired,
// a second of a name, or one that would break its line.
func TestWriteSession(t *testing.T) {
	now := time.Unix(1800000000, 0)
	file, err := writeSession(testSession(), now)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file)
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if lines[0]+"\n" != tools.NetscapeHeader || len(lines) != 4 {
		t.Fatalf("the file holds %q", data)
	}
	for _, line := range lines[1:] {
		f := strings.Split(line, "\t")
		if len(f) != 7 || f[0] != ".youtube.com" || f[1] != "TRUE" || f[2] != "/" || f[3] != "TRUE" {
			t.Errorf("line %q", line)
			continue
		}
		if _, err := strconv.ParseInt(f[4], 10, 64); err != nil {
			t.Errorf("line %q expires at %q", line, f[4])
		}
	}
	if !strings.Contains(string(data), "\tSAPISID\tmadeup-sapisid/AbC\n") || strings.Contains(string(data), "a second one") ||
		strings.Contains(string(data), "OLD") || strings.Contains(string(data), "BROKEN") {
		t.Errorf("the file holds %q", data)
	}
	if !signedInFor(testSession()) || signedInFor(testSession()[1:]) || signedInFor(nil) {
		t.Error("signedInFor doesn't tell what yt-dlp takes for a signed-in session")
	}
}
