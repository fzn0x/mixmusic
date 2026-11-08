package stream

import (
	"context"
	"sync/atomic"
	"testing"
)

// TestResolverKnown tells the stream the resolver found of a video, which
// it holds, without finding one: none before it was found, nor once it
// was forgotten.
func TestResolverKnown(t *testing.T) {
	var runs atomic.Int32
	r := NewResolver(nil, "")
	r.Run = fakeYTDLP("https://example.com/audio", &runs)
	if s, ok := r.Known("w3XzgjlKFjc"); ok {
		t.Errorf("known before it was found: %+v", s)
	}
	if _, err := r.Resolve(context.Background(), "w3XzgjlKFjc"); err != nil {
		t.Fatal(err)
	}
	s, ok := r.Known("w3XzgjlKFjc")
	if !ok || s.Codec != "opus" || s.Bitrate != 139.3 || s.Size != 1000 {
		t.Errorf("known %+v, %v", s, ok)
	}
	if s, ok := r.KnownSurround("w3XzgjlKFjc"); ok {
		t.Errorf("5.1 audio of a video of none: %+v", s)
	}
	r.Forget("w3XzgjlKFjc")
	if _, ok := r.Known("w3XzgjlKFjc"); ok || runs.Load() != 1 {
		t.Errorf("known once forgotten: %v, after %d runs of yt-dlp", ok, runs.Load())
	}
}

// TestResolverKnownSurround tells the 5.1 surround audio yt-dlp found
// beside the stereo audio of a video.
func TestResolverKnownSurround(t *testing.T) {
	r := NewResolver(nil, "")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		return surroundInfo(ytSurround("380", "ac-3", 384, "")), nil
	}
	if _, err := r.Resolve(context.Background(), surroundID); err != nil {
		t.Fatal(err)
	}
	if s, ok := r.KnownSurround(surroundID); !ok || s.Codec != "ac-3" || s.Bitrate != 384 || s.URL != "https://example.com/380" {
		t.Errorf("5.1 audio %+v, %v", s, ok)
	}
}
