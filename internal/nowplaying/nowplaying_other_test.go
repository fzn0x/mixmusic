//go:build !((darwin || linux || windows) && (amd64 || arm64))

package nowplaying

import (
	"errors"
	"testing"
)

func TestUnsupported(t *testing.T) {
	if _, err := Open(func(Request) {}, nil); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Open: %v, want ErrUnsupported", err)
	}
}
