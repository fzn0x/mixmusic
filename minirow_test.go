package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
)

// TestMiniShuffleRepeat shuffles and repeats from the mini player, whose
// buttons show what they do.
func TestMiniShuffleRepeat(t *testing.T) {
	s := playing(t)
	tt := ui.NewTester(s.miniView, 420, 132)
	tt.Frame()
	if a, ok := tt.AccessByRole(ui.RoleToggleButton, "Shuffle"); !ok || a.Checked {
		t.Fatalf("shuffle's button: %+v (%v)", a, ok)
	}
	click(t, s, tt, "Shuffle")
	s.settle(t, func(st player.State) bool { return st.Shuffle })
	tt.Frame()
	if a, _ := tt.Access("Shuffle"); !a.Checked {
		t.Error("shuffle's button doesn't tell the queue is shuffled")
	}
	for _, c := range []struct {
		label string
		want  player.Repeat
	}{{"Repeat: off", player.RepeatAll}, {"Repeat: all", player.RepeatOne}, {"Repeat: one", player.RepeatOff}} {
		click(t, s, tt, c.label)
		s.settle(t, func(st player.State) bool { return st.Repeat == c.want })
		tt.Frame()
	}
}

// TestMiniRowOf fits the transport row in the room it has: every button
// at every size the window takes, its gaps growing with the room, and,
// narrower, repeat, then shuffle, the dislike and the like go into the
// menu.
func TestMiniRowOf(t *testing.T) {
	for _, c := range []struct {
		w      float32
		rated  bool
		hidden miniOptional
	}{
		{264, true, 0},
		{276, true, 0},
		{400, true, 0},
		{264, false, miniLike | miniDislike},
		{250, true, miniRepeat},
		{230, true, miniRepeat | miniShuffle},
		{180, true, miniRepeat | miniShuffle | miniDislike},
		{100, true, miniRepeat | miniShuffle | miniDislike | miniLike},
	} {
		r := miniRowOf(c.w, c.rated)
		need, gaps := float32(miniAlways), float32(2)
		for _, b := range []miniOptional{miniShuffle, miniRepeat} {
			if r.shows(b) {
				need, gaps = need+miniToggles, gaps+1
			}
		}
		for _, b := range []miniOptional{miniLike, miniDislike} {
			if r.shows(b) {
				need += miniRating
			}
		}
		used := need + gaps*r.gap + r.groupGap
		if r.hidden != c.hidden || r.gap < 2 || r.gap > 6 || r.groupGap < 4 || r.groupGap > 12 || used > max(c.w, need+gaps*2+4)+0.01 {
			t.Errorf("in %v, rated %v: %+v takes %v; want hidden %b", c.w, c.rated, r, used, c.hidden)
		}
	}
	if r := miniRowOf(264, true); r.gap != 2 || r.groupGap != 4 {
		t.Errorf("the row is as narrow as the window takes, with gaps of %v and %v", r.gap, r.groupGap)
	}
	if r := miniRowOf(400, true); r.gap != 6 || r.groupGap != 12 {
		t.Errorf("a wide row has gaps of %v and %v", r.gap, r.groupGap)
	}
}

// TestMiniMenuHolds puts the buttons a row too narrow has no room for in
// the mini player's menu, above the speed, which works them.
func TestMiniMenuHolds(t *testing.T) {
	s := playing(t)
	// Narrower than the window takes, which leaves the artwork no room.
	tt := ui.NewTester(s.miniView, 250, 112)
	tt.Frame()
	if _, ok := tt.Find("Repeat: off"); ok {
		t.Fatalf("the row has repeat in 250 DIPs: %q", tt.Texts())
	}
	if err := tt.RightClick(testTracks[0].Title); err != nil {
		t.Fatal(err)
	}
	if menu := tt.Menu(); len(menu) != 3 || menu[0] != "Repeat: off" || menu[2] != "Speed of songs" {
		t.Fatalf("the mini player's menu holds %q", menu)
	}
	if err := tt.ChooseMenuItem("Repeat: off"); err != nil {
		t.Fatal(err)
	}
	s.settle(t, func(st player.State) bool { return st.Repeat == player.RepeatAll })
}
