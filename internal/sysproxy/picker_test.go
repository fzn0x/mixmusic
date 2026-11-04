package sysproxy

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"
)

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestParseServers(t *testing.T) {
	for _, tc := range []struct {
		setting     string
		http, https string // the proxies of an http and an https request
	}{
		{"", "", ""},
		{"proxy.test:3128", "http://proxy.test:3128", "http://proxy.test:3128"},
		{"http://proxy.test:3128", "http://proxy.test:3128", "http://proxy.test:3128"},
		{"proxy.test", "http://proxy.test", "http://proxy.test"},
		{"http=one.test:80;https=two.test:443", "http://one.test:80", "http://two.test:443"},
		{" HTTP=one.test:80 ; https=two.test:443 ", "http://one.test:80", "http://two.test:443"},
		{"http=http://one.test:80;https=https://two.test:443", "http://one.test:80", "https://two.test:443"},
		// An http= alone leaves https direct, as in browsers.
		{"http=one.test:80", "http://one.test:80", ""},
		{"https=two.test:443", "", "http://two.test:443"},
		// socks= serves what has no proxy of its own.
		{"socks=socks.test:1080", "socks5://socks.test:1080", "socks5://socks.test:1080"},
		{"http=one.test:80;socks=socks.test:1080", "http://one.test:80", "socks5://socks.test:1080"},
		{"socks5://socks.test:1080", "socks5://socks.test:1080", "socks5://socks.test:1080"},
		// Go speaks no SOCKS4, and no FTP proxy serves http.
		{"socks4://socks.test:1080", "", ""},
		{"ftp=ftp.test:21", "", ""},
		// A setup script's answers.
		{"PROXY auto.test:8080; DIRECT", "http://auto.test:8080", "http://auto.test:8080"},
		{"PROXY auto.test:8080; PROXY spare.test:8080", "http://auto.test:8080", "http://auto.test:8080"},
		{"HTTPS secure.test:443", "https://secure.test:443", "https://secure.test:443"},
		{"SOCKS5 socks.test:1080", "socks5://socks.test:1080", "socks5://socks.test:1080"},
		{"DIRECT", "", ""},
		{"DIRECT; PROXY auto.test:8080", "", ""},
	} {
		sv := parseServers(tc.setting)
		if got := address(sv.forScheme("http")); got != tc.http {
			t.Errorf("%q, for http: %q, want %q", tc.setting, got, tc.http)
		}
		if got := address(sv.forScheme("https")); got != tc.https {
			t.Errorf("%q, for https: %q, want %q", tc.setting, got, tc.https)
		}
	}
}

func TestParseBypass(t *testing.T) {
	b := parseBypass("*.corp.test;10.*; <local>, 192.168.0.0/16;http://plain.test;ported.test:8080;.dotted.test;[fd00::1]")
	for _, tc := range []struct {
		url  string
		want bool
	}{
		{"https://music.corp.test/", true},
		{"https://MUSIC.CORP.TEST/", true},
		{"https://corp.test/", false},
		{"http://10.1.2.3/", true},
		{"http://110.1.2.3/", false},
		{"http://printer/", true},
		{"http://printer.lan/", false},
		{"http://192.168.5.5:8080/", true},
		{"http://192.169.5.5/", false},
		{"http://plain.test/", true},
		{"https://plain.test/", false},
		{"http://ported.test:8080/", true},
		{"http://ported.test/", false},
		{"https://a.dotted.test/", true},
		{"https://dotted.test/", false},
		{"http://[fd00::1]/", true},
		{"https://music.youtube.com/", false},
	} {
		if got := b.matches(mustURL(t, tc.url)); got != tc.want {
			t.Errorf("%s: %v, want %v", tc.url, got, tc.want)
		}
	}
	// <-loopback> only says loopback goes through the proxy, which it
	// never does here.
	if b := parseBypass("<-loopback>"); b.local || len(b.rules) != 0 {
		t.Errorf("<-loopback> = %+v", b)
	}
}

func TestWildcard(t *testing.T) {
	for _, tc := range []struct {
		pattern, s string
		want       bool
	}{
		{"a.test", "a.test", true},
		{"a.test", "b.a.test", false},
		{"*", "anything", true},
		{"*.a.test", "b.a.test", true},
		{"*.a.test", "c.b.a.test", true},
		{"*.a.test", "a.test", false},
		{"10.*", "10.0.0.1", true},
		{"*mid*", "a-mid-b", true},
		{"a*a", "a", false},
		{"a*b*c", "abc", true},
		{"a*b*c", "acb", false},
	} {
		if got := wildcard(tc.pattern, tc.s); got != tc.want {
			t.Errorf("wildcard(%q, %q) = %v", tc.pattern, tc.s, got)
		}
	}
}

func TestPickerPicksTheServer(t *testing.T) {
	p := NewPicker(Settings{Server: "proxy.test:3128", Bypass: "*.near.test;<local>"})
	for _, tc := range []struct{ url, want string }{
		{"https://music.youtube.com/watch?v=x", "http://proxy.test:3128"},
		{"https://rr1---sn-test.googlevideo.com/videoplayback", "http://proxy.test:3128"},
		{"https://a.near.test/", ""},
		{"http://nas/", ""},
		// The player's own stream proxy, and the like, never.
		{"http://127.0.0.1:41234/yt/x", ""},
		{"http://localhost:8080/", ""},
		{"http://[::1]:8080/", ""},
	} {
		if got := address(p.Pick(mustURL(t, tc.url))); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.url, got, tc.want)
		}
	}
	if got := address(NewPicker(Settings{}).Pick(mustURL(t, "https://music.youtube.com/"))); got != "" {
		t.Errorf("no settings: %q", got)
	}
}

// fakeAuto is automatic setup answering from a map of sites.
type fakeAuto struct {
	answers map[string]string // site → its proxy setting
	err     error
	global  bool
	asked   []string
}

func (f *fakeAuto) ask(target string) (string, string, bool, error) {
	f.asked = append(f.asked, target)
	if f.err != nil {
		return "", "", f.global, f.err
	}
	return f.answers[target], "", false, nil
}

func autoPicker(s Settings, f *fakeAuto, now *time.Time) *Picker {
	p := NewPicker(s)
	p.auto = f.ask
	p.now = func() time.Time { return *now }
	return p
}

func TestPickerAsksAutomaticSetupFirst(t *testing.T) {
	logs := captureLogs(t)
	now := time.Unix(1_800_000_000, 0)
	f := &fakeAuto{answers: map[string]string{
		"https://music.youtube.com/": "PROXY auto.test:8080",
		"https://www.youtube.com/":   "DIRECT",
	}}
	p := autoPicker(Settings{AutoDetect: true, Server: "server.test:3128"}, f, &now)

	if got := address(p.Pick(mustURL(t, "https://music.youtube.com/watch?v=x"))); got != "http://auto.test:8080" {
		t.Errorf("music.youtube.com: %q", got)
	}
	if got := address(p.Pick(mustURL(t, "https://www.youtube.com/"))); got != "" {
		t.Errorf("www.youtube.com: %q, want direct", got)
	}
	// The answer holds for autoFor, and is asked again after.
	p.Pick(mustURL(t, "https://music.youtube.com/browse"))
	if len(f.asked) != 2 {
		t.Errorf("asked %q", f.asked)
	}
	now = now.Add(autoFor)
	p.Pick(mustURL(t, "https://music.youtube.com/"))
	if len(f.asked) != 3 {
		t.Errorf("asked %q after %s", f.asked, autoFor)
	}
	// Only the site is asked about: a setup script sees no path.
	for _, target := range f.asked {
		if strings.Contains(target, "watch") || strings.Contains(target, "browse") {
			t.Errorf("asked %q", target)
		}
	}
	if !strings.Contains(logs.String(), "through http://auto.test:8080, first to music.youtube.com") {
		t.Errorf("log:\n%s", logs)
	}
	// Loopback never asks.
	p.Pick(mustURL(t, "http://127.0.0.1:1234/"))
	if len(f.asked) != 3 {
		t.Errorf("asked about loopback: %q", f.asked)
	}
}

func TestPickerFallsBackWhenAutomaticSetupFails(t *testing.T) {
	logs := captureLogs(t)
	now := time.Unix(1_800_000_000, 0)
	f := &fakeAuto{err: errors.New("automatic detection found no setup"), global: true}
	p := autoPicker(Settings{AutoDetect: true, Server: "server.test:3128", Bypass: "*.near.test"}, f, &now)
	for range 3 {
		if got := address(p.Pick(mustURL(t, "https://music.youtube.com/"))); got != "http://server.test:3128" {
			t.Errorf("failed setup: %q, want the server", got)
		}
		if got := address(p.Pick(mustURL(t, "https://a.near.test/"))); got != "" {
			t.Errorf("failed setup, bypassed: %q", got)
		}
	}
	// A setup that failed as a whole isn't asked again for autoFor, for
	// any site.
	if len(f.asked) != 1 {
		t.Errorf("asked %q", f.asked)
	}
	if n := strings.Count(logs.String(), "automatic proxy setup failed, using the proxy server"); n != 1 {
		t.Errorf("logged the failure %d times:\n%s", n, logs)
	}
	now = now.Add(autoFor)
	f.err = nil
	f.answers = map[string]string{"https://music.youtube.com/": "PROXY auto.test:8080"}
	if got := address(p.Pick(mustURL(t, "https://music.youtube.com/"))); got != "http://auto.test:8080" {
		t.Errorf("after %s: %q", autoFor, got)
	}

	// One site's failure is that site's, and remembered as long.
	f = &fakeAuto{err: errors.New("no such address")}
	p = autoPicker(Settings{Script: "http://wpad.test/proxy.pac"}, f, &now)
	p.Pick(mustURL(t, "https://music.youtube.com/"))
	p.Pick(mustURL(t, "https://music.youtube.com/"))
	if got := address(p.Pick(mustURL(t, "https://music.youtube.com/"))); got != "" || len(f.asked) != 1 {
		t.Errorf("a site's failure: %q, asked %q", got, f.asked)
	}
}

func TestSettingsLogNoCredentials(t *testing.T) {
	s := Settings{
		AutoDetect: true,
		Script:     "http://who:secret@wpad.test/proxy.pac?token=hidden",
		Server:     "http=http://who:secret@one.test:80;https=two.test:443",
		Bypass:     "<local>;*.corp.test",
	}
	got := s.String()
	for _, leak := range []string{"who", "secret", "hidden"} {
		if strings.Contains(got, leak) {
			t.Errorf("%q names %q", got, leak)
		}
	}
	want := "detect settings automatically, setup script http://wpad.test/proxy.pac, proxy server http=http://one.test:80;https=http://two.test:443, except 2 addresses"
	if got != want {
		t.Errorf("String() = %q\nwant       %q", got, want)
	}
	if got := (Settings{}).String(); got != "no proxy" {
		t.Errorf("no settings: %q", got)
	}
}

// captureLogs collects what the package logs, for the test.
func captureLogs(t *testing.T) *strings.Builder {
	var b strings.Builder
	old := logf
	logf = func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	t.Cleanup(func() { logf = old })
	return &b
}
