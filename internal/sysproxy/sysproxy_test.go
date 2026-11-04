package sysproxy

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

// withEnv makes the environment hold vars, and its proxy envProxy, for the
// test.
func withEnv(t *testing.T, vars map[string]string, envProxy string) {
	oldLookup, oldEnv := lookupEnv, environment
	lookupEnv = func(name string) string { return vars[name] }
	environment = func(*http.Request) (*url.URL, error) {
		if envProxy == "" {
			return nil, nil
		}
		return url.Parse(envProxy)
	}
	t.Cleanup(func() { lookupEnv, environment = oldLookup, oldEnv })
}

// withPicker makes p the picker of Windows' settings, for the test.
func withPicker(t *testing.T, p *Picker) {
	old := current.Swap(p)
	t.Cleanup(func() { current.Store(old) })
}

func proxyOf(t *testing.T, raw string) string {
	t.Helper()
	u, err := Proxy(&http.Request{URL: mustURL(t, raw)})
	if err != nil {
		t.Fatal(err)
	}
	return address(u)
}

func TestProxyPrefersTheEnvironment(t *testing.T) {
	withPicker(t, NewPicker(Settings{Server: "windows.test:3128"}))

	withEnv(t, nil, "")
	if got := proxyOf(t, "https://music.youtube.com/"); got != "http://windows.test:3128" {
		t.Errorf("no environment: %q, want Windows' proxy", got)
	}
	if got := proxyOf(t, "http://127.0.0.1:41234/yt/x"); got != "" {
		t.Errorf("loopback: %q", got)
	}

	withEnv(t, map[string]string{"https_proxy": "http://env.test:8080"}, "http://env.test:8080")
	if got := proxyOf(t, "https://music.youtube.com/"); got != "http://env.test:8080" {
		t.Errorf("https_proxy: %q, want the environment's", got)
	}
	// The environment decides alone, NO_PROXY and all.
	withEnv(t, map[string]string{"HTTP_PROXY": "http://env.test:8080"}, "")
	if got := proxyOf(t, "https://music.youtube.com/"); got != "" {
		t.Errorf("HTTP_PROXY, which leaves https direct: %q", got)
	}

	// Before Install, and elsewhere than Windows, the environment's alone.
	withEnv(t, nil, "")
	withPicker(t, nil)
	if got := proxyOf(t, "https://music.youtube.com/"); got != "" {
		t.Errorf("no settings read: %q", got)
	}
}

func TestYTDLPArgs(t *testing.T) {
	withEnv(t, nil, "")
	const target = "https://music.youtube.com/"
	now := time.Unix(1_800_000_000, 0)
	for _, tc := range []struct {
		name     string
		settings Settings
		auto     string // automatic setup's answer, with AutoDetect
		want     []string
	}{
		{"no proxy", Settings{}, "", nil},
		// yt-dlp reads the proxy server itself.
		{"the server", Settings{Server: "server.test:3128"}, "", nil},
		{"the server, per scheme", Settings{Server: "http=one.test:80;https=two.test:443"}, "", nil},
		// It doesn't read the bypass list…
		{"bypassed", Settings{Server: "server.test:3128", Bypass: "*.youtube.com"}, "", []string{"--proxy", ""}},
		// …or automatic setup.
		{"automatic", Settings{AutoDetect: true}, "PROXY auto.test:8080", []string{"--proxy", "http://auto.test:8080"}},
		{"automatic, direct", Settings{AutoDetect: true, Server: "server.test:3128"}, "DIRECT", []string{"--proxy", ""}},
		{"automatic, direct, no server", Settings{AutoDetect: true}, "DIRECT", nil},
		{"automatic, the server", Settings{AutoDetect: true, Server: "server.test:3128"}, "PROXY server.test:3128", nil},
	} {
		f := &fakeAuto{answers: map[string]string{target: tc.auto}}
		withPicker(t, autoPicker(tc.settings, f, &now))
		if got := YTDLPArgs(target); !slices.Equal(got, tc.want) {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}

	// With a proxy in the environment, yt-dlp reads it there.
	withPicker(t, NewPicker(Settings{Server: "server.test:3128", Bypass: "*.youtube.com"}))
	withEnv(t, map[string]string{"HTTPS_PROXY": "http://env.test:8080"}, "http://env.test:8080")
	if got := YTDLPArgs(target); got != nil {
		t.Errorf("environment: %q", got)
	}
	withPicker(t, nil)
	withEnv(t, nil, "")
	if got := YTDLPArgs(target); got != nil {
		t.Errorf("no settings read: %q", got)
	}
}

func TestInstallLogsTheEnvironmentsProxyWithoutCredentials(t *testing.T) {
	logs := captureLogs(t)
	withPicker(t, nil)
	withEnv(t, map[string]string{"HTTPS_PROXY": "http://who:secret@env.test:8080"}, "http://env.test:8080")
	Install()
	got := logs.String()
	if !strings.Contains(got, "connecting through http://env.test:8080, from HTTPS_PROXY") {
		t.Errorf("log:\n%s", got)
	}
	if strings.Contains(got, "who") || strings.Contains(got, "secret") {
		t.Errorf("the log names the credentials:\n%s", got)
	}
	if current.Load() != nil {
		t.Error("read Windows' settings, though the environment decides")
	}
}

func TestLoopback(t *testing.T) {
	for host, want := range map[string]bool{
		"localhost":         true,
		"LOCALHOST.":        true,
		"app.localhost":     true,
		"127.0.0.1":         true,
		"127.8.9.10":        true,
		"::1":               true,
		"10.0.0.1":          false,
		"music.youtube.com": false,
		"localhost.test":    false,
	} {
		if got := loopback(host); got != want {
			t.Errorf("loopback(%q) = %v", host, got)
		}
	}
}
