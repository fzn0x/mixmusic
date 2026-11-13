package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/remote/remotetest"
	"github.com/fzn0x/mixmusic/locales"
)

// remoteTiming is the protocol's timing in these tests.
var remoteTiming = remote.Timing{Ping: 100 * time.Millisecond, Silence: 3 * time.Second, Code: 20 * time.Second,
	StateEvery: 10 * time.Millisecond, Rest: 100 * time.Millisecond}

// newRemoteShell is a test shell of the computer named name on the network
// n: it listens on 127.0.0.1 alone, with a key of its own.
func newRemoteShell(t *testing.T, n *remotetest.Network, name string) *shell {
	return newRemoteShellOf(t, n, name, nil)
}

// newRemoteShellOf is newRemoteShell of an app whose options set changes.
func newRemoteShellOf(t *testing.T, n *remotetest.Network, name string, set func(o *app.Options)) *shell {
	t.Helper()
	id, err := remote.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	var s *shell
	s = newTestShellOf(t, func(o *app.Options) {
		o.Remote = &app.RemoteOptions{Discovery: n, Listen: remotetest.Listen, Identity: id, Name: name, Timing: remoteTiming,
			Player: func() *player.Player { return s.p }}
		if set != nil {
			set(o)
		}
	})
	return s
}

// toggle turns the switch labeled label, beside the text about, with the
// keys, as the tests of the settings do: the focus is on the switch once
// it is on label, and not on about, beside it.
func toggle(t *testing.T, s *shell, tt *ui.Tester, label, about string) {
	t.Helper()
	focused := func() bool { return tt.Focused(label) && !tt.Focused(about) }
	for i := 0; i < 300 && !focused(); i++ {
		tt.Key(0, ui.KeyTab)
	}
	if !focused() {
		t.Fatalf("Tab doesn't reach %q: %q", label, tt.Texts())
	}
	tt.Key(0, ui.KeySpace)
	s.drain()
	tt.Frame()
}

// untilBoth draws frames of both windows until ok, failing the test after
// a few seconds with what each shows.
func untilBoth(t *testing.T, a *shell, at *ui.Tester, b *shell, bt *ui.Tester, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		a.drain()
		at.Frame()
		b.drain()
		bt.Frame()
		if ok() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("waiting for %s; the windows show %q and %q", what, at.Texts(), bt.Texts())
		}
	}
}

// found waits for the computer named name with remote control on.
func found(t *testing.T, s *shell, name string) app.RemoteComputer {
	t.Helper()
	s.a.Remote().Find()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		for _, c := range s.a.Remote().Computers() {
			if c.Name == name {
				return c
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s wasn't found: %+v", name, s.a.Remote().Computers())
		}
	}
}

// TestRemoteInDevicesMenu lists a computer with remote control on in the
// devices menu, the cast button's, which opens its page.
func TestRemoteInDevicesMenu(t *testing.T) {
	n := remotetest.New()
	desk := newRemoteShell(t, n, "Desktop")
	lap := newRemoteShell(t, n, "Laptop")
	tt := tester(t, lap)
	if _, ok := tt.Find("Cast to a device"); ok {
		t.Fatal("a devices button with nothing on the network")
	}
	if err := desk.a.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	pc := found(t, lap, "Desktop")
	tt.Frame()
	click(t, lap, tt, "Cast to a device")
	want := []string{"This computer", "-", "Desktop · MixMusic"}
	if got := tt.Menu(); !slices.Equal(got, want) {
		t.Errorf("menu %q, want %q", got, want)
	}
	if err := tt.ChooseMenuItem("Desktop · MixMusic"); err != nil {
		t.Fatal(err)
	}
	lap.drain()
	tt.Frame()
	if got, want := lap.router.Path(), remotePath(pc.ID); got != want {
		t.Errorf("went to %q, want %q", got, want)
	}
	until(t, lap, tt, "the page", func() bool {
		return tt.HasText("Pair with Desktop to see what plays there and control it from here. Desktop shows a code of 6 digits, which you type here.")
	})
}

// TestRemotePairAndControl pairs from the page of a computer, by the code
// its window shows, then shows what plays there and controls it; then the
// computer removes the one that controls it, whose page says so.
func TestRemotePairAndControl(t *testing.T) {
	n := remotetest.New()
	desk := newRemoteShell(t, n, "Desktop")
	lap := newRemoteShell(t, n, "Laptop")
	dt, lt := tester(t, desk), tester(t, lap)
	if err := desk.a.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	desk.p.PlayTracks(testTracks, 0)
	desk.settle(t, func(st player.State) bool { return st.Playing })
	pc := found(t, lap, "Desktop")
	show(lap, lt, remotePath(pc.ID))
	click(t, lap, lt, "pair")

	var code string
	untilBoth(t, desk, dt, lap, lt, "the code", func() bool {
		c, ok := desk.a.Remote().Code()
		code = c.Code
		return ok && dt.HasText("pair with Laptop") && lt.HasText("Type the code Desktop shows.")
	})
	if !dt.HasText(code[:3] + " " + code[3:]) {
		t.Errorf("the code %s doesn't show: %q", code, dt.Texts())
	}
	lt.Type(code)
	lap.drain()
	lt.Frame()
	click(t, lap, lt, "pair")
	untilBoth(t, desk, dt, lap, lt, "what plays there", func() bool {
		return lt.HasText("Midnight City") && lt.HasText("M83 · Hurry Up, We're Dreaming") && lt.HasText("Massive Attack") &&
			lt.HasText("Teardrop")
	})
	if dt.HasText(code[:3] + " " + code[3:]) {
		t.Error("the code still shows once paired")
	}

	click(t, lap, lt, "Pause")
	desk.settle(t, func(st player.State) bool { return !st.Playing })
	untilBoth(t, desk, dt, lap, lt, "paused there", func() bool { _, ok := lt.Find("Play"); return ok })
	click(t, lap, lt, "Next")
	desk.settle(t, func(st player.State) bool { return st.Index == 1 })
	untilBoth(t, desk, dt, lap, lt, "the next track there", func() bool { return lt.HasText("Teardrop") })

	// The desktop's settings list the laptop, and remove it.
	show(desk, dt, "/settings/desktop")
	untilBoth(t, desk, dt, lap, lt, "the paired computer", func() bool {
		return dt.HasText("Laptop") && dt.HasText("Can control this computer. Paired")
	})
	click(t, desk, dt, "Remove Laptop")
	untilBoth(t, desk, dt, lap, lt, "removed", func() bool {
		return lt.HasText("Desktop removed this computer: pair again to control it.")
	})
	if got := desk.a.Remote().Paired(); len(got) != 0 {
		t.Errorf("the desktop keeps %+v", got)
	}
}

// TestRemoteCodeCanceled refuses a pairing from the dialog of the code,
// which the page of the other computer tells.
func TestRemoteCodeCanceled(t *testing.T) {
	n := remotetest.New()
	desk := newRemoteShell(t, n, "Desktop")
	lap := newRemoteShell(t, n, "Laptop")
	dt, lt := tester(t, desk), tester(t, lap)
	if err := desk.a.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	pc := found(t, lap, "Desktop")
	show(lap, lt, remotePath(pc.ID))
	click(t, lap, lt, "pair")
	untilBoth(t, desk, dt, lap, lt, "the code", func() bool { return dt.HasText("pair with Laptop") })
	click(t, desk, dt, "cancel")
	untilBoth(t, desk, dt, lap, lt, "the pairing canceled", func() bool {
		return lt.HasText("The pairing was canceled on Desktop.") && !dt.HasText("pair with Laptop")
	})
	if _, ok := desk.a.Remote().Code(); ok {
		t.Error("the code still shows")
	}
}

// TestRemoteSettings turns remote control on and off from Settings ›
// Desktop, which says no computer is paired yet.
func TestRemoteSettings(t *testing.T) {
	n := remotetest.New()
	s := newRemoteShell(t, n, "Desktop")
	tt := testerOf(t, s, 1280, 2400)
	show(s, tt, "/settings/desktop")
	if !tt.HasText(locales.RemoteSwitch()) || !tt.HasText("No computer is paired yet.") {
		t.Fatalf("Settings › Desktop shows %q", tt.Texts())
	}
	toggle(t, s, tt, locales.RemoteSwitch(), locales.RemoteAbout("Desktop"))
	until(t, s, tt, "remote control on", func() bool { return s.a.Remote().On() && s.remote.turning == nil })
	if n.Announced() != 1 {
		t.Errorf("%d computers announced", n.Announced())
	}
	toggle(t, s, tt, locales.RemoteSwitch(), locales.RemoteAbout("Desktop"))
	until(t, s, tt, "remote control off", func() bool { return !s.a.Remote().On() && s.remote.turning == nil })
	if n.Announced() != 0 {
		t.Errorf("%d computers announced", n.Announced())
	}
	// The search of the settings finds it.
	show(s, tt, "/settings?q=remote")
	until(t, s, tt, "the search", func() bool { return tt.HasText(locales.RemoteSwitch()) })
}

// TestRemotePosition moves the position on while the track plays there,
// and no further than its end.
func TestRemotePosition(t *testing.T) {
	at := time.Now()
	st := remote.State{Playing: true, PositionMs: 10_000, DurationMs: 12_000}
	if got := remotePosition(st, at, at.Add(1500*time.Millisecond)); got != 11500*time.Millisecond {
		t.Errorf("playing: %v", got)
	}
	if got := remotePosition(st, at, at.Add(time.Hour)); got != 12*time.Second {
		t.Errorf("past the end: %v", got)
	}
	st.Playing = false
	if got := remotePosition(st, at, at.Add(time.Hour)); got != 10*time.Second {
		t.Errorf("paused: %v", got)
	}
	if !strings.HasPrefix(remotePath("ab cd"), "/remote/ab%20cd") {
		t.Errorf("path %q", remotePath("ab cd"))
	}
}
