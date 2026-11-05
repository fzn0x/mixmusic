package youtube

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// A Watch reports a playback of a video to the account's history on
// YouTube Music while it plays, as YouTube's own player does: what was
// heard since the report before and where it is now, to the player
// response's videostatsWatchtimeUrl. Where the history says the user is in
// an episode is where YouTube Music resumes it, on any device.
//
// The player sends its reports 10, 20 and 30 seconds after a playback
// starts, then every 40, as the response's playbackTracking tells in
// videostatsScheduledFlushWalltimeSeconds and
// videostatsDefaultFlushIntervalSeconds, each schedule a few seconds early
// at random; and one as it pauses, one as it plays again, and a last, with
// final=1, as it moves on. Gap tells the schedule; the caller sends the
// reports.
type Watch struct {
	c   *Client
	url *url.URL
	cpn string
	// gaps are the waits between the first reports, and every the one
	// between the others.
	gaps  []time.Duration
	every time.Duration
	// early and late are how many seconds before their time, at random,
	// the first report and those after the first three go: the player
	// tells them as idpj and ldpj.
	early, late int
}

// The schedule of the reports while the response tells none: 10, 20 and
// 30 seconds after the playback started, then every 40.
var (
	defaultFlushes = []int{10, 20, 30}
	defaultEvery   = 40
)

// jitter is a number from 0 to n-1 at random, which moves the reports'
// schedule as the player does: tests fix it.
var jitter = rand.IntN

// StartWatch reports a play of videoID, as ReportPlay does, and returns the
// Watch of the playback, whose reports go with the play's playback nonce.
// Signed out, it reports nothing: ErrSignedOut.
func (c *Client) StartWatch(ctx context.Context, videoID string) (*Watch, error) {
	if err := c.session(); err != nil {
		return nil, err
	}
	cpn := playbackNonce()
	n, err := c.reportPlay(ctx, videoID, cpn)
	if err != nil {
		return nil, err
	}
	tracking := n.get("playbackTracking")
	base := tracking.get("videostatsWatchtimeUrl", "baseUrl").str()
	if base == "" {
		return nil, noReport(n, "watch time")
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	if !reportsTo(u) {
		return nil, fmt.Errorf("youtube music: refusing to report the watch time to %s", u.Host)
	}
	q := u.Query()
	if q.Get("len") == "" {
		if length := n.get("videoDetails", "lengthSeconds").str(); length != "" {
			q.Set("len", length)
		}
	}
	u.RawQuery = q.Encode()
	w := &Watch{c: c, url: u, cpn: cpn, early: -jitter(10), late: -jitter(40)}
	var flushes []int
	for _, f := range tracking.get("videostatsScheduledFlushWalltimeSeconds").list() {
		flushes = append(flushes, f.number())
	}
	every := tracking.get("videostatsDefaultFlushIntervalSeconds").number()
	if !w.schedule(flushes, every) {
		w.schedule(defaultFlushes, defaultEvery)
	}
	return w, nil
}

// schedule sets the waits between the reports from the seconds after the
// playback starts that the first go and every how many seconds the others
// do, as the player makes them: the first early seconds early, the one
// after them late seconds early. It reports whether they make a schedule.
func (w *Watch) schedule(flushes []int, every int) bool {
	if len(flushes) == 0 || every <= 0 {
		return false
	}
	gaps := []int{flushes[0] + w.early}
	for i := 1; i < len(flushes); i++ {
		gaps = append(gaps, flushes[i]-flushes[i-1])
	}
	gaps = append(gaps, every+w.late-w.early)
	w.gaps = w.gaps[:0]
	for _, g := range gaps {
		if g <= 0 {
			return false
		}
		w.gaps = append(w.gaps, time.Duration(g)*time.Second)
	}
	w.every = time.Duration(every) * time.Second
	return true
}

// Gap returns how long the report n waits after the one before, from 0,
// which waits after the playback started, or after it played again from
// a pause: the player waits the next gap each time.
func (w *Watch) Gap(n int) time.Duration {
	if n >= 0 && n < len(w.gaps) {
		return w.gaps[n]
	}
	return w.every
}

// A Stretch is a part of a video heard without a break, from From to To in
// the video's own time, at Speed: 0 is 1.
type Stretch struct {
	From, To time.Duration
	Speed    float64
}

// A WatchReport is what one report of a Watch tells.
type WatchReport struct {
	// Heard is what was heard since the report before, the earliest
	// first: none tells the playback stayed where it is.
	Heard []Stretch
	// At is where the playback is, in the video's own time.
	At time.Duration
	// Elapsed is how long ago the playback started, and Next how long
	// after it started the next report goes, or 0 when none waits, as
	// while it's paused.
	Elapsed, Next time.Duration
	// Paused tells the playback is paused, and Final that it ended: the
	// last report, of a playback paused.
	Paused, Final bool
}

// Report sends r, as the player does: cmt is where the playback is; st,
// et and rate the stretches heard, by their starts, ends and speeds; rt
// how long ago it started, and rtn when the next report goes; state
// "playing" or "paused"; final=1 on the last. Signed out since the watch
// started, it sends nothing.
func (w *Watch) Report(ctx context.Context, r WatchReport) error {
	if w.c.session() != nil {
		return nil
	}
	u := *w.url
	q := u.Query()
	q.Set("ver", "2")
	q.Set("c", clientName)
	q.Set("cver", clientVersion(time.Now()))
	q.Set("cpn", w.cpn)
	q.Set("cmt", seconds(r.At))
	heard := r.Heard
	if len(heard) == 0 {
		heard = []Stretch{{From: r.At, To: r.At}}
	}
	var st, et, rate []string
	sped := false
	for _, s := range heard {
		speed := s.Speed
		if speed <= 0 {
			speed = 1
		}
		sped = sped || speed != 1
		st, et = append(st, seconds(s.From)), append(et, seconds(s.To))
		rate = append(rate, strconv.FormatFloat(speed, 'f', -1, 64))
	}
	q.Set("st", strings.Join(st, ","))
	q.Set("et", strings.Join(et, ","))
	if sped {
		q.Set("rate", strings.Join(rate, ","))
	}
	q.Set("rt", seconds(r.Elapsed))
	if r.Next > 0 && !r.Paused && !r.Final {
		q.Set("rtn", seconds(r.Next))
	}
	state := "playing"
	if r.Paused || r.Final {
		state = "paused"
	}
	q.Set("state", state)
	if r.Final {
		q.Set("final", "1")
	}
	q.Set("idpj", strconv.Itoa(w.early))
	q.Set("ldpj", strconv.Itoa(w.late))
	u.RawQuery = q.Encode()
	return w.c.ping(ctx, &u)
}

// seconds writes d as the player writes times: seconds, to the
// millisecond, without trailing zeros, as "12.5".
func seconds(d time.Duration) string {
	return strconv.FormatFloat(float64(d.Round(time.Millisecond))/float64(time.Second), 'f', -1, 64)
}
