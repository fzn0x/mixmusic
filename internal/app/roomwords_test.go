package app

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"

	"github.com/fzn0x/mixmusic/internal/portmap"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/locales"
)

// TestRoomAndRemoteWords says, in en-XA, which brackets every message of
// the catalogs, the errors and notes of the rooms and of remote control:
// each comes from the catalogs, with the system's own words inside.
func TestRoomAndRemoteWords(t *testing.T) {
	was := i18n.Locale()
	i18n.SetLocaleFunc(func() string { return "en-XA" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
	said := map[string]string{}
	for _, err := range []error{errInRoom, errNotInRoom, errCantHost, room.ErrCantHost, errMoving, room.ErrMoving, room.ErrMoved,
		room.ErrDropped, room.ErrLeft, errCantPlaySong, fmt.Errorf("%w: %w", errRoomListen, errors.New("address in use")),
		fmt.Errorf("%w: %w", errRoomAnnounce, errors.New("no multicast"))} {
		said["room: "+err.Error()] = RoomErrorText(err)
	}
	for _, err := range []error{errNotConnected, errNothingPlays, errKeyBusy, fmt.Errorf("%w: %w", errRemoteListen, errors.New("address in use")),
		&peerError{name: "Desk", err: errHandOverLate}, &peerError{name: "Desk", err: remote.ErrOlder}} {
		said["remote: "+err.Error()] = RemoteErrorText(err)
	}
	for _, err := range []error{remote.ErrWrongCode, remote.ErrOff, remote.ErrBusy, remote.ErrCanceled, remote.ErrNotKept,
		remote.ErrNotPaired, remote.ErrNotSame, remote.ErrVersion, remote.ErrOlder, remote.ErrLost, context.DeadlineExceeded,
		errors.New("refused")} {
		said["remote problem: "+err.Error()] = remoteProblem("Desk", err)
	}
	for _, reason := range []string{"", room.SongWaits, room.SongDeclined, room.SongsAreOff, room.TooMany, "other"} {
		said["song "+reason] = songAnswer("Desk", "Teardrop", reason)
	}
	for _, reason := range []string{room.TooMany, "off"} {
		said["control "+reason] = controlRefusal("Desk", reason)
	}
	for _, err := range []error{&portmap.SharedError{Addr: netip.MustParseAddr("100.64.0.7")}, &portmap.RefusedError{Reason: "ConflictInMappingEntry"},
		errors.New("no answer")} {
		said["reach: "+err.Error()] = reachProblem(err, 41234)
	}
	for _, err := range []error{errAddrNoPort, errCodeShape} {
		said["knock: "+err.Error()] = roomProblem(roomByAddr, err)
	}
	for what, txt := range said {
		if !strings.HasPrefix(txt, "[") {
			t.Errorf("%s says %q, which no catalog translates", what, txt)
		}
	}
	if got := RoomErrorText(fmt.Errorf("%w: %w", errRoomListen, errors.New("address in use"))); !strings.Contains(got, "address in use") {
		t.Errorf("the system's words go: %q", got)
	}
	if got := RoomErrorText(errors.New("something else")); got != "something else" {
		t.Errorf("an error of no message: %q, as Go says it", got)
	}
	if got := reachProblem(errors.New("no answer"), 41234); !strings.Contains(got, "41234") {
		t.Errorf("the port isn't a number as typed: %q", got)
	}
	// A room known by its address alone is someone's.
	if got := shownName(roomByAddr); got != locales.RoomSomeone() {
		t.Errorf("shownName(%q) = %q", roomByAddr, got)
	}
}
