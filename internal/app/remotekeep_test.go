package app

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/remote/remotetest"
)

// The computer paired with keeps the pairing before it tells the other it
// is paired: these tests end a pairing as that keeping goes.

// TestRemoteNotKept tells a pairing the other computer couldn't keep in
// the interface's words.
func TestRemoteNotKept(t *testing.T) {
	if got := remoteProblem("Desk", remote.ErrNotKept); !strings.Contains(got, "Desk couldn't keep the pairing") {
		t.Errorf("the page says %q", got)
	}
}

// TestRemoteKeptUnheard drops what the computer controlled kept of a
// pairing the other didn't hear finish, and says so: a computer it didn't
// know goes, and one paired before comes back as it was, at its place.
// One that heard stays.
func TestRemoteKeptUnheard(t *testing.T) {
	desk := newRemoteApp(t, remotetest.New(), "Desktop")
	notes := notesOf(t, desk)
	if err := desk.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	key := func() remote.Fingerprint {
		id, err := remote.NewIdentity()
		if err != nil {
			t.Fatal(err)
		}
		return id.Fingerprint()
	}
	phone, tablet, laptop := key(), key(), key()
	// The desktop controls the phone, and the tablet controls the desktop.
	if err := desk.keepPaired(phone, "Phone", false); err != nil {
		t.Fatal(err)
	}
	if err := desk.keepPaired(tablet, "Tablet", true); err != nil {
		t.Fatal(err)
	}
	before := desk.Remote().Paired()
	same := func(a, b PairedComputer) bool {
		return a.ID == b.ID && a.Key == b.Key && a.Name == b.Name && a.ControlsThis == b.ControlsThis &&
			a.Controlled == b.Controlled && a.Paired.Equal(b.Paired)
	}
	host := remoteHost{desk}
	start := func(key remote.Fingerprint, name string) remote.PairEnd {
		t.Helper()
		end, err := host.Pairing(remote.PairRequest{Name: name, Peer: key, Code: "123456", Cancel: func() {}})
		if err != nil {
			t.Fatal(err)
		}
		if err := end.Keep(); err != nil {
			t.Fatal(err)
		}
		if !host.Allowed(key) {
			t.Errorf("%s may not control the desktop once kept", name)
		}
		return end
	}
	for _, c := range []struct {
		key  remote.Fingerprint
		name string
	}{{laptop, "Laptop"}, {phone, "Phone"}, {tablet, "Tablet"}} {
		start(c.key, c.name+" again").Done(errors.New("use of closed network connection"))
		if got := desk.Remote().Paired(); !slices.EqualFunc(got, before, same) {
			t.Errorf("%s: kept %+v, not %+v", c.name, got, before)
		}
		if !notes.has("Didn't pair with " + c.name + " again: the pairing didn't finish") {
			t.Errorf("%s: told %q", c.name, notes.told)
		}
	}
	if host.Allowed(laptop) || host.Allowed(phone) || !host.Allowed(tablet) {
		t.Error("what may control the desktop changed")
	}
	// Kept later than the other waits.
	start(laptop, "Laptop").Done(remote.ErrNotKept)
	if got := desk.Remote().Paired(); !slices.EqualFunc(got, before, same) {
		t.Errorf("kept %+v, not %+v", got, before)
	}
	if !notes.has("Didn't pair with Laptop: the pairing didn't finish, as keeping it took longer than Laptop waits") {
		t.Errorf("told %q", notes.told)
	}

	start(laptop, "Laptop").Done(nil)
	if got := desk.Remote().Paired(); len(got) != 3 || got[2].ID != laptop.ID() || !got[2].ControlsThis {
		t.Errorf("kept %+v", got)
	}
	if !notes.has("Paired with Laptop") {
		t.Errorf("told %q", notes.told)
	}
}
