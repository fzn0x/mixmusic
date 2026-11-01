package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/tools"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// cookieNames returns the names and values of cookies, in order.
func cookieNames(cookies []*http.Cookie) []string {
	var names []string
	for _, c := range cookies {
		names = append(names, c.Name+"="+c.Value)
	}
	return names
}

// TestYouTubeCookies keeps, of a browser's cookies, those a request to
// music.youtube.com sends, as the sign-in window keeps them.
func TestYouTubeCookies(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "cookies.txt"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1790000000, 0)
	got := youtubeCookies(data, now)
	want := []string{"SAPISID=yt-sapisid", "__Secure-3PAPISID=yt-3p", "SID=yt-sid", "YSC=browser-session", "HOSTONLY=music-only"}
	if names := cookieNames(got); !slices.Equal(names, want) {
		t.Fatalf("kept %q, want %q", names, want)
	}
	sapisid, threeP, sid, ysc, host := got[0], got[1], got[2], got[3], got[4]
	if sapisid.Domain != ".youtube.com" || sapisid.Path != "/" || !sapisid.Secure || sapisid.HttpOnly || !sapisid.Expires.Equal(time.Unix(4102444800, 0)) {
		t.Errorf("SAPISID = %+v", sapisid)
	}
	if !threeP.Expires.Equal(time.Unix(4102444800, 0)) {
		t.Errorf("an expiry with a fraction gave %v", threeP.Expires)
	}
	if !sid.HttpOnly {
		t.Error("a #HttpOnly_ line lost its HttpOnly")
	}
	// Cookies that end with the browser's session have no expiry, which
	// the client would take for long expired.
	if !ysc.Expires.IsZero() || ysc.Secure || !host.Expires.IsZero() || host.Domain != "music.youtube.com" {
		t.Errorf("session cookies = %+v, %+v", ysc, host)
	}

	crlf := strings.ReplaceAll(".youtube.com\tTRUE\t/\tTRUE\t0\tSAPISID\tx\n", "\n", "\r\n")
	if got := cookieNames(youtubeCookies([]byte(crlf), now)); !slices.Equal(got, []string{"SAPISID=x"}) {
		t.Errorf("Windows line ends gave %q", got)
	}
}

// TestPastedCookies reads what the user pastes: a Cookie header, with its
// name or without, wrapped or in the headers of its request, or the text of
// a cookies.txt file.
func TestPastedCookies(t *testing.T) {
	now := time.Unix(1790000000, 0)
	for text, want := range map[string][]string{
		"SAPISID=abc; SID=def":                 {"SAPISID=abc", "SID=def"},
		"Cookie: SAPISID=abc; SID=def;":        {"SAPISID=abc", "SID=def"},
		"  cookie:SAPISID=abc;;  SID=def ; \n": {"SAPISID=abc", "SID=def"},
		"SAPISID=abc; SID=de\r\nf":             {"SAPISID=abc", "SID=def"},
		"SAPISID=abc; bad name=1; ok=2; SID":   {"SAPISID=abc", "ok=2"},
		"SAPISID=abc; SAPISID=again":           {"SAPISID=abc"},
		`SAPISID="quoted"`:                     {"SAPISID=quoted"},
		"just some words":                      nil,
		":authority: music.youtube.com\ncookie: SAPISID=abc; x=y\nuser-agent: Mozilla/5.0":        {"SAPISID=abc", "x=y"},
		".google.com\tTRUE\t/\tTRUE\t0\tSAPISID\tg\n.youtube.com\tTRUE\t/\tTRUE\t0\tSAPISID\ty\n": {"SAPISID=y"},
	} {
		got := pastedCookies(text, now)
		if names := cookieNames(got); !slices.Equal(names, want) {
			t.Errorf("pasting %q kept %q, want %q", text, names, want)
		}
		for _, c := range got {
			if c.Domain != ".youtube.com" || c.Path != "/" || !c.Secure {
				t.Errorf("pasting %q gave %+v", text, c)
			}
		}
	}
}

func TestYouTubeBrowsers(t *testing.T) {
	names := func(goos string) []string {
		var n []string
		for _, b := range youtubeBrowsers(goos) {
			n = append(n, b.Name)
		}
		return n
	}
	want := []string{"Firefox", "Chrome", "Chromium", "Edge", "Brave", "Opera", "Vivaldi", "Helium", "Zen"}
	if got := names("windows"); !slices.Equal(got, want) {
		t.Errorf("on Windows: %q", got)
	}
	if got := names("darwin"); !slices.Equal(got, append(want, "Safari")) {
		t.Errorf("on macOS: %q", got)
	}
}

func browserOf(t *testing.T, id string) cookieBrowser {
	t.Helper()
	for _, b := range cookieBrowsers {
		if b.id == id {
			return b
		}
	}
	t.Fatalf("no browser %s", id)
	return cookieBrowser{}
}

// TestBrowserSpec names the browser and its profile as yt-dlp does: the
// cookies of no Firefox container, and the profiles of the browsers yt-dlp
// doesn't know by name in their folders.
func TestBrowserSpec(t *testing.T) {
	env := map[string]string{"APPDATA": `C:\Users\u\AppData\Roaming`, "LOCALAPPDATA": `C:\Users\u\AppData\Local`}
	getenv := func(k string) string { return env[k] }
	there := map[string]bool{}
	saved := isDir
	isDir = func(p string) bool { return there[p] }
	t.Cleanup(func() { isDir = saved })

	for _, c := range []struct{ browser, profile, want string }{
		{"firefox", "", "firefox::none"},
		{"firefox", "  abc.default-release ", "firefox:abc.default-release::none"},
		{"firefox", `C:\Profiles\x.default`, `firefox:C:\Profiles\x.default::none`},
		{"chrome", "", "chrome"},
		{"chrome", "Profile 1", "chrome:Profile 1"},
		{"edge", "", "edge"},
		{"safari", "anything", "safari"},
	} {
		got, err := browserOf(t, c.browser).spec(c.profile, "windows", `C:\Users\u`, getenv)
		if err != nil || got != c.want {
			t.Errorf("%s %q: %q, %v; want %q", c.browser, c.profile, got, err, c.want)
		}
	}

	zen, helium := browserOf(t, "zen"), browserOf(t, "helium")
	if _, err := zen.spec("", "windows", `C:\Users\u`, getenv); err == nil || !strings.Contains(err.Error(), "profiles of Zen") {
		t.Errorf("Zen without its folder: %v", err)
	}
	there[filepath.Join(env["APPDATA"], "zen")] = true
	if got, err := zen.spec("", "windows", `C:\Users\u`, getenv); err != nil || got != "firefox:"+filepath.Join(env["APPDATA"], "zen")+"::none" {
		t.Errorf("Zen = %q, %v", got, err)
	}
	if got, err := zen.spec("/somewhere/zen", "linux", "/home/u", getenv); err != nil || got != "firefox:/somewhere/zen::none" {
		t.Errorf("Zen in a folder given = %q, %v", got, err)
	}
	linux := filepath.Join("/home/u", ".config", "net.imput.helium", "Default")
	there[linux] = true
	if got, err := helium.spec("", "linux", "/home/u", getenv); err != nil || got != "chromium:"+linux {
		t.Errorf("Helium = %q, %v", got, err)
	}
	mac := zenDirs("darwin", "/Users/u", getenv)
	if len(mac) != 1 || mac[0] != filepath.Join("/Users/u", "Library", "Application Support", "zen") {
		t.Errorf("Zen's folders on macOS: %q", mac)
	}
}

// cookieApp is an app whose YouTube Music is answer, with a store of the
// YouTube session of its own, and the temporary folder of the test.
func cookieApp(t *testing.T, answer http.HandlerFunc) (*App, *atomic.Int32) {
	t.Helper()
	tmp := t.TempDir()
	for _, v := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(v, tmp)
	}
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		answer(w, r)
	}))
	t.Cleanup(srv.Close)
	yt := youtube.New()
	yt.BaseURL = srv.URL
	set, err := store.Open(filepath.Join(t.TempDir(), "settings.json"), settings.Default())
	if err != nil {
		t.Fatal(err)
	}
	auth := spotify.NewAuth("client")
	a := &App{auth: auth, spotify: spotify.NewClient(auth), youtube: yt, settings: set,
		youtubeStore: testYouTube(t, newKeeper(), nil), tools: tools.New(t.TempDir())}
	a.watchYouTube()
	return a, &requests
}

// signedIn answers as YouTube Music does to a session it takes, or to one
// it doesn't, and changes a cookie of the session meanwhile.
func signedIn(taken bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "SIDCC", Value: "changed", Path: "/"})
		if !taken {
			w.Write([]byte(`{"responseContext":{"serviceTrackingParams":[{"params":[{"key":"logged_in","value":"0"}]}]}}`))
			return
		}
		w.Write([]byte(`{"responseContext":{"serviceTrackingParams":[{"params":[{"key":"logged_in","value":"1"}]}]},
			"actions":[{"openPopupAction":{"popup":{"multiPageMenuRenderer":{"header":{"activeAccountHeaderRenderer":{
				"accountName":{"runs":[{"text":"Ada"}]},"channelHandle":{"runs":[{"text":"@ada"}]}}}}}}}]}`))
	}
}

// fakeExport has yt-dlp's export of a browser write jar, or fail with
// fail, checking the file it gets as yt-dlp would: no browser is read.
func fakeExport(t *testing.T, jar string, undecrypted bool, fail error) *[]string {
	t.Helper()
	var browsers []string
	saved := exportCookies
	exportCookies = func(_ context.Context, _ *tools.Manager, browser, file string) (bool, error) {
		browsers = append(browsers, browser)
		if data, err := os.ReadFile(file); err != nil || string(data) != tools.NetscapeHeader {
			t.Errorf("yt-dlp got %q, %v", data, err)
		}
		if info, err := os.Stat(file); runtime.GOOS != "windows" && (err != nil || info.Mode().Perm() != 0o600) {
			t.Errorf("the file of the cookies is %v, %v", info.Mode(), err)
		}
		if fail != nil {
			return false, fail
		}
		return undecrypted, os.WriteFile(file, []byte(tools.NetscapeHeader+jar), 0o600)
	}
	t.Cleanup(func() { exportCookies = saved })
	return &browsers
}

// leftExports returns the copies of browsers' cookies left in the
// temporary folder.
func leftExports(t *testing.T) []string {
	left, err := filepath.Glob(filepath.Join(os.TempDir(), cookieExportPrefix+"*"))
	if err != nil {
		t.Fatal(err)
	}
	return left
}

// TestImportCookies signs in with the YouTube Music session of a browser:
// it keeps YouTube's cookies alone, encrypted, and deletes the browser's
// copy at once.
func TestImportCookies(t *testing.T) {
	jar, err := os.ReadFile(filepath.Join("testdata", "cookies.txt"))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := cookieApp(t, signedIn(true))
	set := a.settings.Get()
	set.YouTubeChannel = "123456789012345678901"
	a.settings.Set(set)
	a.youtube.SetAuthUser(3)
	browsers := fakeExport(t, string(jar), false, nil)

	status, err := (&Accounts{a}).YouTubeImportCookies(context.Background(), "firefox", "")
	if err != nil {
		t.Fatal(err)
	}
	if !status.YouTube.SignedIn || status.YouTube.Account == nil || status.YouTube.Account.Name != "Ada" {
		t.Errorf("status = %+v", status.YouTube)
	}
	if !slices.Equal(*browsers, []string{"firefox::none"}) {
		t.Errorf("yt-dlp read %q", *browsers)
	}
	if left := leftExports(t); len(left) != 0 {
		t.Errorf("the browser's cookies were left in %q", left)
	}
	kept, err := a.youtubeStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"SAPISID=yt-sapisid", "__Secure-3PAPISID=yt-3p", "SID=yt-sid", "YSC=browser-session", "HOSTONLY=music-only", "SIDCC=changed"}
	if got := cookieNames(kept.cookies); !slices.Equal(got, want) {
		t.Errorf("kept %q, want %q", got, want)
	}
	// A new session starts as its first account, itself.
	if kept.authUser != 0 || a.youtube.AuthUser() != 0 || a.youtube.Channel() != "" || a.settings.Get().YouTubeChannel != "" {
		t.Errorf("signed in as account %d, channel %q", a.youtube.AuthUser(), a.settings.Get().YouTubeChannel)
	}
}

// TestImportRefused keeps nothing of a session YouTube Music doesn't take,
// not even the cookies it changed while it was asked, and says so rather
// than sign in.
func TestImportRefused(t *testing.T) {
	a, _ := cookieApp(t, signedIn(false))
	fakeExport(t, ".youtube.com\tTRUE\t/\tTRUE\t0\tSAPISID\tstale\n", false, nil)
	_, err := (&Accounts{a}).YouTubeImportCookies(context.Background(), "chrome", "Profile 1")
	if !errors.Is(err, youtube.ErrSessionRejected) || !strings.Contains(err.Error(), "did not accept the session") {
		t.Errorf("a session YouTube Music refused gave %v", err)
	}
	if a.youtube.SignedIn() {
		t.Error("signed in with a session YouTube Music refused")
	}
	if _, err := a.youtubeStore.Load(); err == nil {
		t.Error("kept a session YouTube Music refused")
	}
	if left := leftExports(t); len(left) != 0 {
		t.Errorf("the browser's cookies were left in %q", left)
	}
}

// TestImportProblems says why there was nothing to sign in with, in the
// words of the interface, and asks YouTube Music nothing.
func TestImportProblems(t *testing.T) {
	for _, c := range []struct {
		name, browser, jar string
		undecrypted        bool
		fail               error
		want               string
	}{
		{"no session", "firefox", ".youtube.com\tTRUE\t/\tTRUE\t0\tVISITOR_INFO1_LIVE\tv\n", false, nil, "Firefox has no signed-in YouTube Music session"},
		{"only Google's", "firefox", ".google.com\tTRUE\t/\tTRUE\t0\tSAPISID\tg\n", false, nil, "Firefox has no signed-in YouTube Music session"},
		{"undecrypted", "chrome", "", true, nil, "Chrome seals its cookies"},
		{"sealed", "chrome", "", false, tools.ErrCookiesSealed, "Chrome seals its cookies"},
		{"open", "edge", "", false, tools.ErrBrowserOpen, "Edge is open"},
		{"none", "brave", "", false, tools.ErrNoBrowserCookies, "found no cookies of Brave: is it installed?"},
		{"no yt-dlp", "vivaldi", "", false, tools.ErrNoYTDLP, "yt-dlp is not installed"},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, requests := cookieApp(t, signedIn(true))
			fakeExport(t, c.jar, c.undecrypted, c.fail)
			_, err := (&Accounts{a}).YouTubeImportCookies(context.Background(), c.browser, "")
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("%v, want %q", err, c.want)
			}
			if c.fail != nil && !errors.Is(err, c.fail) {
				t.Errorf("%v is not %v", err, c.fail)
			}
			if requests.Load() != 0 || a.youtube.SignedIn() {
				t.Errorf("%d requests, signed in %v", requests.Load(), a.youtube.SignedIn())
			}
		})
	}
	a, _ := cookieApp(t, signedIn(true))
	if _, err := (&Accounts{a}).YouTubeImportCookies(context.Background(), "netscape", ""); err == nil {
		t.Error("imported from a browser that isn't one")
	}
	if runtime.GOOS != "darwin" {
		if _, err := (&Accounts{a}).YouTubeImportCookies(context.Background(), "safari", ""); err == nil {
			t.Error("imported from Safari, away from macOS")
		}
	}
}

func TestPasteCookies(t *testing.T) {
	a, requests := cookieApp(t, signedIn(true))
	accounts := &Accounts{a}
	if _, err := accounts.YouTubePasteCookies(context.Background(), "PREF=f6=40000000; VISITOR_INFO1_LIVE=v"); err == nil || !strings.Contains(err.Error(), "isn't a signed-in YouTube Music cookie header") {
		t.Errorf("a header without a session gave %v", err)
	}
	if requests.Load() != 0 {
		t.Errorf("asked YouTube Music %d times about a header without a session", requests.Load())
	}
	status, err := accounts.YouTubePasteCookies(context.Background(), "Cookie: SAPISID=pasted; SID=s;")
	if err != nil || !status.YouTube.SignedIn {
		t.Fatalf("pasting a session: %+v, %v", status.YouTube, err)
	}
	kept, err := a.youtubeStore.Load()
	if err != nil || !slices.Contains(cookieNames(kept.cookies), "SAPISID=pasted") {
		t.Errorf("kept %q, %v", cookieNames(kept.cookies), err)
	}
}

// TestRemoveCookieExports deletes the copies of browsers' cookies that a
// MixMusic that stopped while it imported one left, and nothing else.
func TestRemoveCookieExports(t *testing.T) {
	tmp := t.TempDir()
	for _, v := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(v, tmp)
	}
	left := filepath.Join(tmp, cookieExportPrefix+"123.txt")
	other := filepath.Join(tmp, "other.txt")
	for _, f := range []string{left, other} {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	removeCookieExports()
	if _, err := os.Stat(left); !errors.Is(err, os.ErrNotExist) {
		t.Error("a copy of a browser's cookies stayed")
	}
	if _, err := os.Stat(other); err != nil {
		t.Error("another file went")
	}
}

// TestChooseAccountAndChannel uses YouTube Music as a channel of another
// Google account of the session, and keeps the account beside the
// session, through the cookies YouTube changes.
func TestChooseAccountAndChannel(t *testing.T) {
	var last atomic.Pointer[http.Request]
	accounts := `{"actions":[{"getMultiPageMenuAction":{"menu":{"multiPageMenuRenderer":{"sections":[
		{"accountSectionListRenderer":{"header":{"googleAccountHeaderRenderer":{"email":{"simpleText":"ada@example.com"}}},
			"contents":[{"accountItemSectionRenderer":{"contents":[{"accountItem":{"accountName":{"simpleText":"Ada"},
				"serviceEndpoint":{"selectActiveIdentityEndpoint":{"supportedTokens":[{"accountSigninToken":{"signinUrl":"/signin?authuser=0"}}]}}}}]}}]}},
		{"accountSectionListRenderer":{"header":{"googleAccountHeaderRenderer":{"email":{"simpleText":"bob@example.com"}}},
			"contents":[{"accountItemSectionRenderer":{"contents":[{"accountItem":{"accountName":{"simpleText":"Bob's Band"},
				"serviceEndpoint":{"selectActiveIdentityEndpoint":{"supportedTokens":[{"pageIdToken":{"pageId":"222222222222222222222"}},
					{"accountSigninToken":{"signinUrl":"/signin?pageid=222222222222222222222&authuser=1"}}]}}}}]}}]}}
	]}}}}]}`
	menu := signedIn(true)
	a, _ := cookieApp(t, func(w http.ResponseWriter, r *http.Request) {
		last.Store(r)
		if strings.HasSuffix(r.URL.Path, "/accounts_list") {
			http.SetCookie(w, &http.Cookie{Name: "SIDCC", Value: "rotated", Path: "/"})
			w.Write([]byte(accounts))
			return
		}
		menu(w, r)
	})
	s := &Accounts{a}
	ctx := context.Background()
	if _, err := s.YouTubePasteCookies(ctx, "SAPISID=pasted"); err != nil {
		t.Fatal(err)
	}
	list, err := s.YouTubeChannels(ctx)
	if err != nil || len(list) != 2 || !list[0].Current || list[1].Current || list[1].AuthUser != 1 || list[1].Email != "bob@example.com" {
		t.Fatalf("channels = %+v, %v", list, err)
	}

	band := list[1]
	if _, err := s.SetYouTubeChannel(ctx, band.AuthUser, band.PageID); err != nil {
		t.Fatal(err)
	}
	if r := last.Load(); r.Header.Get("X-Goog-AuthUser") != "1" || r.Header.Get("X-Goog-PageId") != band.PageID {
		t.Errorf("the account was read as %q, %q", r.Header.Get("X-Goog-AuthUser"), r.Header.Get("X-Goog-PageId"))
	}
	if a.settings.Get().YouTubeChannel != band.PageID {
		t.Errorf("the channel kept is %q", a.settings.Get().YouTubeChannel)
	}
	if list, _ := s.YouTubeChannels(ctx); list[0].Current || !list[1].Current {
		t.Errorf("after choosing: %+v", list)
	}
	// YouTube changed a cookie meanwhile: the account stays with it.
	kept, err := a.youtubeStore.Load()
	if err != nil || kept.authUser != 1 || !slices.Contains(cookieNames(kept.cookies), "SIDCC=rotated") {
		t.Errorf("kept account %d with %q, %v", kept.authUser, cookieNames(kept.cookies), err)
	}

	// A channel that isn't one of the session any more is none of the list.
	if _, err := s.SetYouTubeChannel(ctx, 1, "333333333333333333333"); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.YouTubeChannels(ctx); list[0].Current || list[1].Current {
		t.Errorf("a channel gone is current: %+v", list)
	}
	if _, err := s.SetYouTubeChannel(ctx, -1, ""); err == nil {
		t.Error("chose an account that can't be")
	}
}
