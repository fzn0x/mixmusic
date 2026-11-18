package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/remote/remotetest"
	"github.com/fzn0x/mixmusic/locales"
)

// TestPseudoLocaleRemote pairs with a wrong code in en-XA: the remote
// control page of the computer that typed it, and the toast of the one
// that showed it, tell why in the language.
func TestPseudoLocaleRemote(t *testing.T) {
	usePseudoLocale(t)
	n := remotetest.New()
	desk := newRemoteShell(t, n, "Desktop")
	lap := newRemoteShell(t, n, "Laptop")
	dt, lt := tester(t, desk), tester(t, lap)
	dt.SetLocale("en-XA")
	lt.SetLocale("en-XA")
	if err := desk.a.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	pc := found(t, lap, "Desktop")
	show(lap, lt, remotePath(pc.ID))
	lap.a.Remote().Pair(pc.ID)
	// The laptop takes a code only once it asks for one, which may come
	// after the desktop shows it.
	untilBoth(t, desk, dt, lap, lt, "the code", func() bool {
		_, ok := desk.a.Remote().Code()
		return ok && lap.a.Remote().View(pc.ID).Step == app.RemoteCodeAsked
	})
	code, _ := desk.a.Remote().Code()
	wrong := "000000"
	if code.Code == wrong {
		wrong = "111111"
	}
	lap.a.Remote().EnterCode(pc.ID, wrong)
	untilBoth(t, desk, dt, lap, lt, "the wrong code told", func() bool { return lt.HasText(locales.RemoteProblemWrongCode("Desktop")) })
	checkPseudo(t, "The remote control page", lt.Texts(), "Desktop", "Laptop")
	checkPseudo(t, "The computer that showed the code", dt.Texts(), "Desktop", "Laptop")
}
