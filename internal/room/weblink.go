package room

// What an invite link carries for friends outside the host's network
// (#562): the addresses that reach the host, after an @, and the link's
// https form, for the chat apps that don't open mixmusic:// links. The
// https form opens a page of MixMusic's website (site/templates/join.html),
// which opens the app's link: what follows its #, the room's code too,
// stays in the browser, which never sends it to the website.

import (
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

// WebPage is the page of MixMusic's website that opens invite links.
const WebPage = "https://mixmusic.functionx.dev/join/"

// webPages are the pages that open invite links: the website's, and its
// copy on GitHub Pages.
var webPages = []string{"mixmusic.functionx.dev/join/", "fzn0x.github.io/mixmusic/join/"}

// MaxAddrs bounds the addresses of an invite link.
const MaxAddrs = 4

// WebLink is the invite link's https form: the website's page, then # and
// the code, a dot, and what follows the # of Link.
func (in Invite) WebLink() string {
	link := in.Link()
	_, frag, _ := strings.Cut(link, "#")
	return WebPage + "#" + in.Code + "." + frag
}

// parseWebLink reads the https form of an invite link.
func parseWebLink(u *url.URL) (Invite, error) {
	page := strings.ToLower(u.Host) + u.Path
	if !strings.HasSuffix(page, "/") {
		page += "/"
	}
	known := false
	for _, p := range webPages {
		known = known || page == p
	}
	code, frag, ok := strings.Cut(u.Fragment, ".")
	if !known || !ok {
		return Invite{}, ErrNotLink
	}
	return InviteOf(code, frag)
}

// addrsPart is what a link carries of addrs: @ and the addresses, between
// commas, with the brackets of IPv6 escaped, or nothing.
func addrsPart(addrs []string) string {
	if len(addrs) == 0 {
		return ""
	}
	r := strings.NewReplacer("[", "%5B", "]", "%5D")
	return "@" + r.Replace(strings.Join(addrs, ","))
}

// parseAddrs reads the addresses after a link's @, MaxAddrs at most: false
// when one isn't an address.
func parseAddrs(s string) ([]string, bool) {
	if s == "" {
		return nil, true
	}
	parts := strings.Split(s, ",")
	if len(parts) > MaxAddrs {
		return nil, false
	}
	var out []string
	for _, p := range parts {
		a, ok := cleanAddr(p)
		if !ok {
			return nil, false
		}
		out = append(out, a)
	}
	return out, true
}

// cleanAddr is s, a host and a port, as an address of an invite: an IP
// address, without a zone, or a name of the DNS, and a port of 1 to 65535.
func cleanAddr(s string) (string, bool) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(s))
	if err != nil {
		return "", false
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 {
		return "", false
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if ip.Zone() != "" || ip.IsUnspecified() {
			return "", false
		}
		return netip.AddrPortFrom(ip.Unmap(), uint16(p)).String(), true
	}
	if !dnsName(host) {
		return "", false
	}
	return net.JoinHostPort(strings.ToLower(strings.TrimSuffix(host, ".")), strconv.Itoa(p)), true
}

// dnsName reports whether s is a name of the DNS: labels of letters,
// digits and hyphens, of 63 characters at most, 253 in all.
func dnsName(s string) bool {
	s = strings.TrimSuffix(s, ".")
	if s == "" || len(s) > 253 {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}

// InviteAddr reads an address as the user typed it, for invites or to
// join a room at: a host, an IP address or a name, with a port or not, port
// when not. It is false for what isn't an address.
func InviteAddr(typed string, port int) (string, bool) {
	s := strings.TrimSpace(typed)
	if s == "" {
		return "", false
	}
	if _, _, err := net.SplitHostPort(s); err != nil {
		// No port: a bare host, or an IPv6 address, with brackets or not.
		s = net.JoinHostPort(strings.TrimSuffix(strings.TrimPrefix(s, "["), "]"), strconv.Itoa(port))
	}
	return cleanAddr(s)
}
