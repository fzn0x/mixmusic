package settings

import (
	"testing"
	"time"
)

// TestSleepTimerClean keeps the fade one of SleepFades, the nearest, the
// length in range, and seven windows of times of the day.
func TestSleepTimerClean(t *testing.T) {
	d := defaultSleepTimer()
	if d.Fade != 30 || d.Minutes != 30 || d.Bedtime || d.Every != defaultBedtime || len(d.Days) != 7 {
		t.Fatalf("the defaults are %+v", d)
	}
	got := SleepTimer{Fade: 45, Minutes: 12000, Every: BedtimeWindow{From: "25:00", To: "7:30"},
		Days: []BedtimeWindow{{On: true, From: "23:15", To: "x"}}}.clean()
	if got.Fade != 30 && got.Fade != 60 || got.Minutes != MaxSleepMinutes {
		t.Errorf("cleaned to a fade of %d and %d minutes", got.Fade, got.Minutes)
	}
	if got.Every.From != "22:00" || got.Every.To != "7:30" {
		t.Errorf("the window of every day is %+v", got.Every)
	}
	if len(got.Days) != 7 || got.Days[0] != (BedtimeWindow{On: true, From: "23:15", To: "06:00"}) || got.Days[6] != defaultBedtime {
		t.Errorf("the days are %+v", got.Days)
	}
	for _, c := range []struct{ in, want int }{{-5, 0}, {4, 0}, {6, 10}, {100, 120}, {1000, 120}} {
		if f := (SleepTimer{Fade: c.in, Minutes: 1}).clean().Fade; f != c.want {
			t.Errorf("a fade of %d cleans to %d, want %d", c.in, f, c.want)
		}
	}
	if m := (SleepTimer{}).clean().Minutes; m != 30 {
		t.Errorf("no length cleans to %d minutes", m)
	}
}

// TestDayMinutes reads times of the day in 24-hour time, and nothing else.
func TestDayMinutes(t *testing.T) {
	for in, want := range map[string]int{"00:00": 0, "6:30": 390, "22:00": 1320, "23:59": 1439, " 07:05 ": 425} {
		if got, ok := DayMinutes(in); !ok || got != want {
			t.Errorf("%q reads as %d, %v", in, got, ok)
		}
	}
	for _, in := range []string{"", "24:00", "12:60", "12", "12:5", "-1:00", "+1:00", "12:+5", "1:2:3", "ab:cd", "123:00"} {
		if got, ok := DayMinutes(in); ok {
			t.Errorf("%q reads as %d", in, got)
		}
	}
	if DayTime(390) != "06:30" || DayTime(-60) != "23:00" || DayTime(24*60) != "00:00" {
		t.Errorf("DayTime writes %q, %q, %q", DayTime(390), DayTime(-60), DayTime(24*60))
	}
}

// TestBedtimeWindow finds the window of the night now is in, of today or
// of the day before across midnight, by day of the week when asked.
func TestBedtimeWindow(t *testing.T) {
	at := func(day, h, m int) time.Time { return time.Date(2026, time.November, day, h, m, 0, 0, time.UTC) }
	// 15 November 2026 is a Sunday.
	s := defaultSleepTimer()
	if _, in := s.Window(at(16, 23, 0)); in {
		t.Error("a window with bedtime off")
	}
	s.Bedtime = true
	for _, c := range []struct {
		now time.Time
		in  bool
		end time.Time
	}{
		{at(16, 21, 59), false, time.Time{}},
		{at(16, 22, 0), true, at(17, 6, 0)},
		{at(17, 3, 0), true, at(17, 6, 0)},
		{at(17, 6, 0), false, time.Time{}},
		{at(17, 12, 0), false, time.Time{}},
	} {
		end, in := s.Window(c.now)
		if in != c.in || !end.Equal(c.end) {
			t.Errorf("at %v: in %v, until %v", c.now, in, end)
		}
	}

	// By day: Sunday's window ends Monday morning, and Monday's is off.
	s.ByDay = true
	s.Days[time.Sunday] = BedtimeWindow{On: true, From: "21:00", To: "07:30"}
	s.Days[time.Monday] = BedtimeWindow{On: false, From: "22:00", To: "06:00"}
	s.Days[time.Tuesday] = BedtimeWindow{On: true, From: "13:00", To: "15:00"}
	for _, c := range []struct {
		now time.Time
		in  bool
		end time.Time
	}{
		{at(15, 21, 30), true, at(16, 7, 30)},
		{at(16, 7, 0), true, at(16, 7, 30)},
		{at(16, 23, 0), false, time.Time{}},
		{at(17, 2, 0), false, time.Time{}},
		{at(17, 14, 0), true, at(17, 15, 0)},
		{at(17, 16, 0), false, time.Time{}},
	} {
		end, in := s.Window(c.now)
		if in != c.in || !end.Equal(c.end) {
			t.Errorf("by day, at %v: in %v, until %v", c.now, in, end)
		}
	}
}
