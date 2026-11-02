package main

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// speedButtonLabel is the label of the speed button by the player at a
// speed it can change.
func speedButtonLabel(v float64) string {
	if v == 1 {
		return "Speed"
	}
	return "Speed: " + speedLabel(v)
}

// textCount counts the texts and labels of the window that are s, not
// those that hold it, as 1.1× holds 1×.
func textCount(tt *ui.Tester, s string) int {
	n := 0
	for _, text := range tt.Texts() {
		if text == s {
			n++
		}
	}
	return n
}

// showsExactly reports whether the window shows the text s itself, or an
// element labeled s.
func showsExactly(tt *ui.Tester, s string) bool { return textCount(tt, s) > 0 }

// openSpeedMenu opens the menu of the speed button by the player, which
// shows the speed v, and checks it does.
func openSpeedMenu(t *testing.T, s *shell, tt *ui.Tester, v float64, title string) {
	t.Helper()
	click(t, s, tt, speedButtonLabel(v))
	if !showsExactly(tt, title) {
		t.Fatalf("the speed's menu doesn't show %q: %q", title, tt.Texts())
	}
	for _, step := range []string{"0.75×", "1×", "1.1×", "1.25×", "1.5×", "1.75×", "2×"} {
		if !showsExactly(tt, step) {
			t.Errorf("the speed's menu doesn't offer %s: %q", step, tt.Texts())
		}
	}
}

// takeSettings has the player take in the settings the shell saved, as the
// app has it do.
func takeSettings(t *testing.T, s *shell, speed float64) {
	t.Helper()
	s.p.SettingsChanged(s.svc.Preferences.Get())
	s.settle(t, func(st player.State) bool { return st.Speed == speed })
}

// TestSpeedMenuSteps sets the speed of the song that plays from the steps
// of the speed button's menu, which close it, and shows it on the button
// unless it's 1×.
func TestSpeedMenuSteps(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := tester(t, s)
	if _, ok := tt.Find("Speed"); !ok || showsExactly(tt, "1×") {
		t.Fatalf("at 1×, the player bar shows %q", tt.Texts())
	}
	was := 1.0
	for _, c := range []struct {
		step string
		want float64
	}{{"1.5×", 1.5}, {"0.75×", 0.75}, {"2×", 2}, {"1.1×", 1.1}, {"1.75×", 1.75}, {"1.25×", 1.25}, {"1×", 1}} {
		openSpeedMenu(t, s, tt, was, "speed of songs")
		click(t, s, tt, c.step)
		if got := s.svc.Preferences.Get(); got.SongSpeed != c.want || got.PodcastSpeed != 1 {
			t.Errorf("%s keeps songs at %v and podcasts at %v", c.step, got.SongSpeed, got.PodcastSpeed)
		}
		if showsExactly(tt, "speed of songs") {
			t.Errorf("the menu stays after %s", c.step)
		}
		if _, ok := tt.Find(speedButtonLabel(c.want)); !ok {
			t.Errorf("after %s, the button isn't %q: %q", c.step, speedButtonLabel(c.want), tt.Texts())
		}
		if on := showsExactly(tt, c.step); on != (c.want != 1) {
			t.Errorf("after %s, the button shows the speed: %v", c.step, on)
		}
		takeSettings(t, s, c.want)
		was = c.want
	}
}

// TestSpeedMenuSlider sets the speeds past the steps with the slider of
// the speed button's menu, which stays open, and tells the speed set.
func TestSpeedMenuSlider(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := tester(t, s)
	openSpeedMenu(t, s, tt, 1, "speed of songs")
	r, ok := tt.Find("Speed of songs")
	if !ok {
		t.Fatalf("no slider in the speed's menu: %q", tt.Texts())
	}
	// The ends, and somewhere near 1.5×, where a pixel of the faces of
	// another system moves the slider a step.
	var got float64
	for _, c := range []struct {
		x        float32
		from, to float64
	}{{r.X + r.W - 1, 3, 3}, {r.X + 1, 0.5, 0.5}, {r.X + 0.4*r.W, 1.4, 1.6}} {
		tt.ClickAt(c.x, r.Y+r.H/2)
		s.drain()
		tt.Frame()
		got = s.svc.Preferences.Get().SongSpeed
		if got < c.from || got > c.to || settings.CleanSpeed(got) != got {
			t.Errorf("the slider keeps the speed %v, want from %v to %v, by steps", got, c.from, c.to)
		}
		if !showsExactly(tt, "speed of songs") {
			t.Fatalf("the menu closed as the slider moved to %v", got)
		}
		// In the menu, by the slider, and on the button.
		if textCount(tt, speedLabel(got)) < 2 {
			t.Errorf("at %v, the menu and the button show %q", got, tt.Texts())
		}
	}
	takeSettings(t, s, got)

	// The button's tip, which would cover the menu, waits for it to close.
	label := speedButtonLabel(got)
	b, ok := tt.Find(label)
	if !ok {
		t.Fatalf("the button isn't %q: %q", label, tt.Texts())
	}
	tips := func() int {
		tt.Move(0, 0)
		tt.Frame()
		tt.Move(b.X+b.W/2, b.Y+b.H/2)
		tt.Advance(time.Second)
		tt.Frame()
		return textCount(tt, label)
	}
	if n := tips(); n != 1 {
		t.Errorf("with the menu open, the button shows its tip: %d of it", n)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if showsExactly(tt, "speed of songs") {
		t.Fatal("Escape leaves the menu open")
	}
	if n := tips(); n != 2 {
		t.Errorf("with the menu closed, the button doesn't show its tip: %d of it", n)
	}
}

// savedSettings are testPlayer's, whose settings are those the app keeps,
// as the app's own services give the player.
type savedSettings struct{ testPlayer }

func (p savedSettings) Settings() settings.Settings { return p.a.Services().Preferences.Get() }

// TestSpeedMenuOfEpisode sets the podcasts' speed while an episode plays,
// which plays at it, and the songs' while a song plays, each apart.
func TestSpeedMenuOfEpisode(t *testing.T) {
	s := newTestShellWith(t, nil, func(a *app.App) player.Services { return savedSettings{testPlayer{a: a}} }, player.Saved{})
	s.p.PlayTracks([]media.Track{testShow.Episodes[0], testTracks[0]}, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := tester(t, s)
	openSpeedMenu(t, s, tt, 1, "speed of podcasts")
	click(t, s, tt, "1.5×")
	if got := s.svc.Preferences.Get(); got.PodcastSpeed != 1.5 || got.SongSpeed != 1 {
		t.Errorf("with an episode, podcasts play at %v and songs at %v", got.PodcastSpeed, got.SongSpeed)
	}
	takeSettings(t, s, 1.5)
	tt.Frame()
	if _, ok := tt.Find("Speed: 1.5×"); !ok {
		t.Errorf("the button doesn't show the podcasts' speed: %q", tt.Texts())
	}

	// The song after it plays at the songs' speed, which the button shows,
	// and the episode before it at the podcasts'.
	s.p.Next()
	s.settle(t, func(st player.State) bool { return st.Index == 1 && st.Playing && st.Speed == 1 })
	tt.Frame()
	openSpeedMenu(t, s, tt, 1, "speed of songs")
	click(t, s, tt, "1.25×")
	if got := s.svc.Preferences.Get(); got.PodcastSpeed != 1.5 || got.SongSpeed != 1.25 {
		t.Errorf("with a song, podcasts play at %v and songs at %v", got.PodcastSpeed, got.SongSpeed)
	}
	takeSettings(t, s, 1.25)

	// The palette's commands change the speed of what plays too.
	s.p.Previous()
	s.settle(t, func(st player.State) bool { return st.Index == 0 && st.Playing && st.Speed == 1.5 })
	tt.Frame()
	s.stepSpeed(1)
	if got := s.svc.Preferences.Get(); got.PodcastSpeed != 1.75 || got.SongSpeed != 1.25 {
		t.Errorf("Speed up, with an episode, keeps podcasts at %v and songs at %v", got.PodcastSpeed, got.SongSpeed)
	}
	if n := s.notes[len(s.notes)-1]; n.Message != "Podcasts play at 1.75×" {
		t.Errorf("Speed up told %q", n.Message)
	}
	s.changeSpeed(1)
	if n := s.notes[len(s.notes)-1]; n.Message != "Podcasts play at their own speed" {
		t.Errorf("Normal speed told %q", n.Message)
	}
}

// deviceServices are testPlayer's, which play Spotify tracks on the
// Spotify device.
type deviceServices struct{ testPlayer }

func (d deviceServices) Prepare(ctx context.Context, t media.Track) (player.Source, error) {
	if t.Source == media.SourceSpotify {
		return player.Source{Mode: player.ModeConnect, Track: t}, nil
	}
	return d.testPlayer.Prepare(ctx, t)
}

// TestSpeedButtonOnSpotify dims the speed button while a Spotify device
// plays the track, which opens no menu, and whose tip tells why, as the
// mini player's menu does.
func TestSpeedButtonOnSpotify(t *testing.T) {
	s := newTestShellWith(t, nil, func(a *app.App) player.Services { return deviceServices{testPlayer{a: a}} }, player.Saved{})
	s.p.PlayTracks([]media.Track{testTracks[1], testTracks[0]}, 0)
	s.settle(t, func(st player.State) bool { return st.Source != nil && st.Source.Mode == player.ModeConnect })
	tt := tester(t, s)
	const label = "Speed: Spotify plays this one at its own speed"
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("the speed button isn't dimmed: %q", tt.Texts())
	}
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	s.drain()
	tt.Frame()
	if showsExactly(tt, "speed of songs") || showsExactly(tt, "0.75×") {
		t.Errorf("the dimmed button opened its menu: %q", tt.Texts())
	}
	// The click hid the tip until the pointer leaves: it comes back.
	tt.Move(0, 0)
	tt.Frame()
	before := textCount(tt, label)
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(time.Second)
	tt.Frame()
	if got := textCount(tt, label); got != before+1 {
		t.Errorf("hovering the dimmed button shows %d of %q, not its tip too: %q", got, label, tt.Texts())
	}

	mini := ui.NewTester(s.miniView, 420, 132)
	mini.Frame()
	if err := mini.RightClick(testTracks[1].Title); err != nil {
		t.Fatal(err)
	}
	if got := mini.Menu(); !slices.Equal(got, []string{label}) {
		t.Errorf("the mini player's menu holds %q", got)
	}
	if err := mini.ChooseMenuItem(label); err == nil {
		t.Error("the mini player's menu changes the speed of a Spotify device")
	}

	// The song after it plays in MixMusic, at a speed the button sets.
	s.p.Next()
	s.settle(t, func(st player.State) bool {
		return st.Index == 1 && st.Source != nil && st.Source.Mode == player.ModeStream
	})
	tt.Frame()
	openSpeedMenu(t, s, tt, 1, "speed of songs")
}

// TestSpeedButtonAhead dims the speed button while a Spotify track waits
// to play on its device, as one the last run left, unless Spotify tracks
// play from YouTube Music; and while a live stream plays.
func TestSpeedButtonAhead(t *testing.T) {
	s := newTestShellWith(t, nil, func(a *app.App) player.Services { return testPlayer{a: a} }, player.Saved{Tracks: []media.Track{testTracks[1]}})
	st := s.svc.Preferences.Get()
	st.SpotifyPlayback = settings.SpotifyPlaybackConnect
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	tt := tester(t, s)
	if _, ok := tt.Find("Speed: Spotify plays this one at its own speed"); !ok {
		t.Errorf("a Spotify track on its way to the device leaves the button %q", tt.Texts())
	}
	st.SpotifyPlayback = settings.SpotifyPlaybackMatch
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	tt.Frame()
	if _, ok := tt.Find("Speed"); !ok {
		t.Errorf("a Spotify track from YouTube Music leaves the button %q", tt.Texts())
	}

	s.p.PlayTracks([]media.Track{liveRadio}, 0)
	s.settle(t, func(st player.State) bool { return st.Live && st.Playing })
	tt.Frame()
	if _, ok := tt.Find("Speed: this live stream plays at its own speed"); !ok {
		t.Errorf("a live stream leaves the button %q", tt.Texts())
	}
}

// TestMiniSpeedMenu sets the speed of what plays from the mini player's
// menu: of the steps, and of the speed set when it's none of them.
func TestMiniSpeedMenu(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks([]media.Track{testShow.Episodes[0], testTracks[0]}, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
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
	choose("Speed of podcasts", "1.25×")
	if got := s.svc.Preferences.Get(); got.PodcastSpeed != 1.25 || got.SongSpeed != 1 {
		t.Errorf("with an episode, podcasts play at %v and songs at %v", got.PodcastSpeed, got.SongSpeed)
	}
	s.setSpeed(1.35)
	s.drain()
	tt.Frame()
	choose("Speed of podcasts", "1.35×")
	choose("Speed of podcasts", "2×")
	if got := s.svc.Preferences.Get().PodcastSpeed; got != 2 {
		t.Errorf("podcasts play at %v", got)
	}
	s.p.Next()
	s.settle(t, func(st player.State) bool { return st.Index == 1 && st.Playing })
	tt.Frame()
	choose("Speed of songs", "0.75×")
	if got := s.svc.Preferences.Get(); got.PodcastSpeed != 2 || got.SongSpeed != 0.75 {
		t.Errorf("with a song, podcasts play at %v and songs at %v", got.PodcastSpeed, got.SongSpeed)
	}
}

// TestMenuSpeeds offers the steps, and the speed set among them, in order.
func TestMenuSpeeds(t *testing.T) {
	for v, want := range map[float64]string{
		1:    "0.75× 1× 1.1× 1.25× 1.5× 1.75× 2×",
		1.35: "0.75× 1× 1.1× 1.25× 1.35× 1.5× 1.75× 2×",
		0.5:  "0.5× 0.75× 1× 1.1× 1.25× 1.5× 1.75× 2×",
		3:    "0.75× 1× 1.1× 1.25× 1.5× 1.75× 2× 3×",
	} {
		var got []string
		for _, s := range menuSpeeds(v) {
			got = append(got, speedLabel(s))
		}
		if strings.Join(got, " ") != want {
			t.Errorf("at %v×, the menu offers %q", v, got)
		}
	}
	if menuSpeeds(1.35); !slices.Equal(speedSteps, []float64{0.75, 1, 1.1, 1.25, 1.5, 1.75, 2}) {
		t.Errorf("the steps became %v", speedSteps)
	}
}

// TestSpeedButtonFits keeps the player bar's buttons in the window at its
// least width, with the speed shown: the volume's slider gives up room.
func TestSpeedButtonFits(t *testing.T) {
	for _, layout := range []settings.Layout{settings.LayoutBlend, settings.LayoutTiles} {
		s := newTestShell(t)
		st := s.svc.Preferences.Get()
		st.Layout, st.SongSpeed = layout, 1.75
		if _, err := s.svc.Preferences.Set(st); err != nil {
			t.Fatal(err)
		}
		s.refresh()
		s.p.PlayTracks(testTracks[:1], 0)
		s.settle(t, func(st player.State) bool { return st.Playing })
		tt := testerOf(t, s, 940, 600)
		for _, label := range []string{"Speed: 1.75×", "Volume", "Open the mini player"} {
			if r, ok := tt.Find(label); !ok || r.W == 0 || r.X+r.W > 940 {
				t.Errorf("%s: %q is at %v, out of a window 940 wide", layout, label, r)
			}
		}
		if r, _ := tt.Find("Volume"); r.W < 40 {
			t.Errorf("%s: the volume's slider is %v wide", layout, r.W)
		}
	}
}
