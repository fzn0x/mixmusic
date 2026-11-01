package native

import (
	"context"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// offlinePlayer is testPlayer without the network: no track can be found,
// and the network never answers.
type offlinePlayer struct{ testPlayer }

func (offlinePlayer) Prepare(context.Context, media.Track) (player.Source, error) {
	return player.Source{}, player.ErrOffline
}

func (p offlinePlayer) PrepareStream(ctx context.Context, t media.Track) (player.Source, error) {
	return p.Prepare(ctx, t)
}

func (offlinePlayer) Online(context.Context) bool { return false }

func (offlinePlayer) WaitOnline(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (offlinePlayer) KickOnline() {}

// offlineShell is a shell whose player has no network.
func offlineShell(t *testing.T) *shell {
	s := newTestShell(t)
	stream := &testStream{}
	p := player.New(offlinePlayer{}, stream, testDevice{}, player.Saved{})
	stream.mu.Lock()
	stream.p = p
	stream.mu.Unlock()
	t.Cleanup(p.Close)
	s.p = p
	return s
}

// TestWaitingForTheNetwork shows a track that waits for the network, in
// the player bar, the mini player and the now playing page, with a play
// button that chooses whether it plays once the network is back.
func TestWaitingForTheNetwork(t *testing.T) {
	s := offlineShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Waiting && st.WillPlay })
	tt := tester(t, s)
	waiting := names(testTracks[0].Artists) + " · waiting for the network"
	if !tt.HasText(waiting) {
		t.Errorf("the player bar doesn't tell the wait: %q", tt.Texts())
	}
	click(t, s, tt, "Pause, waiting for the network")
	s.settle(t, func(st player.State) bool { return st.Waiting && !st.WillPlay })
	tt.Frame()
	click(t, s, tt, "Play once the network is back")
	s.settle(t, func(st player.State) bool { return st.Waiting && st.WillPlay })

	mini := ui.NewTester(s.miniView, 420, 132)
	mini.Frame()
	if !mini.HasText(waiting) {
		t.Errorf("the mini player doesn't tell the wait: %q", mini.Texts())
	}
	if err := mini.Click("Pause, waiting for the network"); err != nil {
		t.Errorf("the mini player's toggle: %v", err)
	}
	s.settle(t, func(st player.State) bool { return st.Waiting && !st.WillPlay })

	show(s, tt, "/now-playing")
	if !tt.HasText("waiting for the network") {
		t.Errorf("the now playing page doesn't tell the wait: %q", tt.Texts())
	}
	if dir := os.Getenv("MIXMUSIC_RENDER"); dir != "" {
		s.p.Play()
		s.settle(t, func(st player.State) bool { return st.WillPlay })
		renderTo(t, s, tt, dir, "now-playing-waiting")
		show(s, tt, "/")
		renderTo(t, s, tt, dir, "home-waiting")
	}
}
