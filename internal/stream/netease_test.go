package stream

import "testing"

func TestNetEaseKey(t *testing.T) {
	key := NetEaseKey("1901371647")
	if !ValidVideoID(key) {
		t.Fatalf("%q isn't a valid key", key)
	}
	if song, ok := NetEaseSong(key); !ok || song != "1901371647" {
		t.Errorf("NetEaseSong(%q) = %q, %v", key, song, ok)
	}
	if _, ok := NetEaseSong("dQw4w9WgXcQ"); ok {
		t.Error("a video's ID read as a NetEase song")
	}
}
