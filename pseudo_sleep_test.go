package main

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// TestPseudoLocaleSleepTimer shows the sleep timer in en-XA: the moon
// button and its menu while a timer runs, and its group of Settings ›
// Playback with bedtime on, by day of the week. Only the names of the
// tracks are English.
func TestPseudoLocaleSleepTimer(t *testing.T) {
	usePseudoLocale(t)
	s := newTestShell(t)
	var names []string
	for _, tr := range testTracks {
		names = append(names, tr.Title, tr.Album.Name, tr.Artists[0].Name)
	}
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	s.p.StartSleep(player.SleepTimer{Mode: player.SleepTracks, N: 3})
	s.settle(t, func(st player.State) bool { return st.Sleep != nil })
	tt := tester(t, s)
	tt.SetLocale("en-XA")
	tt.Frame()
	xa := locales.In("en-XA")
	left, _ := s.st.SleepLeft(time.Now())
	click(t, s, tt, xa.SleepTimerLeft(player.SleepClock(left)))
	for _, want := range []string{xa.SleepStopsAfterTracks(3), xa.SleepAdd(5), xa.SleepCancel(), xa.SleepMinutes(90),
		xa.SleepAfterTracksItem(2), xa.SleepFinish(), xa.SleepMinutesFieldLabel()} {
		if !tt.HasText(want) {
			t.Errorf("the menu doesn't show %q: %q", want, tt.Texts())
		}
	}
	checkPseudo(t, "The sleep timer's menu", tt.Texts(), names...)

	// A length that isn't one says why.
	click(t, s, tt, xa.SleepMinutesFieldLabel())
	tt.Type("0")
	click(t, s, tt, xa.SleepStart())
	if !tt.HasText(xa.SleepMinutesInvalid(settings.MaxSleepMinutes)) {
		t.Errorf("the menu doesn't say why: %q", tt.Texts())
	}
	checkPseudo(t, "The sleep timer's menu, of a length it doesn't take", tt.Texts(), names...)

	st := s.svc.Preferences.Get()
	st.SleepTimer.Bedtime, st.SleepTimer.ByDay = true, true
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	show(s, tt, "/settings/playback")
	if !tt.HasText(xa.SleepBedtimeHint()) {
		t.Errorf("Settings › Playback shows no windows of bedtime: %q", tt.Texts())
	}
	checkPseudo(t, "Settings › Playback, bedtime by day", tt.Texts(), names...)
}
