package remote

import (
	"net"
	"runtime"
	"testing"
	"time"
)

// TestManyUnpairedLengths is #736's reproduction: a computer not paired
// opens many connections, and sends on each the length of a megabyte,
// 00 10 00 00, then nothing. The server made room for a megabyte on each,
// and held it until its wait for a hello ran out. It now ends each at its
// length, and holds next to nothing for them.
func TestManyUnpairedLengths(t *testing.T) {
	const n = 64
	_, addr := serve(t, newFakeHost(), patient)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	conns := make([]net.Conn, 0, n)
	for range n {
		c := dialUnpaired(t, addr)
		if _, err := c.Write([]byte{0x00, 0x10, 0x00, 0x00}); err != nil {
			t.Fatal(err)
		}
		conns = append(conns, c)
	}
	// Each is ended by now, or waited for, as before, with room made for
	// its megabyte as its length came.
	by := time.Now().Add(3 * time.Second)
	ended := 0
	for _, c := range conns {
		if endedWithin(c, time.Until(by)) {
			ended++
		}
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	grew := float64(int64(after.HeapAlloc)-int64(before.HeapAlloc)) / (1 << 20)
	t.Logf("%d connections, each the length of a megabyte: %d ended, the heap grew %.1f MiB", n, ended, grew)
	if ended != n {
		t.Errorf("%d of %d connections went on", n-ended, n)
	}
	if grew > n/4 {
		t.Errorf("%d connections held %.1f MiB", n, grew)
	}
}
