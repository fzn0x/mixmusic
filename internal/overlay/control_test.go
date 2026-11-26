package overlay

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeControl is the player, as tests give it: what Extra tells, and the
// commands it was asked to run.
type fakeControl struct {
	mu    sync.Mutex
	extra Extra
	done  []Command
	err   error
	watch []func()
}

func (f *fakeControl) Extra() Extra {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.extra
}

func (f *fakeControl) Do(c Command) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.done = append(f.done, c)
	if f.err != nil {
		return f.err
	}
	switch c.Op {
	case "volume":
		if c.Percent != nil {
			f.extra.Volume = int(*c.Percent)
		}
	case "mute":
		f.extra.Muted = !f.extra.Muted
	case "repeat":
		switch c.Repeat {
		case "off", "all", "one":
			f.extra.Repeat = c.Repeat
		default:
			if f.extra.Repeat == "off" {
				f.extra.Repeat = "all"
			} else {
				f.extra.Repeat = "off"
			}
		}
	}
	return nil
}

func (f *fakeControl) Watch(fn func()) func() {
	f.mu.Lock()
	f.watch = append(f.watch, fn)
	f.mu.Unlock()
	return func() {}
}

// changed tells the watchers that what Extra tells changed.
func (f *fakeControl) changed() {
	f.mu.Lock()
	fns := append([]func(){}, f.watch...)
	f.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// commands is what Do was asked to run.
func (f *fakeControl) commands() []Command {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Command{}, f.done...)
}

const testToken = "0123456789abcdef0123456789abcdef"

// startControl returns a server that takes commands, and its address.
func startControl(t *testing.T) (*Server, string, *fakeControl) {
	t.Helper()
	s := New()
	ctl := &fakeControl{extra: Extra{Volume: 40, Repeat: "off", Queue: []Track{{ID: "yt:2", Title: "Next", Artists: []string{"A"}}}}}
	s.Control = ctl
	s.SetToken(testToken)
	s.Update(song)
	t.Cleanup(s.Close)
	if err := s.Apply(false, true, 0); err != nil {
		t.Fatal(err)
	}
	st := s.Status()
	if !st.On || !st.Control || st.Serving {
		t.Fatalf("status %+v, want control on and the reads off", st)
	}
	return s, "http://" + st.Addr, ctl
}

// command sends a command, with the token.
func command(t *testing.T, base, op, body string) (*http.Response, []byte) {
	t.Helper()
	req, _ := http.NewRequest("POST", base+"/control/"+op, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out := make([]byte, 0, 4096)
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		out = append(out, buf[:n]...)
		if err != nil {
			break
		}
	}
	return resp, out
}

// authed is req with the token.
func authed(req *http.Request) *http.Request {
	req.Header.Set("Authorization", "Bearer "+testToken)
	return req
}

func TestControlNeedsToken(t *testing.T) {
	s, base, _ := startControl(t)
	// No token, another token, and a token with no prefix.
	for _, h := range []string{"", "Bearer the-wrong-token", testToken} {
		req, _ := http.NewRequest("GET", base+"/control/state", nil)
		if h != "" {
			req.Header.Set("Authorization", h)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("token %q has status %d, want 401", h, resp.StatusCode)
		}
		if got := resp.Header.Get("WWW-Authenticate"); !strings.HasPrefix(got, "Bearer") {
			t.Errorf("token %q tells %q", h, got)
		}
	}
	if resp, _ := get(t, base+"/control/state"); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("a plain GET has status %d", resp.StatusCode)
	}
	// The token.
	req, _ := http.NewRequest("GET", base+"/control/state", nil)
	resp, err := http.DefaultClient.Do(authed(req))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("the token has status %d", resp.StatusCode)
	}
	// Another token, after the user makes one.
	s.SetToken("fedcba9876543210fedcba9876543210")
	if resp, _ := get(t, base+"/control/state"); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("the old token has status %d", resp.StatusCode)
	}
}

func TestControlRefusesWebPages(t *testing.T) {
	_, base, _ := startControl(t)
	for _, h := range []string{"Origin", "Sec-Fetch-Site"} {
		req, _ := http.NewRequest("POST", base+"/control/toggle", strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer "+testToken)
		req.Header.Set(h, "https://example.com")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s has status %d, want 403", h, resp.StatusCode)
		}
	}
	// No answer to a page of another origin.
	req, _ := http.NewRequest("GET", base+"/control/state", nil)
	resp, err := http.DefaultClient.Do(authed(req))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("control answers pages of %q", got)
	}
}

func TestControlState(t *testing.T) {
	_, base, _ := startControl(t)
	req, _ := http.NewRequest("GET", base+"/control/state", nil)
	resp, err := http.DefaultClient.Do(authed(req))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	want := []string{"active", "playing", "id", "title", "artists", "artist", "album", "source",
		"positionMs", "durationMs", "live", "speed", "cover", "volume", "muted", "shuffle",
		"repeat", "liked", "canNext", "canPrevious", "queue"}
	for _, k := range want {
		if _, ok := raw[k]; !ok {
			t.Errorf("the state lacks %q: %v", k, raw)
		}
	}
	if len(raw) != len(want) {
		t.Errorf("the state has %d fields, want %d: %v", len(raw), len(want), raw)
	}
	if raw["title"] != "Song" || raw["volume"] != float64(40) || raw["repeat"] != "off" {
		t.Errorf("the state %v", raw)
	}
	queue, _ := raw["queue"].([]any)
	if len(queue) != 1 {
		t.Fatalf("the queue %v", raw["queue"])
	}
	if q, _ := queue[0].(map[string]any); q["id"] != "yt:2" || q["title"] != "Next" {
		t.Errorf("the queue's track %v", queue[0])
	}
}

func TestControlCommands(t *testing.T) {
	_, base, ctl := startControl(t)
	cases := []struct{ op, body string }{
		{"toggle", ""}, {"play", ""}, {"pause", ""}, {"next", ""}, {"previous", ""},
		{"mute", ""}, {"shuffle", ""}, {"repeat", ""}, {"repeat", `{"repeat":"one"}`},
		{"like", ""}, {"dislike", ""}, {"track", `{"id":"yt:9"}`},
		{"seek", `{"position_seconds":72}`}, {"seek", `{"by_seconds":-10}`},
		{"volume", `{"percent":55}`}, {"volume", `{"by_percent":-5}`},
	}
	for _, c := range cases {
		resp, body := command(t, base, c.op, c.body)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s %s has status %d: %s", c.op, c.body, resp.StatusCode, body)
			continue
		}
		var st statePayload
		if err := json.Unmarshal(body, &st); err != nil {
			t.Errorf("%s answers %s", c.op, body)
		}
		if !st.Active || st.ID != song.ID {
			t.Errorf("%s answers state %s", c.op, body)
		}
	}
	got := ctl.commands()
	if len(got) != len(cases) {
		t.Fatalf("the control ran %d commands, want %d", len(got), len(cases))
	}
	if got[12].Position == nil || *got[12].Position != 72 {
		t.Errorf("seek to 72: %+v", got[12])
	}
	if got[13].By == nil || *got[13].By != -10 {
		t.Errorf("seek by -10: %+v", got[13])
	}
	if got[14].Percent == nil || *got[14].Percent != 55 {
		t.Errorf("volume 55: %+v", got[14])
	}
	if got[15].ByPercent == nil || *got[15].ByPercent != -5 {
		t.Errorf("volume by -5: %+v", got[15])
	}
	// The likes and dislikes are toggles, as the shortcuts are: the
	// control decides what they do.
	if got[9].Op != "like" || got[10].Op != "dislike" {
		t.Errorf("the likes %+v", got[9:11])
	}
}

func TestControlRefusesBadFields(t *testing.T) {
	_, base, _ := startControl(t)
	for _, c := range []struct {
		op, body string
		status   int
	}{
		{"track", `{}`, http.StatusBadRequest},
		{"seek", `{}`, http.StatusBadRequest},
		{"volume", `{}`, http.StatusBadRequest},
		{"repeat", `{"repeat":"sometimes"}`, http.StatusBadRequest},
		{"volume", `{"percent":`, http.StatusBadRequest},
		{"volume", `{"percent":10,"title":"` + strings.Repeat("x", 2000) + `"}`, http.StatusRequestEntityTooLarge},
		{"nothing", `{}`, http.StatusNotFound},
	} {
		resp, body := command(t, base, c.op, c.body)
		if resp.StatusCode != c.status {
			t.Errorf("%s %.40q has status %d, want %d: %s", c.op, c.body, resp.StatusCode, c.status, body)
		}
	}
	// The reads take no commands, and the commands take no reads.
	resp, _ := command(t, base, "state", `{}`)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST /control/state has status %d", resp.StatusCode)
	}
	req, _ := http.NewRequest("GET", base+"/control/play", nil)
	resp, err := http.DefaultClient.Do(authed(req))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /control/play has status %d", resp.StatusCode)
	}
}

func TestControlSaysWhyItCant(t *testing.T) {
	_, base, ctl := startControl(t)
	ctl.err = ErrNothingLoaded
	resp, body := command(t, base, "play", "")
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("nothing loaded has status %d: %s", resp.StatusCode, body)
	}
	var out map[string]string
	if err := json.Unmarshal(body, &out); err != nil || out["error"] != ErrNothingLoaded.Error() {
		t.Errorf("nothing loaded answers %s", body)
	}
	ctl.err = ErrNoPlayer
	if resp, _ := command(t, base, "play", ""); resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("no player has status %d", resp.StatusCode)
	}
	ctl.err = ErrNoTrack
	if resp, _ := command(t, base, "track", `{"id":"yt:zzz"}`); resp.StatusCode != http.StatusConflict {
		t.Errorf("no such track has status %d", resp.StatusCode)
	}
}

func TestControlPace(t *testing.T) {
	_, base, _ := startControl(t)
	burst := 0
	for range controlBurst {
		if resp, body := command(t, base, "next", ""); resp.StatusCode != http.StatusOK {
			t.Fatalf("the burst stopped at %d: %d %s", burst, resp.StatusCode, body)
		}
		burst++
	}
	resp, _ := command(t, base, "next", "")
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("past the burst: status %d, want 429", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Error("a refusal tells no Retry-After")
	}
	// A moment later, one comes again.
	time.Sleep(time.Second / controlPace)
	for range 3 {
		time.Sleep(time.Second / controlPace)
	}
	if resp, _ := command(t, base, "next", ""); resp.StatusCode != http.StatusOK {
		t.Errorf("after a moment: status %d", resp.StatusCode)
	}
}

// stateOf reads one event of the control's stream.
func stateOf(t *testing.T, ch <-chan event) statePayload {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("the stream ended")
		}
		if ev.name != "state" {
			t.Fatalf("an event named %q", ev.name)
		}
		var st statePayload
		if err := json.Unmarshal([]byte(ev.data), &st); err != nil {
			t.Fatal(err)
		}
		return st
	case <-time.After(5 * time.Second):
		t.Fatal("no event came")
	}
	return statePayload{}
}

// controlStream reads /control/events with the token.
func controlStream(t *testing.T, base string) (<-chan event, func()) {
	t.Helper()
	req, _ := http.NewRequest("GET", base+"/control/events", nil)
	req = authed(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	out := make(chan event, 64)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		r := bufio.NewScanner(resp.Body)
		var ev event
		for r.Scan() {
			line := r.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				ev.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				ev.data = strings.TrimPrefix(line, "data: ")
			case line == "" && ev.name != "":
				out <- ev
				ev = event{}
			}
		}
	}()
	return out, func() { resp.Body.Close() }
}

func TestControlEvents(t *testing.T) {
	s, base, ctl := startControl(t)
	ch, stop := controlStream(t, base)
	defer stop()
	st := stateOf(t, ch)
	if st.Volume != 40 || st.Title != "Song" {
		t.Errorf("the first state %+v", st)
	}
	// A command's change comes at once.
	if resp, body := command(t, base, "volume", `{"percent":70}`); resp.StatusCode != http.StatusOK {
		t.Fatalf("volume: %d %s", resp.StatusCode, body)
	}
	if st := stateOf(t, ch); st.Volume != 70 {
		t.Errorf("after the volume: %+v", st)
	}
	// So does what the player tells by itself, as a track liked, a queue
	// changed or a shuffle.
	other := song
	other.ID, other.Title = "yt:2", "Another"
	s.Update(other)
	if st := stateOf(t, ch); st.Title != "Another" {
		t.Errorf("after a track: %+v", st)
	}
	ctl.mu.Lock()
	ctl.extra.Shuffle = true
	ctl.mu.Unlock()
	ctl.changed()
	if st := stateOf(t, ch); !st.Shuffle {
		t.Errorf("after a shuffle: %+v", st)
	}
}

func TestControlOffAndOn(t *testing.T) {
	s, base, _ := startControl(t)
	ch, stop := controlStream(t, base)
	defer stop()
	stateOf(t, ch)
	// Only the reads, on the same port: /control goes, and its stream ends.
	addr := s.Status().Addr
	if err := s.Apply(true, false, 0); err != nil {
		t.Fatal(err)
	}
	if got := s.Status().Addr; got != addr {
		t.Errorf("the port moved from %s to %s", addr, got)
	}
	base = "http://" + addr
	if resp, _ := get(t, base+"/control/state"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("control off has status %d", resp.StatusCode)
	}
	select {
	case _, ok := <-ch:
		if ok {
			<-ch
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the control's stream didn't end as control went off")
	}
	// The reads answer.
	if resp, body := get(t, base+"/now-playing"); resp.StatusCode != 200 || !strings.Contains(string(body), `"title":"Song"`) {
		t.Errorf("the reads with control off: %d %s", resp.StatusCode, body)
	}
}

func TestReadsOffWhileControlOn(t *testing.T) {
	_, base, _ := startControl(t)
	for _, p := range []string{"/now-playing", "/events", "/overlay", "/cover"} {
		if resp, _ := get(t, base+p); resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s has status %d with the reads off", p, resp.StatusCode)
		}
	}
	// A track and a cover aren't told either.
	req, _ := http.NewRequest("GET", base+"/control/state", nil)
	resp, err := http.DefaultClient.Do(authed(req))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("control with the reads off: %d", resp.StatusCode)
	}
}

func TestControlListensOnLoopbackOnly(t *testing.T) {
	var asked string
	s := New()
	s.Listen = func(network, addr string) (net.Listener, error) {
		asked = addr
		return net.Listen(network, "127.0.0.1:0")
	}
	defer s.Close()
	if err := s.Apply(false, true, 8284); err != nil {
		t.Fatal(err)
	}
	if asked != "127.0.0.1:8284" {
		t.Errorf("listened on %q, want 127.0.0.1:8284", asked)
	}
}

// TestControlNoPlayer shows a server the app gave no player: a command
// says so, rather than doing nothing.
func TestControlNoPlayer(t *testing.T) {
	s := New()
	s.SetToken(testToken)
	defer s.Close()
	if err := s.Apply(false, true, 0); err != nil {
		t.Fatal(err)
	}
	resp, body := command(t, "http://"+s.Status().Addr, "play", "")
	if resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(body), ErrNoPlayer.Error()) {
		t.Errorf("no player: %d %s", resp.StatusCode, body)
	}
	// The state still tells what it can.
	req, _ := http.NewRequest("GET", "http://"+s.Status().Addr+"/control/state", nil)
	r2, err := http.DefaultClient.Do(authed(req))
	if err != nil {
		t.Fatal(err)
	}
	r2.Body.Close()
	if r2.StatusCode != http.StatusOK {
		t.Errorf("the state with no player: %d", r2.StatusCode)
	}
}

var _ Control = (*fakeControl)(nil)

// The unwatched kinds of failure are errors, not panics.
func TestSameToken(t *testing.T) {
	if sameToken("", "") {
		t.Error("no token matches a server's empty one")
	}
	if !sameToken("abc", "abc") || sameToken("abc", "abd") || sameToken("ab", "abc") {
		t.Error("sameToken tells two tokens apart")
	}
	if errors.Is(ErrNoTrack, ErrNothingLoaded) {
		t.Error("the reasons are one")
	}
}
