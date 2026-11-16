package remote

import (
	"encoding/binary"
	"errors"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestFramesHoldNoRoomForALengthAlone reads a length of MaxMessage with a
// few bytes after it, then nothing: Frames makes room for what came, not
// for the megabyte the length tells (#736).
func TestFramesHoldNoRoomForALengthAlone(t *testing.T) {
	least := ^uint64(0)
	// Other goroutines of the test binary allocate too, now and then: the
	// least of a few tries is Read's.
	for range 3 {
		a, b := net.Pipe()
		go func() {
			var head [4]byte
			binary.BigEndian.PutUint32(head[:], MaxMessage)
			a.Write(append(head[:], `{"type":`...))
		}()
		f := NewFrames(b)
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		var v map[string]any
		err := f.Receive(&v, 100*time.Millisecond)
		runtime.ReadMemStats(&after)
		a.Close()
		b.Close()
		if !errors.Is(err, ErrLost) {
			t.Fatalf("a message that stopped coming: %v, not ErrLost", err)
		}
		least = min(least, after.TotalAlloc-before.TotalAlloc)
	}
	if least > 64<<10 {
		t.Errorf("a length of %d bytes alone took %d bytes", MaxMessage, least)
	}
}

// TestFramesReadALongMessageAsItComes reads a message of almost MaxMessage
// sent in pieces of odd sizes, and a short one after it.
func TestFramesReadALongMessageAsItComes(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	long := strings.Repeat("0123456789abcdef", (MaxMessage-64)/16)
	go func() {
		var buf strings.Builder
		var head [4]byte
		for _, m := range []string{`{"type":"state","name":"` + long + `"}`, `{"type":"ping"}`} {
			binary.BigEndian.PutUint32(head[:], uint32(len(m)))
			buf.Write(head[:])
			buf.WriteString(m)
		}
		data := []byte(buf.String())
		for size := 1; len(data) > 0; size = size*3 + 7 {
			n := min(size, len(data))
			if _, err := a.Write(data[:n]); err != nil {
				return
			}
			data = data[n:]
		}
	}()
	f := NewFrames(b)
	var m message
	if err := f.Receive(&m, 5*time.Second); err != nil || m.Type != "state" || m.Name != long {
		t.Fatalf("the long message: %q of %d bytes, %v", m.Type, len(m.Name), err)
	}
	if err := f.Receive(&m, 5*time.Second); err != nil || m.Type != "ping" {
		t.Fatalf("the message after: %q, %v", m.Type, err)
	}
}

// TestFramesMessageCutShort ends a connection in the middle of a message:
// the read fails, as one that ended early.
func TestFramesMessageCutShort(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	go func() {
		var head [4]byte
		binary.BigEndian.PutUint32(head[:], 100)
		a.Write(append(head[:], `{"type":"pi`...))
		a.Close()
	}()
	var v map[string]any
	if err := NewFrames(b).Receive(&v, 5*time.Second); err == nil || errors.Is(err, ErrLost) {
		t.Fatalf("a message cut short: %v", err)
	}
}
