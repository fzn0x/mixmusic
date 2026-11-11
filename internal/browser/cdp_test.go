package browser

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeDevtools is a browser at the other end of the pipes, which answers
// each command as answer says, after an event the client ignores.
func fakeDevtools(t *testing.T, answer func(method string) string) (*devtools, func()) {
	t.Helper()
	toBrowser, fromClient := io.Pipe()
	fromBrowser, toClient := io.Pipe()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r := bufio.NewReader(toBrowser)
		for {
			msg, err := r.ReadBytes(0)
			if err != nil {
				return
			}
			var cmd struct {
				ID     int             `json:"id"`
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if err := json.Unmarshal(msg[:len(msg)-1], &cmd); err != nil || string(cmd.Params) != "{}" {
				t.Errorf("the browser got %q", msg)
			}
			reply := answer(cmd.Method)
			if reply == "" { // the browser exits
				toClient.Close()
				toBrowser.Close()
				return
			}
			fmt.Fprintf(toClient, `{"method":"Target.targetInfoChanged","params":{}}%c{"id":%d,%s}%c`, 0, cmd.ID, reply, 0)
		}
	}()
	d := newDevtools(fromBrowser, fromClient)
	return d, func() {
		d.close()
		toClient.Close()
		wg.Wait()
	}
}

// TestDevtoolsCookies reads the browser's cookies, those of a session
// too, but not those kept for one site embedded in another; and tells once
// the browser has no page open, after it had one, twice in a row.
func TestDevtoolsCookies(t *testing.T) {
	var mu sync.Mutex
	pages := `{"type":"browser"},{"type":"page","url":"https://music.youtube.com/"}`
	d, stop := fakeDevtools(t, func(method string) string {
		mu.Lock()
		defer mu.Unlock()
		switch method {
		case "Target.getTargets":
			return `"result":{"targetInfos":[` + pages + `]}`
		case "Storage.getCookies":
			return `"result":{"cookies":[
				{"name":"SAPISID","value":"a","domain":".youtube.com","path":"/","expires":4102444800.5,"httpOnly":false,"secure":true,"session":false},
				{"name":"YSC","value":"b","domain":".youtube.com","path":"/","expires":-1,"httpOnly":true,"secure":true,"session":true},
				{"name":"NID","value":"c","domain":".google.com","path":"/","expires":4102444800,"secure":true,"partitionKey":{"topLevelSite":"https://example.com","hasCrossSiteAncestor":false}},
				{"name":"OLD","value":"d","domain":"music.youtube.com","path":"/","expires":4102444800,"partitionKey":null}]}`
		}
		return `"error":{"code":-32601,"message":"no such method"}`
	})
	defer stop()
	c := &chromium{dt: d}
	ctx := context.Background()
	cookies, err := c.cookies(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, ck := range cookies {
		got = append(got, fmt.Sprintf("%s=%s@%s secure=%v httpOnly=%v expires=%d", ck.Name, ck.Value, ck.Domain, ck.Secure, ck.HttpOnly, ck.Expires.Unix()))
	}
	zero := time.Time{}.Unix()
	want := []string{
		"SAPISID=a@.youtube.com secure=true httpOnly=false expires=4102444800",
		fmt.Sprintf("YSC=b@.youtube.com secure=true httpOnly=true expires=%d", zero),
		"OLD=d@music.youtube.com secure=false httpOnly=false expires=4102444800",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("read\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	mu.Lock()
	pages = `{"type":"browser"}`
	mu.Unlock()
	if _, err := c.cookies(ctx); err != nil {
		t.Errorf("no page once gave %v", err)
	}
	if _, err := c.cookies(ctx); !errors.Is(err, errWindowsClosed) {
		t.Errorf("no page twice gave %v", err)
	}

	if err := d.call(ctx, "Nope.nothing", nil, nil); err == nil || !strings.Contains(err.Error(), "no such method") {
		t.Errorf("an unknown method gave %v", err)
	}
}

// TestDevtoolsBroken fails the commands that wait, once the browser exits.
func TestDevtoolsBroken(t *testing.T) {
	d, stop := fakeDevtools(t, func(string) string { return "" })
	defer stop()
	err := d.call(context.Background(), "Target.getTargets", nil, nil)
	if !errors.Is(err, errPipeBroken) {
		t.Errorf("a command to an exited browser gave %v", err)
	}
	if err := d.call(context.Background(), "Target.getTargets", nil, nil); !errors.Is(err, errPipeBroken) {
		t.Errorf("a command after gave %v", err)
	}
}

// TestDevtoolsCancel stops waiting for a browser that doesn't answer once
// the context ends.
func TestDevtoolsCancel(t *testing.T) {
	block := make(chan struct{})
	d, stop := fakeDevtools(t, func(string) string { <-block; return `"result":{}` })
	defer func() { close(block); stop() }()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := d.call(ctx, "Browser.close", nil, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a command the browser didn't answer gave %v", err)
	}
}
