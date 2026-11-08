package audio

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
)

// localSource is a Source of bytes all there, which tells whether they are
// on this computer as local says.
type localSource struct {
	Source
	local bool
}

func (s localSource) Local() bool { return s.local }

// TestDeckFileFailures fails a track on this computer that can't be read
// as the file, not as the network: one cut short, or whose read fails, as
// on a drive unplugged. A file it can't play is still told as such.
func TestDeckFileFailures(t *testing.T) {
	b, err := os.ReadFile(cuesFront)
	if err != nil {
		t.Fatal(err)
	}
	unplugged := errors.New("read D:\\Music\\a.webm: The device is not ready.")
	for _, c := range []struct {
		name  string
		src   func() Source
		local bool
		want  error
	}{
		{"cut short", func() Source { return memSource{bytes.NewReader(b[:len(b)/2])} }, true, errFile},
		{"a read failing", func() Source { return &brokenSource{Reader: bytes.NewReader(b), n: int64(len(b) / 2), why: unplugged} }, true, errFile},
		{"not WebM", func() Source { return memSource{bytes.NewReader([]byte("hello, this is not a WebM file at all"))} }, true, errUnsupported},
		{"cut short, elsewhere", func() Source { return memSource{bytes.NewReader(b[:len(b)/2])} }, false, errNetwork},
	} {
		t.Run(c.name, func(t *testing.T) {
			open := func(ctx context.Context, url string, interrupted func() bool) (Source, error) {
				return localSource{c.src(), c.local}, nil
			}
			d := newDeck("x", 0, open, func() {})
			defer d.close()
			drain(t, d)
			if err := d.failure(); !errors.Is(err, c.want) {
				t.Errorf("failure %v, want %v", err, c.want)
			}
		})
	}
}
