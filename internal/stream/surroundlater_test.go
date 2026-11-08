package stream

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// toldOf returns the 5.1 audio the TV client told of beside s, waiting for
// its answer when it came after the stream.
func toldOf(t *testing.T, s Stream) *Stream {
	t.Helper()
	if s.Surround != nil || s.later == nil {
		return s.Surround
	}
	select {
	case <-s.later.done:
		return s.later.surround
	case <-time.After(5 * time.Second):
		t.Fatal("the TV client's answer didn't come")
		return nil
	}
}

// knownSurround returns the resolver's stream of a video once it tells of
// the video's 5.1 audio, which the TV client's answer, coming beside the
// stream or after it, tells.
func knownSurround(t *testing.T, r *Resolver, videoID string) Stream {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		s, err := r.Resolve(context.Background(), videoID)
		if err != nil {
			t.Fatal(err)
		}
		if s.Surround != nil {
			return s
		}
		if time.Now().After(deadline) {
			t.Fatalf("the stream of %s tells of no 5.1 audio", videoID)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// holdTV has f's TV client answer with the recorded answer of a video of
// 5.1 audio once the returned function is called, or the test ends.
func holdTV(t *testing.T, f *fakeTube) (release func()) {
	held := make(chan struct{})
	var once sync.Once
	release = func() { once.Do(func() { close(held) }) }
	t.Cleanup(release)
	f.answersTV(func(playerRequest) []byte {
		<-held
		return f.recordedTV("-surround")
	})
	return release
}

// TestDirectDoesntWaitForTheTVClient finds the stereo audio of a video as
// soon as its stream is checked, while the TV client's answer of its 5.1
// audio hasn't come: the answer comes after, with the stream.
func TestDirectDoesntWaitForTheTVClient(t *testing.T) {
	f := newFakeTube(t)
	f.answersWith("-surround")
	release := holdTV(t, f)
	// Were the stream to wait for the answer, it would come in 5 s.
	guard := time.AfterFunc(5*time.Second, release)
	defer guard.Stop()
	start := time.Now()
	s, err := f.direct().Stream(context.Background(), surroundID, QualityBest)
	if !guard.Stop() || err != nil || s.Codec != "opus" || s.Surround != nil || s.later == nil {
		t.Fatalf("stream %+v, %v, after %v", s, err, time.Since(start))
	}
	if _, came := s.later.now(); came {
		t.Fatal("the TV client answered before it was let to")
	}
	release()
	if sur := toldOf(t, s); sur == nil || sur.Codec != "ac-3" || sur.URL != "" || sur.Size != surround380 {
		t.Errorf("told after the stream: %+v", sur)
	}
}

// TestDirectTellsSurroundThatCameInTime finds the 5.1 audio the TV client
// told of before the stream was checked with the stream itself.
func TestDirectTellsSurroundThatCameInTime(t *testing.T) {
	f := newFakeTube(t)
	f.answersWith("-surround")
	f.answersTVWith("-surround")
	// The check takes long enough for the TV client's answer to come.
	f.serves(func(w http.ResponseWriter, r *http.Request) {
		for deadline := time.Now().Add(5 * time.Second); len(f.tvPlayers()) == 0 && time.Now().Before(deadline); {
			time.Sleep(time.Millisecond)
		}
		time.Sleep(200 * time.Millisecond)
		serveClen(w, r)
	})
	s, err := f.direct().Stream(context.Background(), surroundID, QualityBest)
	if err != nil || s.Surround == nil || s.Surround.Codec != "ac-3" || s.later != nil {
		t.Errorf("surround %+v, later %v, %v", s.Surround, s.later, err)
	}
}

// TestResolverTellsSurroundToldLater keeps the 5.1 audio the TV client
// told of after the stream with the stream the resolver keeps, and tells
// so once: the 5.1 audio then plays, which yt-dlp finds.
func TestResolverTellsSurroundToldLater(t *testing.T) {
	f := newFakeTube(t)
	f.answersWith("-surround")
	release := holdTV(t, f)
	told := make(chan string, 4)
	r := NewResolver(nil, "")
	r.Direct = f.direct()
	r.Told = func(id string) { told <- id }
	var runs atomic.Int32
	r.Run = func(context.Context, ...string) ([]byte, error) {
		runs.Add(1)
		return surroundInfo(ytSurround("380", "ac-3", 384, "")), nil
	}
	ctx := context.Background()
	s, err := r.Resolve(ctx, surroundID)
	if err != nil || s.Codec != "opus" || s.Surround != nil || runs.Load() != 0 {
		t.Fatalf("stream %+v, surround %+v, %v, after %d runs of yt-dlp", s, s.Surround, err, runs.Load())
	}
	if _, err := r.Surround(ctx, surroundID); !errors.Is(err, errNoSurround) {
		t.Errorf("5.1 before the answer came: %v", err)
	}
	release()
	select {
	case id := <-told:
		if id != surroundID {
			t.Errorf("told of %s", id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("not told of the 5.1 audio")
	}
	s, err = r.Resolve(ctx, surroundID)
	if err != nil || s.Surround == nil || s.Surround.Codec != "ac-3" || s.Surround.URL != "" || len(f.players()) != 1 {
		t.Fatalf("after the answer: surround %+v, %v, after %d requests", s.Surround, err, len(f.players()))
	}
	if sur, err := r.Surround(ctx, surroundID); err != nil || sur.URL != "https://example.com/380" || runs.Load() != 1 {
		t.Errorf("5.1 found: %+v, %v, after %d runs of yt-dlp", sur, err, runs.Load())
	}
	select {
	case id := <-told:
		t.Errorf("told of %s again", id)
	default:
	}
}

// TestResolverWithoutTheTVClientsAnswer finds the stereo audio of a video,
// and no 5.1 audio, when the TV client's answer never comes, or comes as
// a refusal, or of none: nothing is told.
func TestResolverWithoutTheTVClientsAnswer(t *testing.T) {
	for name, answer := range map[string]func(f *fakeTube){
		"never": func(f *fakeTube) {
			held := make(chan struct{})
			t.Cleanup(func() { close(held) })
			f.answersTV(func(playerRequest) []byte {
				<-held
				return nil
			})
		},
		"refused": func(f *fakeTube) { f.answersTV(func(playerRequest) []byte { return nil }) },
		"of none": func(f *fakeTube) { f.answersTVWith("") },
	} {
		f := newFakeTube(t)
		f.answersWith("-surround")
		answer(f)
		r := NewResolver(nil, "")
		r.Direct = f.direct()
		r.Told = func(id string) { t.Errorf("%s: told of %s", name, id) }
		r.Run = func(context.Context, ...string) ([]byte, error) {
			t.Errorf("%s: yt-dlp ran", name)
			return nil, errNoStream
		}
		s, err := r.Resolve(context.Background(), surroundID)
		if err != nil || s.Codec != "opus" || !strings.Contains(s.URL, "c=VISIONOS") || s.Surround != nil {
			t.Errorf("%s: stream %+v, %v", name, s, err)
		}
		// The answer, if any, is in a moment after its request.
		for deadline := time.Now().Add(5 * time.Second); len(f.tvPlayers()) == 0 && time.Now().Before(deadline); {
			time.Sleep(time.Millisecond)
		}
		time.Sleep(50 * time.Millisecond)
		if _, err := r.Surround(context.Background(), surroundID); !errors.Is(err, errNoSurround) {
			t.Errorf("%s: 5.1: %v", name, err)
		}
	}
}
