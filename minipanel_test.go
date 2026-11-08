package main

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/cast"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/lyrics"
)

// miniShell is a test shell playing the test's tracks, whose computer has
// the output devices given and whose network has the cast devices given:
// the output panel asks for both as it opens. The first track has the
// lyrics of lanterns, and the others none, which the lyrics panel shows
// without asking for them.
func miniShell(t *testing.T, outputs []app.OutputDevice, casts ...cast.Device) *shell {
	t.Helper()
	s := newTestShellOf(t, func(o *app.Options) {
		o.OutputDevices = func() ([]app.OutputDevice, error) { return outputs, nil }
		o.CastBrowse = func(context.Context) ([]cast.Device, error) { return casts, nil }
	})
	for i, tr := range testTracks {
		l := &loaded[lyrics.Lyrics]{done: true, err: lyrics.ErrNotFound}
		if i == 0 {
			l = &loaded[lyrics.Lyrics]{done: true,
				value: lyrics.Lyrics{Synced: true, Source: "Unison", SourceURL: "https://unison.boidu.dev", Language: "en", Lines: lanterns}}
		}
		s.loads["lyrics:"+tr.ID] = l
	}
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	return s
}

// showsAll fails the test unless the window shows each of texts, or an
// element labeled so.
func showsAll(t *testing.T, tt *ui.Tester, where string, texts ...string) {
	t.Helper()
	for _, text := range texts {
		if !tt.HasText(text) {
			t.Errorf("%s doesn't show %q: %q", where, text, tt.Texts())
		}
	}
}

// TestMiniPanelQueue opens the queue below the mini player, where the
// screen has room: the window grows by the panel's height over a moment,
// the player staying where it was, and the panel lists what plays from the
// track playing, with its place in the queue. A click on an entry plays
// it, each has its menu, and closed, the window shrinks back.
func TestMiniPanelQueue(t *testing.T) {
	s := miniShell(t, nil)
	tt, f := miniAt(t, s, 100, 100, 420, 132)
	click(t, s, tt, "Queue")
	f.settle(t, s)
	if want := (mygo.Rectangle{X: 100, Y: 100, Width: 420, Height: 132 + miniPanelHeight}); f.rect != want {
		t.Fatalf("with the queue open, the window is at %+v, not %+v", f.rect, want)
	}
	steps := 0
	for i, r := range f.moves {
		if r.X != 100 || r.Y != 100 || i > 0 && r.Height <= f.moves[i-1].Height {
			t.Fatalf("the window grew by %+v", f.moves)
		}
		if r.Height > 132 && r.Height < 132+miniPanelHeight {
			steps++
		}
	}
	if steps < 3 {
		t.Errorf("the window grew at once, not over a moment: %+v", f.moves)
	}
	if r, _ := tt.Find("Open MixMusic"); r.Y > 30 {
		t.Errorf("the player moved down with the panel below it: its button is at %+v", r)
	}
	showsAll(t, tt, "the queue", s.lk.words("Up next"), "1 of 6", testTracks[0].Title+", playing", "Play Teardrop", "Massive Attack")
	if a, ok := tt.Access("Queue"); !ok || !a.Expanded {
		t.Errorf("the queue's button doesn't tell it shows its panel: %+v", a)
	}

	click(t, s, tt, "Play Strobe")
	s.settle(t, func(st player.State) bool { return st.Index == 2 })
	tt.Frame()
	showsAll(t, tt, "the queue", "3 of 6", testTracks[2].Title+", playing", "Play Windowlicker")
	if tt.HasText("Play Teardrop") {
		t.Errorf("the queue lists what played before the track playing: %q", tt.Texts())
	}

	if err := tt.RightClick("Play Windowlicker"); err != nil {
		t.Fatal(err)
	}
	if menu := tt.Menu(); !slices.Contains(menu, "Play") || !slices.Contains(menu, "Remove from the queue") {
		t.Fatalf("an entry's menu holds %q", menu)
	}
	if err := tt.ChooseMenuItem("Remove from the queue"); err != nil {
		t.Fatal(err)
	}
	s.settle(t, func(st player.State) bool { return len(st.Queue) == 5 })
	tt.Frame()
	showsAll(t, tt, "the queue", "3 of 5")

	click(t, s, tt, "Close the queue")
	f.settle(t, s)
	if want := (mygo.Rectangle{X: 100, Y: 100, Width: 420, Height: 132}); f.rect != want {
		t.Errorf("with the queue closed, the window is at %+v, not %+v", f.rect, want)
	}
	if tt.HasText("3 of 5") {
		t.Errorf("the queue shows once closed: %q", tt.Texts())
	}
}

// TestMiniPanelAbove opens the panel above the mini player at the foot of
// the screen, the player staying where it was, and back.
func TestMiniPanelAbove(t *testing.T) {
	s := miniShell(t, nil)
	y := workArea.Height - 132 - 20
	tt, f := miniAt(t, s, 1400, y, 420, 132)
	click(t, s, tt, "Lyrics")
	f.settle(t, s)
	if want := (mygo.Rectangle{X: 1400, Y: y - miniPanelHeight, Width: 420, Height: 132 + miniPanelHeight}); f.rect != want {
		t.Fatalf("with the lyrics open, the window is at %+v, not %+v", f.rect, want)
	}
	for _, r := range f.moves {
		if r.Y+r.Height != y+132 {
			t.Fatalf("the player moved as the window grew above it: %+v", f.moves)
		}
	}
	if r, _ := tt.Find("Open MixMusic"); r.Y < miniPanelHeight {
		t.Errorf("the player isn't under the panel: its button is at %+v", r)
	}
	tt.Key(0, ui.KeyEscape)
	f.settle(t, s)
	if want := (mygo.Rectangle{X: 1400, Y: y, Width: 420, Height: 132}); f.rect != want {
		t.Errorf("with the lyrics closed, the window is at %+v, not %+v", f.rect, want)
	}
}

// TestMiniPanelPlace places the panel where the screen has room for it:
// below, above at the screen's foot, or where there's the most room on a
// short screen, the window kept on it.
func TestMiniPanelPlace(t *testing.T) {
	for _, c := range []struct {
		name      string
		at, work  mygo.Rectangle
		below     bool
		grown     mygo.Rectangle
		shrunkTop int
	}{
		{"room below", mygo.Rectangle{X: 100, Y: 100, Width: 420, Height: 132}, workArea, true,
			mygo.Rectangle{X: 100, Y: 100, Width: 420, Height: 352}, 100},
		{"at the screen's foot", mygo.Rectangle{X: 100, Y: 900, Width: 420, Height: 132}, workArea, false,
			mygo.Rectangle{X: 100, Y: 680, Width: 420, Height: 352}, 900},
		{"a short screen", mygo.Rectangle{X: 0, Y: 150, Width: 340, Height: 112}, mygo.Rectangle{Width: 1280, Height: 400}, false,
			mygo.Rectangle{X: 0, Y: 0, Width: 340, Height: 332}, 220},
		{"a screen under another", mygo.Rectangle{X: 0, Y: 1100, Width: 420, Height: 132}, mygo.Rectangle{Y: 1080, Width: 1920, Height: 1040}, true,
			mygo.Rectangle{X: 0, Y: 1100, Width: 420, Height: 352}, 1100},
	} {
		below := panelBelow(c.at, c.work, miniPanelHeight)
		grown := grownFor(c.at, c.work, miniPanelHeight, below)
		if below != c.below || grown != c.grown {
			t.Errorf("%s: the panel goes below %v, the window to %+v; want %v, %+v", c.name, below, grown, c.below, c.grown)
		}
		if got := shrunkFrom(grown, c.at.Height, below); got.Y != c.shrunkTop || got.Height != c.at.Height {
			t.Errorf("%s: the window shrinks back to %+v", c.name, got)
		}
	}
}

// TestMiniPanelKept opens the mini player with the panel it showed as it
// closed, its window keeping the player's size.
func TestMiniPanelKept(t *testing.T) {
	s := miniShell(t, nil)
	tt, f := miniAt(t, s, 100, 100, 420, 132)
	click(t, s, tt, "Lyrics")
	f.settle(t, s)
	if got := s.svc.Preferences.Get().MiniPanel; got != settings.MiniLyrics {
		t.Fatalf("the settings keep the panel %q", got)
	}
	// Closed with the panel open, the window keeps the player's size.
	s.compactMini()
	if want := (mygo.Rectangle{X: 100, Y: 100, Width: 420, Height: 132}); f.rect != want {
		t.Errorf("closed with the lyrics open, the window is at %+v, not %+v", f.rect, want)
	}

	next := newShell(s.a, s.p)
	// What the first shell was given in place of the network.
	maps.Copy(next.loads, s.loads)
	tt2, f2 := miniAt(t, next, 100, 100, 420, 132)
	if f2.rect.Height != 132+miniPanelHeight || next.mini.panel != settings.MiniLyrics {
		t.Fatalf("the next mini player opens at %+v, with the panel %q", f2.rect, next.mini.panel)
	}
	if a, ok := tt2.Access("Lyrics"); !ok || !a.Expanded {
		t.Errorf("the lyrics' button doesn't tell its panel shows: %+v", a)
	}
	click(t, next, tt2, "Close the lyrics")
	f2.settle(t, next)
	if got := next.svc.Preferences.Get().MiniPanel; got != "" {
		t.Errorf("the settings keep the panel %q once it closed", got)
	}
}

// TestMiniPanelSizes opens each panel at sizes the mini player takes, the
// least with Windows' borders too: the window grows by the panel's height,
// the player's controls stay in the player, in the window, and its menu
// holds none of them.
func TestMiniPanelSizes(t *testing.T) {
	for _, size := range [][2]int{{420, 132}, {404, 124}, {340, 112}, {324, 104}, {600, 180}} {
		w, h := size[0], size[1]
		s := miniShell(t, nil)
		tt, f := miniAt(t, s, 100, 100, w, h)
		for _, p := range []string{"Queue", "Lyrics", "Output device"} {
			click(t, s, tt, p)
			f.settle(t, s)
			if f.rect.Height != h+miniPanelHeight || f.rect.Width != w {
				t.Errorf("in %dx%d, the window with the %s is at %+v", w, h, p, f.rect)
			}
			for _, label := range []string{"Open MixMusic", "Close the mini player", "Queue", "Lyrics", "Output device",
				"Shuffle", "Previous", "Pause", "Next", "Repeat: off", "Like", "Dislike", "Mute", "Seek"} {
				r, ok := tt.Find(label)
				if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > float32(w)+0.01 || r.Y+r.H > float32(h)+0.01 {
					t.Errorf("in %dx%d with the %s open, %s is at %+v (%v), out of the player", w, h, p, label, r, ok)
				}
			}
			if s.mini.hidden != 0 {
				t.Errorf("in %dx%d, the mini player's menu holds buttons: %b", w, h, s.mini.hidden)
			}
			tt.Key(0, ui.KeyEscape)
			f.settle(t, s)
			if f.rect.Height != h {
				t.Errorf("in %dx%d, the window with the %s closed is at %+v", w, h, p, f.rect)
			}
		}
	}
}

// TestMiniPanelZoom grows the window by the panel's height at the zoom,
// and holds it there while the panel shows: the window can't be made
// shorter than the player and the panel, and back.
func TestMiniPanelZoom(t *testing.T) {
	s := miniShell(t, nil)
	s.set.Zoom = 1.25
	tt, f := miniAt(t, s, 100, 100, 525, 165)
	click(t, s, tt, "Queue")
	f.settle(t, s)
	if f.rect.Height != 165+275 || f.minH != 440 || f.maxH != 500 {
		t.Errorf("at 125%%, the window with the queue is at %+v, from %d to %d high", f.rect, f.minH, f.maxH)
	}
	click(t, s, tt, "Close the queue")
	f.settle(t, s)
	if f.rect.Height != 165 || f.minH != 140 || f.maxH != 225 {
		t.Errorf("at 125%%, the window without the queue is at %+v, from %d to %d high", f.rect, f.minH, f.maxH)
	}
}

// TestMiniPanelSwitch shows one panel at a time: another's button shows
// its panel in the place of the first, and the window doesn't move.
func TestMiniPanelSwitch(t *testing.T) {
	s := miniShell(t, nil)
	tt, f := miniAt(t, s, 100, 100, 420, 132)
	click(t, s, tt, "Queue")
	f.settle(t, s)
	moves := len(f.moves)
	click(t, s, tt, "Output device")
	f.settle(t, s)
	if len(f.moves) != moves || f.rect.Height != 132+miniPanelHeight {
		t.Errorf("the window moved from the queue to the output devices: %+v", f.moves[moves:])
	}
	if tt.HasText("1 of 6") || !tt.HasText("System default") {
		t.Errorf("the output devices don't show in the queue's place: %q", tt.Texts())
	}
	if a, _ := tt.Access("Queue"); a.Expanded {
		t.Error("the queue's button tells its panel shows in the output devices'")
	}
}

// TestMiniPanelMotion grows the window at once where the computer asks for
// less motion.
func TestMiniPanelMotion(t *testing.T) {
	s := miniShell(t, nil)
	tt, f := miniAt(t, s, 100, 100, 420, 132)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	click(t, s, tt, "Queue")
	f.settle(t, s)
	if len(f.moves) != 1 || f.rect.Height != 132+miniPanelHeight {
		t.Errorf("under Reduce Motion, the window grew by %+v", f.moves)
	}
}

// TestMiniPanelKeys reaches every control of the mini player with Tab, the
// panel's after the player's, and closes the panel with Escape, which
// gives the focus back to its button; Escape in the volume's slider
// closes only the slider. Screen readers name the controls, and tell the
// state of shuffle and of the panels' buttons.
func TestMiniPanelKeys(t *testing.T) {
	s := miniShell(t, nil)
	tt, f := miniAt(t, s, 100, 100, 420, 132)
	for _, label := range []string{"Open MixMusic", "Close the mini player", "Queue", "Lyrics", "Output device",
		"Shuffle", "Previous", "Pause", "Next", "Repeat: off", "Like", "Dislike", "Mute", "Volume", "Seek"} {
		tabTo(t, tt, label, "")
		if _, ok := tt.Access(label); !ok {
			t.Errorf("screen readers don't find %q", label)
		}
	}
	tabTo(t, tt, "Queue", "")
	tt.Key(0, ui.KeyEnter)
	f.settle(t, s)
	if s.mini.panel != settings.MiniQueue {
		t.Fatalf("Enter on the queue's button shows the panel %q", s.mini.panel)
	}
	tabTo(t, tt, "Seek", "")
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if !tt.Focused("Close the queue") {
		t.Errorf("Tab after the player doesn't go into the panel")
	}
	tabTo(t, tt, "Play Teardrop", "")
	// The volume's slider, which Tab passed by, and the tip under the
	// pointer go: each would take Escape first.
	tt.Move(2, 2)
	tt.Advance(time.Second)
	tt.Key(0, ui.KeyEscape)
	f.settle(t, s)
	if s.mini.panel != "" || f.rect.Height != 132 {
		t.Fatalf("Escape leaves the panel %q, the window at %+v", s.mini.panel, f.rect)
	}
	if !tt.Focused("Queue") {
		t.Error("Escape doesn't give the focus back to the queue's button")
	}

	click(t, s, tt, "Lyrics")
	f.settle(t, s)
	tabTo(t, tt, "Mute", "")
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if !tt.Focused("Volume") {
		t.Fatal("Tab from the volume button doesn't go into its slider")
	}
	tt.Key(0, ui.KeyEscape)
	f.settle(t, s)
	if _, ok := tt.Find("Volume"); ok || s.mini.panel != settings.MiniLyrics {
		t.Errorf("Escape in the volume's slider: the slider shows %v, the panel is %q", ok, s.mini.panel)
	}
}

// TestMiniLyricsPanel follows the song in the lyrics panel: the line sung
// now, word by word, and the line after it.
func TestMiniLyricsPanel(t *testing.T) {
	s := miniShell(t, nil)
	tt, f := miniAt(t, s, 100, 100, 420, 132)
	click(t, s, tt, "Lyrics")
	f.settle(t, s)
	s.p.Time(2500*time.Millisecond, 200*time.Second)
	s.settle(t, func(st player.State) bool { return st.Position == 2500*time.Millisecond })
	tt.Frame()
	showsAll(t, tt, "the lyrics", "Lanterns drifting home", "Quiet harbour", "https://unison.boidu.dev")
	sung, ok := tt.Find("Lanterns drifting home")
	next, _ := tt.Find("Quiet harbour")
	if !ok || next.Y <= sung.Y {
		t.Errorf("the line after the one sung shows at %+v, the one sung at %+v", next, sung)
	}
	img := tt.Image()
	words := miniWordsOf(s, sung, "Lanterns drifting home", "Lanterns", "home")
	if !lit(t, s, img, words[0], "Lanterns") || lit(t, s, img, words[1], "home") {
		t.Error("the words of the line sung now don't light up as they're sung")
	}
	s.p.Time(60200*time.Millisecond, 200*time.Second)
	s.settle(t, func(st player.State) bool { return st.Position == 60200*time.Millisecond })
	tt.Frame()
	if tt.HasText("Lanterns drifting home") || !tt.HasText("Quiet harbour") {
		t.Errorf("the lyrics don't follow the song: %q", tt.Texts())
	}
}

// miniWordsOf is where the words given of the line text show in the mini
// player's lyrics, at r, which has no padding.
func miniWordsOf(s *shell, r ui.Rect, text string, words ...string) []ui.Rect {
	k := &s.lk
	font := ui.Font{Family: k.body, Size: k.size(1.25), Weight: k.leadWeight()}
	width := func(s string) float32 { w, _ := ui.ShapeText(s, font, 0).Size(); return w }
	var out []ui.Rect
	for _, w := range words {
		before := text[:strings.Index(text, w)]
		x0, x1 := width(before), width(before+w)
		out = append(out, ui.Rect{X: r.X + x0 + 1, Y: r.Y + 2, W: x1 - x0 - 2, H: r.H - 4})
	}
	return out
}

// TestMiniOutputPanel chooses where what plays plays from the output
// panel: the devices of this computer, and the cast devices on the
// network beside it, which screen readers read as choices.
func TestMiniOutputPanel(t *testing.T) {
	s := miniShell(t, []app.OutputDevice{{ID: "hp", Name: "Headphones"}, {ID: "sp", Name: "Speakers", Default: true}},
		cast.Device{ID: "tv", Name: "Living Room TV", Model: "Chromecast", Host: "192.0.2.1", Port: 8009})
	tt, f := miniAt(t, s, 100, 100, 420, 132)
	click(t, s, tt, "Output device")
	f.settle(t, s)
	until(t, s, tt, "the devices", func() bool {
		f.run()
		return tt.HasText("Headphones") && tt.HasText("Living Room TV")
	})
	if settings.NativeAudio {
		if a, ok := tt.AccessByRole(ui.RoleRadio, "System default, Speakers"); !ok || !a.Checked {
			t.Errorf("the system's default isn't the choice: %+v (%v)", a, ok)
		}
		click(t, s, tt, "Headphones")
		if s.set.OutputDevice.ID != "hp" {
			t.Errorf("choosing the headphones chose %+v", s.set.OutputDevice)
		}
		tt.Frame()
		if a, _ := tt.Access("Headphones"); !a.Checked {
			t.Error("the headphones chosen don't show it")
		}
	}
	if a, ok := tt.AccessByRole(ui.RoleRadio, "This computer"); !ok || !a.Checked {
		t.Errorf("this computer isn't the cast's choice: %+v (%v)", a, ok)
	}
	showsAll(t, tt, "the cast devices", "Living Room TV", "Chromecast")
}

// TestMiniConnectPanel lists the Spotify Connect devices in the output
// panel while Spotify tracks play on Spotify Connect.
func TestMiniConnectPanel(t *testing.T) {
	s := miniShell(t, nil)
	st := s.svc.Preferences.Get()
	st.SpotifyPlayback = settings.SpotifyPlaybackConnect
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	tt, f := miniAt(t, s, 100, 100, 420, 132)
	click(t, s, tt, "Output device")
	// What Spotify answers, without asking it.
	s.loads["devices"] = &loaded[[]spotify.Device]{done: true, value: []spotify.Device{{ID: "phone", Name: "Phone", Active: true}}}
	f.settle(t, s)
	showsAll(t, tt, "Spotify Connect's devices", "The active device", "Phone", "active")
	if a, ok := tt.AccessByRole(ui.RoleRadio, "The active device"); !ok || !a.Checked {
		t.Errorf("the active device isn't the choice: %+v (%v)", a, ok)
	}
}

// TestMiniErrorPanel tells of a track that couldn't play since the mini
// player opened, with a button whose panel shows why, with Copy; closed,
// the button goes until another fails.
func TestMiniErrorPanel(t *testing.T) {
	var copied []string
	was := copyText
	copyText = func(text string) { copied = append(copied, text) }
	t.Cleanup(func() { copyText = was })
	s := newTestShellWith(t, func(o *app.Options) {
		o.CastBrowse = func(context.Context) ([]cast.Device, error) { return nil, nil }
	}, func(a *app.App) player.Services { return failingPlayer{testPlayer{a: a}} }, player.Saved{})
	tt, f := miniAt(t, s, 100, 100, 420, 132)
	if _, ok := tt.Find("Why it couldn't play"); ok {
		t.Fatal("an error's button before a track failed")
	}
	s.p.PlayTracks([]media.Track{brokenTrack}, 0)
	until(t, s, tt, "the error's button", func() bool { _, ok := tt.Find("Why it couldn't play"); return ok })
	click(t, s, tt, "Why it couldn't play")
	f.settle(t, s)
	showsAll(t, tt, "the error's panel", "Broken Song, by The Band", "What was tried", "2. yt-dlp: this track is private")
	click(t, s, tt, "copy the details")
	if len(copied) != 1 || !strings.Contains(copied[0], "- Track: Broken Song, by The Band") {
		t.Fatalf("copied %q", copied)
	}
	if got := s.svc.Preferences.Get().MiniPanel; got != "" {
		t.Errorf("the settings keep the error's panel as %q", got)
	}
	click(t, s, tt, "Close the details of the error")
	f.settle(t, s)
	if _, ok := tt.Find("Why it couldn't play"); ok {
		t.Error("the error's button stays once its panel closed")
	}
}

// TestMiniPages opens the page a menu of the mini player goes to in the
// main window: the mini player shows none.
func TestMiniPages(t *testing.T) {
	s := miniShell(t, nil)
	tt, f := miniAt(t, s, 100, 100, 420, 132)
	click(t, s, tt, "Queue")
	f.settle(t, s)
	if err := tt.RightClick("Play Teardrop"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Go to album"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	s.drain()
	tt.Frame()
	if at := s.router.Location(); at != "/" {
		t.Errorf("the mini player went to %q", at)
	}
}
