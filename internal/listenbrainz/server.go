package listenbrainz

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
)

// uuidRe matches a token of ListenBrainz's own: a UUID, 36 characters.
var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Plausible reports whether token may be one of the server's: on
// ListenBrainz's own, a token is a UUID; another server's may be anything
// but empty.
func Plausible(server, token string) bool {
	if server == DefaultServer {
		return uuidRe.MatchString(token)
	}
	return token != ""
}

// ErrNotServer is ParseServer's error for an address that isn't a web
// address.
var ErrNotServer = errors.New("this isn't the address of a server: write it as https://listenbrainz.example.com")

// ParseServer returns the address of the API that addr, as the user wrote
// it, names: "" is DefaultServer, and an address without a scheme is one
// of https. A trailing /1, the API's version, goes, so that addr may be
// what the server's docs give, as Maloja's https://…/apis/listenbrainz.
// plain reports whether calls go over plain http, unencrypted.
func ParseServer(addr string) (server string, plain bool, err error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return DefaultServer, false, nil
	}
	if !strings.Contains(addr, "://") {
		addr = "https://" + addr
	}
	u, err := url.Parse(addr)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		u.Scheme != "https" && u.Scheme != "http" {
		return "", false, ErrNotServer
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimSuffix(strings.TrimRight(u.Path, "/"), "/1")
	u.RawPath = ""
	server = strings.TrimRight(u.String(), "/")
	if server == "https://listenbrainz.org" || server == "https://www.listenbrainz.org" {
		server = DefaultServer // the website's address, for the API's
	}
	return server, u.Scheme == "http", nil
}

// Resolver finds the addresses of a host name, as net.Resolver's
// LookupNetIP does.
type Resolver func(ctx context.Context, host string) ([]netip.Addr, error)

// Local reports whether the host of the address server is on the local
// network: this computer, an address of a private, link-local or unique
// local range, a name only the local network knows (a name of one label,
// or one ending in .local, .lan, .home.arpa or .internal), or a name whose
// addresses, as resolve finds them, are all of those.
func Local(ctx context.Context, server string, resolve Resolver) bool {
	u, err := url.Parse(server)
	if err != nil {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return false
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return localAddr(ip)
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || !strings.Contains(host, ".") {
		return true
	}
	for _, suffix := range []string{".local", ".lan", ".home.arpa", ".internal"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	if resolve == nil {
		resolve = func(ctx context.Context, host string) ([]netip.Addr, error) {
			return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		}
	}
	addrs, err := resolve(ctx, host)
	if err != nil || len(addrs) == 0 {
		return false
	}
	for _, ip := range addrs {
		if !localAddr(ip) {
			return false
		}
	}
	return true
}

// localAddr reports whether ip is an address of the local network.
func localAddr(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}
