package youtube

import (
	"context"
	"errors"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// watchYouTube is a fake YouTube Music whose player answers with the
// playbackTracking tracking, its URLs on the fake's own address, and
// whose reports' host check takes what allow allows: the fake isn't one
// of YouTube's hosts. The schedule's jitter is 3 and 7 seconds early.
func watchYouTube(t *testing.T, tracking func(srv string) map[string]any, allow func(*url.URL) bool) (*Client, func() []recorded) {
	t.Helper()
	var srv *httptest.Server
	c, requests, srv := fakeYouTube(t, func(path string) any {
		if path == "/youtubei/v1/player" {
			return map[string]any{"playbackTracking": tracking(srv.URL), "videoDetails": map[string]any{"lengthSeconds": "6000"}}
		}
		return map[string]any{}
	})
	saved, savedJitter := reportsTo, jitter
	reportsTo = allow
	jitter = func(n int) int { return map[int]int{10: 3, 40: 7}[n] }
	t.Cleanup(func() { reportsTo, jitter = saved, savedJitter })
	return c, requests
}

// episodeTracking is the playbackTracking of an episode, with the schedule
// of its reports, as YouTube's player response has it.
func episodeTracking(srv string) map[string]any {
	return map[string]any{
		"videostatsPlaybackUrl":                   map[string]any{"baseUrl": srv + "/api/stats/playback?docid=ep12&ei=x&el=detailpage&len=6000&ns=yt"},
		"videostatsWatchtimeUrl":                  map[string]any{"baseUrl": srv + "/api/stats/watchtime?docid=ep12&ei=x&el=detailpage&ns=yt&plid=p"},
		"videostatsScheduledFlushWalltimeSeconds": []any{10.0, 20.0, 30.0},
		"videostatsDefaultFlushIntervalSeconds":   40.0,
	}
}

func allowAll(*url.URL) bool { return true }

// TestStartWatch reports the play as ReportPlay does, and makes the watch
// of the playback with the same playback nonce, on the schedule the
// response tells, jittered as YouTube's player jitters it. Signed out, it
// sends nothing.
func TestStartWatch(t *testing.T) {
	c, requests := watchYouTube(t, episodeTracking, allowAll)
	if w, err := c.StartWatch(context.Background(), "ep12"); !errors.Is(err, ErrSignedOut) || w != nil || len(requests()) != 0 {
		t.Fatalf("signed out: %v, %v, %d requests", w, err, len(requests()))
	}
	c.SetCookies(session)
	w, err := c.StartWatch(context.Background(), "ep12")
	if err != nil {
		t.Fatal(err)
	}
	got := requests()
	if len(got) != 2 || got[0].body["videoId"] != "ep12" || got[1].path != "/api/stats/playback" {
		t.Fatalf("requests = %+v", got)
	}
	if play := got[1].query; play.Get("ver") != "2" || play.Get("c") != "WEB_REMIX" || len(play.Get("cpn")) != 16 || play.Get("cpn") != w.cpn {
		t.Errorf("the play's report = %s", play.Encode())
	}
	// 10, 20 and 30 s, then every 40: the first 3 s early, the fourth 7 s.
	for n, want := range []time.Duration{7, 10, 10, 36, 40, 40} {
		if g := w.Gap(n); g != want*time.Second {
			t.Errorf("Gap(%d) = %v, want %ds", n, g, want)
		}
	}
}

// TestWatchReport sends a report as YouTube's player does, to the
// response's videostatsWatchtimeUrl with the session: where the playback
// is, what was heard, at what speed, when, and whether it paused or ended.
func TestWatchReport(t *testing.T) {
	c, requests := watchYouTube(t, episodeTracking, allowAll)
	c.SetCookies(session)
	w, err := c.StartWatch(context.Background(), "ep12")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	reports := []WatchReport{
		{Heard: []Stretch{{From: 1200 * time.Second, To: 1210 * time.Second}}, At: 1210 * time.Second, Elapsed: 10 * time.Second, Next: 20 * time.Second},
		{Heard: []Stretch{{From: 1210 * time.Second, To: 1213500 * time.Millisecond}, {From: 1500 * time.Second, To: 1509 * time.Second, Speed: 1.5}},
			At: 1509 * time.Second, Elapsed: 20 * time.Second, Next: 30 * time.Second},
		{At: 1509 * time.Second, Elapsed: 25 * time.Second, Paused: true},
		{Heard: []Stretch{{From: 1509 * time.Second, To: 1520 * time.Second}}, At: 1520 * time.Second, Elapsed: 400 * time.Second, Final: true},
	}
	for _, r := range reports {
		if err := w.Report(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	got := requests()[2:]
	if len(got) != 4 {
		t.Fatalf("%d reports", len(got))
	}
	want := []map[string]string{
		{"cmt": "1210", "st": "1200", "et": "1210", "rate": "", "rt": "10", "rtn": "20", "state": "playing", "final": ""},
		{"cmt": "1509", "st": "1210,1500", "et": "1213.5,1509", "rate": "1,1.5", "rt": "20", "rtn": "30", "state": "playing", "final": ""},
		{"cmt": "1509", "st": "1509", "et": "1509", "rate": "", "rt": "25", "rtn": "", "state": "paused", "final": ""},
		{"cmt": "1520", "st": "1509", "et": "1520", "rate": "", "rt": "400", "rtn": "", "state": "paused", "final": "1"},
	}
	for i, r := range got {
		q := r.query
		if r.path != "/api/stats/watchtime" {
			t.Errorf("report %d went to %s", i, r.path)
		}
		// What the URL had stays, with the length the response tells.
		if q.Get("docid") != "ep12" || q.Get("ei") != "x" || q.Get("el") != "detailpage" || q.Get("ns") != "yt" || q.Get("plid") != "p" || q.Get("len") != "6000" {
			t.Errorf("report %d lost its URL's own: %s", i, q.Encode())
		}
		if q.Get("ver") != "2" || q.Get("c") != "WEB_REMIX" || q.Get("cver") == "" || q.Get("cpn") != w.cpn || q.Get("idpj") != "-3" || q.Get("ldpj") != "-7" {
			t.Errorf("report %d = %s", i, q.Encode())
		}
		for k, v := range want[i] {
			if q.Get(k) != v {
				t.Errorf("report %d: %s = %q, want %q", i, k, q.Get(k), v)
			}
		}
		if r.header.Get("Cookie") == "" || r.header.Get("Authorization") == "" {
			t.Errorf("report %d went without the session", i)
		}
	}

	// Signed out since, the watch sends nothing.
	c.SetCookies(nil)
	if err := w.Report(ctx, reports[0]); err != nil || len(requests()) != 6 {
		t.Errorf("signed out: %v, %d requests", err, len(requests()))
	}
}

// TestWatchHosts reports the watch time to YouTube's hosts alone, with the
// session: the response's URL to another stops the watch before it starts.
func TestWatchHosts(t *testing.T) {
	playOnly := func(u *url.URL) bool { return u.Path == "/api/stats/playback" }
	c, requests := watchYouTube(t, episodeTracking, playOnly)
	c.SetCookies(session)
	if w, err := c.StartWatch(context.Background(), "ep12"); err == nil || w != nil {
		t.Errorf("a watch time report to %v", w.url)
	}
	if got := requests(); len(got) != 2 || got[1].path != "/api/stats/playback" {
		t.Errorf("requests = %+v", got)
	}

	// Nor does a watch's report go once its host isn't one.
	reportsTo = allowAll
	w, err := c.StartWatch(context.Background(), "ep12")
	if err != nil {
		t.Fatal(err)
	}
	reportsTo = playOnly
	if err := w.Report(context.Background(), WatchReport{At: time.Minute, Elapsed: 10 * time.Second}); err == nil || len(requests()) != 4 {
		t.Errorf("reported to a host that isn't YouTube's: %v, %d requests", err, len(requests()))
	}

	for _, u := range []string{"https://s.youtube.com/api/stats/watchtime", "https://music.youtube.com/api/stats/watchtime", "https://youtube.com/x"} {
		if v, _ := url.Parse(u); !youtubeHosts(v) {
			t.Errorf("%s refused", u)
		}
	}
	for _, u := range []string{"http://s.youtube.com/api/stats/watchtime", "https://youtube.com.example/x", "https://evilyoutube.com/x"} {
		if v, _ := url.Parse(u); youtubeHosts(v) {
			t.Errorf("%s taken", u)
		}
	}
}

// youtubeHosts is the host check reportsTo is outside tests.
var youtubeHosts = reportsTo

// TestWatchSchedule falls back on the player's own schedule when the
// response tells none, or one that goes back in time.
func TestWatchSchedule(t *testing.T) {
	for name, tracking := range map[string]func(string) map[string]any{
		"none": func(srv string) map[string]any {
			v := episodeTracking(srv)
			delete(v, "videostatsScheduledFlushWalltimeSeconds")
			delete(v, "videostatsDefaultFlushIntervalSeconds")
			return v
		},
		"backwards": func(srv string) map[string]any {
			v := episodeTracking(srv)
			v["videostatsScheduledFlushWalltimeSeconds"] = []any{30.0, 20.0}
			return v
		},
	} {
		c, _ := watchYouTube(t, tracking, allowAll)
		c.SetCookies(session)
		w, err := c.StartWatch(context.Background(), "ep12")
		if err != nil {
			t.Fatal(err)
		}
		for n, want := range []time.Duration{7, 10, 10, 36, 40} {
			if g := w.Gap(n); g != want*time.Second {
				t.Errorf("%s: Gap(%d) = %v, want %ds", name, n, g, want)
			}
		}
	}
	// Without the watch time's URL, the play is reported, and the watch
	// fails.
	c, requests := watchYouTube(t, func(srv string) map[string]any {
		v := episodeTracking(srv)
		delete(v, "videostatsWatchtimeUrl")
		return v
	}, allowAll)
	c.SetCookies(session)
	if _, err := c.StartWatch(context.Background(), "ep12"); err == nil || len(requests()) != 2 {
		t.Errorf("without a watch time URL: %v, %d requests", err, len(requests()))
	}
}
