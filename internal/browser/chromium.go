package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"
)

// chromiumArgs are the arguments of a browser made from Chromium, with
// the profile in dir and the pipes' own, at url:
//   - its own profile, which the DevTools protocol may reach, as Chrome
//     136 and later refuse it on the user's own (--user-data-dir);
//   - none of the pages and questions of a new profile's first run;
//   - on Linux, no keyring, which may ask for its password, and on macOS
//     no Keychain, which may ask to use Chrome's key: the profile's
//     cookies are read through the protocol, not from its files;
//   - on Windows, from a MixMusic run as administrator, no starting again
//     as the user, through Explorer, which would leave the pipes behind.
func chromiumArgs(goos, dir string, pipes []string, url string) []string {
	args := []string{
		"--user-data-dir=" + dir,
		"--remote-debugging-pipe",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-breakpad",
	}
	switch goos {
	case "linux":
		args = append(args, "--password-store=basic")
	case "darwin":
		args = append(args, "--use-mock-keychain")
	case "windows":
		args = append(args, "--do-not-de-elevate")
	}
	args = append(args, pipes...)
	args = append(args, extraArgs...)
	return append(args, url)
}

// chromium is a browser made from Chromium, which MixMusic reaches through
// the DevTools protocol.
type chromium struct {
	p  *process
	dt *devtools
	// pages is whether the browser had a page open, and closedReads how
	// many reads in a row found none since.
	pages       bool
	closedReads int
}

func startChromium(ctx context.Context, b Browser, dir, url string) (session, error) {
	// MixMusic writes to the browser through one pipe, and reads it
	// through the other.
	childRead, ourWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	ourRead, childWrite, err := os.Pipe()
	if err != nil {
		childRead.Close()
		ourWrite.Close()
		return nil, err
	}
	pipes := pipeArgs(childRead, childWrite)
	p, err := startProcess(b.Path, chromiumArgs(runtime.GOOS, dir, pipes, url), func(cmd *exec.Cmd) {
		givePipes(cmd, childRead, childWrite)
	})
	// The browser has its own copies of its ends: once it exits, reading
	// MixMusic's end ends.
	childRead.Close()
	childWrite.Close()
	if err != nil {
		ourRead.Close()
		ourWrite.Close()
		return nil, err
	}
	c := &chromium{p: p, dt: newDevtools(ourRead, ourWrite)}
	go func() {
		<-p.done
		ourRead.Close()
	}()
	return c, nil
}

func (c *chromium) exited() <-chan struct{} { return c.p.done }

// devtoolsCookie is a cookie as Storage.getCookies tells it.
type devtoolsCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	Session  bool    `json:"session"`
	// PartitionKey is set on a cookie kept for one site that embeds
	// another, which isn't the user's session of either.
	PartitionKey json.RawMessage `json:"partitionKey"`
}

// cookies reads the browser's cookies, once it tells which pages it has
// open: none, after it had some, means the user closed its last window,
// which on macOS leaves it running.
func (c *chromium) cookies(ctx context.Context) ([]*http.Cookie, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var targets struct {
		TargetInfos []struct {
			Type string `json:"type"`
		} `json:"targetInfos"`
	}
	if err := c.dt.call(ctx, "Target.getTargets", nil, &targets); err != nil {
		return nil, err
	}
	pages := 0
	for _, t := range targets.TargetInfos {
		if t.Type == "page" {
			pages++
		}
	}
	switch {
	case pages > 0:
		c.pages, c.closedReads = true, 0
	case c.pages:
		// Twice in a row, as a window may be between pages.
		if c.closedReads++; c.closedReads >= 2 {
			return nil, errWindowsClosed
		}
	}
	var got struct {
		Cookies []devtoolsCookie `json:"cookies"`
	}
	if err := c.dt.call(ctx, "Storage.getCookies", nil, &got); err != nil {
		return nil, err
	}
	var cookies []*http.Cookie
	for _, k := range got.Cookies {
		if len(k.PartitionKey) > 0 && string(k.PartitionKey) != "null" {
			continue
		}
		ck := &http.Cookie{Name: k.Name, Value: k.Value, Domain: k.Domain, Path: k.Path, Secure: k.Secure, HttpOnly: k.HTTPOnly}
		if !k.Session && k.Expires > 0 {
			ck.Expires = time.Unix(int64(k.Expires), 0)
		}
		cookies = append(cookies, ck)
	}
	return cookies, nil
}

// quit asks the browser to close, which it does at once, and ends it if
// it doesn't.
func (c *chromium) quit() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	c.dt.call(ctx, "Browser.close", nil, nil)
	cancel()
	c.p.stop(5 * time.Second)
	c.dt.close()
}
