package sysproxy

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Settings is a proxy setup as Windows keeps it, in Settings › Network &
// internet › Proxy.
type Settings struct {
	// AutoDetect is "Automatically detect settings": WPAD.
	AutoDetect bool
	// Script is the address of the setup script, a PAC file.
	Script string
	// Server is the proxy server, when it is on: "host:port", or one for
	// each scheme, as "http=host:port;https=host:port".
	Server string
	// Bypass is the addresses that don't use Server, separated by
	// semicolons: "*.example.com;10.*;<local>".
	Bypass string
}

// auto asks the setup script, or the one automatic detection finds, for
// the proxy of target: its answer has the forms of Server and Bypass, and
// an empty proxy connects directly. global reports whether err is the
// whole setup's, as no script found, rather than target's.
type auto func(target string) (proxy, bypass string, global bool, err error)

// autoFor is how long an answer of automatic setup holds, and how long a
// setup that failed is not asked again.
const autoFor = 5 * time.Minute

// Picker picks the proxy of a request by Settings.
type Picker struct {
	settings Settings
	servers  servers
	bypass   bypass
	auto     auto // nil without automatic setup, or where there is none

	asking  sync.Mutex // held while asking auto: one target at a time
	mu      sync.Mutex // guards what follows
	answers map[string]answer
	failed  time.Time // when automatic setup failed as a whole
	logged  map[string]bool
	now     func() time.Time
}

// answer is what automatic setup chose for a target, or that it failed,
// and when.
type answer struct {
	proxy  *url.URL
	failed bool
	at     time.Time
}

// NewPicker returns a picker of s's proxies. It reads the proxy server and
// the bypass list; automatic setup is Windows' own, which it adds.
func NewPicker(s Settings) *Picker {
	return &Picker{
		settings: s,
		servers:  parseServers(s.Server),
		bypass:   parseBypass(s.Bypass),
		answers:  map[string]answer{},
		logged:   map[string]bool{},
		now:      time.Now,
	}
}

// Proxy returns the proxy of r, for http.Transport.
func (p *Picker) Proxy(r *http.Request) (*url.URL, error) {
	return p.Pick(r.URL), nil
}

// Pick returns the proxy of u, nil to connect directly: automatic setup's
// first, as browsers do; when there is none, or it fails, the proxy server,
// unless the bypass list names u.
func (p *Picker) Pick(u *url.URL) *url.URL {
	if loopback(u.Hostname()) {
		return nil
	}
	if p.auto != nil && (p.settings.AutoDetect || p.settings.Script != "") {
		if proxy, ok := p.ask(u); ok {
			return proxy
		}
	}
	if p.bypass.matches(u) {
		return nil
	}
	return p.servers.forScheme(u.Scheme)
}

// ask asks automatic setup for the proxy of u's site, as remembered for
// autoFor; ok is false when it failed, and the proxy server stands.
func (p *Picker) ask(u *url.URL) (*url.URL, bool) {
	// The site alone: a setup script needs no more, and sees no path.
	target := u.Scheme + "://" + u.Host + "/"
	if proxy, known, ok := p.known(target); known {
		return proxy, ok
	}
	// Detection can take seconds: a site already known doesn't wait for
	// it, one asked meanwhile waits and takes its answer.
	p.asking.Lock()
	defer p.asking.Unlock()
	if proxy, known, ok := p.known(target); known {
		return proxy, ok
	}
	proxy, bypass, global, err := p.auto(target)
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	if err != nil {
		if global {
			p.failed = now
			p.logOnce("failed", "mixmusic: automatic proxy setup failed, %s: %v", p.fallback(), err)
		} else {
			p.answers[target] = answer{failed: true, at: now}
			p.logOnce("failed "+u.Host, "mixmusic: automatic proxy setup failed for %s, %s: %v", u.Host, p.fallback(), err)
		}
		return nil, false
	}
	var chosen *url.URL
	if !parseBypass(bypass).matches(u) {
		chosen = parseServers(proxy).forScheme(u.Scheme)
	}
	p.answers[target] = answer{proxy: chosen, at: now}
	if chosen == nil {
		p.logOnce("direct", "mixmusic: the proxy setup connects directly, first to %s", u.Host)
	} else {
		p.logOnce(chosen.String(), "mixmusic: the proxy setup connects through %s, first to %s", redacted(chosen), u.Host)
	}
	return chosen, true
}

// known returns automatic setup's answer for target, when it is remembered:
// known is false when it must be asked, ok is false when it failed.
func (p *Picker) known(target string) (proxy *url.URL, known, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	if !p.failed.IsZero() && now.Sub(p.failed) < autoFor {
		return nil, true, false
	}
	if a, found := p.answers[target]; found && now.Sub(a.at) < autoFor {
		return a.proxy, true, !a.failed
	}
	return nil, false, false
}

// fallback says what MixMusic connects through when automatic setup fails.
func (p *Picker) fallback() string {
	if p.servers.empty() {
		return "connecting directly"
	}
	return "using the proxy server"
}

// logOnce logs a line once for each key.
func (p *Picker) logOnce(key, format string, args ...any) {
	if p.logged[key] {
		return
	}
	p.logged[key] = true
	logf(format, args...)
}

// String describes the settings for the log, with no user name, password
// or query in it.
func (s Settings) String() string {
	var parts []string
	if s.AutoDetect {
		parts = append(parts, "detect settings automatically")
	}
	if s.Script != "" {
		parts = append(parts, "setup script "+redactedScript(s.Script))
	}
	if sv := parseServers(s.Server); !sv.empty() {
		parts = append(parts, "proxy server "+sv.String())
		if s.Bypass != "" {
			parts = append(parts, "except "+parseBypass(s.Bypass).String())
		}
	}
	if len(parts) == 0 {
		return "no proxy"
	}
	return strings.Join(parts, ", ")
}

// servers is a proxy server setting: one proxy for every scheme, or one
// for each.
type servers struct {
	all    *url.URL
	scheme map[string]*url.URL // "http", "https", "socks"…
	order  []string            // the schemes of scheme, as written
}

// parseServers reads a proxy server setting: "host:port", or one for each
// scheme, as "http=host:port;https=host:port;socks=host:port", each with or
// without a scheme of its own, as "http://host:port". It reads a setup
// script's answer too, "PROXY host:port; DIRECT". Of a list of proxies to
// fall back on it keeps the first: MixMusic doesn't fall back. A proxy it
// can't use, as SOCKS4, is left out.
func parseServers(s string) servers {
	var sv servers
	for _, entry := range strings.Split(s, ";") {
		key, value := "", strings.TrimSpace(entry)
		if value == "" {
			continue
		}
		if kind, rest, ok := strings.Cut(value, " "); ok {
			// A setup script's answer.
			switch strings.ToUpper(kind) {
			case "PROXY", "HTTP":
			case "HTTPS":
				rest = "https://" + strings.TrimSpace(rest)
			case "SOCKS", "SOCKS5":
				rest = "socks5://" + strings.TrimSpace(rest)
			default:
				continue
			}
			value = strings.TrimSpace(rest)
		} else if strings.EqualFold(value, "DIRECT") {
			if sv.empty() {
				break // directly first: no proxy
			}
			continue
		}
		if i := strings.IndexByte(value, '='); i > 0 && !strings.Contains(value[:i], "://") {
			key, value = strings.ToLower(value[:i]), value[i+1:]
		}
		u, err := proxyURL(value, key)
		if err != nil {
			continue
		}
		switch {
		case key == "":
			if sv.all == nil {
				sv.all = u
			}
		case sv.scheme[key] == nil:
			if sv.scheme == nil {
				sv.scheme = map[string]*url.URL{}
			}
			sv.scheme[key] = u
			sv.order = append(sv.order, key)
		}
	}
	return sv
}

// proxyURL reads one proxy, "host:port" or "scheme://host:port", whose
// scheme, when it names none, is http, or socks5 for the socks= of a
// setting: Windows means SOCKS4 by it, which Go doesn't speak, and the
// proxy apps that set it speak SOCKS5 too.
func proxyURL(s, key string) (*url.URL, error) {
	if !strings.Contains(s, "://") {
		scheme := "http"
		if key == "socks" {
			scheme = "socks5"
		}
		s = scheme + "://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	switch u.Scheme = strings.ToLower(u.Scheme); u.Scheme {
	case "http", "https", "socks5", "socks5h":
	case "socks":
		u.Scheme = "socks5"
	default:
		return nil, fmt.Errorf("a %s proxy", u.Scheme)
	}
	if u.Hostname() == "" {
		return nil, errors.New("a proxy with no host")
	}
	u.Path, u.RawQuery, u.Fragment = "", "", ""
	return u, nil
}

func (sv servers) empty() bool { return sv.all == nil && len(sv.scheme) == 0 }

// forScheme returns the proxy of a request of scheme: its own, else the
// one of every scheme, else the SOCKS proxy; nil to connect directly. An
// "http=" alone leaves https requests direct, as in browsers.
func (sv servers) forScheme(scheme string) *url.URL {
	if u := sv.scheme[scheme]; u != nil {
		return u
	}
	if sv.all != nil {
		return sv.all
	}
	return sv.scheme["socks"]
}

// String lists the proxies, without user names or passwords.
func (sv servers) String() string {
	if sv.all != nil && len(sv.scheme) == 0 {
		return redacted(sv.all)
	}
	var parts []string
	if sv.all != nil {
		parts = append(parts, redacted(sv.all))
	}
	for _, key := range sv.order {
		parts = append(parts, key+"="+redacted(sv.scheme[key]))
	}
	return strings.Join(parts, ";")
}

// bypass is a bypass list: the addresses that connect directly.
type bypass struct {
	local bool // <local>: names with no dot, as "printer"
	rules []bypassRule
}

// bypassRule is one entry of a bypass list: a host, maybe with wildcards
// or as a CIDR block, maybe only for a scheme or a port.
type bypassRule struct {
	scheme, host, port string
	block              *net.IPNet
}

// parseBypass reads a bypass list, separated by semicolons, commas or
// spaces: "<local>", "*.example.com", ".example.com", "10.*",
// "192.168.0.0/16", "http://example.com", "example.com:8080". "<-loopback>"
// changes nothing: loopback addresses always connect directly.
func parseBypass(s string) bypass {
	var b bypass
	for _, entry := range strings.FieldsFunc(s, func(r rune) bool { return r == ';' || r == ',' || r == ' ' || r == '\t' }) {
		entry = strings.ToLower(entry)
		switch entry {
		case "<local>":
			b.local = true
			continue
		case "<-loopback>":
			continue
		}
		var r bypassRule
		if scheme, rest, ok := strings.Cut(entry, "://"); ok {
			r.scheme, entry = scheme, rest
		}
		if _, block, err := net.ParseCIDR(entry); err == nil {
			r.block = block
			b.rules = append(b.rules, r)
			continue
		}
		if host, port, err := net.SplitHostPort(entry); err == nil {
			entry, r.port = host, port
		}
		entry = strings.Trim(entry, "[]")
		if strings.HasPrefix(entry, ".") {
			entry = "*" + entry
		}
		if entry == "" {
			continue
		}
		r.host = entry
		b.rules = append(b.rules, r)
	}
	return b
}

// matches reports whether u connects directly.
func (b bypass) matches(u *url.URL) bool {
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	ip := net.ParseIP(host)
	if b.local && ip == nil && !strings.Contains(host, ".") {
		return true
	}
	port := u.Port()
	if port == "" {
		port = map[string]string{"http": "80", "https": "443"}[u.Scheme]
	}
	for _, r := range b.rules {
		if r.scheme != "" && r.scheme != u.Scheme || r.port != "" && r.port != port {
			continue
		}
		if r.block != nil {
			if ip != nil && r.block.Contains(ip) {
				return true
			}
			continue
		}
		if wildcard(r.host, host) {
			return true
		}
	}
	return false
}

// String describes the list for the log.
func (b bypass) String() string {
	n := len(b.rules)
	if b.local {
		n++
	}
	if n == 1 {
		return "1 address"
	}
	return fmt.Sprintf("%d addresses", n)
}

// wildcard reports whether s matches pattern, whose stars match any run of
// characters, dots too.
func wildcard(pattern, s string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == s
	}
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	for _, part := range parts[1 : len(parts)-1] {
		i := strings.Index(s, part)
		if i < 0 {
			return false
		}
		s = s[i+len(part):]
	}
	return strings.HasSuffix(s, parts[len(parts)-1])
}

// redacted is a proxy's address for the log: its scheme, host and port.
func redacted(u *url.URL) string {
	if u == nil {
		return "no proxy"
	}
	return u.Scheme + "://" + u.Host
}

// redactedServer is the address of a proxy the environment names, for the
// log.
func redactedServer(s string) string {
	u, err := proxyURL(s, "")
	if err != nil {
		return "a proxy"
	}
	return redacted(u)
}

// redactedScript is a setup script's address for the log, with no user
// name, password or query.
func redactedScript(s string) string {
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return "(an address that doesn't read)"
	}
	return u.Scheme + "://" + u.Host + u.Path
}
