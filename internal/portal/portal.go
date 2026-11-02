// Package portal asks the desktop's XDG portals for what MixMusic can't do
// by itself inside a Flatpak's sandbox: to keep running without a window,
// and to start when the user logs in, which the Background portal decides.
// The sandbox's own autostart folder is one the desktop never reads.
package portal

import (
	"context"
	"errors"
	"strings"
)

// ErrUnsupported is returned where MixMusic can't reach the portals: on
// systems other than Linux, or without GIO.
var ErrUnsupported = errors.New("portal: the desktop's portals are not supported here")

// ErrNoBackground is returned where the desktop has no Background portal.
var ErrNoBackground = errors.New("portal: the desktop has no Background portal")

// Background is what MixMusic asks of the Background portal.
type Background struct {
	// Reason tells the user why, should the desktop ask them.
	Reason string
	// Autostart starts MixMusic when the user logs in, and false stops it:
	// every request writes or removes the autostart entry.
	Autostart bool
	// Commandline is the command the autostart entry runs, inside the
	// sandbox.
	Commandline []string
}

// Answer is what the desktop allowed.
type Answer struct {
	// Background lets MixMusic run without a window.
	Background bool
	// Autostart tells whether MixMusic starts at login now.
	Autostart bool
}

// RequestBackground asks the desktop to let MixMusic run in the background,
// and to start it at login or not, and waits for its answer, which may
// wait for the user's choice, until ctx is done. A refusal is an Answer;
// the error tells that the request failed or got no answer.
func RequestBackground(ctx context.Context, b Background) (Answer, error) {
	return requestBackground(ctx, b)
}

// requestPath returns the object path of the request of token made by the
// connection whose unique name is sender: portals build it so, which lets
// a client subscribe to its Response before it asks.
func requestPath(sender, token string) string {
	s := strings.ReplaceAll(strings.TrimPrefix(sender, ":"), ".", "_")
	return "/org/freedesktop/portal/desktop/request/" + s + "/" + token
}

// remoteError returns the name of the D-Bus error that a message of GIO's
// tells, as org.freedesktop.DBus.Error.UnknownMethod, or "".
func remoteError(message string) string {
	rest, ok := strings.CutPrefix(message, "GDBus.Error:")
	if !ok {
		return ""
	}
	name, _, _ := strings.Cut(rest, ":")
	return name
}

// missing tells whether a D-Bus error says that the desktop has no such
// portal: no portal at all, or none of that interface.
func missing(name string) bool {
	switch name {
	case "org.freedesktop.DBus.Error.ServiceUnknown",
		"org.freedesktop.DBus.Error.NameHasNoOwner",
		"org.freedesktop.DBus.Error.UnknownObject",
		"org.freedesktop.DBus.Error.UnknownInterface",
		"org.freedesktop.DBus.Error.UnknownMethod":
		return true
	}
	return false
}
