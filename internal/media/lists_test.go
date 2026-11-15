package media

import (
	"errors"
	"fmt"
	"testing"
)

// TestShortList tells how much of a list came and why the rest didn't,
// and is found in an error that wraps it.
func TestShortList(t *testing.T) {
	down := errors.New("HTTP 503")
	err := fmt.Errorf("Spotify: %w", &ShortList{Got: 50, Total: 120, Err: down})
	short, ok := Short(err)
	if !ok || short.Got != 50 || short.Total != 120 || !errors.Is(err, down) {
		t.Fatalf("Short(%v) = %+v, %v", err, short, ok)
	}
	if got := err.Error(); got != "Spotify: only 50 of 120 came: HTTP 503" {
		t.Errorf("error %q", got)
	}
	// A list whose count the service didn't tell says what came.
	if got := (&ShortList{Got: 50, Err: down}).Error(); got != "only 50 came: HTTP 503" {
		t.Errorf("error %q", got)
	}
	if _, ok := Short(down); ok {
		t.Error("another error is a short list")
	}
	if _, ok := Short(nil); ok {
		t.Error("no error is a short list")
	}
}
