package native

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/lyrics"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// tester is a window of the shell, without one.
func tester(t *testing.T, s *shell) *ui.Tester { return testerOf(t, s, 1280, 820) }

// testerOf is a window of the shell of a size.
func testerOf(t *testing.T, s *shell, w, h int) *ui.Tester {
	t.Helper()
	for _, key := range []string{"home", "explore", "charts"} {
		s.loads[key] = &loaded[[]media.Shelf]{done: true}
	}
	tt := ui.NewTester(s.view, w, h)
	tt.Frame()
	return tt
}

// show shows the page at path, once its transition ends.
func show(s *shell, tt *ui.Tester, path string) {
	s.navigate(path)
	s.drain()
	tt.Frame()
	tt.Advance(time.Second)
}

func click(t *testing.T, s *shell, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatalf("clicking %q: %v; the window shows %q", label, err, tt.Texts())
	}
	s.drain()
	tt.Frame()
}

func TestNavigation(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	for _, n := range []struct{ label, path string }{
		{"library", "/library"}, {"stats", "/stats"}, {"settings", "/settings"}, {"explore", "/explore"}, {"home", "/"},
	} {
		click(t, s, tt, n.label)
		if got := s.router.Path(); got != n.path {
			t.Errorf("clicking %s went to %q", n.label, got)
		}
	}
	click(t, s, tt, "Back")
	if got := s.router.Path(); got != "/explore" {
		t.Errorf("back went to %q", got)
	}
}

func TestTransportClicks(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := tester(t, s)
	click(t, s, tt, "Pause")
	s.settle(t, func(st player.State) bool { return !st.Playing })
	tt.Frame()
	tt.Key(0, ui.KeySpace)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt.Frame()
	click(t, s, tt, "Next")
	s.settle(t, func(st player.State) bool { return st.Index == 1 })
	tt.Frame()
	click(t, s, tt, "Shuffle")
	s.settle(t, func(st player.State) bool { return st.Shuffle })
	tt.Frame()
	click(t, s, tt, "Repeat: off")
	s.settle(t, func(st player.State) bool { return st.Repeat == player.RepeatAll })
	tt.Frame()
	if !tt.HasText(testTracks[1].Title) {
		t.Errorf("the window doesn't show what plays: %q", tt.Texts())
	}
}

func TestQueuePanel(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := tester(t, s)
	// The queue lists what plays next.
	for _, tr := range testTracks[1:] {
		if !tt.HasText(tr.Title) {
			t.Fatalf("the queue doesn't list %s: %q", tr.Title, tt.Texts())
		}
	}
	click(t, s, tt, "Play "+testTracks[3].Title)
	s.settle(t, func(st player.State) bool { return st.Index == 3 })
	tt.Frame()
	click(t, s, tt, "Clear the queue")
	s.settle(t, func(st player.State) bool { return len(st.Queue) == 4 })
	tt.Frame()
	click(t, s, tt, "Close up next")
	if s.panel != "" {
		t.Error("the queue didn't close")
	}
}

// TestQueueMenu plays an entry of the queue from its menu, keeping the
// queue, as a double click does, and takes one out.
func TestQueueMenu(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := tester(t, s)
	choose := func(title string, item ...string) {
		t.Helper()
		if err := tt.RightClick(title); err != nil {
			t.Fatal(err)
		}
		if err := tt.ChooseMenuItem(item...); err != nil {
			t.Fatalf("%v: the menu holds %q", err, tt.Menu())
		}
		tt.Frame()
	}
	choose(testTracks[3].Title, "Play")
	s.settle(t, func(st player.State) bool { return st.Index == 3 })
	if n := len(s.p.State().Queue); n != len(testTracks) {
		t.Errorf("playing from the menu left %d tracks in the queue, of %d", n, len(testTracks))
	}
	tt.Frame()
	choose(testTracks[5].Title, "Remove from the queue")
	s.settle(t, func(st player.State) bool { return len(st.Queue) == len(testTracks)-1 })
}

// TestNowPlayingRelated shows what YouTube Music relates to the track
// under the player, and nothing for a track it relates nothing to.
func TestNowPlayingRelated(t *testing.T) {
	s := newTestShell(t)
	lua := testTrack("yt:9", "Lua", "Bright Eyes", "I'm Wide Awake, It's Morning", media.SourceYouTube)
	s.loads["related-"+testTracks[0].ID] = &loaded[[]media.Shelf]{done: true, value: []media.Shelf{
		{Title: "you might also like", Items: []media.Item{{Kind: media.ItemTrack, Track: &lua}}},
	}}
	s.panel = ""
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	// The shelf is below the fold of a window of the usual size.
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, "/now-playing")
	if !tt.HasText("you might also like") || !tt.HasText("Lua") {
		t.Fatalf("no related shelf: %q", tt.Texts())
	}
	s.p.Next()
	s.settle(t, func(st player.State) bool { return st.Index == 1 })
	tt.Frame()
	tt.Advance(time.Second)
	if tt.HasText("you might also like") {
		t.Error("the shelf of the track before stays")
	}
}

// TestLyricsTwice shows the lyrics on the now playing page and in the side
// panel at once, each in a list of its own.
func TestLyricsTwice(t *testing.T) {
	s := newTestShell(t)
	s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "LRCLIB",
		Lines: []lyrics.Line{{TimeMs: 0, Text: "Waiting in a car"}, {TimeMs: 60000, Text: "Waiting for a ride in the dark"}}}}
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, "/now-playing")
	s.open("/lyrics")
	tt.Frame()
	tt.Advance(time.Second)
	n := 0
	for _, text := range tt.Texts() {
		if text == "Waiting in a car" {
			n++
		}
	}
	if n != 2 {
		t.Errorf("the lyrics show %d times, not on the page and in the panel", n)
	}
}

func TestPaletteRuns(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Frame()
	if !s.paletteOpen {
		t.Fatal("Mod+K didn't open the palette")
	}
	tt.Type("stats")
	tt.Frame()
	// Enter searches what was typed; Down reaches the command.
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
	if s.paletteOpen || s.router.Path() != "/stats" {
		t.Errorf("the palette is open: %v; the page is %q", s.paletteOpen, s.router.Path())
	}
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Frame()
	tt.Type("lua")
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
	if got := s.router.Location(); got != searchPath("lua") {
		t.Errorf("Enter in the palette went to %q, not the search", got)
	}
	tt.Key(ui.Alt, ui.Key3)
	tt.Frame()
	if got := s.router.Path(); got != "/library/liked" {
		t.Errorf("Alt+3 went to %q", got)
	}
}

func TestSearchBox(t *testing.T) {
	s := newTestShell(t)
	s.loads["search:daft punk"] = &loaded[app.SearchResponse]{done: true}
	tt := tester(t, s)
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	tt.Type("daft punk")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if got := s.router.Location(); got != "/search?q=daft+punk" {
		t.Errorf("searching went to %q", got)
	}
}

// TestSettingsKeep shows the settings without changing them: Spotify
// tracks played inside MixMusic, which the native interface's choices
// don't name, stay so.
func TestSettingsKeep(t *testing.T) {
	s := newTestShell(t)
	st := s.svc.Preferences.Get()
	st.SpotifyPlayback = settings.SpotifyPlaybackHere
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	tt := tester(t, s)
	for _, path := range []string{"/settings/playback", "/settings/appearance", "/settings/desktop", "/settings/about"} {
		s.navigate(path)
		s.drain()
		tt.Frame()
		tt.Advance(time.Second)
	}
	if got := s.svc.Preferences.Get().SpotifyPlayback; got != settings.SpotifyPlaybackHere {
		t.Errorf("showing the settings made Spotify playback %q", got)
	}
	click(t, s, tt, "playback")
	click(t, s, tt, "in mixmusic, from youtube music")
	if got := s.svc.Preferences.Get().SpotifyPlayback; got != settings.SpotifyPlaybackMatch {
		t.Errorf("choosing YouTube Music made Spotify playback %q", got)
	}
}

func TestShortcutParsing(t *testing.T) {
	for in, ok := range map[string]bool{"Space": true, "Mod+Shift+L": true, "Alt+1": true, "Mod+,": true, "Shift+ArrowRight": true, "": false, "Hyper+K": false, "Mod+NoSuchKey": false} {
		if _, _, got := parseShortcut(in); got != ok {
			t.Errorf("parseShortcut(%q) = %v, want %v", in, got, ok)
		}
	}
	mods, key, _ := parseShortcut("Mod+Shift+L")
	if mods != ui.Cmd|ui.Shift || key != ui.KeyL {
		t.Errorf("Mod+Shift+L is %v %v", mods, key)
	}
}

// TestCrashReport turns down the offer to report a crash, which isn't made
// again.
func TestCrashReport(t *testing.T) {
	s := newTestShellOf(t, func(o *app.Options) { o.LastCrash = filepath.Join(o.LogsDir, "crash-20261021.log") })
	tt := tester(t, s)
	s.offerCrashReport()
	tt.Frame()
	if !tt.HasText("crash-20261021.log") {
		t.Fatalf("the offer doesn't name the report: %q", tt.Texts())
	}
	click(t, s, tt, "Cancel")
	if s.asking != nil || s.svc.System.LastCrash() != "" {
		t.Errorf("turning the offer down left it: asking %v, crash %q", s.asking != nil, s.svc.System.LastCrash())
	}
}

// TestQueueDrag drags an upcoming entry of the queue onto another, which
// takes it in its place.
func TestQueueDrag(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := tester(t, s)
	from, ok := tt.Find(testTracks[4].Title)
	to, ok2 := tt.Find(testTracks[1].Title)
	if !ok || !ok2 {
		t.Fatalf("the queue doesn't list the tracks: %q", tt.Texts())
	}
	tt.Press(from.X+4, from.Y+4)
	for step := float32(1); step <= 8; step++ {
		tt.Move(from.X+4, from.Y+4+(to.Y-from.Y)*step/8)
		tt.Frame()
	}
	tt.Release(to.X+4, to.Y+4)
	tt.Frame()
	s.settle(t, func(st player.State) bool { return st.Queue[1].Track.ID == testTracks[4].ID })
	if got := s.p.State().Queue[2].Track.ID; got != testTracks[1].ID {
		t.Errorf("the entry it went before is now %s", got)
	}
}
