package player

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// gatedRadio are the fake services, whose radios tell what they start
// from, then wait to be let through.
type gatedRadio struct {
	*fakeServices
	asked chan string
	gate  chan struct{}
}

func (g gatedRadio) Radio(ctx context.Context, t media.Track) ([]media.Track, error) {
	g.asked <- t.ID
	<-g.gate
	return g.fakeServices.Radio(ctx, t)
}

// TestPlayWithRadio plays a song at once, as a suggested one, and puts its
// radio after it as soon as it comes, before the song ends; what the user
// adds goes before the radio. A queue replaced meanwhile gets none of it,
// and without autoplay the song plays alone.
func TestPlayWithRadio(t *testing.T) {
	g := gatedRadio{asked: make(chan string, 4), gate: make(chan struct{})}
	h := newHarnessWith(t, Saved{}, func(f *fakeServices) Services { g.fakeServices = f; return g }, func(f *fakeServices) {
		f.settings.Autoplay = true
		f.radio = []media.Track{track("a", "Artist a"), track("x", "X"), track("y", "Y")}
	})
	h.p.PlayWithRadio(track("a", "Artist a"))
	h.wait("a playing", playing("a"))
	if seed := <-g.asked; seed != "yt:a" {
		t.Fatalf("the radio starts from %s", seed)
	}
	if got := queueIDs(h.p.State()); !slices.Equal(got, []string{"a"}) {
		t.Fatalf("before its radio comes, the queue is %v", got)
	}
	g.gate <- struct{}{}
	h.wait("the radio queued", func(p *Player) bool { return len(p.s.Queue) == 3 })
	s := h.p.State()
	if got := queueIDs(s); !slices.Equal(got, []string{"a", "x*", "y*"}) {
		t.Errorf("the queue is %v", got)
	}
	if cur, ok := s.Current(); !ok || cur.Track.ID != "yt:a" {
		t.Errorf("the song stopped playing for its radio: %+v", cur)
	}
	h.p.Enqueue(tracks("e"))
	h.wait("e queued", func(p *Player) bool { return len(p.s.Queue) == 4 })
	if got := queueIDs(h.p.State()); !slices.Equal(got, []string{"a", "e", "x*", "y*"}) {
		t.Errorf("what the user adds goes at %v", got)
	}

	// The queue replaced before the radio of b comes.
	h.p.PlayWithRadio(track("b", "Artist b"))
	if seed := <-g.asked; seed != "yt:b" {
		t.Fatalf("the radio starts from %s", seed)
	}
	h.p.PlayTracks(tracks("c"), 0)
	h.wait("c playing", playing("c"))
	g.gate <- struct{}{}
	// The radio's answer, had it gone in, would be in the queue by then.
	time.Sleep(50 * time.Millisecond)
	if got := queueIDs(h.p.State()); !slices.Equal(got, []string{"c"}) {
		t.Errorf("the radio of b went into the queue played after it: %v", got)
	}

	h.svc.mu.Lock()
	h.svc.settings.Autoplay = false
	h.svc.mu.Unlock()
	h.p.PlayWithRadio(track("d", "Artist d"))
	h.wait("d playing", playing("d"))
	select {
	case seed := <-g.asked:
		t.Errorf("without autoplay, a radio of %s was asked for", seed)
	default:
	}
	if got := queueIDs(h.p.State()); !slices.Equal(got, []string{"d"}) {
		t.Errorf("without autoplay, the queue is %v", got)
	}
}
