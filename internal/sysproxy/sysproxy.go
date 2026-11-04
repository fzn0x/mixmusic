// Package sysproxy picks the proxy of MixMusic's requests: the one the
// environment names, as Go's own clients do, and on Windows, failing that,
// the one set in Windows' settings, which browsers, WebView2 and yt-dlp use
// too. It sets it as the Proxy of http.DefaultTransport as it initializes,
// so that every client using it, or cloning it later, picks it up. A
// package that clones http.DefaultTransport as it initializes imports this
// one, as internal/online does, so that its clone has it too.
//
// macOS and Linux keep the environment's proxy alone, as before.
package sysproxy

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
)

// current picks the proxies of Windows' settings, once Install read them;
// nil before and elsewhere.
var current atomic.Pointer[Picker]

// logf logs, lookupEnv reads the environment, and environment picks its
// proxy; variables for tests.
var (
	logf        = log.Printf
	lookupEnv   = os.Getenv
	environment = http.ProxyFromEnvironment
)

// Proxy returns the proxy of r: the environment's when it names one, as
// http.ProxyFromEnvironment does; then none for a loopback address; then
// Windows' setting, once Install read it.
func Proxy(r *http.Request) (*url.URL, error) {
	if fromEnv() {
		return environment(r)
	}
	if loopback(r.URL.Hostname()) {
		return nil, nil
	}
	if p := current.Load(); p != nil {
		return p.Proxy(r)
	}
	return nil, nil
}

// Install reads the proxy of Windows' settings and follows it as it
// changes, logging what MixMusic connects through. Elsewhere it does
// nothing.
func Install() {
	if fromEnv() {
		for _, name := range []string{"HTTPS_PROXY", "HTTP_PROXY"} {
			if v := getenv(name); v != "" {
				logf("mixmusic: connecting through %s, from %s", redactedServer(v), name)
				return
			}
		}
	}
	install()
}

// YTDLPArgs returns the --proxy yt-dlp needs to reach target through the
// proxy MixMusic would use, when yt-dlp wouldn't find it itself: it reads
// the environment and the proxy server of Windows' settings, but not a
// setup script, automatic detection or the bypass list.
func YTDLPArgs(target string) []string {
	p := current.Load()
	if p == nil || fromEnv() {
		return nil
	}
	u, err := url.Parse(target)
	if err != nil {
		return nil
	}
	got := p.Pick(u)
	if address(got) == address(p.servers.forScheme(u.Scheme)) {
		return nil
	}
	// An empty --proxy connects directly.
	return []string{"--proxy", address(got)}
}

// address is u in full, or "" for none.
func address(u *url.URL) string {
	if u == nil {
		return ""
	}
	return u.String()
}

// fromEnv reports whether the environment names a proxy, which then
// decides alone, as before.
func fromEnv() bool {
	for _, name := range []string{"HTTPS_PROXY", "HTTP_PROXY"} {
		if getenv(name) != "" {
			return true
		}
	}
	return false
}

// getenv reads an environment variable in capitals or in lower case, as
// http.ProxyFromEnvironment does.
func getenv(name string) string {
	if v := lookupEnv(name); v != "" {
		return v
	}
	return lookupEnv(strings.ToLower(name))
}

// loopback reports whether host is this computer, which no proxy can reach
// for MixMusic: the player's own stream proxy, say.
func loopback(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
