package app

import (
	"encoding/json"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// sleepLeft is how long the timer of minutes that runs has left, or -1.
func sleepLeft(st player.State) time.Duration {
	if st.Sleep == nil || st.Sleep.Mode != player.SleepMinutes {
		return -1
	}
	return time.Until(st.Sleep.Ends)
}

// TestSleepGlobalCommands starts the sleep timer at the length chosen
// last, and cancels it, as their global shortcuts do; a timer the user
// starts keeps its length.
func TestSleepGlobalCommands(t *testing.T) {
	a := newQuietApp(t, nil)
	p := a.GoPlayer()
	if _, err := (&Preferences{a}).Update(func(s *settings.Settings) { s.SleepTimer.Minutes = 45 }); err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, gc := range GlobalCommands {
		ids = append(ids, gc.ID)
	}
	if !slices.Contains(ids, "player.sleepLast") || !slices.Contains(ids, "player.sleepCancel") {
		t.Fatalf("the global commands are %v", ids)
	}
	a.globalCommand("player.sleepLast")
	waitPlayer(t, p, "45 minutes", func(s player.State) bool {
		left := sleepLeft(s)
		return left > 44*time.Minute && left <= 45*time.Minute
	})
	a.globalCommand("player.sleepCancel")
	waitPlayer(t, p, "the timer off", func(s player.State) bool { return s.Sleep == nil })

	p.StartSleep(player.SleepTimer{Mode: player.SleepMinutes, N: 20})
	for deadline := time.Now().Add(5 * time.Second); a.settings.Get().SleepTimer.Minutes != 20; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the length kept is %d", a.settings.Get().SleepTimer.Minutes)
		}
	}
}

// trayItem finds the item of label in items.
func trayItem(items []*mygo.MenuItem, label string) *mygo.MenuItem {
	for _, it := range items {
		if it.Label == label {
			return it
		}
	}
	return nil
}

// TestSleepTrayMenu offers the sleep timer's lengths and ends in a
// submenu of the tray icon's menu, and while it runs, what it does, more
// minutes and Cancel.
func TestSleepTrayMenu(t *testing.T) {
	a := newQuietApp(t, nil)
	p := a.GoPlayer()
	sub := trayItem(a.trayMenu(), "Sleep timer")
	if sub == nil {
		t.Fatal("no Sleep timer in the tray's menu")
	}
	var labels []string
	for _, it := range sub.Submenu {
		labels = append(labels, it.Label)
	}
	want := []string{"5 minutes", "10 minutes", "15 minutes", "30 minutes", "45 minutes", "60 minutes", "90 minutes",
		"End of this track", "End of the queue"}
	if runtime.GOOS == "linux" {
		want = append([]string{"+5 min", "+15 min", "Cancel", ""}, want...)
	}
	if !slices.Equal(labels, want) {
		t.Errorf("the submenu is %q, want %q", labels, want)
	}
	trayItem(sub.Submenu, "30 minutes").Click(nil, nil)
	waitPlayer(t, p, "30 minutes", func(s player.State) bool { return sleepLeft(s) > 29*time.Minute })

	sub = trayItem(a.trayMenu(), "Sleep timer")
	if runtime.GOOS != "linux" {
		if first := sub.Submenu[0]; !strings.HasPrefix(first.Label, "Stops in 29:") && first.Label != "Stops in 30:00" || !first.Disabled {
			t.Errorf("the submenu starts with %+v", first)
		}
	}
	ends := sleepLeft(p.State())
	trayItem(sub.Submenu, "+15 min").Click(nil, nil)
	waitPlayer(t, p, "15 more", func(s player.State) bool { return sleepLeft(s) > ends+14*time.Minute })
	trayItem(sub.Submenu, "End of the queue").Click(nil, nil)
	waitPlayer(t, p, "at the end of the queue", func(s player.State) bool { return s.Sleep != nil && s.Sleep.Mode == player.SleepQueue })
	trayItem(sub.Submenu, "Cancel").Click(nil, nil)
	waitPlayer(t, p, "the timer off", func(s player.State) bool { return s.Sleep == nil })
}

// TestEngineFades has the engine, the player's stream, fade for the sleep
// timer, through a room's stream too.
func TestEngineFades(t *testing.T) {
	var s player.Stream = roomStream{engineStream{}, nil}
	if _, ok := s.(player.Fader); !ok {
		t.Error("the player's stream doesn't fade")
	}
}

// TestAssistantSleepTimer sets the sleep timer, adds to it, tells it and
// cancels it, each with a toast that tells an assistant did, and refuses
// what the timer doesn't take.
func TestAssistantSleepTimer(t *testing.T) {
	a, toasts := newAssistantApp(t, &fakeCatalog{})
	c := connectAssistant(t, a)
	p := a.GoPlayer()
	var st assistantSleep
	if err := json.Unmarshal([]byte(c.ok("sleep_timer", nil)), &st); err != nil || st.Running {
		t.Errorf("with none, %+v, %v", st, err)
	}
	p.PlayTracks([]media.Track{song("a", "First Light", "Made Up"), song("b", "Second Wind", "Made Up")}, 0)
	waitPlayer(t, p, "a playing", func(s player.State) bool { return s.Playing })

	if text := c.ok("sleep_timer", map[string]any{"minutes": 20}); !strings.Contains(text, "in 20 minutes") {
		t.Errorf("minutes: %s", text)
	}
	waitPlayer(t, p, "20 minutes", func(s player.State) bool { return sleepLeft(s) > 19*time.Minute })
	toasts.wait(t, "An AI assistant set the sleep timer: the music stops in 20 minutes")
	if err := json.Unmarshal([]byte(c.ok("sleep_timer", nil)), &st); err != nil || !st.Running || !strings.HasPrefix(st.Stops, "in 19:") && st.Stops != "in 20:00" ||
		st.SecondsLeft < 19*60 {
		t.Errorf("running, %+v, %v", st, err)
	}
	c.ok("sleep_timer", map[string]any{"add_minutes": 10})
	waitPlayer(t, p, "10 more", func(s player.State) bool { return sleepLeft(s) > 29*time.Minute })
	toasts.wait(t, "An AI assistant set the sleep timer: the music stops in 30 minutes")
	if a.settings.Get().SleepTimer.Minutes != 30 {
		t.Error("an assistant changed the length kept")
	}

	c.ok("sleep_timer", map[string]any{"after_tracks": 2})
	waitPlayer(t, p, "2 tracks", func(s player.State) bool { return s.Sleep != nil && s.Sleep.Tracks == 2 })
	toasts.wait(t, "An AI assistant set the sleep timer: the music stops after 2 tracks")
	c.ok("sleep_timer", map[string]any{"until": "end_of_queue"})
	waitPlayer(t, p, "the queue", func(s player.State) bool { return s.Sleep != nil && s.Sleep.Mode == player.SleepQueue })
	if err := json.Unmarshal([]byte(c.ok("sleep_timer", nil)), &st); err != nil || st.Stops != "at the end of the queue" {
		t.Errorf("at the end of the queue, %+v, %v", st, err)
	}
	c.ok("sleep_timer", map[string]any{"until": "end_of_track"})
	waitPlayer(t, p, "the track", func(s player.State) bool {
		return s.Sleep != nil && s.Sleep.Mode == player.SleepTracks && s.Sleep.Tracks == 1
	})
	toasts.wait(t, "An AI assistant set the sleep timer: the music stops at the end of this track")

	for _, args := range []map[string]any{{"minutes": 0}, {"minutes": 10000}, {"after_tracks": 21}, {"until": "dawn"},
		{"minutes": 5, "cancel": true}, {"add_minutes": -1}} {
		c.fails("sleep_timer", args)
	}
	c.ok("sleep_timer", map[string]any{"cancel": true})
	waitPlayer(t, p, "the timer off", func(s player.State) bool { return s.Sleep == nil })
	toasts.wait(t, "An AI assistant turned the sleep timer off")
	if text := c.ok("sleep_timer", map[string]any{"cancel": true}); text != "No sleep timer runs." {
		t.Errorf("cancel with none: %s", text)
	}
	if text := c.fails("sleep_timer", map[string]any{"add_minutes": 5}); !strings.Contains(text, "no sleep timer runs") {
		t.Errorf("add with none: %s", text)
	}
}
