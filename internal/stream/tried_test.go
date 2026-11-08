package stream

import (
	"context"
	"errors"
	"iter"
	"slices"
	"testing"
)

// TestResolveTellsBothSteps tells what YouTube's player API answered and
// what yt-dlp printed when neither found a stream, for the details of the
// track (#351): the error reads as yt-dlp's, of its kind, as before.
func TestResolveTellsBothSteps(t *testing.T) {
	f := newFakeTube(t)
	f.answersWith("-age")
	r := NewResolver(nil, "")
	r.Direct = f.direct()
	r.Run = func(context.Context, ...string) ([]byte, error) {
		return nil, ytdlpFailure("ERROR: [youtube] " + songID + ": Private video. Sign in if you've been granted access to this video\n")
	}
	_, err := r.Resolve(context.Background(), songID)
	if err == nil || err.Error() != "this track is private" || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("both failed: %v", err)
	}
	st, ok := err.(interface {
		Steps() iter.Seq2[string, string]
	})
	if !ok {
		t.Fatalf("%T tells no steps", err)
	}
	var steps [][2]string
	for what, why := range st.Steps() {
		steps = append(steps, [2]string{what, why})
	}
	want := [][2]string{
		{"YouTube's player API", "YouTube's player API answered LOGIN_REQUIRED: Sign in to confirm your age"},
		{"yt-dlp", "this track is private"},
	}
	if !slices.Equal(steps, want) {
		t.Errorf("steps %q, want %q", steps, want)
	}
	// Without the player API, the error of yt-dlp alone tells no steps.
	r.Direct = nil
	if _, err := r.Resolve(context.Background(), "XqZsoesa55w"); err == nil || errors.As(err, new(bothFailed)) {
		t.Errorf("yt-dlp alone: %#v", err)
	}
}
