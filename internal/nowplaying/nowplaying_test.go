package nowplaying

import (
	"sync"
	"testing"
	"time"
)

func TestPosition(t *testing.T) {
	at := time.Unix(1000, 0)
	i := Info{Duration: 60 * time.Second, Position: 10 * time.Second, At: at, Playing: true}
	if got := i.position(at.Add(5 * time.Second)); got != 15*time.Second {
		t.Errorf("playing: %v, want 15s", got)
	}
	if got := i.position(at.Add(time.Hour)); got != 60*time.Second {
		t.Errorf("past the end: %v, want the duration", got)
	}
	i.Playing = false
	if got := i.position(at.Add(5 * time.Second)); got != 10*time.Second {
		t.Errorf("paused: %v, want 10s", got)
	}
	i.Duration, i.Playing = 0, true
	if got := i.position(at.Add(time.Hour)); got != time.Hour+10*time.Second {
		t.Errorf("without a duration: %v", got)
	}
}

func TestDiff(t *testing.T) {
	at := time.Unix(1000, 0)
	base := &Info{ID: "yt:a", Title: "A", Artists: []string{"X"}, Duration: time.Minute, Position: 10 * time.Second, At: at, Playing: true, Volume: 1}
	with := func(f func(*Info)) *Info {
		i := *base
		i.Artists = append([]string(nil), base.Artists...)
		f(&i)
		return &i
	}
	for _, c := range []struct {
		name string
		next *Info
		want change
	}{
		{"a second on", with(func(i *Info) { i.Position, i.At = 11*time.Second, at.Add(time.Second) }), change{}},
		{"a second on, a little late", with(func(i *Info) { i.Position, i.At = 10200*time.Millisecond, at.Add(time.Second) }), change{}},
		{"a seek ahead", with(func(i *Info) { i.Position, i.At = 40*time.Second, at.Add(time.Second) }), change{seeked: true}},
		{"a seek back", with(func(i *Info) { i.Position, i.At = 2*time.Second, at.Add(time.Second) }), change{seeked: true}},
		{"a stall", with(func(i *Info) { i.At = at.Add(3 * time.Second) }), change{seeked: true}},
		{"paused", with(func(i *Info) {
			i.Playing, i.Position, i.At = false, 10500*time.Millisecond, at.Add(500*time.Millisecond)
		}), change{playing: true}},
		{"another track", with(func(i *Info) { i.ID, i.Position = "yt:b", 0 }), change{track: true}},
		{"another artist", with(func(i *Info) { i.Artists = []string{"X", "Y"} }), change{track: true}},
		{"its artwork", with(func(i *Info) { i.Artwork = "https://example.com/a.jpg" }), change{track: true}},
		{"its duration", with(func(i *Info) { i.Duration = 2 * time.Minute }), change{track: true}},
		{"the volume", with(func(i *Info) { i.Volume = 0.5 }), change{volume: true}},
	} {
		if got := diff(base, c.next); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
	if c := diff(nil, base); !c.track || !c.playing {
		t.Errorf("from nothing: %+v", c)
	}
}

// fakePlatform keeps what it was asked to show.
type fakePlatform struct {
	mu     sync.Mutex
	shown  []*Info
	closed bool
	// wait holds show until closed, when set.
	wait chan struct{}
}

func (f *fakePlatform) show(i *Info) {
	if f.wait != nil {
		<-f.wait
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shown = append(f.shown, i)
}

func (f *fakePlatform) close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
}

func (f *fakePlatform) calls() []*Info {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*Info(nil), f.shown...)
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !ok(); {
		if time.Now().After(deadline) {
			t.Fatalf("%s didn't happen", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func request(t *testing.T, got <-chan Request) Request {
	t.Helper()
	select {
	case r := <-got:
		return r
	case <-time.After(5 * time.Second):
		t.Fatal("no request")
	}
	return Request{}
}

func TestControls(t *testing.T) {
	f := &fakePlatform{wait: make(chan struct{})}
	got := make(chan Request, requests)
	c := newControls()
	c.start(f, func(r Request) { got <- r })

	// Shows while the system is busy come down to the last.
	artists := []string{"X"}
	c.Show(&Info{ID: "1", Title: "One\xff", Artists: artists})
	time.Sleep(20 * time.Millisecond) // the first show waits on f
	c.Show(&Info{ID: "2", Title: "Two"})
	c.Show(&Info{ID: "3", Title: "Three"})
	artists[0] = "changed" // Show copied it
	close(f.wait)
	eventually(t, "showing the last", func() bool {
		calls := f.calls()
		return len(calls) > 0 && calls[len(calls)-1] != nil && calls[len(calls)-1].ID == "3"
	})
	calls := f.calls()
	if len(calls) != 2 || calls[0].ID != "1" || calls[0].Artists[0] != "X" {
		t.Fatalf("shown: %d calls, the first %+v", len(calls), calls[0])
	}
	if calls[0].Title != "One�" {
		t.Errorf("a title that isn't UTF-8 showed as %q", calls[0].Title)
	}

	c.ask(Request{Command: Seek, Position: time.Second})
	if r := <-got; r.Command != Seek || r.Position != time.Second {
		t.Errorf("the handler got %+v", r)
	}

	c.Show(nil)
	eventually(t, "showing nothing", func() bool {
		calls := f.calls()
		return calls[len(calls)-1] == nil
	})
	c.Show(nil) // nothing shows already
	c.Close()
	if calls := f.calls(); len(calls) != 3 || !f.closed {
		t.Errorf("after close: %d calls, closed %v", len(calls), f.closed)
	}
	// What the system asks after Close goes nowhere, and doesn't block.
	for range requests + 1 {
		c.ask(Request{Command: Play})
	}
}

func TestCloseShowsNothing(t *testing.T) {
	f := &fakePlatform{}
	c := newControls()
	c.start(f, func(Request) {})
	c.Show(&Info{ID: "1"})
	eventually(t, "showing", func() bool { return len(f.calls()) == 1 })
	c.Close()
	if calls := f.calls(); len(calls) != 2 || calls[1] != nil || !f.closed {
		t.Errorf("closing what shows: %d calls, closed %v", len(calls), f.closed)
	}
}

// In a Flatpak, the player takes a name under the Flatpak's ID, the only
// names its sandbox lets it own, and gives the Flatpak's desktop entry.
func TestMPRISNames(t *testing.T) {
	for _, c := range []struct{ id, bus, entry string }{
		{"", "org.mpris.MediaPlayer2.mixmusic", "mixmusic"},
		{"io.github.fzn0x.mixmusic", "org.mpris.MediaPlayer2.io.github.fzn0x.mixmusic", "io.github.fzn0x.mixmusic"},
	} {
		if bus, entry := mprisNames(c.id); bus != c.bus || entry != c.entry {
			t.Errorf("mprisNames(%q) = %q, %q, want %q, %q", c.id, bus, entry, c.bus, c.entry)
		}
	}
}
