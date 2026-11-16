package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/remote/remotetest"
	"github.com/fzn0x/mixmusic/internal/sponsorblock"
)

// TestRemoteMarks marks what the state of the computer controlled tells:
// its parts and its highlight, none of a live stream or a track of no
// length, and none of an older MixMusic, which tells no parts.
func TestRemoteMarks(t *testing.T) {
	st := remote.State{Track: &remote.Track{Title: "Low Tide"}, DurationMs: 211_000, Parts: []remote.Part{
		{Kind: sponsorblock.Sponsor, FromMs: 20_000, ToMs: 60_000}, {Kind: sponsorblock.Highlight, FromMs: 150_000}}}
	m := remoteMarks(st)
	if len(m.parts) != 1 || m.parts[0] != (player.Part{Kind: sponsorblock.Sponsor, From: 20 * time.Second, To: time.Minute}) ||
		m.highlight != 150*time.Second {
		t.Errorf("marks %+v", m)
	}
	if tip, ok := m.at(40.0/211, 211*time.Second, 0.01); !ok || tip != "Sponsor, 0:20 to 1:00" {
		t.Errorf("over the sponsor read: %q, %v", tip, ok)
	}
	for _, other := range []remote.State{{Track: st.Track, Live: true, Parts: st.Parts}, {Track: st.Track, Parts: st.Parts},
		{Track: st.Track, DurationMs: 211_000}} {
		if m := remoteMarks(other); m.any() {
			t.Errorf("%+v marks %+v", other, m)
		}
	}
}

// TestRemotePageMarksParts marks, on the remote page's seek bar, the parts
// the computer controlled passes over, in their kinds' colours, which the
// pointer tells, and jumps to its highlight there.
func TestRemotePageMarksParts(t *testing.T) {
	n := remotetest.New()
	id, err := remote.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	var desk *shell
	desk = newTestShellWith(t, func(o *app.Options) {
		o.Remote = &app.RemoteOptions{Discovery: n, Listen: remotetest.Listen, Identity: id, Name: "Desktop", Timing: remoteTiming,
			Player: func() *player.Player { return desk.p }}
	}, func(a *app.App) player.Services { return markedPlayer{testPlayer{a: a}} }, player.Saved{})
	lap := newRemoteShell(t, n, "Laptop")
	dt, lt := tester(t, desk), tester(t, lap)
	if err := desk.a.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	desk.p.PlayTracks(testTracks, 0)
	desk.settle(t, func(st player.State) bool { return st.Playing })
	desk.p.Time(100*time.Second, 211*time.Second)
	desk.settle(t, func(st player.State) bool { return len(st.Parts) == 1 && st.Highlight > 0 })

	pc := found(t, lap, "Desktop")
	show(lap, lt, remotePath(pc.ID))
	click(t, lap, lt, "pair")
	var code string
	untilBoth(t, desk, dt, lap, lt, "the code", func() bool {
		c, ok := desk.a.Remote().Code()
		code = c.Code
		return ok && lt.HasText("Type the code Desktop shows.")
	})
	lt.Type(code)
	lap.drain()
	lt.Frame()
	click(t, lap, lt, "pair")
	untilBoth(t, desk, dt, lap, lt, "the highlight there", func() bool {
		_, ok := lt.Access("Jump to the highlight")
		return ok && lt.HasText("Midnight City")
	})

	// The remote page's seek bar comes before the player bar's.
	r, ok := lt.Find("Seek")
	if !ok {
		t.Fatalf("no seek bar: %q", lt.Texts())
	}
	bar := seekPixels(t, lt)
	w := bar.Bounds().Dx()
	// Half the knob, 5 spaces of 4 DIPs wide, within the bar on each side.
	pad := 10
	at := func(t time.Duration) int { return pad + int(float64(w-2*pad)*t.Seconds()/211) }
	sponsor := lap.lk.partColor(sponsorblock.Sponsor)
	if n := inked(bar, sponsor, at(25*time.Second), at(55*time.Second)); n == 0 {
		t.Error("the sponsor read isn't marked on the remote page")
	}
	if n := inked(bar, sponsor, at(70*time.Second), at(140*time.Second)); n != 0 {
		t.Errorf("%d pixels of the sponsor read's colour past it", n)
	}
	if n := inked(bar, lap.lk.danger, at(148*time.Second), at(152*time.Second)); n == 0 {
		t.Error("the highlight isn't marked on the remote page")
	}
	lt.Move(r.X+float32(pad)+(r.W-float32(2*pad))*40/211, r.Y+r.H/2)
	lt.Frame()
	lt.Advance(time.Second)
	lt.Frame()
	if !lt.HasText("Sponsor, 0:20 to 1:00") {
		t.Errorf("hovering the sponsor read doesn't tell it: %q", lt.Texts())
	}

	click(t, lap, lt, "Jump to the highlight")
	desk.settle(t, func(st player.State) bool { return st.Position == 150*time.Second })
}

// TestRemotePageWithoutMarks shows the theme's seek bar, and no Jump to
// the highlight, of a computer that tells no parts, as an older MixMusic.
func TestRemotePageWithoutMarks(t *testing.T) {
	s := newTestShell(t)
	tt := ui.NewTester(func(c *ui.Context) {
		s.remotePlayer(c, "desk", app.RemoteView{HasState: true, Step: app.RemoteConnected,
			State: remote.State{Track: &remote.Track{Title: "Low Tide"}, Playing: true, PositionMs: 100_000, DurationMs: 211_000}})
	}, 900, 600)
	tt.Frame()
	if _, ok := tt.Find("Seek"); !ok {
		t.Fatalf("no seek bar: %q", tt.Texts())
	}
	if _, ok := tt.Access("Jump to the highlight"); ok {
		t.Error("a computer that tells no highlight offers to jump to it")
	}
}
