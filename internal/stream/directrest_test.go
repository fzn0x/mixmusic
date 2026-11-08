package stream

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// clock is a time that a test moves on, which the route's goroutines may
// read meanwhile.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// clocked returns a direct route to f whose time is c's.
func clocked(f *fakeTube, c *clock) *Direct {
	d := f.direct()
	d.now = c.now
	return d
}

// restLog is what the log says when the route rests for minutes after
// refusals, the last with a new visitor ID too.
func restLog(refused, minutes int) string {
	return fmt.Sprintf("YouTube's player API refused %d streams in a row, the last with a new visitor ID too; yt-dlp alone finds them for %d minutes",
		refused, minutes)
}

// TestDirectRestsAfterRefusals rests the direct route restFirst once
// YouTube's player API refused pauseAfter streams in a row, the last of
// them asked again with a new visitor ID: of any refusal, and never of
// what says a video has no stream for the route, or that the network
// failed.
func TestDirectRestsAfterRefusals(t *testing.T) {
	logs := captureLog(t)
	f := newFakeTube(t)
	clk := &clock{t: time.Now()}
	d := clocked(f, clk)
	stream := func() error {
		_, err := d.Stream(context.Background(), songID, QualityBest)
		return err
	}

	for _, suffix := range []string{"-age", "-unplayable"} {
		f.answersWith(suffix)
		for range pauseAfter + 1 {
			stream()
		}
	}
	f.answers(func(playerRequest) []byte {
		return []byte(strings.ReplaceAll(string(withoutOpus(f.recorded(""))), `codecs=\"mp4a.40.2\"`, `codecs=\"mp4a.40.5\"`))
	})
	for range pauseAfter + 1 {
		if err := stream(); !errors.Is(err, errNoStream) {
			t.Fatalf("no Opus and no AAC-LC: %v", err)
		}
	}
	down := &Direct{HTTP: &http.Client{Transport: offline{}}, BaseURL: f.srv.URL, now: d.now}
	for range pauseAfter + 1 {
		if _, err := down.Stream(context.Background(), songID, QualityBest); errors.Is(err, errDirectPaused) {
			t.Fatal("rested offline")
		}
	}

	// A refusal of the media server, URLs that need yt-dlp, a bot check a
	// new visitor ID doesn't pass, and an HTTP error each count.
	for name, refuse := range map[string]func(){
		"media server": func() { f.refuses(http.StatusForbidden) },
		"signatures": func() {
			f.answers(func(playerRequest) []byte {
				return []byte(strings.ReplaceAll(string(f.recorded("")), "&expire=", "&n=AbCdEfGhIjKl&expire="))
			})
		},
		"bot check": func() { f.answersWith("-bot-check") },
		"HTTP 500":  func() { f.answers(func(playerRequest) []byte { return nil }) },
	} {
		f.answersWith("")
		f.serves(serveClen)
		if err := stream(); err != nil {
			t.Fatalf("%s: before: %v", name, err)
		}
		refuse()
		perTry := 0
		for i := range pauseAfter - 1 {
			before := len(f.players())
			if err := stream(); err == nil || errors.Is(err, errDirectPaused) {
				t.Fatalf("%s: refusal %d: %v", name, i+1, err)
			}
			perTry = len(f.players()) - before
		}
		// The last is asked again, with a new visitor ID.
		visitors, before := f.visitorCount(), len(f.players())
		if err := stream(); err == nil || errors.Is(err, errDirectPaused) {
			t.Fatalf("%s: refusal %d: %v", name, pauseAfter, err)
		}
		if n := len(f.players()) - before; n != 2*perTry || f.visitorCount() == visitors {
			t.Errorf("%s: %d player requests for the last refusal, of %d each, and %d visitor IDs", name, n, perTry, f.visitorCount()-visitors)
		}
		asked := len(f.players())
		if err := stream(); !errors.Is(err, errDirectPaused) || len(f.players()) != asked {
			t.Fatalf("%s: not resting after %d refusals: %v", name, pauseAfter, err)
		}
		clk.add(restFirst - time.Second)
		if err := stream(); !errors.Is(err, errDirectPaused) {
			t.Fatalf("%s: rested less than %v: %v", name, restFirst, err)
		}
		clk.add(time.Second)
	}

	// A stream found sets the count back.
	f.answersWith("")
	f.serves(serveClen)
	if err := stream(); err != nil {
		t.Fatal(err)
	}
	f.refuses(http.StatusForbidden)
	stream()
	stream()
	f.serves(serveClen)
	if err := stream(); err != nil {
		t.Fatal(err)
	}
	f.refuses(http.StatusForbidden)
	stream()
	if err := stream(); !errors.Is(err, errRefused) {
		t.Errorf("after refusals that a stream found came between: %v", err)
	}

	if n := strings.Count(logs.String(), restLog(pauseAfter, 2)); n != 4 || strings.Contains(logs.String(), "videoplayback") ||
		strings.Contains(logs.String(), testVisitor) {
		t.Errorf("%d rests logged, or a stream URL or the visitor ID:\n%s", n, logs.String())
	}
}

// TestDirectRestsLongerWhileRefusalsGoOn rests the direct route twice as
// long each time it is refused again after a rest, a new visitor ID too,
// up to restMax, and restFirst again once it found a stream.
func TestDirectRestsLongerWhileRefusalsGoOn(t *testing.T) {
	logs := captureLog(t)
	f := newFakeTube(t)
	clk := &clock{t: time.Now()}
	d := clocked(f, clk)
	stream := func() error {
		_, err := d.Stream(context.Background(), songID, QualityBest)
		return err
	}
	f.refuses(http.StatusForbidden)
	for range pauseAfter {
		stream()
	}
	refused, rest := pauseAfter, restFirst
	for _, want := range []time.Duration{4 * time.Minute, 8 * time.Minute, 16 * time.Minute, restMax, restMax} {
		// The rest before ends: the next stream asks again, once more with
		// a new visitor ID, refused both times.
		clk.add(rest)
		visitors := f.visitorCount()
		if err := stream(); !errors.Is(err, errRefused) || f.visitorCount() != visitors+1 {
			t.Fatalf("after a rest of %v: %v, after %d visitor IDs", rest, err, f.visitorCount()-visitors)
		}
		refused++
		if !strings.Contains(logs.String(), restLog(refused, int(want/time.Minute))) {
			t.Fatalf("no rest of %v logged:\n%s", want, logs.String())
		}
		clk.add(want - time.Second)
		if err := stream(); !errors.Is(err, errDirectPaused) {
			t.Fatalf("rested less than %v: %v", want, err)
		}
		clk.add(-(want - time.Second))
		rest = want
	}

	// A stream found sets the rest back to restFirst, after pauseAfter
	// refusals again.
	clk.add(rest)
	f.serves(serveClen)
	if err := stream(); err != nil {
		t.Fatal(err)
	}
	f.refuses(http.StatusForbidden)
	for range pauseAfter - 1 {
		if err := stream(); !errors.Is(err, errRefused) {
			t.Fatalf("after a stream found: %v", err)
		}
	}
	stream()
	if n := strings.Count(logs.String(), restLog(pauseAfter, 2)); n != 2 {
		t.Errorf("%d rests of 2 minutes after %d refusals:\n%s", n, pauseAfter, logs.String())
	}
}

// TestDirectRetriesWithANewVisitorID finds the stream, asked again with a
// new visitor ID, of the refusal that would rest the direct route: YouTube
// refused the streams of the one it gave first. The count starts again.
func TestDirectRetriesWithANewVisitorID(t *testing.T) {
	logs := captureLog(t)
	f := newFakeTube(t)
	clk := &clock{t: time.Now()}
	d := clocked(f, clk)
	f.answers(func(playerRequest) []byte {
		if f.visitorCount() < 2 {
			return nil
		}
		return f.recorded("")
	})
	for i := range pauseAfter - 1 {
		if _, err := d.Stream(context.Background(), songID, QualityBest); err == nil {
			t.Fatalf("stream %d found", i+1)
		}
	}
	s, err := d.Stream(context.Background(), songID, QualityBest)
	if err != nil || s.Size != song251 || f.visitorCount() != 2 {
		t.Fatalf("asked again with a new visitor ID: %+v, %v, after %d visitor IDs", s, err, f.visitorCount())
	}
	reqs := f.players()
	if last := reqs[len(reqs)-1]; last.Visitor != testVisitor {
		t.Errorf("asked again with visitor %q", last.Visitor)
	}
	if strings.Contains(logs.String(), "yt-dlp alone finds") {
		t.Errorf("rested:\n%s", logs.String())
	}
	f.answers(func(playerRequest) []byte { return nil })
	for range pauseAfter - 1 {
		if _, err := d.Stream(context.Background(), songID, QualityBest); err == nil || errors.Is(err, errDirectPaused) {
			t.Fatalf("after the stream found: %v", err)
		}
	}
	if f.visitorCount() != 2 {
		t.Errorf("%d visitor IDs asked for before %d refusals", f.visitorCount(), pauseAfter)
	}
}

// deadlines is a transport to f that keeps how long each request of the
// player endpoint as the visionOS client had left to go.
type deadlines struct {
	base http.RoundTripper
	mu   sync.Mutex
	left []time.Duration
}

func (d *deadlines) RoundTrip(r *http.Request) (*http.Response, error) {
	if strings.HasSuffix(r.URL.Path, "/player") && r.Header.Get("X-YouTube-Client-Name") == visionNumber {
		if at, ok := r.Context().Deadline(); ok {
			d.mu.Lock()
			d.left = append(d.left, time.Until(at))
			d.mu.Unlock()
		}
	}
	return d.base.RoundTrip(r)
}

func (d *deadlines) all() []time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]time.Duration(nil), d.left...)
}

// hangs has f's player endpoint not answer while hang is set, until the
// test ends.
func hangs(t *testing.T, f *fakeTube, hang *atomic.Bool) {
	released := make(chan struct{})
	t.Cleanup(func() { close(released) })
	f.answers(func(playerRequest) []byte {
		if hang.Load() {
			<-released
			return nil
		}
		return f.recorded("")
	})
}

// TestDirectWaitsLessOnceSlow bounds the try of the direct route after one
// that didn't answer in its time by slowTimeout, and the one after a
// stream found by its own time again.
func TestDirectWaitsLessOnceSlow(t *testing.T) {
	f := newFakeTube(t)
	var hang atomic.Bool
	hang.Store(true)
	hangs(t, f, &hang)
	d := f.direct()
	dl := &deadlines{base: f.srv.Client().Transport}
	d.HTTP = &http.Client{Transport: dl}
	try := func(within time.Duration) error {
		ctx, cancel := context.WithTimeout(context.Background(), within)
		defer cancel()
		_, err := d.Stream(ctx, songID, QualityBest)
		return err
	}
	if err := try(50 * time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("no answer: %v", err)
	}
	hang.Store(false)
	for i := range 2 {
		if err := try(directTimeout); err != nil {
			t.Fatalf("try %d after: %v", i+1, err)
		}
	}
	left := dl.all()
	if len(left) != 3 || left[0] > 50*time.Millisecond || left[1] > slowTimeout || left[1] < slowTimeout/2 ||
		left[2] <= slowTimeout {
		t.Errorf("the player endpoint asked with %v left", left)
	}
}

// TestDirectRestsWhenItDoesntAnswer rests the direct route once pauseAfter
// tries in a row didn't answer in their time, with no new visitor ID asked
// for, which wouldn't make an answer come; a stream no longer asked for
// isn't counted.
func TestDirectRestsWhenItDoesntAnswer(t *testing.T) {
	logs := captureLog(t)
	f := newFakeTube(t)
	var hang atomic.Bool
	hang.Store(true)
	hangs(t, f, &hang)
	clk := &clock{t: time.Now()}
	d := clocked(f, clk)
	try := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_, err := d.Stream(ctx, songID, QualityBest)
		return err
	}
	// Given up on, the streams don't count.
	for range pauseAfter + 1 {
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(20*time.Millisecond, cancel)
		if _, err := d.Stream(ctx, songID, QualityBest); !errors.Is(err, context.Canceled) {
			t.Fatalf("given up on: %v", err)
		}
	}
	for i := range pauseAfter {
		if err := try(); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("try %d: %v", i+1, err)
		}
	}
	if err := try(); !errors.Is(err, errDirectPaused) || f.visitorCount() != 1 {
		t.Errorf("after %d tries without an answer: %v, after %d visitor IDs", pauseAfter, err, f.visitorCount())
	}
	want := fmt.Sprintf("refused or didn't answer %d streams in a row, the last not in time; yt-dlp alone finds them for 2 minutes", pauseAfter)
	if !strings.Contains(logs.String(), want) {
		t.Errorf("no rest logged:\n%s", logs.String())
	}
	// After the rest, a try has its whole time again.
	clk.add(restFirst)
	hang.Store(false)
	if _, err := d.Stream(context.Background(), songID, QualityBest); err != nil {
		t.Errorf("after the rest: %v", err)
	}
}
