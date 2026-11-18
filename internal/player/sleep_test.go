package player

import (
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// The fake engine fades as the engine does, for the sleep timer.
func (s *fakeStream) FadeAway(d time.Duration) { s.record("fade away %v", d) }
func (s *fakeStream) FadeBack(d time.Duration) { s.record("fade back %v", d) }

// quickSleep has the sleep timer's second last a millisecond: a minute of
// it lasts 60 ms, and a fade of 10 s 10 ms.
func quickSleep(t *testing.T) {
	t.Helper()
	sleepSecond = time.Millisecond
	t.Cleanup(func() { sleepSecond = time.Second })
}

// sleepHarness plays a, b and c as streams, with the sleep timer's fade
// of fade seconds and Let the track finish as finish.
func sleepHarness(t *testing.T, fade int, finish bool, setup ...func(*fakeServices)) *harness {
	t.Helper()
	quickSleep(t)
	h := newHarness(t, Saved{}, append([]func(*fakeServices){func(f *fakeServices) {
		f.settings.SleepTimer.Fade, f.settings.SleepTimer.Finish = fade, finish
	}}, setup...)...)
	return h
}

// told reports whether a notice said msg.
func (h *harness) told(msg string) bool {
	return slices.ContainsFunc(h.notices(), func(n Notice) bool { return n.Message == msg })
}

// ends ends the track that plays, as the engine tells it.
func (h *harness) ends() {
	h.p.Playing(false)
	h.p.Ended()
}

// noSleep reports whether no sleep timer runs.
func noSleep(p *Player) bool { return p.s.Sleep == nil }

// heldAt reports whether the entry id is current, held paused at its
// start, loaded.
func heldAt(id string) func(*Player) bool {
	return func(p *Player) bool {
		cur, ok := p.s.Current()
		return ok && cur.Track.ID == "yt:"+id && !p.s.Playing && !p.s.Loading && p.s.Position == 0 && p.loaded
	}
}

// TestSleepMinutes pauses the music once the minutes ran out, fades it out
// before, apart from the volume, and puts the fade back once paused. The
// last length is kept.
func TestSleepMinutes(t *testing.T) {
	quickSleep(t)
	var kept []int
	var mu sync.Mutex
	h := newHarnessWith(t, Saved{}, func(f *fakeServices) Services { return keeper{f, &mu, &kept} }, func(f *fakeServices) {
		f.settings.SleepTimer.Fade = 10
	})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	start := time.Now()
	h.p.StartSleep(SleepTimer{Mode: SleepMinutes, N: 1})
	h.wait("the timer on", func(p *Player) bool { return p.s.Sleep != nil })
	if st := h.p.State(); st.Sleep.Mode != SleepMinutes || st.Sleep.Ends.Sub(start) < 59*time.Millisecond {
		t.Errorf("the timer is %+v", st.Sleep)
	}
	h.wait("a paused", func(p *Player) bool { return !p.s.Playing && noSleep(p) })
	if took := time.Since(start); took < 60*time.Millisecond {
		t.Errorf("paused after %v", took)
	}
	h.wait("the fade back", func(*Player) bool { return h.stream.has("fade back 0s") })
	calls := h.stream.copy()
	away := slices.IndexFunc(calls, func(c string) bool { return strings.HasPrefix(c, "fade away") })
	pause := slices.Index(calls, "pause")
	back := slices.Index(calls, "fade back 0s")
	if away < 0 || pause < away || back < pause {
		t.Errorf("the stream did %q", calls)
	}
	if h.stream.count("load") != 1 {
		t.Errorf("the queue moved: %q", calls)
	}
	if !h.told("Sleep timer on: the music stops in 1 minute") || !h.told("The sleep timer paused the music") {
		t.Errorf("notices %+v", h.notices())
	}
	h.svc.with(func(f *fakeServices) {
		if len(f.volumes) != 0 {
			t.Errorf("the volume was kept: %v", f.volumes)
		}
	})
	eventually(t, "the length kept", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return slices.Equal(kept, []int{1})
	})
}

// keeper is the fake services, which keep the length of the timer.
type keeper struct {
	*fakeServices
	mu   *sync.Mutex
	kept *[]int
}

func (k keeper) KeepSleepMinutes(n int) {
	k.mu.Lock()
	defer k.mu.Unlock()
	*k.kept = append(*k.kept, n)
}

// eventually waits until ok.
func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !ok(); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%s didn't happen", what)
		}
	}
}

// TestSleepPausesLoading pauses a track that still loads as the time runs
// out, as the computer's sleep does: it stays paused once loaded.
func TestSleepPausesLoading(t *testing.T) {
	found := make(chan struct{})
	release := sync.OnceFunc(func() { close(found) })
	t.Cleanup(release)
	h := sleepHarness(t, 0, false, func(f *fakeServices) {
		f.prepare = func(t media.Track) (Source, error) {
			<-found
			return streamOf(t), nil
		}
	})
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a loading", func(p *Player) bool { return p.s.Loading })
	h.p.StartSleep(SleepTimer{Mode: SleepMinutes, N: 1})
	h.wait("the timer over", noSleep)
	release()
	h.wait("a held", func(p *Player) bool { return h.stream.lastLoad() == "load url:yt:a 0s false" && p.loaded })
	h.on(func(p *Player) {
		if p.s.Playing || p.s.Loading {
			t.Errorf("playing %v, loading %v", p.s.Playing, p.s.Loading)
		}
	})
}

// TestSleepRanOutPaused ends a timer whose time ran out while nothing
// played.
func TestSleepRanOutPaused(t *testing.T) {
	h := sleepHarness(t, 10, false)
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	h.p.Pause()
	h.wait("a paused", func(p *Player) bool { return !p.s.Playing })
	h.p.StartSleep(SleepTimer{Mode: SleepMinutes, N: 1})
	h.wait("the timer over", func(p *Player) bool { return h.told("The sleep timer ran out with nothing playing") && noSleep(p) })
}

// TestSleepEndOfTrack stops the music at the end of the track that plays:
// the next waits, paused, at its start. On repeat one, the track's end is
// one too.
func TestSleepEndOfTrack(t *testing.T) {
	h := sleepHarness(t, 0, false)
	h.p.PlayTracks(tracks("a", "b", "c"), 0)
	h.wait("a playing", playing("a"))
	h.p.StartSleep(SleepTimer{Mode: SleepTracks, N: 1})
	h.wait("the timer on", func(p *Player) bool { return p.s.Sleep != nil })
	h.p.Next() // a skip isn't an end: b's end is the timer's
	h.wait("b playing", playing("b"))
	h.ends()
	h.wait("c held", heldAt("c"))
	h.on(func(p *Player) {
		if p.s.Sleep != nil {
			t.Errorf("the timer runs on: %+v", p.s.Sleep)
		}
	})
	if !h.told("Sleep timer on: the music stops at the end of this track") || !h.told("The sleep timer paused the music") {
		t.Errorf("notices %+v", h.notices())
	}

	h.p.Play()
	h.wait("c playing", playing("c"))
	h.p.CycleRepeat()
	h.p.CycleRepeat() // one
	h.wait("repeat one", func(p *Player) bool { return p.s.Repeat == RepeatOne })
	h.p.StartSleep(SleepTimer{Mode: SleepTracks, N: 2})
	h.ends()
	h.wait("c again", func(p *Player) bool { return p.s.Sleep != nil && p.s.Sleep.Tracks == 1 && playing("c")(p) })
	h.ends()
	h.wait("c stopped", func(p *Player) bool { return noSleep(p) && !p.s.Playing && p.atEnd && p.s.Index == 2 })
}

// TestSleepAfterTracks counts the tracks that end, those faded into the
// next too, and lets the last play to its end, with no crossfade.
func TestSleepAfterTracks(t *testing.T) {
	crossfaded := func(t *testing.T) *harness {
		h := sleepHarness(t, 0, false, func(f *fakeServices) { f.settings.Crossfade = 5 })
		h.p.PlayTracks(tracks("a", "b", "c", "d"), 0)
		h.wait("a playing", playing("a"))
		return h
	}
	h := crossfaded(t)
	h.p.StartSleep(SleepTimer{Mode: SleepTracks, N: 3})
	h.wait("the timer on", func(p *Player) bool { return p.s.Sleep != nil })
	// a fades into b, and b ends as it is.
	h.p.Time(196*time.Second, 200*time.Second)
	h.wait("b faded into", func(p *Player) bool { return h.stream.has("crossfade url:yt:b 5s") && p.s.Sleep.Tracks == 2 })
	h.p.Time(time.Second, 200*time.Second)
	h.ends()
	h.wait("c playing", func(p *Player) bool { return playing("c")(p) && p.s.Sleep.Tracks == 1 })
	h.ends()
	h.wait("d held", heldAt("d"))

	h = crossfaded(t)
	h.p.StartSleep(SleepTimer{Mode: SleepTracks, N: 1})
	h.p.Time(196*time.Second, 200*time.Second)
	h.p.Time(197*time.Second, 200*time.Second)
	h.on(func(p *Player) {
		if p.crossfading || h.stream.count("crossfade") > 0 {
			t.Error("the last track faded into the next")
		}
	})
	h.ends()
	h.wait("b held", heldAt("b"))
}

// TestSleepEndOfQueue stops at the end of the queue: repeat all doesn't
// go round, and the radio after the queue doesn't start, unless it plays
// already.
func TestSleepEndOfQueue(t *testing.T) {
	h := sleepHarness(t, 0, false)
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.CycleRepeat() // all
	h.p.StartSleep(SleepTimer{Mode: SleepQueue})
	h.ends()
	h.wait("b playing", playing("b"))
	h.ends()
	h.wait("b stopped", func(p *Player) bool { return noSleep(p) && !p.s.Playing && p.atEnd && p.s.Index == 1 })

	h.p.PlayRadio(track("s"), tracks("r1", "r2"))
	h.wait("s playing", playing("s"))
	h.p.StartSleep(SleepTimer{Mode: SleepQueue})
	h.ends()
	h.wait("s stopped before its radio", func(p *Player) bool { return noSleep(p) && p.atEnd && p.s.Index == 0 })

	h.p.PlayAt(1)
	h.wait("r1 playing", playing("r1"))
	h.p.StartSleep(SleepTimer{Mode: SleepQueue})
	h.ends()
	h.wait("r2 playing", playing("r2"))
	h.ends()
	h.wait("r2 stopped", func(p *Player) bool { return noSleep(p) && p.atEnd && p.s.Index == 2 })
}

// TestSleepFadeAtEnd fades the last track out in its last seconds, ending
// half a second before it, and brings the fade back when the track is
// moved back from there.
func TestSleepFadeAtEnd(t *testing.T) {
	h := sleepHarness(t, 30, false)
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.StartSleep(SleepTimer{Mode: SleepTracks, N: 1})
	end := 200 * time.Second
	h.p.Time(end-40*time.Millisecond, end)
	h.on(func(p *Player) {
		if p.s.Sleep.Fading {
			t.Error("faded early")
		}
	})
	h.p.Time(end-20*time.Millisecond, end)
	h.wait("the fade", func(p *Player) bool { return p.s.Sleep.Fading && h.stream.has("fade away 19.5ms") })
	h.p.Seek(time.Minute)
	h.p.Time(time.Minute, end)
	h.wait("the fade back", func(p *Player) bool { return !p.s.Sleep.Fading && h.stream.has("fade back 1ms") })
	h.p.Time(end-10*time.Millisecond, end)
	h.wait("the fade again", func(p *Player) bool { return p.s.Sleep.Fading && h.stream.has("fade away 9.5ms") })
	h.ends()
	h.wait("b held", heldAt("b"))
	h.wait("the fade back", func(p *Player) bool { return h.stream.has("fade back 0s") })
}

// TestSleepLetFinish lets the track that plays as the time runs out play
// to its end, where the music stops, with no fade before.
func TestSleepLetFinish(t *testing.T) {
	h := sleepHarness(t, 10, true)
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.StartSleep(SleepTimer{Mode: SleepMinutes, N: 1})
	h.wait("time's up", func(p *Player) bool { return p.s.Sleep != nil && p.s.Sleep.Finishing })
	if !h.told("Time's up: the music stops at the end of this track") {
		t.Errorf("notices %+v", h.notices())
	}
	h.on(func(p *Player) {
		if !p.s.Playing || p.s.Sleep.Fading || h.stream.has("fade away 10ms") {
			t.Error("the music stopped, or faded, before the track's end")
		}
	})
	h.ends()
	h.wait("b held", heldAt("b"))
	h.wait("the timer over", noSleep)
}

// TestSleepAddAndCancel adds minutes to the timer, and to one that stops
// at the end of a track, which runs on in minutes, and cancels it: a fade
// under way comes back.
func TestSleepAddAndCancel(t *testing.T) {
	quickSleep(t)
	sleepSecond = 50 * time.Millisecond // a minute of 3 s, for room
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	h.p.StartSleep(SleepTimer{Mode: SleepMinutes, N: 1})
	h.wait("the timer on", func(p *Player) bool { return p.s.Sleep != nil })
	ends := h.p.State().Sleep.Ends
	h.p.AddSleep(5, false)
	h.wait("5 more", func(p *Player) bool { return p.s.Sleep.Ends.Sub(ends) == 5*60*sleepSecond })
	if !h.told("Sleep timer on: the music stops in 6 minutes") {
		t.Errorf("notices %+v", h.notices())
	}

	h.p.StartSleep(SleepTimer{Mode: SleepTracks, N: 1})
	h.p.Time(200*time.Second-6*sleepSecond, 200*time.Second)
	h.wait("fading", func(p *Player) bool { return p.s.Sleep.Fading })
	h.p.AddSleep(15, true)
	h.wait("15 more", func(p *Player) bool { return p.s.Sleep.Mode == SleepMinutes && !p.s.Sleep.Fading })
	left := time.Until(h.p.State().Sleep.Ends)
	if want := 15*60*sleepSecond + 6*sleepSecond; left > want || left < want-time.Second {
		t.Errorf("%v left, want %v", left, want)
	}
	if !h.told("An AI assistant set the sleep timer: the music stops in 16 minutes") || !h.stream.has("fade back 50ms") {
		t.Errorf("notices %+v, the stream did %q", h.notices(), h.stream.copy())
	}
	h.p.CancelSleep(false)
	h.wait("the timer off", noSleep)
	h.p.CancelSleep(true)
	if !h.told("Sleep timer off") || h.told("An AI assistant turned the sleep timer off") {
		t.Errorf("notices %+v", h.notices())
	}
	h.on(func(p *Player) {
		if !p.s.Playing {
			t.Error("cancelling paused the music")
		}
	})
}

// TestSleepOnDevice pauses a Spotify Connect device elsewhere, steps its
// own volume down once a second as it fades, and puts it back once
// paused: the volume of the settings stays.
func TestSleepOnDevice(t *testing.T) {
	quickDevice(t)
	h := sleepHarness(t, 30, false, func(f *fakeServices) { f.prepare = onDevice })
	sleepSecond = 4 * time.Millisecond
	h.p.SetVolume(0.5)
	h.p.PlayTracks([]media.Track{spotifyTrack("1")}, 0)
	h.wait("the device playing", func(p *Player) bool { return p.connect.active() && p.s.Playing })
	h.on(func(p *Player) {
		if ahead := p.aheadOnDevice(); len(ahead) != 0 {
			t.Errorf("ahead %v", ahead)
		}
	})
	h.p.StartSleep(SleepTimer{Mode: SleepMinutes, N: 1})
	h.wait("paused", func(p *Player) bool { return noSleep(p) && h.device.has("pause") })
	h.wait("the volume back", func(*Player) bool {
		h.device.mu.Lock()
		defer h.device.mu.Unlock()
		return h.device.calls[len(h.device.calls)-1] == "volume 0.5"
	})
	h.device.mu.Lock()
	calls := slices.Clone(h.device.calls)
	h.device.mu.Unlock()
	var steps []string
	for _, c := range calls {
		if strings.HasPrefix(c, "volume ") && c != "volume 0.5" {
			steps = append(steps, c)
		}
	}
	// A fade of 120 ms in steps of 4 ms at most, and its last step silent.
	if len(steps) < 3 || len(steps) > 40 || steps[len(steps)-1] != "volume 0" {
		t.Errorf("the device did %q", calls)
	}
	h.svc.with(func(f *fakeServices) {
		if len(f.volumes) != 1 {
			t.Errorf("the settings kept the volumes %v", f.volumes)
		}
	})
}

// TestSleepAheadOnDevice gives a device that plays tracks by itself none
// ahead while the timer stops the music at the end of a track.
func TestSleepAheadOnDevice(t *testing.T) {
	h := sleepHarness(t, 0, false)
	h.p.PlayTracks([]media.Track{track("a"), spotifyTrack("1"), spotifyTrack("2")}, 0)
	h.wait("a playing", playing("a"))
	if ahead := h.p.State(); len(ahead.Queue) != 3 {
		t.Fatal("no queue")
	}
	h.on(func(p *Player) {
		p.s.Index = 1
		if len(p.aheadOnDevice()) != 1 {
			t.Error("no tracks ahead without a timer")
		}
		p.s.Sleep = &Sleep{Mode: SleepQueue}
		if len(p.aheadOnDevice()) != 0 {
			t.Error("tracks ahead at the end of the queue")
		}
		p.s.Sleep = &Sleep{Mode: SleepMinutes, Ends: time.Now().Add(time.Hour)}
		if len(p.aheadOnDevice()) != 1 {
			t.Error("no tracks ahead with a timer of minutes")
		}
		p.s.Sleep, p.s.Index = nil, 0
	})
}

// TestBedtime starts the timer, at the length chosen last, as playback
// starts inside a window of bedtime, rather than plays on; cancelled, none
// starts until the window ends. The computer's sleep ends it.
func TestBedtime(t *testing.T) {
	now := time.Now()
	window := settings.BedtimeWindow{On: true, From: settings.DayTime(now.Hour()*60 + now.Minute() - 60),
		To: settings.DayTime(now.Hour()*60 + now.Minute() + 60)}
	h := sleepHarness(t, 0, false, func(f *fakeServices) {
		f.settings.SleepTimer.Bedtime, f.settings.SleepTimer.Every, f.settings.SleepTimer.Minutes = true, window, 90
	})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("the timer on", func(p *Player) bool { return p.s.Sleep != nil && p.s.Sleep.Mode == SleepMinutes })
	if left := time.Until(h.p.State().Sleep.Ends); left < 89*60*sleepSecond {
		t.Errorf("%v left", left)
	}
	if !h.told("It's bedtime, so the sleep timer is on: the music stops in 90 minutes") {
		t.Errorf("notices %+v", h.notices())
	}
	h.p.Sleeping()
	h.wait("the timer off", noSleep)
	if !h.told("Sleep timer off, as the computer went to sleep") {
		t.Errorf("notices %+v", h.notices())
	}
	// Played again after a while, it starts again, as sleep doesn't hold it
	// off.
	time.Sleep(2 * sleepRun())
	h.p.Play()
	h.wait("the timer on again", func(p *Player) bool { return p.s.Sleep != nil && p.s.Playing })
	h.p.CancelSleep(false)
	h.wait("the timer off", noSleep)
	h.p.Pause()
	h.wait("paused", func(p *Player) bool { return !p.s.Playing })
	time.Sleep(2 * sleepRun())
	h.p.Play()
	h.wait("playing", playing("a"))
	time.Sleep(10 * sleepSecond)
	h.on(func(p *Player) {
		if p.s.Sleep != nil {
			t.Error("the timer started again in the window it was cancelled in")
		}
	})
}

// TestBedtimePlaysOn starts no timer as the next track plays, inside the
// window, while the music played on.
func TestBedtimePlaysOn(t *testing.T) {
	now := time.Now()
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(time.Second, 200*time.Second)
	h.svc.with(func(f *fakeServices) {
		f.settings.SleepTimer.Bedtime = true
		f.settings.SleepTimer.Every = settings.BedtimeWindow{On: true, From: settings.DayTime(now.Hour()*60 + now.Minute() - 60),
			To: settings.DayTime(now.Hour()*60 + now.Minute() + 60)}
	})
	h.ends()
	h.wait("b playing", playing("b"))
	h.on(func(p *Player) {
		if p.s.Sleep != nil {
			t.Error("a timer started as the music played on")
		}
	})
}

// TestSleepLeft tells how long the timer runs on: until its time, or to
// the end of the tracks it lets play, at their speed.
func TestSleepLeft(t *testing.T) {
	now := time.Now()
	q := []Item{{UID: "a", Track: track("a")}, {UID: "b", Track: track("b")}, {UID: "x", Track: track("x"), Skip: SkipDisliked},
		{UID: "c", Track: track("c")}, {UID: "r", Track: track("r"), Radio: true}}
	st := State{Queue: q, Index: 0, Position: 50 * time.Second, Duration: 200 * time.Second, Speed: 1}
	if _, ok := st.SleepLeft(now); ok {
		t.Error("a timer left with none")
	}
	for _, c := range []struct {
		sleep  Sleep
		repeat Repeat
		speed  float64
		want   time.Duration
	}{
		{Sleep{Mode: SleepMinutes, Ends: now.Add(time.Minute)}, RepeatOff, 1, time.Minute},
		{Sleep{Mode: SleepMinutes, Ends: now.Add(-time.Minute)}, RepeatOff, 1, 0},
		{Sleep{Mode: SleepMinutes, Finishing: true}, RepeatOff, 1, 150 * time.Second},
		{Sleep{Mode: SleepTracks, Tracks: 1}, RepeatOff, 2, 75 * time.Second},
		{Sleep{Mode: SleepTracks, Tracks: 3}, RepeatOff, 1, 550 * time.Second},
		{Sleep{Mode: SleepTracks, Tracks: 9}, RepeatOff, 1, 750 * time.Second},
		{Sleep{Mode: SleepTracks, Tracks: 6}, RepeatAll, 1, 1150 * time.Second},
		{Sleep{Mode: SleepTracks, Tracks: 3}, RepeatOne, 1, 550 * time.Second},
		{Sleep{Mode: SleepQueue}, RepeatOff, 1, 550 * time.Second},
		{Sleep{Mode: SleepQueue}, RepeatOne, 1, 150 * time.Second},
	} {
		st.Sleep, st.Repeat, st.Speed = &c.sleep, c.repeat, c.speed
		if got, ok := st.SleepLeft(now); !ok || got != c.want {
			t.Errorf("%+v on repeat %s at %v×: %v left, want %v", c.sleep, c.repeat, c.speed, got, c.want)
		}
	}
	// The radio plays: the queue ends after it.
	st.Index, st.Sleep, st.Repeat, st.Speed = 4, &Sleep{Mode: SleepQueue}, RepeatOff, 1
	if got, _ := st.SleepLeft(now); got != 150*time.Second {
		t.Errorf("in the radio, %v left", got)
	}
}
