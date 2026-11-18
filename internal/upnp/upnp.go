// Package upnp plays on DLNA and UPnP media renderers: smart TVs, AV
// receivers and network speakers without Chromecast built in. Search finds
// them on the network by SSDP, and Open starts a session on one, which
// plays the media at a URL it's given, through the renderer's AVTransport,
// RenderingControl and ConnectionManager services, over SOAP.
//
// It speaks the UPnP Device Architecture and the AV services as their
// public specifications tell them. Nothing here comes from a UPnP library:
// what MixMusic asks of a renderer is small enough to write.
package upnp

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Renderer is a media renderer on the network, as its description tells
// of it.
type Renderer struct {
	// ID is its UDN, which stays as its name and address change.
	ID string
	// Name is the name the user gave it, as "Living Room TV", and Model
	// what it is, as its maker names it: "" when it names nothing.
	Name, Model string
	// Host is its IPv4 address, where it answered from, and Location the
	// address of its description.
	Host     string
	Location string
	// AVTransport, RenderingControl and ConnectionManager are its
	// services: AVTransport is there on each renderer Search finds, and
	// the others are when their control address isn't "".
	AVTransport, RenderingControl, ConnectionManager Service
}

// Service is a service of a renderer: its type, as
// "urn:schemas-upnp-org:service:AVTransport:1", whose version is the
// renderer's, and the address its actions go to.
type Service struct {
	Type    string
	Control string
}

// Service types, without their version: a renderer has any version of
// each.
const (
	avTransport       = "urn:schemas-upnp-org:service:AVTransport:"
	renderingControl  = "urn:schemas-upnp-org:service:RenderingControl:"
	connectionManager = "urn:schemas-upnp-org:service:ConnectionManager:"
)

// Features are the DLNA content features MixMusic serves its audio with,
// in the fourth field of a protocol info and in the contentFeatures.dlna.org
// header: a stream the renderer can seek in by bytes (DLNA.ORG_OP=01),
// unconverted, and played as it comes.
const Features = "DLNA.ORG_OP=01;DLNA.ORG_CI=0;DLNA.ORG_FLAGS=01700000000000000000000000000000"

// ErrTaken is why a session ends that another app took over: the renderer
// plays an address that isn't MixMusic's. It wraps nothing the windows
// tell apart from a renderer that stopped playing from MixMusic.
var ErrTaken = errors.New("another app plays on the renderer now")

// A Fault is an action the renderer turned down, with the UPnP error code
// and description it told.
type Fault struct {
	Action      string
	Code        int
	Description string
}

func (f *Fault) Error() string {
	if f.Description != "" {
		return fmt.Sprintf("upnp: %s: error %d (%s)", f.Action, f.Code, f.Description)
	}
	return fmt.Sprintf("upnp: %s: error %d", f.Action, f.Code)
}

// localAddress reports whether ip is an address of a network at home: a
// private or link-local IPv4 address. A renderer elsewhere isn't played on.
func localAddress(ip net.IP) bool {
	ip = ip.To4()
	return ip != nil && (ip.IsPrivate() || ip.IsLinkLocalUnicast())
}

// sameHost reports whether the address raw is an http:// address of host,
// an IPv4 address: a renderer's description and control addresses are its
// own, never another computer's.
func sameHost(raw, host string) bool {
	u, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(u.Scheme, "http") {
		return false
	}
	return u.Hostname() == host
}
