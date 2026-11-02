package app

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/portal"
)

// fakeDesktop stands in for MyGo's autostart entry and the desktop's
// Background portal.
type fakeDesktop struct {
	answer portal.Answer
	err    error
	login  bool
	// asked are the autostart choices the portal got; written, those
	// MyGo's entry got.
	asked, written []bool
}

func useFakeDesktop(t *testing.T, flatpak bool) *fakeDesktop {
	d := &fakeDesktop{}
	saved := []any{inFlatpak, askBackground, setOpenAtLogin, openAtLogin}
	t.Cleanup(func() {
		inFlatpak = saved[0].(func() bool)
		askBackground = saved[1].(func(bool) (portal.Answer, error))
		setOpenAtLogin = saved[2].(func(bool) error)
		openAtLogin = saved[3].(func() bool)
	})
	inFlatpak = func() bool { return flatpak }
	askBackground = func(autostart bool) (portal.Answer, error) {
		d.asked = append(d.asked, autostart)
		return d.answer, d.err
	}
	setOpenAtLogin = func(open bool) error {
		d.written = append(d.written, open)
		return nil
	}
	openAtLogin = func() bool { return d.login }
	return d
}

// Outside a Flatpak, MyGo's autostart entry starts MixMusic at login.
func TestStartAtLoginOutsideFlatpak(t *testing.T) {
	d := useFakeDesktop(t, false)
	if err := setLogin(true); err != nil {
		t.Fatal(err)
	}
	if len(d.asked) != 0 || !slices.Equal(d.written, []bool{true}) {
		t.Errorf("asked the portal %v, wrote %v", d.asked, d.written)
	}
	if err := keepInBackground(); err != nil || len(d.asked) != 0 {
		t.Errorf("closing to the tray asked the portal %v: %v", d.asked, err)
	}
}

// In a Flatpak, the desktop's Background portal writes the autostart
// entry: MyGo's, in the sandbox, follows its answer.
func TestStartAtLoginInFlatpak(t *testing.T) {
	d := useFakeDesktop(t, true)
	d.answer = portal.Answer{Background: true, Autostart: true}
	if err := setLogin(true); err != nil {
		t.Fatal(err)
	}
	d.answer = portal.Answer{Background: true}
	if err := setLogin(false); err != nil {
		t.Fatal(err)
	}
	if want := []bool{true, false}; !slices.Equal(d.asked, want) || !slices.Equal(d.written, want) {
		t.Errorf("asked the portal %v, wrote %v, want %v both", d.asked, d.written, want)
	}

	// The user refused, or forbade MixMusic to run in the background.
	d.asked, d.written = nil, nil
	d.answer = portal.Answer{}
	if err := setLogin(true); err == nil || !strings.Contains(err.Error(), "didn't let MixMusic start at login") {
		t.Errorf("refused: %v", err)
	}
	if len(d.written) != 0 {
		t.Errorf("refused, MyGo's entry was written: %v", d.written)
	}

	// The request failed: nothing changes.
	d.err = errors.New("the desktop didn't answer")
	if err := setLogin(true); err == nil || !strings.Contains(err.Error(), "didn't answer") {
		t.Errorf("failed: %v", err)
	}
	if len(d.written) != 0 {
		t.Errorf("failed, MyGo's entry was written: %v", d.written)
	}

	// A desktop without the portal can't start MixMusic, and has nothing
	// to stop.
	d.err = portal.ErrNoBackground
	if err := setLogin(true); err == nil || !strings.Contains(err.Error(), "no Background portal") {
		t.Errorf("without the portal: %v", err)
	}
	if err := setLogin(false); err != nil {
		t.Errorf("turning off without the portal: %v", err)
	}
	if !slices.Equal(d.written, []bool{false}) {
		t.Errorf("without the portal, wrote %v", d.written)
	}
}

// Closing to the tray asks to run in the background, with the Start at
// login choice, as every request sets it.
func TestKeepInBackground(t *testing.T) {
	d := useFakeDesktop(t, true)
	d.answer = portal.Answer{Background: true}
	if err := keepInBackground(); err != nil {
		t.Fatal(err)
	}
	d.login = true
	d.answer = portal.Answer{Background: true, Autostart: true}
	if err := keepInBackground(); err != nil {
		t.Fatal(err)
	}
	if want := []bool{false, true}; !slices.Equal(d.asked, want) {
		t.Errorf("asked the portal %v, want %v", d.asked, want)
	}
	if len(d.written) != 0 {
		t.Errorf("closing to the tray wrote MyGo's entry: %v", d.written)
	}
	d.answer = portal.Answer{}
	if err := keepInBackground(); err == nil {
		t.Error("refused, closing to the tray said nothing")
	}
	d.answer, d.err = portal.Answer{}, portal.ErrNoBackground
	if err := keepInBackground(); err != nil {
		t.Errorf("without the portal: %v", err)
	}
}
