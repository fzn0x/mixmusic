package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/locales"
)

// sleepMinutesLeft is how long the timer of minutes that runs has left,
// or -1.
func sleepMinutesLeft(st player.State) time.Duration {
	if st.Sleep == nil || st.Sleep.Mode != player.SleepMinutes {
		return -1
	}
	return time.Until(st.Sleep.Ends)
}

// sleepLabel finds the label the moon button has: "Sleep timer", or while
// the timer runs, the time it has left.
func sleepLabel(tt *ui.Tester) string {
	for _, txt := range tt.Texts() {
		if txt == "Sleep timer" || strings.HasPrefix(txt, "Sleep timer: ") && strings.HasSuffix(txt, " left") {
			return txt
		}
	}
	return ""
}

// checked reports whether the item labeled label is checked.
func checked(tt *ui.Tester, label string) bool {
	info, ok := tt.Access(label)
	return ok && info.Checked
}

// openSleepMenu opens the moon button's menu, and checks it does.
func openSleepMenu(t *testing.T, s *shell, tt *ui.Tester) {
	t.Helper()
	click(t, s, tt, sleepLabel(tt))
	if !showsExactly(tt, "End of the queue") && !showsExactly(tt, "90 minutes") {
		t.Fatalf("the sleep timer's menu doesn't show: %q", tt.Texts())
	}
}

// TestSleepMenu starts the timer from the moon button's menu, which closes:
// the button shows the time left, to the second, and its menu what the
// timer does, more minutes and Cancel.
func TestSleepMenu(t *testing.T) {
	s := playing(t)
	tt := tester(t, s)
	if got := sleepLabel(tt); got != "Sleep timer" {
		t.Fatalf("the moon button is %q: %q", got, tt.Texts())
	}
	openSleepMenu(t, s, tt)
	for _, item := range []string{"5 minutes", "10 minutes", "15 minutes", "30 minutes", "45 minutes", "60 minutes", "90 minutes",
		"End of this track", "End of the queue", "After 2 tracks", "Let the track finish", "Length of the sleep timer, in minutes", "Start"} {
		if !showsExactly(tt, item) {
			t.Errorf("the menu doesn't offer %s: %q", item, tt.Texts())
		}
	}
	if showsExactly(tt, "Cancel") {
		t.Error("Cancel with no timer")
	}
	click(t, s, tt, "30 minutes")
	s.settle(t, func(st player.State) bool { return sleepMinutesLeft(st) > 29*time.Minute })
	tt.Frame()
	if showsExactly(tt, "90 minutes") {
		t.Error("the menu stays open")
	}
	label := sleepLabel(tt)
	if label != "Sleep timer: 30:00 left" && !strings.HasPrefix(label, "Sleep timer: 29:5") {
		t.Errorf("the moon button is %q", label)
	}
	if !showsExactly(tt, strings.TrimSuffix(strings.TrimPrefix(label, "Sleep timer: "), " left")) {
		t.Errorf("the moon button doesn't show the time left: %q", tt.Texts())
	}

	openSleepMenu(t, s, tt)
	if !slices.ContainsFunc(tt.Texts(), func(txt string) bool { return strings.HasPrefix(txt, "Stops in ") }) {
		t.Errorf("the menu doesn't tell what the timer does: %q", tt.Texts())
	}
	ends := s.st.Sleep.Ends
	click(t, s, tt, "+5 min")
	s.settle(t, func(st player.State) bool { return st.Sleep.Ends.Sub(ends) == 5*time.Minute })
	tt.Frame()
	click(t, s, tt, "+15 min")
	s.settle(t, func(st player.State) bool { return st.Sleep.Ends.Sub(ends) == 20*time.Minute })
	tt.Frame()
	if !strings.HasPrefix(sleepLabel(tt), "Sleep timer: 49:") && sleepLabel(tt) != "Sleep timer: 50:00 left" {
		t.Errorf("after 20 more minutes, the moon button is %q", sleepLabel(tt))
	}
	click(t, s, tt, "Cancel")
	s.settle(t, func(st player.State) bool { return st.Sleep == nil })
	tt.Frame()
	if got := sleepLabel(tt); got != "Sleep timer" {
		t.Errorf("cancelled, the moon button is %q", got)
	}
	if showsExactly(tt, "90 minutes") {
		t.Error("the menu stays open after Cancel")
	}
}

// TestSleepMenuEnds stops at the end of this track, of the queue or of a
// few tracks, which the menu checks, and remembers Let the track finish.
func TestSleepMenuEnds(t *testing.T) {
	s := playing(t)
	tt := tester(t, s)
	openSleepMenu(t, s, tt)
	click(t, s, tt, "End of this track")
	s.settle(t, func(st player.State) bool {
		return st.Sleep != nil && st.Sleep.Mode == player.SleepTracks && st.Sleep.Tracks == 1
	})
	tt.Frame()
	openSleepMenu(t, s, tt)
	if !showsExactly(tt, "Stops at the end of this track") {
		t.Errorf("the menu doesn't tell what the timer does: %q", tt.Texts())
	}
	if !checked(tt, "End of this track") || checked(tt, "End of the queue") {
		t.Error("End of this track isn't checked, alone")
	}
	click(t, s, tt, "End of the queue")
	s.settle(t, func(st player.State) bool { return st.Sleep != nil && st.Sleep.Mode == player.SleepQueue })
	tt.Frame()

	openSleepMenu(t, s, tt)
	click(t, s, tt, "More tracks")
	click(t, s, tt, "More tracks")
	click(t, s, tt, "Fewer tracks")
	click(t, s, tt, "After 3 tracks")
	s.settle(t, func(st player.State) bool {
		return st.Sleep != nil && st.Sleep.Mode == player.SleepTracks && st.Sleep.Tracks == 3
	})
	tt.Frame()
	openSleepMenu(t, s, tt)
	if !showsExactly(tt, "Stops after 3 tracks") || !checked(tt, "After 3 tracks") {
		t.Errorf("the menu shows %q", tt.Texts())
	}
	clickSwitch(t, s, tt, "Let the track finish")
	if !s.svc.Preferences.Get().SleepTimer.Finish {
		t.Error("Let the track finish isn't kept")
	}
}

// TestSleepMenuMinutes starts the timer for the minutes typed, and turns
// down what isn't a length it takes.
func TestSleepMenuMinutes(t *testing.T) {
	s := playing(t)
	tt := tester(t, s)
	openSleepMenu(t, s, tt)
	invalid := locales.SleepMinutesInvalid(9999)
	field := "Length of the sleep timer, in minutes"
	typeIn := func(text string) {
		t.Helper()
		click(t, s, tt, field)
		for range 8 {
			tt.Key(0, ui.KeyBackspace)
		}
		tt.Type(text)
		tt.Frame()
		if info, _ := tt.Access(field); info.Value != text {
			t.Fatalf("the field holds %q, not %q", info.Value, text)
		}
	}
	for _, typed := range []string{"0", "10000", "2.5", "soon"} {
		typeIn(typed)
		click(t, s, tt, "Start")
		if !showsExactly(tt, invalid) || s.p.State().Sleep != nil {
			t.Errorf("%q started a timer, or said nothing: %q", typed, tt.Texts())
		}
	}
	typeIn("125")
	tt.Key(0, ui.KeyEnter)
	s.drain()
	s.settle(t, func(st player.State) bool { return sleepMinutesLeft(st) > 124*time.Minute })
	tt.Frame()
	if showsExactly(tt, invalid) || showsExactly(tt, "90 minutes") {
		t.Errorf("the menu stays: %q", tt.Texts())
	}
	if label := sleepLabel(tt); !strings.HasPrefix(label, "Sleep timer: 2:0") {
		t.Errorf("the moon button is %q", label)
	}
}

// TestSleepMenuLive offers a live stream, which never ends, minutes only.
func TestSleepMenuLive(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks([]media.Track{liveRadio}, 0)
	s.settle(t, func(st player.State) bool { return st.Live && st.Playing })
	tt := tester(t, s)
	openSleepMenu(t, s, tt)
	for _, item := range []string{"End of this track", "End of the queue", "Let the track finish"} {
		if showsExactly(tt, item) {
			t.Errorf("a live stream offers %s", item)
		}
	}
	if !showsExactly(tt, "45 minutes") {
		t.Errorf("a live stream offers no minutes: %q", tt.Texts())
	}
}

// TestMiniSleepMenu starts and cancels the timer from the mini player's
// menu.
func TestMiniSleepMenu(t *testing.T) {
	s := playing(t)
	tt := ui.NewTester(s.miniView, 420, 132)
	tt.Frame()
	choose := func(path ...string) {
		t.Helper()
		if err := tt.RightClick(s.st.Queue[s.st.Index].Track.Title); err != nil {
			t.Fatal(err)
		}
		if err := tt.ChooseMenuItem(path...); err != nil {
			t.Fatalf("%v: the menu holds %q", err, tt.Menu())
		}
		s.drain()
		tt.Frame()
	}
	choose("Sleep timer", "15 minutes")
	s.settle(t, func(st player.State) bool { return sleepMinutesLeft(st) > 14*time.Minute })
	choose("Sleep timer", "+5 min")
	s.settle(t, func(st player.State) bool { return sleepMinutesLeft(st) > 19*time.Minute })
	choose("Sleep timer", "End of the queue")
	s.settle(t, func(st player.State) bool { return st.Sleep != nil && st.Sleep.Mode == player.SleepQueue })
	choose("Sleep timer", "Cancel")
	s.settle(t, func(st player.State) bool { return st.Sleep == nil })
}

// TestSleepCommands has the palette's commands start the timer, at a
// length or the length chosen last, and cancel it.
func TestSleepCommands(t *testing.T) {
	s := playing(t)
	run := func(id string) {
		t.Helper()
		for _, cmd := range s.commands() {
			if cmd.id == id {
				cmd.run()
				return
			}
		}
		t.Fatalf("no command %s", id)
	}
	titles := map[string]string{}
	for _, cmd := range s.commands() {
		titles[cmd.id] = cmd.title
	}
	for id, want := range map[string]string{"player.sleep60": "Sleep timer: 60 minutes", "player.sleepTrack": "Sleep timer: end of this track",
		"player.sleepQueue": "Sleep timer: end of the queue", "player.sleepLast": "Sleep timer: last length",
		"player.sleepCancel": "Sleep timer: cancel"} {
		if titles[id] != want {
			t.Errorf("%s is %q, want %q", id, titles[id], want)
		}
	}
	run("player.sleep60")
	s.settle(t, func(st player.State) bool { return sleepMinutesLeft(st) > 59*time.Minute })
	run("player.sleepCancel")
	s.settle(t, func(st player.State) bool { return st.Sleep == nil })
	s.set.SleepTimer.Minutes = 25
	run("player.sleepLast")
	s.settle(t, func(st player.State) bool {
		left := sleepMinutesLeft(st)
		return left > 24*time.Minute && left <= 25*time.Minute
	})
	run("player.sleepTrack")
	s.settle(t, func(st player.State) bool { return st.Sleep != nil && st.Sleep.Mode == player.SleepTracks })
	for _, id := range []string{"player.sleepLast", "player.sleepCancel"} {
		found := false
		for _, cmd := range s.commands() {
			found = found || cmd.id == id
		}
		if !found {
			t.Errorf("no window command shares %s's global shortcut", id)
		}
	}
}

// TestSleepSettings sets the fade, Let the track finish and bedtime in
// Settings › Playback › Sleep timer: a window for every day, or one for
// each day of the week, each with its switch.
func TestSleepSettings(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 1600)
	show(s, tt, "/settings/playback")
	for _, txt := range []string{"Sleep timer", "Fade out", "30 s", "120 s", "Let the track finish", "Start the sleep timer by itself"} {
		if !showsExactly(tt, txt) {
			t.Errorf("Settings › Playback doesn't show %q", txt)
		}
	}
	if showsExactly(tt, "by day of the week") {
		t.Error("the windows show with bedtime off")
	}
	if !showsExactly(tt, "Off") && !showsExactly(tt, "off") {
		t.Errorf("Fade out offers no Off: %q", tt.Texts())
	}
	click(t, s, tt, "60 s")
	clickSwitch(t, s, tt, "Start the sleep timer by itself")
	got := s.svc.Preferences.Get().SleepTimer
	if got.Fade != 60 || !got.Bedtime {
		t.Fatalf("the settings keep %+v", got)
	}
	tt.Frame()
	if !showsExactly(tt, "End of the window of Every day") {
		t.Fatalf("no window of every day: %q", tt.Texts())
	}
	// From 06:00, three half hours later, with the keys of its popup.
	click(t, s, tt, "End of the window of Every day")
	for range 3 {
		tt.Key(0, ui.KeyDown)
	}
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
	if got := s.svc.Preferences.Get().SleepTimer.Every; got.To != "07:30" || got.From != "22:00" {
		t.Errorf("the window of every day is %+v", got)
	}
	click(t, s, tt, "by day of the week") // as the Tiles layout words it
	tt.Frame()
	monday := dateIn(time.Date(2026, time.November, 16, 12, 0, 0, 0, time.UTC), "EEEE")
	if !showsExactly(tt, "Window of "+monday) || !showsExactly(tt, "Start of the window of "+monday) {
		t.Fatalf("no window of %s: %q", monday, tt.Texts())
	}
	clickSwitch(t, s, tt, "Window of "+monday)
	got = s.svc.Preferences.Get().SleepTimer
	if !got.ByDay || got.Days[time.Monday].On || !got.Days[time.Tuesday].On {
		t.Errorf("the settings keep %+v", got)
	}
}

// TestSleepGivesWay keeps the device button in a window 1280 wide, where
// the player bar has room for it alone: the moon gives way while no timer
// runs, and shows its time left once one does.
func TestSleepGivesWay(t *testing.T) {
	s := outputShell(t, func() []app.OutputDevice {
		return []app.OutputDevice{{ID: "sp", Name: "Speakers"}, {ID: "hp", Name: "Headphones"}}
	})
	tt := testerOf(t, s, 1280, 820)
	settleOutput(t, s, tt)
	if _, ok := tt.Find("Output device: System default"); !ok {
		t.Fatalf("no device button at 1280: %q", tt.Texts())
	}
	if got := sleepLabel(tt); got != "" {
		t.Errorf("the moon %q took the device button's room", got)
	}
	s.p.StartSleep(player.SleepTimer{Mode: player.SleepMinutes, N: 30})
	s.settle(t, func(st player.State) bool { return sleepMinutesLeft(st) > 0 })
	outputFrames(s, tt)
	if got := sleepLabel(tt); !strings.HasPrefix(got, "Sleep timer: ") {
		t.Errorf("a timer that runs shows no moon: %q", tt.Texts())
	}
}
