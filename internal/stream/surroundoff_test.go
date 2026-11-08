package stream

import (
	"context"
	"testing"
	"time"
)

// surroundDirect returns a direct route to f with 5.1 surround on, which
// asks the TV client of a video's 5.1 audio.
func (f *fakeTube) surroundDirect() *Direct {
	d := f.direct()
	d.SetSurround(true)
	return d
}

// TestDirectAsksNoTVClientWithSurroundOff finds the stereo audio of a video
// of 5.1 audio with 5.1 surround off, as it is by default: the TV client is
// never asked, so the stream comes without waiting for its answer, of no
// 5.1 audio. Turned on, the route asks it again.
func TestDirectAsksNoTVClientWithSurroundOff(t *testing.T) {
	f := newFakeTube(t)
	f.answersWith("-surround")
	// An answer of the TV client that doesn't come would hold a stream
	// surroundGrace.
	released := make(chan struct{})
	t.Cleanup(func() { close(released) })
	f.answersTV(func(playerRequest) []byte {
		<-released
		return nil
	})
	d := f.direct()
	if d.Surround() {
		t.Fatal("a new route asks of 5.1 audio")
	}
	for i := range 3 {
		start := time.Now()
		s, err := d.Stream(context.Background(), surroundID, QualityBest)
		if took := time.Since(start); err != nil || s.Codec != "opus" || s.Surround != nil || took >= surroundGrace {
			t.Fatalf("stream %d: %+v, surround %+v, %v, after %v", i+1, s, s.Surround, err, took)
		}
	}
	if n := len(f.tvPlayers()); n != 0 {
		t.Errorf("the TV client was asked %d times with 5.1 off", n)
	}

	f.answersTVWith("-surround")
	d.SetSurround(true)
	if !d.Surround() {
		t.Fatal("5.1 on, the route doesn't ask of it")
	}
	s, err := d.Stream(context.Background(), surroundID, QualityBest)
	if err != nil || s.Surround == nil || s.Surround.Codec != "ac-3" || len(f.tvPlayers()) != 1 {
		t.Errorf("5.1 on: surround %+v, %v, after %d requests of the TV client", s.Surround, err, len(f.tvPlayers()))
	}
	d.SetSurround(false)
	if s, err := d.Stream(context.Background(), surroundID, QualityBest); err != nil || s.Surround != nil || len(f.tvPlayers()) != 1 {
		t.Errorf("5.1 off again: surround %+v, %v, after %d requests of the TV client", s.Surround, err, len(f.tvPlayers()))
	}
}
