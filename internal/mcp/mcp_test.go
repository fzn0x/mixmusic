package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// testTools are an echo, a tool that fails, one that panics, and one that
// waits until it is cancelled.
func testTools(started chan<- struct{}, cancelled *atomic.Bool) []Tool {
	return []Tool{
		{Name: "echo", Title: "Echo", Description: "Says what it's told.", ReadOnly: true,
			Input: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"],"additionalProperties":false}`),
			Call: func(_ context.Context, args json.RawMessage) (Result, error) {
				a, err := Args[struct {
					Text string `json:"text"`
				}](args)
				if err != nil {
					return Result{}, err
				}
				return Result{Text: a.Text, Data: map[string]string{"text": a.Text}}, nil
			}},
		{Name: "fail", Description: "Fails.", Call: func(context.Context, json.RawMessage) (Result, error) {
			return Result{}, errors.New("nothing to play")
		}},
		{Name: "panic", Description: "Panics.", Call: func(context.Context, json.RawMessage) (Result, error) {
			panic("a bug")
		}},
		{Name: "wait", Description: "Waits.", Call: func(ctx context.Context, _ json.RawMessage) (Result, error) {
			started <- struct{}{}
			<-ctx.Done()
			cancelled.Store(true)
			return Result{Text: "too late"}, nil
		}},
	}
}

// testServer is a server of testTools, running, in a folder of its own.
type testServer struct {
	*Server
	started   chan struct{}
	cancelled atomic.Bool
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	ts := &testServer{started: make(chan struct{}, 1)}
	ts.Server = &Server{Name: "mixmusic", Title: "MixMusic", Version: "test", Instructions: "Play music.",
		Tools: testTools(ts.started, &ts.cancelled), Dir: t.TempDir()}
	if err := ts.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ts.Stop)
	return ts
}

// client is an assistant, which speaks through Relay as through the
// command it starts.
type client struct {
	t     *testing.T
	in    *io.PipeWriter
	lines chan string
	done  chan error
}

func connect(t *testing.T, dir string) *client {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	c := &client{t: t, in: inW, lines: make(chan string, 100), done: make(chan error, 1)}
	go func() {
		err := Relay(dir, inR, outW)
		outW.Close()
		c.done <- err
	}()
	go func() {
		sc := bufio.NewScanner(outR)
		sc.Buffer(nil, maxMessage)
		for sc.Scan() {
			c.lines <- sc.Text()
		}
		close(c.lines)
	}()
	t.Cleanup(func() { inW.Close() })
	return c
}

func (c *client) send(v any) {
	c.t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		c.t.Fatal(err)
	}
	if _, err := c.in.Write(append(data, '\n')); err != nil {
		c.t.Fatalf("writing %s: %v", data, err)
	}
}

func (c *client) sendLine(line string) {
	c.t.Helper()
	if _, err := c.in.Write([]byte(line + "\n")); err != nil {
		c.t.Fatal(err)
	}
}

// reply is a message the server sent.
type reply struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params map[string]any  `json:"params"`
	Result map[string]any  `json:"result"`
	Error  *struct {
		Code    int            `json:"code"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
	} `json:"error"`
	raw string
}

func (c *client) recv() reply {
	c.t.Helper()
	select {
	case line, ok := <-c.lines:
		if !ok {
			c.t.Fatal("the session ended")
		}
		var r reply
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			c.t.Fatalf("%q: %v", line, err)
		}
		r.raw = line
		return r
	case <-time.After(10 * time.Second):
		c.t.Fatal("no answer in 10 s")
	}
	return reply{}
}

// call sends a request and returns its answer.
func (c *client) call(id int, method string, params any) reply {
	c.t.Helper()
	msg := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	c.send(msg)
	r := c.recv()
	if string(r.ID) != strings.TrimSpace(mustJSON(id)) {
		c.t.Fatalf("%s: answer %s to another request", method, r.raw)
	}
	return r
}

func mustJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

// modernMeta is the _meta of a request of revision Latest.
func modernMeta() map[string]any {
	return map[string]any{metaVersion: Latest, metaCapabilities: map[string]any{},
		metaClient: map[string]any{"name": "test-client", "title": "Test Assistant", "version": "1"}}
}

func text(t *testing.T, r reply) string {
	t.Helper()
	content, _ := r.Result["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("content %v", r.raw)
	}
	item, _ := content[0].(map[string]any)
	if item["type"] != "text" {
		t.Fatalf("content %v", r.raw)
	}
	s, _ := item["text"].(string)
	return s
}

// TestLegacySession starts with initialize, at the revision the assistant
// asks for or the newest before Latest, lists the tools and calls one,
// with results as revisions before Latest have them.
func TestLegacySession(t *testing.T) {
	ts := newTestServer(t)
	c := connect(t, ts.Dir)
	r := c.call(1, "initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "claude-ai", "title": "Claude", "version": "1"}})
	if r.Error != nil || r.Result["protocolVersion"] != "2025-06-18" || r.Result["instructions"] != "Play music." {
		t.Fatalf("initialize: %s", r.raw)
	}
	info, _ := r.Result["serverInfo"].(map[string]any)
	caps, _ := r.Result["capabilities"].(map[string]any)
	if info["name"] != "mixmusic" || info["title"] != "MixMusic" || caps["tools"] == nil {
		t.Errorf("initialize: %s", r.raw)
	}
	c.send(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	r = c.call(2, "tools/list", nil)
	tools, _ := r.Result["tools"].([]any)
	if len(tools) != 4 || r.Result["resultType"] != nil {
		t.Fatalf("tools/list: %s", r.raw)
	}
	echo, _ := tools[0].(map[string]any)
	notes, _ := echo["annotations"].(map[string]any)
	if echo["name"] != "echo" || echo["inputSchema"] == nil || notes["readOnlyHint"] != true {
		t.Errorf("echo: %v", echo)
	}
	fail, _ := tools[1].(map[string]any)
	notes, _ = fail["annotations"].(map[string]any)
	if notes["readOnlyHint"] != false || notes["destructiveHint"] != false {
		t.Errorf("fail: %v", fail)
	}
	r = c.call(3, "tools/call", map[string]any{"name": "echo", "arguments": map[string]any{"text": "hi"}})
	data, _ := r.Result["structuredContent"].(map[string]any)
	if text(t, r) != "hi" || r.Result["isError"] != false || data["text"] != "hi" {
		t.Errorf("echo: %s", r.raw)
	}
	if got := ts.Sessions(); len(got) != 1 || got[0].Client != "Claude" {
		t.Errorf("sessions %+v, want Claude's", got)
	}
	// Another revision than those it knows: the newest it does.
	c2 := connect(t, ts.Dir)
	r = c2.call(1, "initialize", map[string]any{"protocolVersion": "2099-01-01", "capabilities": map[string]any{}})
	if r.Result["protocolVersion"] != legacy[0] {
		t.Errorf("initialize of an unknown revision: %s", r.raw)
	}
}

// TestModernRequests serves the requests of revision Latest each on its
// own, by their _meta, and refuses those without it or of a revision it
// doesn't speak.
func TestModernRequests(t *testing.T) {
	ts := newTestServer(t)
	c := connect(t, ts.Dir)
	r := c.call(1, "ping", nil)
	if r.Error != nil {
		t.Errorf("ping: %s", r.raw)
	}
	r = c.call(2, "tools/list", nil)
	if r.Error == nil || r.Error.Code != codeInvalidParams {
		t.Errorf("tools/list without _meta nor initialize: %s", r.raw)
	}
	r = c.call(3, "server/discover", map[string]any{"_meta": modernMeta()})
	supported, _ := r.Result["supportedVersions"].([]any)
	server, _ := r.Result["_meta"].(map[string]any)[metaServer].(map[string]any)
	if r.Error != nil || r.Result["resultType"] != "complete" || len(supported) != len(versions) || supported[0] != Latest ||
		r.Result["ttlMs"] == nil || r.Result["cacheScope"] != "public" || server["name"] != "mixmusic" {
		t.Errorf("server/discover: %s", r.raw)
	}
	r = c.call(4, "tools/list", map[string]any{"_meta": modernMeta()})
	if tools, _ := r.Result["tools"].([]any); len(tools) != 4 || r.Result["resultType"] != "complete" || r.Result["ttlMs"] == nil {
		t.Errorf("tools/list: %s", r.raw)
	}
	r = c.call(5, "tools/call", map[string]any{"_meta": modernMeta(), "name": "echo", "arguments": map[string]any{"text": "hey"}})
	if text(t, r) != "hey" || r.Result["resultType"] != "complete" {
		t.Errorf("tools/call: %s", r.raw)
	}
	if got := ts.Sessions(); len(got) != 1 || got[0].Client != "Test Assistant" {
		t.Errorf("sessions %+v", got)
	}
	m := modernMeta()
	m[metaVersion] = "2099-01-01"
	r = c.call(6, "tools/list", map[string]any{"_meta": m})
	if r.Error == nil || r.Error.Code != codeUnsupportedVersion || r.Error.Data["requested"] != "2099-01-01" {
		t.Fatalf("an unknown revision: %s", r.raw)
	}
	if got, _ := r.Error.Data["supported"].([]any); len(got) == 0 || got[0] != Latest {
		t.Errorf("supported: %s", r.raw)
	}
	m = modernMeta()
	delete(m, metaCapabilities)
	r = c.call(7, "tools/list", map[string]any{"_meta": m})
	if r.Error == nil || r.Error.Code != codeInvalidParams {
		t.Errorf("without the client's capabilities: %s", r.raw)
	}
	r = c.call(8, "resources/list", map[string]any{"_meta": modernMeta()})
	if r.Error == nil || r.Error.Code != codeMethodNotFound {
		t.Errorf("resources/list: %s", r.raw)
	}
	r = c.call(9, "initialize", map[string]any{"_meta": modernMeta()})
	if r.Error == nil || r.Error.Code != codeMethodNotFound {
		t.Errorf("initialize of revision Latest: %s", r.raw)
	}
}

// TestBadMessages answers what isn't a request as JSON-RPC says.
func TestBadMessages(t *testing.T) {
	ts := newTestServer(t)
	c := connect(t, ts.Dir)
	for _, tc := range []struct {
		line string
		code int
	}{
		{`{"jsonrpc":"2.0","id":1,`, codeParse},
		{`[{"jsonrpc":"2.0","id":1,"method":"ping"}]`, codeInvalidRequest},
		{`{"jsonrpc":"1.0","id":1,"method":"ping"}`, codeInvalidRequest},
		{`{"jsonrpc":"2.0","id":null,"method":"ping"}`, codeInvalidRequest},
		{`{"jsonrpc":"2.0","id":1.5,"method":"ping"}`, codeInvalidRequest},
	} {
		c.sendLine(tc.line)
		if r := c.recv(); r.Error == nil || r.Error.Code != tc.code {
			t.Errorf("%s: %s, want the error %d", tc.line, r.raw, tc.code)
		}
	}
	// Notifications and answers get no answer: the ping's comes next.
	c.sendLine(`{"jsonrpc":"2.0","method":"notifications/whatever"}`)
	c.sendLine(`{"jsonrpc":"2.0","id":7,"result":{}}`)
	if r := c.call(8, "ping", nil); r.Error != nil {
		t.Errorf("ping: %s", r.raw)
	}
}

// TestToolErrors tells the assistant of an unknown tool as a protocol
// error, and of a tool that fails, of wrong arguments and of a panic in a
// result it reads.
func TestToolErrors(t *testing.T) {
	ts := newTestServer(t)
	c := connect(t, ts.Dir)
	call := func(id int, name string, args any) reply {
		return c.call(id, "tools/call", map[string]any{"_meta": modernMeta(), "name": name, "arguments": args})
	}
	if r := call(1, "nope", nil); r.Error == nil || r.Error.Code != codeInvalidParams || !strings.Contains(r.Error.Message, "nope") {
		t.Errorf("an unknown tool: %s", r.raw)
	}
	for i, tc := range []struct {
		name string
		args any
		want string
	}{
		{"fail", nil, "nothing to play"},
		{"echo", map[string]any{"text": 5}, "the argument text isn't a string"},
		{"echo", map[string]any{"text": "a", "loud": true}, "unknown field"},
		{"panic", nil, "panic failed inside MixMusic"},
	} {
		r := call(10+i, tc.name, tc.args)
		if r.Result["isError"] != true || !strings.Contains(text(t, r), tc.want) {
			t.Errorf("%s %v: %s, want an error that says %q", tc.name, tc.args, r.raw, tc.want)
		}
	}
}

// TestCancel stops a tool the assistant no longer waits for, and sends no
// answer to it.
func TestCancel(t *testing.T) {
	ts := newTestServer(t)
	c := connect(t, ts.Dir)
	c.send(map[string]any{"jsonrpc": "2.0", "id": "slow", "method": "tools/call",
		"params": map[string]any{"_meta": modernMeta(), "name": "wait"}})
	select {
	case <-ts.started:
	case <-time.After(10 * time.Second):
		t.Fatal("the tool didn't start")
	}
	c.send(map[string]any{"jsonrpc": "2.0", "method": "notifications/cancelled", "params": map[string]any{"requestId": "slow"}})
	for deadline := time.Now().Add(10 * time.Second); !ts.cancelled.Load(); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the tool wasn't cancelled")
		}
	}
	if r := c.call(2, "ping", nil); r.Error != nil {
		t.Errorf("after the cancellation, the ping got %s", r.raw)
	}
}

// TestPace refuses the calls past the pace, which the assistant reads.
func TestPace(t *testing.T) {
	ts := newTestServer(t)
	c := connect(t, ts.Dir)
	const n = 3 * callBurst
	for i := range n {
		c.send(map[string]any{"jsonrpc": "2.0", "id": i, "method": "tools/call",
			"params": map[string]any{"_meta": modernMeta(), "name": "echo", "arguments": map[string]any{"text": "x"}}})
	}
	refused := 0
	for range n {
		if r := c.recv(); r.Result["isError"] == true {
			refused++
		}
	}
	if refused == 0 || refused > n-callBurst {
		t.Errorf("%d of %d calls at once refused, want some past the first %d", refused, n, callBurst)
	}
}

// TestStopEndsSessions ends every session as the server stops: a
// subscription gets its last answer, the command ends, and the file that
// tells how to reach the server goes.
func TestStopEndsSessions(t *testing.T) {
	ts := newTestServer(t)
	c := connect(t, ts.Dir)
	c.send(map[string]any{"jsonrpc": "2.0", "id": 4, "method": "subscriptions/listen",
		"params": map[string]any{"_meta": modernMeta(), "notifications": map[string]any{"toolsListChanged": true}}})
	r := c.recv()
	sub, _ := r.Params["_meta"].(map[string]any)
	if r.Method != "notifications/subscriptions/acknowledged" || sub[metaSubscription] != float64(4) {
		t.Fatalf("subscriptions/listen: %s", r.raw)
	}
	other := connect(t, ts.Dir)
	other.call(1, "ping", nil)
	if n := len(ts.Sessions()); n != 2 {
		t.Fatalf("%d sessions, want 2", n)
	}
	ts.Stop()
	if r := c.recv(); string(r.ID) != "4" || r.Result["resultType"] != "complete" {
		t.Errorf("the subscription's last answer: %s", r.raw)
	}
	for _, cl := range []*client{c, other} {
		select {
		case err := <-cl.done:
			if err != nil {
				t.Errorf("the command ended with %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("a session outlived the server")
		}
	}
	if _, err := os.Stat(filepath.Join(ts.Dir, FileName)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the file stays: %v", err)
	}
	if len(ts.Sessions()) != 0 || ts.Running() {
		t.Error("the server runs on")
	}
	// The command finds no server now.
	if err := Relay(ts.Dir, strings.NewReader(""), io.Discard); !errors.Is(err, ErrUnreachable) {
		t.Errorf("Relay with the server stopped: %v", err)
	}
}

// TestAssistantLeaves ends the session as the assistant closes the
// command's input.
func TestAssistantLeaves(t *testing.T) {
	ts := newTestServer(t)
	c := connect(t, ts.Dir)
	c.call(1, "ping", nil)
	c.in.Close()
	select {
	case err := <-c.done:
		if err != nil {
			t.Errorf("the command ended with %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the command didn't end")
	}
	for deadline := time.Now().Add(10 * time.Second); len(ts.Sessions()) > 0; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the session stays: %+v", ts.Sessions())
		}
	}
}

// TestKey lets in only who gives the key of the file, which the user
// alone reads, and changes the key as the server starts again.
func TestKey(t *testing.T) {
	ts := newTestServer(t)
	first, err := readReach(ts.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Key) != 64 {
		t.Errorf("the key %q isn't 256 bits", first.Key)
	}
	// Windows keeps the data folder to the user by its access lists.
	if st, err := os.Stat(filepath.Join(ts.Dir, FileName)); err != nil || runtime.GOOS != "windows" && st.Mode().Perm()&0o077 != 0 {
		t.Errorf("others may read the file: %v", st.Mode())
	}
	wrong := first
	wrong.Key = strings.Repeat("0", 64)
	if err := writeReach(ts.Dir, wrong); err != nil {
		t.Fatal(err)
	}
	if err := Relay(ts.Dir, strings.NewReader(""), io.Discard); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Errorf("Relay with a wrong key: %v", err)
	}
	// Someone who connects without a key gets nothing.
	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(first.Port)))
	if err != nil {
		t.Fatal(err)
	}
	conn.Write([]byte("POST / HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n{}\n"))
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	line, _ := bufio.NewReader(conn).ReadString('\n')
	conn.Close()
	if !strings.Contains(line, `"error"`) || len(ts.Sessions()) != 0 {
		t.Errorf("a request of HTTP got %q", line)
	}
	ts.Stop()
	if err := ts.Start(); err != nil {
		t.Fatal(err)
	}
	second, err := readReach(ts.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if second.Key == first.Key {
		t.Error("the key stays as the server starts again")
	}
}

// TestLoopbackOnly listens on 127.0.0.1, and refuses to listen elsewhere.
func TestLoopbackOnly(t *testing.T) {
	ts := newTestServer(t)
	ts.mu.Lock()
	addr := ts.ln.Addr().(*net.TCPAddr)
	ts.mu.Unlock()
	if !addr.IP.Equal(net.IPv4(127, 0, 0, 1)) {
		t.Errorf("listens on %v", addr)
	}
	s := &Server{Dir: t.TempDir(), Listen: func() (net.Listener, error) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		return everywhere{ln}, err
	}}
	if err := s.Start(); err == nil || s.Running() {
		s.Stop()
		t.Error("it listens on every network")
	}
	if _, err := os.Stat(filepath.Join(s.Dir, FileName)); err == nil {
		t.Error("it wrote the file")
	}
}

// everywhere is a listener of 127.0.0.1 that says it listens on every
// network.
type everywhere struct{ net.Listener }

func (everywhere) Addr() net.Addr { return &net.TCPAddr{IP: net.IPv4zero, Port: 1} }

// TestArgs reads arguments into a struct, none as its zero value.
func TestArgs(t *testing.T) {
	type a struct {
		N int `json:"n"`
	}
	for _, in := range []string{"", "null", " {} "} {
		if v, err := Args[a](json.RawMessage(in)); err != nil || v.N != 0 {
			t.Errorf("%q: %+v, %v", in, v, err)
		}
	}
	if v, err := Args[a](json.RawMessage(`{"n":3}`)); err != nil || v.N != 3 {
		t.Errorf("%+v, %v", v, err)
	}
	if _, err := Args[a](json.RawMessage(`{"n":"3"}`)); err == nil || !strings.Contains(err.Error(), "n isn't a whole number") {
		t.Errorf("a string for a number: %v", err)
	}
}
