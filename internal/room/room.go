// Package room is Listen with friends (#561): a room that one MixMusic
// hosts on the network, which the others join by its code or its invite
// link, and whose guests play what the host plays, in step with it. The
// host's audio is never sent: each guest plays the same track from where
// it can. It speaks over remote control's transport (internal/remote): each
// computer's key, TLS 1.3 and its framing, and announces the rooms by
// DNS-SD over mDNS. docs/architecture/rooms.md tells why it is so.
package room

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// version is the protocol's, which hello and discovery tell.
const version = 1

// alpn is the protocol TLS agrees on, apart from remote control's.
const alpn = "mixmusic-room/1"

// features are what this MixMusic can do in a room beyond the version, as
// hello and welcome tell them, which the files of each add. A guest or a
// host that doesn't know one ignores it.
var features []string

// What a hello wants: to join by the code or the link, which it proves,
// or to come back, by its key alone, as a guest of the room.
const (
	wantJoin   = "join"
	wantRejoin = "rejoin"
)

// How a guest proves it may join: by the code it typed, or by the secret
// of the invite link it opened.
const (
	byCode = "code"
	byLink = "link"
)

// Why the host refuses a guest, or ends its stay, in refused and ended
// messages.
const (
	reasonVersion = "version"
	reasonCode    = "code"
	reasonWait    = "wait"
	reasonBusy    = "busy"
	reasonLocked  = "locked"
	reasonNewCode = "new code"
	reasonBlocked = "blocked"
	reasonFull    = "full"
	reasonGone    = "not a guest"
	reasonRemoved = "removed"
	reasonEnded   = "ended"
	reasonSlow    = "slow"
)

// The errors of joining a room and of a stay in it, which the windows tell
// in their own words.
var (
	// ErrWrongCode: the code typed isn't the room's, or the link isn't of
	// this room, or someone stood between the two.
	ErrWrongCode = errors.New("that isn't the room's code")
	// ErrWait: this computer tried too many wrong codes, and waits a
	// minute.
	ErrWait = errors.New("too many wrong codes from this computer: try again in a minute")
	// ErrLocked: the room took too many wrong codes, from whoever tried
	// them, and takes none for a minute: its invite link still opens it.
	ErrLocked = errors.New("the room isn't taking codes for a minute: ask its host for the invite link")
	// ErrNewCode: the host gave the room a new code, and invite link,
	// since this computer proved the old one.
	ErrNewCode = errors.New("the room has a new code: ask its host for it")
	// ErrBusy: so many try codes at once that the room takes no more for
	// a moment.
	ErrBusy = errors.New("the room checks other codes: try again in a moment")
	// ErrBlocked: the host blocked this computer.
	ErrBlocked = errors.New("the host blocked this computer")
	// ErrFull: the room has as many guests as it takes.
	ErrFull = errors.New("the room is full")
	// ErrNotGuest: the host no longer keeps this computer's place, so it
	// joins again by the code.
	ErrNotGuest = errors.New("the room no longer keeps this computer's place")
	// ErrRemoved: the host removed this computer.
	ErrRemoved = errors.New("the host removed this computer from the room")
	// ErrEnded: the host ended the room.
	ErrEnded = errors.New("the room ended")
	// ErrVersion: the host speaks another version of the protocol.
	ErrVersion = errors.New("that computer's MixMusic is of another version: update both")
	// ErrLost: the host stopped answering.
	ErrLost = remote.ErrLost
	// ErrNotSame: a computer with another key answered where the host of
	// the link was.
	ErrNotSame = remote.ErrNotSame
)

// errOf is the error of a reason the host gave.
func errOf(reason string) error {
	switch reason {
	case reasonVersion:
		return ErrVersion
	case reasonCode:
		return ErrWrongCode
	case reasonWait:
		return ErrWait
	case reasonBusy:
		return ErrBusy
	case reasonLocked:
		return ErrLocked
	case reasonNewCode:
		return ErrNewCode
	case reasonPreviewEnded:
		return ErrPreviewEnded
	case reasonBlocked:
		return ErrBlocked
	case reasonFull:
		return ErrFull
	case reasonDeclined:
		return ErrDeclined
	case reasonMoving:
		return ErrRoomMoves
	case reasonManyAsk:
		return ErrManyAsk
	case reasonGone:
		return ErrNotGuest
	case reasonRemoved:
		return ErrRemoved
	case reasonEnded:
		return ErrEnded
	case reasonSlow:
		return ErrLost
	}
	return fmt.Errorf("the host refused: %s", reason)
}

// Timing is how long the protocol waits. Its zero fields are the
// defaults: tests shorten them.
type Timing struct {
	// Ping is how often a guest pings the host, for the offset of its
	// clock, and the host tells where its track is: 2 s.
	Ping time.Duration
	// Silence is how long a side waits for a message before it takes the
	// other for gone: 6 s.
	Silence time.Duration
	// Prove is how long a connection has, from its accepting, to prove the
	// code, the link or its key, in all: 5 s.
	Prove time.Duration
	// Preview is how long a computer that proved the code or the link may
	// wait before it joins: 5 minutes (previews.go).
	Preview time.Duration
	// Keep is how long the host keeps the place of a guest that dropped
	// off: 15 minutes.
	Keep time.Duration
	// Gate is how long, at a track change, the host waits at most for its
	// guests to have the track ready: 5 s. Lead is how far ahead it then
	// starts them all: 350 ms.
	Gate time.Duration
	Lead time.Duration
	// Wait is how long an address that tried Tries wrong codes waits: a
	// minute.
	Wait time.Duration
	// Changes is how often at most the host tells of changes: 100 ms,
	// so that what changes at once goes as one.
	Changes time.Duration
}

func (t Timing) ping() time.Duration    { return or(t.Ping, 2*time.Second) }
func (t Timing) silence() time.Duration { return or(t.Silence, 6*time.Second) }
func (t Timing) prove() time.Duration   { return or(t.Prove, 5*time.Second) }
func (t Timing) preview() time.Duration { return or(t.Preview, 5*time.Minute) }
func (t Timing) keep() time.Duration    { return or(t.Keep, 15*time.Minute) }
func (t Timing) gate() time.Duration    { return or(t.Gate, 5*time.Second) }
func (t Timing) lead() time.Duration    { return or(t.Lead, 350*time.Millisecond) }
func (t Timing) wait() time.Duration    { return or(t.Wait, time.Minute) }
func (t Timing) changes() time.Duration { return or(t.Changes, 100*time.Millisecond) }

func or(d, def time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return def
}

// Tries is how many wrong codes a source, an IPv4 address or an IPv6 /56,
// may try before it waits (admit.go).
const Tries = 5

// MaxGuests bounds a room's guests.
const MaxGuests = 200

// MaxName bounds a name in rooms, in characters.
const MaxName = 50

// CleanName is a name in rooms as a computer told it: without control
// characters, the marks that turn the way text runs, which would show it
// as another name, or the spaces around it, at most MaxName characters, or
// "Someone" when it told none.
func CleanName(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r) || r == unicode.ReplacementChar {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > MaxName {
		s = strings.TrimSpace(string(r[:MaxName]))
	}
	if s == "" {
		return "Someone"
	}
	return s
}
