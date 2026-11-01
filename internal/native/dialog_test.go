package native

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/player"
)

// shows fails unless the element labelled label shows whole in a window
// h tall, as a row of tall: Find tells the part of it the window shows.
func shows(t *testing.T, tt *ui.Tester, h int, label string, tall float32) {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok || r.H < tall-0.5 || r.Y < 0 || r.Y+r.H > float32(h)+0.5 {
		t.Errorf("%q doesn't show whole in a window %d tall: %v (found %v)", label, h, r, ok)
	}
}

// TestPaletteInShortWindow fits the palette's list to a window too short
// for it, whose last command its end shows.
func TestPaletteInShortWindow(t *testing.T) {
	const w, h = 924, 425
	s := newTestShell(t)
	tt := testerOf(t, s, w, h)
	s.paletteOpen = true
	tt.Frame()
	tt.Advance(time.Second)
	cmds := s.commands()
	first, last := cmds[0].title, cmds[len(cmds)-1].title
	r, ok := tt.Find(first)
	if !ok || r.H == 0 {
		t.Fatalf("the palette doesn't show its first command: %q", tt.Texts())
	}
	tt.Scroll(r.X+r.W/2, r.Y+r.H/2, 0, 1e4)
	tt.Frame()
	shows(t, tt, h, last, 36)
}

// TestPaletteFollowsArrows brings the row the arrows choose into view.
func TestPaletteFollowsArrows(t *testing.T) {
	const w, h = 924, 425
	s := newTestShell(t)
	tt := testerOf(t, s, w, h)
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Frame()
	tt.Type("theme")
	tt.Frame()
	var themes []string
	for _, cmd := range s.commands() {
		if cmd.group == "themes" {
			themes = append(themes, cmd.title)
		}
	}
	if len(themes) < 8 {
		t.Fatalf("only %d themes to choose", len(themes))
	}
	// The search for what was typed comes first, then the themes.
	for _, title := range themes {
		tt.Key(0, ui.KeyDown)
		tt.Frame()
		shows(t, tt, h, title, 36)
	}
	for range themes {
		tt.Key(0, ui.KeyUp)
		tt.Frame()
	}
	shows(t, tt, h, "Search for “theme”", 36)
	// The row chosen doesn't keep the list from scrolling away from it.
	r, _ := tt.Find("Search for “theme”")
	tt.Scroll(r.X+r.W/2, r.Y+r.H/2, 0, 1e4)
	tt.Frame()
	tt.Frame()
	shows(t, tt, h, themes[len(themes)-1], 36)
}

// TestOfferFirstFocus opens the update offer with the focus on later, not
// on skip this version, which a Space pressed for the music, or typed in
// the search box the offer takes the focus from, would press.
func TestOfferFirstFocus(t *testing.T) {
	offer := &app.Release{Version: "9.9.9", Newer: true, Page: "https://example.invalid/notes",
		Download: "https://example.invalid/m.deb", File: "mixmusic-9.9.9-linux-amd64.deb"}
	s := newTestShell(t)
	tt := tester(t, s)
	s.offer = offer
	tt.Frame()
	if !tt.Focused("later") || tt.Focused("skip this version") {
		t.Error("the offer opens with the focus elsewhere than on later")
	}

	s = newTestShell(t)
	tt = tester(t, s)
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	tt.Type("lua")
	tt.Frame()
	s.offer = offer
	tt.Frame()
	if !tt.Focused("later") {
		t.Error("the offer, opening over the search box, leaves the focus elsewhere than on later")
	}
}

// longNotes are the notes of a release longer than a short window shows.
const longNotes = "Added\n- Dislike songs, and skip the ones you dislike.\n- Podcasts on YouTube Music, with their episodes.\n" +
	"- A sleep timer that fades the music out.\nFixed\n- The saved queue resumes on the right track past 500 tracks.\n" +
	"- Shuffle no longer repeats a track before the rest have played.\n- A stream that stalls retries from where it stopped.\n" +
	"- The mini player keeps its place on a second display.\n- Lyrics follow a seek at once."

// testOffer is a release whose notes take more room than a short window
// has.
var testOffer = app.Release{Version: "9.9.9", Newer: true, Page: "https://example.invalid/notes",
	Download: "https://example.invalid/m.deb", File: "mixmusic-9.9.9-linux-amd64.deb", Notes: longNotes}

// atTextSize sets the text size of the shell's settings.
func atTextSize(t *testing.T, s *shell, scale float64) {
	t.Helper()
	st := s.svc.Preferences.Get()
	st.FontScale = scale
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
}

// textStarting returns the first text the window shows that starts with
// prefix.
func textStarting(tt *ui.Tester, prefix string) string {
	for _, text := range tt.Texts() {
		if strings.HasPrefix(text, prefix) {
			return text
		}
	}
	return ""
}

// dialogCase is a dialog to open: its title, its buttons, the one to
// press and what pressing it does, and how the last text of its body
// starts, if it can take more room than a short window has.
type dialogCase struct {
	name    string
	open    func(t *testing.T, s *shell, tt *ui.Tester)
	title   string
	buttons []string
	press   string
	done    func(s *shell) bool
	last    string
}

// dialogCases are the dialogs of the window of s.
func dialogCases(s *shell) []dialogCase {
	u := s.svc.System.Updates()
	act := downloadLabel(u.Kind)
	if u.Enabled {
		act = "install 9.9.9"
	}
	named := ""
	return []dialogCase{
		{name: "update offer", title: "mixmusic 9.9.9 is out", buttons: []string{"skip this version", "what's new", "later", act},
			open: func(t *testing.T, s *shell, tt *ui.Tester) {
				offer := testOffer
				s.offer = &offer
			},
			press: "later", done: func(s *shell) bool { return s.offer == nil }, last: "You have "},
		{name: "new mix", title: "new mix", buttons: []string{"cancel", "create"},
			open: func(t *testing.T, s *shell, tt *ui.Tester) {
				s.promptName("New mix", "Name", "late night drive", "create", func(name string) { named = name })
			},
			press: "create", done: func(*shell) bool { return named == "late night drive" }},
		{name: "rename mix", title: "rename mix", buttons: []string{"cancel", "rename"},
			open: func(t *testing.T, s *shell, tt *ui.Tester) {
				p, err := s.svc.Collection.CreatePlaylist("late night drive", "", nil)
				if err != nil {
					t.Fatal(err)
				}
				show(s, tt, playlistPath(p.ID))
				until(t, s, tt, "the mix's page", func() bool { return tt.HasText("your mix") })
				s.renaming, s.renameTo, s.renameID = true, "early morning drive", p.ID
			},
			press: "rename", done: func(s *shell) bool {
				mixes := s.svc.Collection.Playlists()
				return len(mixes) == 1 && mixes[0].Title == "early morning drive"
			}},
	}
}

// TestDialogsInShortWindows keeps the title and the buttons of each
// dialog in a window shorter than the dialog, and scrolls its body to its
// end: in the least room the window leaves on Windows, in that of a small
// screen at a large scale, and in less still, with text at its usual size
// and at its largest.
func TestDialogsInShortWindows(t *testing.T) {
	for _, size := range [][2]int{{924, 561}, {911, 425}, {640, 360}} {
		for _, scale := range []float64{1, 1.3} {
			for i := range dialogCases(newTestShell(t)) {
				w, h := size[0], size[1]
				s := newTestShell(t)
				atTextSize(t, s, scale)
				d := dialogCases(s)[i]
				t.Run(fmt.Sprintf("%s at %dx%d, text %v", d.name, w, h, scale), func(t *testing.T) {
					tt := testerOf(t, s, w, h)
					d.open(t, s, tt)
					tt.Frame()
					tt.Advance(time.Second)
					if r, ok := tt.Find(d.title); !ok || r.Y < 0 || r.Y+r.H > float32(h) {
						t.Fatalf("the dialog doesn't lie in the window: %v (found %v)", r, ok)
					}
					top := float32(h)
					for _, b := range d.buttons {
						shows(t, tt, h, b, 40)
						r, _ := tt.Find(b)
						top = min(top, r.Y)
					}
					if body, ok := tt.Find("more of " + d.title); ok {
						tt.Scroll(body.X+body.W/2, body.Y+body.H/2, 0, 1e4)
						tt.Frame()
						if r, ok := tt.Find(textStarting(tt, d.last)); d.last != "" && (!ok || r.H < 10 || r.Y+r.H > top+0.5) {
							t.Errorf("the body's end doesn't show over the buttons: %v (found %v), buttons from %v", r, ok, top)
						}
					} else if d.name == "update offer" && h <= 425 {
						t.Error("the offer's body doesn't scroll in a short window")
					}
					click(t, s, tt, d.press)
					if !d.done(s) {
						t.Errorf("pressing %s didn't do what it says", d.press)
					}
				})
			}
		}
	}
}

// TestDialogBodyKeys scrolls the body of a dialog too tall for the window
// from the keyboard: Tab reaches it, and its keys scroll it, not the
// volume.
func TestDialogBodyKeys(t *testing.T) {
	s := newTestShell(t)
	atTextSize(t, s, 1.3)
	s.p.PlayTracks(testTracks, 0)
	s.p.SetVolume(0.5)
	s.settle(t, func(st player.State) bool { return st.Playing && st.Volume == 0.5 })
	tt := testerOf(t, s, 911, 425)
	offer := testOffer
	s.offer = &offer
	tt.Frame()
	tt.Advance(time.Second)
	body := "more of mixmusic 9.9.9 is out"
	for range 8 {
		if tt.Focused(body) {
			break
		}
		tt.Key(0, ui.KeyTab)
		tt.Frame()
	}
	if !tt.Focused(body) {
		t.Fatal("Tab doesn't reach the body")
	}
	sc := &s.dialogScrolls["offer"].state
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.Advance(time.Second)
	if sc.Y == 0 {
		t.Error("Down didn't scroll the body")
	}
	tt.Key(0, ui.KeyEnd)
	tt.Frame()
	tt.Advance(time.Second)
	if sc.Y < sc.MaxY-0.5 {
		t.Errorf("End scrolled the body to %v of %v", sc.Y, sc.MaxY)
	}
	s.drain()
	if v := s.p.State().Volume; v != 0.5 {
		t.Errorf("the keys changed the volume to %v", v)
	}
	for range 8 {
		if tt.Focused("later") {
			break
		}
		tt.Key(0, ui.KeyTab)
		tt.Frame()
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if s.offer != nil {
		t.Error("Enter on later left the offer")
	}
}

// TestDialogsInTallWindows keeps the body of a dialog the window shows
// whole out of the keys' way: Tab never stops there.
func TestDialogsInTallWindows(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	offer := testOffer
	s.offer = &offer
	tt.Frame()
	tt.Advance(time.Second)
	for range 6 {
		tt.Key(0, ui.KeyTab)
		tt.Frame()
		if tt.HasText("more of mixmusic 9.9.9 is out") {
			t.Fatal("the body of an offer the window shows whole takes the focus")
		}
	}
	s.offer = nil
	s.promptName("New mix", "Name", "", "create", func(string) {})
	tt.Frame()
	if !tt.Focused("Name") {
		t.Error("the name asked for doesn't have the focus as the dialog opens")
	}
}

// TestDialogOpensAtTop opens a dialog again at the top of its body.
func TestDialogOpensAtTop(t *testing.T) {
	s := newTestShell(t)
	atTextSize(t, s, 1.3)
	tt := testerOf(t, s, 911, 425)
	offer := testOffer
	s.offer = &offer
	tt.Frame()
	tt.Advance(time.Second)
	body, ok := tt.Find("more of mixmusic 9.9.9 is out")
	if !ok {
		t.Fatal("the offer's body doesn't scroll")
	}
	tt.Scroll(body.X+body.W/2, body.Y+body.H/2, 0, 1e4)
	tt.Frame()
	if s.dialogScrolls["offer"].state.Y == 0 {
		t.Fatal("the body didn't scroll")
	}
	click(t, s, tt, "later")
	s.offer = &offer
	tt.Frame()
	tt.Advance(time.Second)
	if y := s.dialogScrolls["offer"].state.Y; y != 0 {
		t.Errorf("the offer opened again scrolled to %v", y)
	}
}

// TestDialogLongTitle scrolls a title that would leave the body too little
// room with the body, in a short window, and keeps it over the body in a
// window with room for both.
func TestDialogLongTitle(t *testing.T) {
	const title = "New playlist on YouTube Music for the late night listening sessions"
	field := func(w, h int) (*shell, *ui.Tester, ui.Rect) {
		s := newTestShell(t)
		atTextSize(t, s, 1.3)
		tt := testerOf(t, s, w, h)
		s.promptName(title, "Name", "late night drive", "create", func(string) {})
		tt.Frame()
		tt.Advance(time.Second)
		r, _ := tt.Find("Name")
		return s, tt, r
	}
	_, _, whole := field(1280, 820)
	if whole.H < 30 {
		t.Fatalf("the name's field doesn't show in a tall window: %v", whole)
	}
	_, tt, _ := field(640, 360)
	body, ok := tt.Find("more of " + strings.ToLower(title))
	if !ok {
		t.Fatal("the body doesn't scroll in a short window")
	}
	tt.Scroll(body.X+body.W/2, body.Y+body.H/2, 0, 1e4)
	tt.Frame()
	if r, _ := tt.Find("Name"); r.H < whole.H-0.5 {
		t.Errorf("the name's field doesn't show whole at the body's end: %v, of %v", r, whole)
	}
}
