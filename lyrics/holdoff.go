package lyrics

import (
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	// maxHoldoff caps how long a source is left alone after it says to
	// wait, so a wrong header can't silence it for a session.
	maxHoldoff = time.Hour
	// defaultHoldoff is the wait after a 429 without a usable Retry-After.
	defaultHoldoff = time.Minute
	// holdoffJitter is the share of the wait added at random, up to it, as
	// LRCLIB's docs suggest, so apps that were told together don't return
	// together.
	holdoffJitter = 0.3
)

// holdoff keeps the time a source may be asked again, after it answered
// HTTP 429 Too Many Requests. The zero value is ready to use.
type holdoff struct {
	mu    sync.Mutex
	until time.Time
	// now is the clock; nil for time.Now. Tests set it.
	now func() time.Time
}

func (h *holdoff) clock() time.Time {
	if h.now != nil {
		return h.now()
	}
	return time.Now()
}

// active reports whether the source must not be asked yet.
func (h *holdoff) active() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.clock().Before(h.until)
}

// set starts the wait a 429's Retry-After header asks for, plus a random
// part of it.
func (h *holdoff) set(header string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.clock()
	d := retryAfterHeader(header, now)
	if d <= 0 {
		d = defaultHoldoff
	}
	d += time.Duration(rand.Float64() * holdoffJitter * float64(d))
	if until := now.Add(d); until.After(h.until) {
		h.until = until
	}
}

// retryAfterHeader reads a Retry-After header, in seconds or as a date;
// 0 when it is missing or unreadable.
func retryAfterHeader(v string, now time.Time) time.Duration {
	var d time.Duration
	if n, err := strconv.Atoi(v); err == nil {
		d = time.Duration(n) * time.Second
	} else if t, err := http.ParseTime(v); err == nil {
		d = t.Sub(now)
	}
	return min(max(d, 0), maxHoldoff)
}
