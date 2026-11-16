package remote

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
	"unicode"
)

// version is the protocol's, which hello and discovery tell.
const version = 1

// alpn is the protocol TLS agrees on, so that a connection is MixMusic's.
const alpn = "mixmusic-remote/1"

// maxMessage bounds a message, in bytes.
const maxMessage = 1 << 20

// unpairedMessage bounds a message of a computer whose key the server
// doesn't know as paired, in bytes: a hello, a pairing's commit and open,
// and a confirm are a few hundred (#736). A paired computer's hand-over
// may take maxMessage.
const unpairedMessage = 4 << 10

// What a hello wants.
const (
	wantPair    = "pair"
	wantControl = "control"
)

// Why the computer controlled refuses, or a pairing failed, in refused and
// failed messages.
const (
	reasonOff       = "off"
	reasonBusy      = "busy"
	reasonNotPaired = "not paired"
	reasonVersion   = "version"
	reasonCode      = "code"
	reasonCanceled  = "canceled"
	reasonKeep      = "keep"
)

// The errors of pairing and of a session, which the windows tell in their
// own words.
var (
	// ErrOff: the other computer's remote control is off.
	ErrOff = errors.New("remote control is off on that computer")
	// ErrBusy: another computer pairs with it now.
	ErrBusy = errors.New("another computer is pairing with that one")
	// ErrNotPaired: it doesn't know this computer, or no longer.
	ErrNotPaired = errors.New("that computer isn't paired with this one")
	// ErrWrongCode: the code typed isn't the one it showed, or someone
	// stood between the two.
	ErrWrongCode = errors.New("the code isn't the one that computer showed")
	// ErrCanceled: its user canceled the pairing.
	ErrCanceled = errors.New("the pairing was canceled on that computer")
	// ErrNotKept: it couldn't keep the pairing, so neither computer keeps
	// the other.
	ErrNotKept = errors.New("that computer couldn't keep the pairing")
	// ErrNotSame: a computer with another key answered where the one
	// paired was.
	ErrNotSame = errors.New("another computer answered in place of the one paired")
	// ErrLost: it stopped answering.
	ErrLost = errors.New("that computer stopped answering")
	// ErrVersion: it speaks another version of the protocol.
	ErrVersion = errors.New("that computer's MixMusic is of another version: update both")
	// ErrOlder: it speaks this version, but its MixMusic is older, and
	// can't do what was asked, which its welcome doesn't tell.
	ErrOlder = errors.New("that computer's MixMusic is older, and can't do that: update it")
)

// errOf is the error of a reason the other computer gave.
func errOf(reason string) error {
	switch reason {
	case reasonOff:
		return ErrOff
	case reasonBusy:
		return ErrBusy
	case reasonNotPaired:
		return ErrNotPaired
	case reasonCode:
		return ErrWrongCode
	case reasonCanceled:
		return ErrCanceled
	case reasonKeep:
		return ErrNotKept
	case reasonVersion:
		return ErrVersion
	}
	return fmt.Errorf("that computer refused: %s", reason)
}

// message is one of the protocol's messages. Its type says which fields it
// holds.
type message struct {
	Type string `json:"type"`
	// hello: what the computer connecting wants, its name and the
	// protocol's version. welcome and paired: the name of the computer
	// controlled.
	Want    string `json:"want,omitempty"`
	Name    string `json:"name,omitempty"`
	Version int    `json:"version,omitempty"`
	// hello and welcome: what the computer can do beyond the version, as
	// FeatureQueue. An older MixMusic tells none.
	Features []string `json:"features,omitempty"`
	// refused and failed: why.
	Reason string `json:"reason,omitempty"`
	// commit and open: a round of pairing, and its commitment or nonce.
	Round int    `json:"round,omitempty"`
	MAC   []byte `json:"mac,omitempty"`
	Nonce []byte `json:"nonce,omitempty"`
	// state and command: a session's.
	State   *State   `json:"state,omitempty"`
	Command *Command `json:"command,omitempty"`
	// handover and taken: what the computer that controls hands over to
	// the one controlled, and how that one took it.
	HandOver *HandOver `json:"handOver,omitempty"`
	Taken    *Taken    `json:"taken,omitempty"`
}

// wire sends and receives messages over a connection, as Frames does.
// Sends may come from any goroutine.
type wire struct{ f *Frames }

func newWire(conn net.Conn) *wire { return &wire{f: NewFrames(conn)} }

// send sends m, giving up after wait.
func (w *wire) send(m message, wait time.Duration) error { return w.f.Send(m, wait) }

// receive waits up to wait for the next message.
func (w *wire) receive(wait time.Duration) (message, error) {
	var m message
	err := w.f.Receive(&m, wait)
	return m, err
}

// read reads the next message, until the connection's read deadline.
func (w *wire) read() (message, error) {
	var m message
	err := w.f.Read(&m)
	return m, err
}

// quiet is ErrLost for a read that waited too long.
func quiet(err error) error {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return ErrLost
	}
	return err
}

// expect receives the next message, which must be of type typ: a refused
// or failed message is its reason's error.
func (w *wire) expect(typ string, wait time.Duration) (message, error) {
	m, err := w.receive(wait)
	if err != nil {
		return m, err
	}
	switch {
	case m.Type == typ:
		return m, nil
	case m.Type == "refused" || m.Type == "failed":
		return m, errOf(m.Reason)
	}
	return m, fmt.Errorf("remote: a %q message where %q was due", m.Type, typ)
}

// cleanName is a computer's name as the other told it: without control
// characters or the spaces around it, at most 64 characters, or
// "Another computer" when it told none.
func cleanName(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == unicode.ReplacementChar {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > 64 {
		s = strings.TrimSpace(string(r[:64]))
	}
	if s == "" {
		return "Another computer"
	}
	return s
}
