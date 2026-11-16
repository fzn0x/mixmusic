package stream

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

// withCaptions has f's player endpoint answer with the captions of
// testdata, served by a timedtext endpoint of their own that answers
// testdata's timedtext-json3.json, and returns what it was asked.
func withCaptions(t *testing.T, f *fakeTube) func() []string {
	t.Helper()
	json3, err := os.ReadFile("testdata/timedtext-json3.json")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var asked []string
	tt := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked = append(asked, r.URL.RawQuery)
		mu.Unlock()
		if r.URL.Path != "/api/timedtext" {
			http.NotFound(w, r)
			return
		}
		w.Write(json3)
	}))
	t.Cleanup(tt.Close)
	f.answers(func(playerRequest) []byte {
		return bytes.ReplaceAll(f.recorded("-captions"), []byte("{{timedtext}}"), []byte(tt.URL))
	})
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), asked...)
	}
}

// TestDirectCaptions lists a video's captions, as the visionOS client
// gets them, signed out: the language of each, whether YouTube made it
// from speech, and the one it shows of the video; and the video's length.
func TestDirectCaptions(t *testing.T) {
	f := newFakeTube(t)
	withCaptions(t, f)
	caps, err := f.direct().Captions(context.Background(), songID)
	if err != nil {
		t.Fatal(err)
	}
	if len(caps.Tracks) != 3 || caps.Length != 200*time.Second {
		t.Fatalf("captions %+v", caps)
	}
	en, asr, de := caps.Tracks[0], caps.Tracks[1], caps.Tracks[2]
	if en.Language != "en" || en.Name != "English" || en.Speech || !en.Default {
		t.Errorf("first %+v", en)
	}
	if asr.Language != "en" || !asr.Speech || asr.Default || de.Language != "de-DE" || de.Speech || de.Default {
		t.Errorf("others %+v, %+v", asr, de)
	}
	if got := dig(f.requests[0].Body, "context", "client", "clientName"); got != visionName {
		t.Errorf("asked as %v", got)
	}
}

// TestDirectCaptionText reads the captions of a track in json3, a line
// each, in their times, leaving out those of no text.
func TestDirectCaptionText(t *testing.T) {
	f := newFakeTube(t)
	asked := withCaptions(t, f)
	d := f.direct()
	caps, err := d.Captions(context.Background(), songID)
	if err != nil {
		t.Fatal(err)
	}
	lines, err := d.CaptionText(context.Background(), caps.Tracks[0])
	if err != nil {
		t.Fatal(err)
	}
	want := []Caption{
		{1360, 3040, "[♪♪♪]"},
		{18640, 21880, "♪ Paper kites over the bay ♪"},
		{22640, 24760, "♪ I'll wait for you in the night ♪"},
		{27200, 31200, "(crowd cheering)"},
	}
	if len(lines) != len(want) {
		t.Fatalf("lines %+v", lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d: %+v, not %+v", i, lines[i], want[i])
		}
	}
	if q := asked(); len(q) != 1 || !bytes.Contains([]byte(q[0]), []byte("fmt=json3")) || !bytes.Contains([]byte(q[0]), []byte("lang=en")) {
		t.Errorf("timedtext asked %q", q)
	}
}

// TestDirectNoCaptions tells a video without captions, and one YouTube
// gives the route nothing of, from a failure to ask.
func TestDirectNoCaptions(t *testing.T) {
	f := newFakeTube(t)
	if _, err := f.direct().Captions(context.Background(), songID); !errors.Is(err, ErrNoCaptions) {
		t.Errorf("a video without captions: %v", err)
	}
	f.answersWith("-unplayable")
	if _, err := f.direct().Captions(context.Background(), songID); !errors.Is(err, ErrNoCaptions) {
		t.Errorf("an unplayable video: %v", err)
	}
	f.answers(func(playerRequest) []byte { return nil })
	if _, err := f.direct().Captions(context.Background(), songID); err == nil || errors.Is(err, ErrNoCaptions) {
		t.Errorf("the API failing: %v", err)
	}
}
