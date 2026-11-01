package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/tools"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// YouTubeBrowser is a browser whose YouTube Music session MixMusic can
// import.
type YouTubeBrowser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// cookieBrowser is a browser whose cookies yt-dlp reads: engine is its
// name in yt-dlp, or the name of the browser it is made from, with dirs
// where it keeps its profiles, as yt-dlp doesn't know it by name.
type cookieBrowser struct {
	id, name, engine string
	dirs             func(goos, home string, getenv func(string) string) []string
	// darwin is set for Safari, which is on macOS alone.
	darwin bool
}

var cookieBrowsers = []cookieBrowser{
	{id: "firefox", name: "Firefox", engine: "firefox"},
	{id: "chrome", name: "Chrome", engine: "chrome"},
	{id: "chromium", name: "Chromium", engine: "chromium"},
	{id: "edge", name: "Edge", engine: "edge"},
	{id: "brave", name: "Brave", engine: "brave"},
	{id: "opera", name: "Opera", engine: "opera"},
	{id: "vivaldi", name: "Vivaldi", engine: "vivaldi"},
	{id: "helium", name: "Helium", engine: "chromium", dirs: heliumDirs},
	{id: "zen", name: "Zen", engine: "firefox", dirs: zenDirs},
	{id: "safari", name: "Safari", engine: "safari", darwin: true},
}

// zenDirs are where Zen, made from Firefox, keeps its profiles: yt-dlp
// reads the newest of those in the folder.
func zenDirs(goos, home string, getenv func(string) string) []string {
	switch goos {
	case "windows":
		return []string{filepath.Join(getenv("APPDATA"), "zen")}
	case "darwin":
		return []string{filepath.Join(home, "Library", "Application Support", "zen")}
	}
	return []string{filepath.Join(home, ".zen"), filepath.Join(home, ".config", "zen"),
		filepath.Join(home, ".var", "app", "app.zen_browser.zen", ".zen")}
}

// heliumDirs are where Helium, made from Chromium, keeps its first profile:
// yt-dlp finds the key of its cookies in the folder above.
func heliumDirs(goos, home string, getenv func(string) string) []string {
	switch goos {
	case "windows":
		local := getenv("LOCALAPPDATA")
		return []string{filepath.Join(local, "imput", "Helium", "User Data", "Default"),
			filepath.Join(local, "Helium", "User Data", "Default")}
	case "darwin":
		return []string{filepath.Join(home, "Library", "Application Support", "net.imput.helium", "Default")}
	}
	return []string{filepath.Join(home, ".config", "net.imput.helium", "Default"),
		filepath.Join(home, ".config", "helium", "Default")}
}

// isDir reports whether a folder is there. It is a variable for tests.
var isDir = func(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// YouTubeBrowsers returns the browsers whose YouTube Music session MixMusic
// can import on this system.
func (s *Accounts) YouTubeBrowsers() []YouTubeBrowser {
	return youtubeBrowsers(runtime.GOOS)
}

func youtubeBrowsers(goos string) []YouTubeBrowser {
	var list []YouTubeBrowser
	for _, b := range cookieBrowsers {
		if !b.darwin || goos == "darwin" {
			list = append(list, YouTubeBrowser{ID: b.id, Name: b.name})
		}
	}
	return list
}

// spec returns the browser, with its profile, as yt-dlp's
// --cookies-from-browser names it: profile is a profile's name or folder,
// or "" for the one the browser uses. Firefox reads the cookies of no
// container, those of the tabs the user opens unless they choose one.
func (b cookieBrowser) spec(profile, goos, home string, getenv func(string) string) (string, error) {
	profile = strings.TrimSpace(profile)
	if b.engine == "safari" {
		return "safari", nil // which has no profiles
	}
	if profile == "" && b.dirs != nil {
		for _, dir := range b.dirs(goos, home, getenv) {
			if isDir(dir) {
				profile = dir
				break
			}
		}
		if profile == "" {
			return "", fmt.Errorf("MixMusic didn't find the profiles of %s: give the folder of one", b.name)
		}
	}
	spec := b.engine
	if profile != "" {
		spec += ":" + profile
	}
	if b.engine == "firefox" {
		spec += "::none"
	}
	return spec, nil
}

// problem words why yt-dlp could not read the browser's cookies; folder
// tells whether the user named the profile's folder.
func (b cookieBrowser) problem(err error, folder bool) error {
	switch {
	case errors.Is(err, tools.ErrBrowserOpen):
		return worded(b.name+" is open, and keeps its cookies to itself while it is: close it, then import again", err)
	case errors.Is(err, tools.ErrCookiesSealed):
		return worded(b.name+" seals its cookies so that other programs can't read them, as recent Chrome does on Windows: import from Firefox, or paste a cookie header", err)
	case errors.Is(err, tools.ErrNoBrowserCookies) && folder:
		return worded("MixMusic found no cookies of "+b.name+" in that folder: give the folder of one of its profiles", err)
	case errors.Is(err, tools.ErrNoBrowserCookies) && b.dirs != nil:
		return worded("MixMusic found no cookies of "+b.name+" in its folder: give the folder of one of its profiles", err)
	case errors.Is(err, tools.ErrNoBrowserCookies):
		return worded("MixMusic found no cookies of "+b.name+": is it installed? For a profile kept elsewhere, give its folder", err)
	case errors.Is(err, tools.ErrCookiesDenied) && b.engine == "safari":
		return worded("macOS keeps Safari's cookies from MixMusic: allow MixMusic Full Disk Access in System Settings › Privacy & Security, then import again", err)
	case errors.Is(err, tools.ErrCookiesDenied):
		return worded("the system keeps the cookies of "+b.name+" from MixMusic", err)
	}
	return err
}

// exportCookies has yt-dlp copy a browser's cookies into file. It is a
// variable for tests, which never read a real browser.
var exportCookies = func(ctx context.Context, t *tools.Manager, browser, file string) (bool, error) {
	return t.CookiesFromBrowser(ctx, browser, file)
}

// cookieExportPrefix begins the names of the files yt-dlp copies a
// browser's cookies into.
const cookieExportPrefix = "mixmusic-cookies-"

// YouTubeImportCookies signs MixMusic in to YouTube Music with the session
// of a browser the user signed in to YouTube Music with, as yt-dlp reads
// it: browser is one of YouTubeBrowsers, and profile a profile's name or
// folder, or "" for the one the browser uses. Of the browser's cookies,
// MixMusic keeps those of YouTube alone, as the sign-in window does.
func (s *Accounts) YouTubeImportCookies(ctx context.Context, browser, profile string) (AccountsStatus, error) {
	i := -1
	for j, b := range cookieBrowsers {
		if b.id == browser && (!b.darwin || runtime.GOOS == "darwin") {
			i = j
		}
	}
	if i < 0 {
		return AccountsStatus{}, fmt.Errorf("MixMusic can't import the cookies of %q", browser)
	}
	b := cookieBrowsers[i]
	home, _ := os.UserHomeDir()
	spec, err := b.spec(profile, runtime.GOOS, home, os.Getenv)
	if err != nil {
		return AccountsStatus{}, err
	}
	// yt-dlp tells of problems it goes on after, as with the key of some
	// cookies: what it wrote tells whether it worked.
	cookies, undecrypted, err := s.a.browserCookies(ctx, spec)
	switch {
	case youtube.HasSigningCookie(cookies):
		return s.a.youtubeSignIn(ctx, cookies)
	case err != nil:
		return AccountsStatus{}, b.problem(err, strings.TrimSpace(profile) != "")
	case undecrypted:
		return AccountsStatus{}, b.problem(tools.ErrCookiesSealed, false)
	}
	return AccountsStatus{}, fmt.Errorf("%s has no signed-in YouTube Music session: sign in to music.youtube.com in it, close it, then import again", b.name)
}

// browserCookies has yt-dlp copy the cookies of a browser, as yt-dlp
// names it, and returns those of YouTube Music it wrote, whether it could
// not decrypt some, and the problem it told of, if any. The copy holds
// every cookie of the browser: it is a file of the user's alone, which
// goes as soon as it is read.
func (a *App) browserCookies(ctx context.Context, browser string) ([]*http.Cookie, bool, error) {
	f, err := os.CreateTemp("", cookieExportPrefix+"*.txt") // readable by the user alone
	if err != nil {
		return nil, false, err
	}
	file := f.Name()
	defer os.Remove(file)
	// yt-dlp reads the file before it writes it: an empty one isn't one
	// it reads.
	_, err = f.WriteString(tools.NetscapeHeader)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, false, err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	undecrypted, problem := exportCookies(ctx, a.tools, browser, file)
	data, err := os.ReadFile(file)
	os.Remove(file)
	if err != nil {
		return nil, false, errors.Join(problem, err)
	}
	cookies := youtubeCookies(data, time.Now())
	clear(data)
	return cookies, undecrypted, problem
}

// removeCookieExports deletes the copies of browsers' cookies a MixMusic
// that stopped while it imported one left.
func removeCookieExports() {
	left, _ := filepath.Glob(filepath.Join(os.TempDir(), cookieExportPrefix+"*.txt"))
	for _, f := range left {
		os.Remove(f)
	}
}

// YouTubePasteCookies signs MixMusic in to YouTube Music with the Cookie
// header of a request of a signed-in music.youtube.com tab, as a browser's
// developer tools show it, or with the text of a cookies.txt file, of which
// it keeps the cookies of YouTube alone.
func (s *Accounts) YouTubePasteCookies(ctx context.Context, text string) (AccountsStatus, error) {
	cookies := pastedCookies(text, time.Now())
	if !youtube.HasSigningCookie(cookies) {
		return AccountsStatus{}, errors.New("that isn't a signed-in YouTube Music cookie header: copy the Cookie header of a request to music.youtube.com, from a tab signed in to it")
	}
	return s.a.youtubeSignIn(ctx, cookies)
}

// pastedCookies reads a Cookie header, with or without its name, or the
// headers of a request that has one, or the text of a cookies.txt file.
// A header's cookies are of the music.youtube.com tab it was copied from;
// of a cookies.txt file, those of YouTube are kept. Cookies that aren't
// valid are left out.
func pastedCookies(text string, now time.Time) []*http.Cookie {
	if strings.Contains(text, "\t") {
		return youtubeCookies([]byte(text), now)
	}
	header := ""
	for _, line := range strings.Split(text, "\n") {
		if name, value, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(name), "cookie") {
			header = value
			break
		}
	}
	if header == "" {
		// The header's value alone, which a narrow view may have wrapped.
		header = strings.NewReplacer("\r", "", "\n", "").Replace(text)
	}
	var cookies []*http.Cookie
	seen := map[string]bool{}
	for _, part := range strings.Split(header, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parsed, err := http.ParseCookie(part)
		if err != nil || len(parsed) != 1 || seen[parsed[0].Name] {
			continue
		}
		c := parsed[0]
		seen[c.Name] = true
		c.Domain, c.Path, c.Secure = ".youtube.com", "/", true
		cookies = append(cookies, c)
	}
	return cookies
}

// youtubeCookies reads a cookies file of the Netscape format, as yt-dlp
// writes it, and returns the cookies that a request to music.youtube.com
// sends, as the sign-in window keeps them: those of youtube.com for every
// path, that have not expired. The others are not made into strings.
func youtubeCookies(data []byte, now time.Time) []*http.Cookie {
	var cookies []*http.Cookie
	seen := map[string]bool{}
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		line = bytes.TrimRight(line, "\r")
		httpOnly := false
		if rest, ok := bytes.CutPrefix(line, []byte("#HttpOnly_")); ok {
			line, httpOnly = rest, true
		} else if len(line) == 0 || line[0] == '#' {
			continue
		}
		f := bytes.SplitN(line, []byte("\t"), 7)
		if len(f) != 7 || !youtubeDomain(f[0], bytes.EqualFold(f[1], []byte("TRUE"))) || string(f[2]) != "/" {
			continue
		}
		// yt-dlp writes 0, or nothing, for a cookie that ends with the
		// browser's session; the time may have a fraction.
		var expires time.Time
		if e, err := strconv.ParseFloat(string(bytes.TrimSpace(f[4])), 64); err == nil && e > 0 {
			if expires = time.Unix(int64(e), 0); expires.Before(now) {
				continue
			}
		}
		name := string(f[5])
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		cookies = append(cookies, &http.Cookie{
			Name: name, Value: string(f[6]), Domain: strings.ToLower(string(f[0])), Path: "/",
			Secure: bytes.EqualFold(f[3], []byte("TRUE")), HttpOnly: httpOnly, Expires: expires,
		})
	}
	return cookies
}

// youtubeDomain reports whether a cookie of domain, which subdomains share
// when sub is set, goes with requests to music.youtube.com: a cookie of
// youtube.com, of music.youtube.com, or of music.youtube.com alone.
func youtubeDomain(domain []byte, sub bool) bool {
	d := strings.ToLower(string(domain))
	if strings.HasPrefix(d, ".") {
		d, sub = d[1:], true
	}
	if !sub {
		return d == "music.youtube.com"
	}
	return d == "youtube.com" || d == "music.youtube.com"
}
