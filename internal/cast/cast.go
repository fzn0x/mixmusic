// Package cast plays on cast devices: Chromecasts, Google Home and Nest
// speakers, and TVs with Chromecast built in. Browse finds them on the
// network by mDNS, and Open starts a session on one, in the device's
// default media receiver, which plays the media at a URL it's given.
//
// It speaks the Cast v2 protocol, as its public descriptions tell it:
// protocol buffers over TLS, on port 8009, in the namespaces of the
// connection, the heartbeat, the receiver and the media. Nothing here comes
// from a cast library: the protocol is small enough to write.
package cast

import (
	"net"
	"strconv"
)

// Device is a cast device on the network, as it tells of itself.
type Device struct {
	// ID is the device's own, which stays as its name and address change.
	ID string
	// Name is the name the user gave it, as "Living Room TV".
	Name string
	// Model is what it is, as "Chromecast" or "Google Nest Mini".
	Model string
	// Host and Port are where it speaks the protocol.
	Host string
	Port int
}

// Addr returns the device's address, host:port.
func (d Device) Addr() string { return net.JoinHostPort(d.Host, strconv.Itoa(d.Port)) }

// DefaultMediaReceiver is the ID of the receiver app every cast device has,
// which plays media at a URL, and shows its title, artist and picture on
// devices with a screen.
const DefaultMediaReceiver = "CC1AD845"

// The namespaces of the protocol MixMusic speaks.
const (
	nsConnection = "urn:x-cast:com.google.cast.tp.connection"
	nsHeartbeat  = "urn:x-cast:com.google.cast.tp.heartbeat"
	nsReceiver   = "urn:x-cast:com.google.cast.receiver"
	nsMedia      = "urn:x-cast:com.google.cast.media"
)

// receiverID is the destination of the device's platform, which launches
// apps and holds the volume.
const receiverID = "receiver-0"

// groupModel is the model cast groups announce themselves as: groups of
// speakers aren't played on yet.
const groupModel = "Google Cast Group"
