package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"slices"
	"strings"
	"sync"
	"time"
)

// maxMessage bounds a message of an assistant, in bytes.
const maxMessage = 1 << 20

// callWait bounds how long a tool runs.
const callWait = time.Minute

// writeWait is how long the command may take to read a message before
// the session ends, and endWait how long it may take to read the last
// answers of its subscriptions as the session ends.
const (
	writeWait = 10 * time.Second
	endWait   = time.Second
)

// maxCalls bounds the tool calls of a session that run at once.
const maxCalls = 4

// The pace of tool calls a session may make, unless the server sets
// another: a burst of callBurst, then one each callEvery.
const (
	callBurst = 10
	callEvery = 200 * time.Millisecond
)

// The keys of the _meta of requests and results of revision Latest.
const (
	metaVersion      = "io.modelcontextprotocol/protocolVersion"
	metaClient       = "io.modelcontextprotocol/clientInfo"
	metaCapabilities = "io.modelcontextprotocol/clientCapabilities"
	metaServer       = "io.modelcontextprotocol/serverInfo"
	metaSubscription = "io.modelcontextprotocol/subscriptionId"
)

// JSON-RPC's errors, and the protocol's.
const (
	codeParse              = -32700
	codeInvalidRequest     = -32600
	codeMethodNotFound     = -32601
	codeInvalidParams      = -32602
	codeUnsupportedVersion = -32022
)

// listTTL is how long an assistant may keep the list of tools, which
// changes only with MixMusic's version.
const listTTL = time.Hour

// session is an assistant's connection: its messages, one per line.
type session struct {
	srv    *Server
	conn   net.Conn
	r      *bufio.Reader
	ctx    context.Context
	cancel context.CancelFunc
	since  time.Time
	// calls bounds the tool calls that run at once.
	calls chan struct{}

	wmu sync.Mutex // held while a message is written

	mu sync.Mutex
	// version is the revision initialize agreed on, "" before it.
	version string
	// client is the assistant's name, as it tells it.
	client string
	// running are the requests that run beside the session's reading, by
	// their IDs, and subscriptions those of subscriptions/listen, which
	// stay open until the assistant cancels them or the session ends.
	running       map[string]context.CancelFunc
	subscriptions map[string]json.RawMessage
	// burst, every, tokens and filled are the pace of the tool calls.
	burst  float64
	every  time.Duration
	tokens float64
	filled time.Time
	ended  bool
}

func newSession(srv *Server, conn net.Conn, r *bufio.Reader) *session {
	ctx, cancel := context.WithCancel(context.Background())
	ss := &session{srv: srv, conn: conn, r: r, ctx: ctx, cancel: cancel, since: time.Now(),
		calls: make(chan struct{}, maxCalls), running: map[string]context.CancelFunc{},
		subscriptions: map[string]json.RawMessage{}, burst: callBurst, every: callEvery, filled: time.Now()}
	if srv.Burst > 0 {
		ss.burst = float64(srv.Burst)
	}
	if srv.Every > 0 {
		ss.every = srv.Every
	}
	ss.tokens = ss.burst
	return ss
}

func (ss *session) info() Session {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	name := ss.client
	if name == "" {
		name = "An assistant"
	}
	return Session{Client: name, Since: ss.since}
}

// run reads the assistant's messages until it leaves or the session
// ends.
func (ss *session) run() {
	defer ss.end()
	sc := bufio.NewScanner(ss.r)
	sc.Buffer(make([]byte, 0, 64<<10), maxMessage)
	for sc.Scan() {
		line := sc.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		ss.handle(append([]byte(nil), line...))
	}
}

// end ends the session: the subscriptions get their last answer, and the
// connection closes, which the command passes on to the assistant.
func (ss *session) end() {
	ss.mu.Lock()
	if ss.ended {
		ss.mu.Unlock()
		return
	}
	ss.ended = true
	subs := ss.subscriptions
	ss.subscriptions = map[string]json.RawMessage{}
	ss.mu.Unlock()
	ss.cancel()
	for _, id := range subs {
		ss.sendWithin(response{ID: id, Result: map[string]any{"resultType": "complete",
			"_meta": map[string]any{metaSubscription: id}}}, endWait)
	}
	ss.conn.Close()
}

// message is a JSON-RPC message.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
}

// response is a JSON-RPC response, of a result or an error.
type response struct {
	ID     json.RawMessage
	Result any
	Error  *rpcError
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (r response) MarshalJSON() ([]byte, error) {
	id := r.ID
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	if r.Error != nil {
		return json.Marshal(struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Error   *rpcError       `json:"error"`
		}{"2.0", id, r.Error})
	}
	return json.Marshal(struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  any             `json:"result"`
	}{"2.0", id, r.Result})
}

// send writes a message, on a line of its own: encoding/json writes none
// of its own. A command that doesn't read ends the session.
func (ss *session) send(v any) { ss.sendWithin(v, writeWait) }

// sendWithin sends a message, which the command has wait to read.
func (ss *session) sendWithin(v any, wait time.Duration) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("mixmusic: AI assistants: %v", err)
		return
	}
	ss.wmu.Lock()
	defer ss.wmu.Unlock()
	_ = ss.conn.SetWriteDeadline(time.Now().Add(wait))
	if _, err := ss.conn.Write(append(data, '\n')); err != nil {
		go ss.end()
	}
}

func (ss *session) fail(id json.RawMessage, code int, msg string, data any) {
	ss.send(response{ID: id, Error: &rpcError{Code: code, Message: msg, Data: data}})
}

// handle answers a message.
func (ss *session) handle(line []byte) {
	var m message
	if err := json.Unmarshal(line, &m); err != nil {
		var syntax *json.SyntaxError
		if errors.As(err, &syntax) {
			ss.fail(nil, codeParse, "Parse error: "+err.Error(), nil)
		} else {
			// Not an object of JSON-RPC, as a batch, which the protocol
			// no longer has.
			ss.fail(nil, codeInvalidRequest, "Invalid request: a message is one JSON-RPC object", nil)
		}
		return
	}
	if m.JSONRPC != "2.0" {
		ss.fail(validID(m.ID), codeInvalidRequest, `Invalid request: jsonrpc isn't "2.0"`, nil)
		return
	}
	if m.Method == "" {
		if len(m.Result) > 0 || len(m.Error) > 0 {
			return // an answer, though the server asks nothing
		}
		ss.fail(validID(m.ID), codeInvalidRequest, "Invalid request: no method", nil)
		return
	}
	if len(m.ID) == 0 {
		ss.notification(m)
		return
	}
	if validID(m.ID) == nil {
		ss.fail(nil, codeInvalidRequest, "Invalid request: the id is a string or a whole number", nil)
		return
	}
	ss.request(m)
}

// validID is id when it is one a request may have, a string or a whole
// number, else nil.
func validID(id json.RawMessage) json.RawMessage {
	var v any
	if json.Unmarshal(id, &v) != nil {
		return nil
	}
	switch n := v.(type) {
	case string:
		return id
	case float64:
		if n == float64(int64(n)) {
			return id
		}
	}
	return nil
}

// idKey is a request's ID as the session keeps it: its JSON, without
// spaces.
func idKey(id json.RawMessage) string {
	var v any
	if json.Unmarshal(id, &v) != nil {
		return string(id)
	}
	data, _ := json.Marshal(v)
	return string(data)
}

// notification takes what the assistant tells: that it is ready, or that
// it no longer waits for a request.
func (ss *session) notification(m message) {
	switch m.Method {
	case "notifications/initialized":
	case "notifications/cancelled":
		var p struct {
			RequestID json.RawMessage `json:"requestId"`
		}
		if json.Unmarshal(m.Params, &p) != nil || len(p.RequestID) == 0 {
			return
		}
		key := idKey(p.RequestID)
		ss.mu.Lock()
		cancel := ss.running[key]
		delete(ss.running, key)
		delete(ss.subscriptions, key)
		ss.mu.Unlock()
		if cancel != nil {
			cancel()
		}
	}
}

// meta is what a request of revision Latest tells of itself in its _meta.
type meta struct {
	Version      *string         `json:"io.modelcontextprotocol/protocolVersion"`
	Client       *clientInfo     `json:"io.modelcontextprotocol/clientInfo"`
	Capabilities json.RawMessage `json:"io.modelcontextprotocol/clientCapabilities"`
}

type clientInfo struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

// shown is the client's name for people.
func (c *clientInfo) shown() string {
	if c == nil {
		return ""
	}
	name := strings.TrimSpace(c.Title)
	if name == "" {
		name = strings.TrimSpace(c.Name)
	}
	if len(name) > 64 {
		name = name[:64]
	}
	return strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f {
			return -1
		}
		return r
	}, name)
}

// request answers a request, as its revision has it: one of Latest, by
// its _meta, or one of before, by the session's initialize.
func (ss *session) request(m message) {
	var p struct {
		Meta *meta `json:"_meta"`
	}
	if len(m.Params) > 0 && json.Unmarshal(m.Params, &p) != nil {
		ss.fail(m.ID, codeInvalidParams, "Invalid params: params is an object", nil)
		return
	}
	if p.Meta != nil && p.Meta.Version != nil {
		ss.modern(m, *p.Meta)
		return
	}
	if m.Method == "initialize" {
		ss.initialize(m)
		return
	}
	ss.mu.Lock()
	version := ss.version
	ss.mu.Unlock()
	switch {
	case version != "":
		ss.serve(m, version)
	case m.Method == "ping":
		ss.send(response{ID: m.ID, Result: map[string]any{}})
	default:
		ss.fail(m.ID, codeInvalidParams, "Invalid params: a request carries "+metaVersion+" and "+
			metaCapabilities+" in its _meta, or follows initialize", nil)
	}
}

// initialize starts a session of a revision before Latest: the one the
// assistant asks for, or the newest of those.
func (ss *session) initialize(m message) {
	var p struct {
		Version string      `json:"protocolVersion"`
		Client  *clientInfo `json:"clientInfo"`
	}
	if json.Unmarshal(m.Params, &p) != nil {
		ss.fail(m.ID, codeInvalidParams, "Invalid params: initialize names a protocolVersion", nil)
		return
	}
	version := legacy[0]
	if slices.Contains(legacy, p.Version) {
		version = p.Version
	}
	ss.mu.Lock()
	ss.version = version
	if name := p.Client.shown(); name != "" {
		ss.client = name
	}
	ss.mu.Unlock()
	ss.srv.changed()
	result := map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      ss.srv.info(),
	}
	if ss.srv.Instructions != "" {
		result["instructions"] = ss.srv.Instructions
	}
	ss.send(response{ID: m.ID, Result: result})
}

// info is the server's Implementation.
func (s *Server) info() map[string]any {
	info := map[string]any{"name": s.Name, "version": s.Version}
	if s.Title != "" {
		info["title"] = s.Title
	}
	return info
}

// modern answers a request that tells its revision in its _meta: one of
// Latest, served on its own; or one before it, as initialize would have.
func (ss *session) modern(m message, mt meta) {
	version := *mt.Version
	if !slices.Contains(versions, version) {
		ss.fail(m.ID, codeUnsupportedVersion, "Unsupported protocol version",
			map[string]any{"supported": versions, "requested": version})
		return
	}
	var caps map[string]any
	if len(mt.Capabilities) == 0 || json.Unmarshal(mt.Capabilities, &caps) != nil || caps == nil {
		ss.fail(m.ID, codeInvalidParams, "Invalid params: _meta lacks "+metaCapabilities+", an object", nil)
		return
	}
	if name := mt.Client.shown(); name != "" {
		ss.mu.Lock()
		changed := ss.client != name
		ss.client = name
		ss.mu.Unlock()
		if changed {
			ss.srv.changed()
		}
	}
	ss.serve(m, version)
}

// serve answers the methods of every revision.
func (ss *session) serve(m message, version string) {
	modern := version == Latest
	switch m.Method {
	case "ping":
		ss.reply(m.ID, modern, map[string]any{})
	case "server/discover":
		if !modern {
			ss.fail(m.ID, codeMethodNotFound, "Method not found: server/discover", nil)
			return
		}
		result := map[string]any{"supportedVersions": versions,
			"capabilities": map[string]any{"tools": map[string]any{}},
			"ttlMs":        listTTL.Milliseconds(), "cacheScope": "public"}
		if ss.srv.Instructions != "" {
			result["instructions"] = ss.srv.Instructions
		}
		ss.reply(m.ID, modern, result)
	case "tools/list":
		tools := make([]map[string]any, len(ss.srv.Tools))
		for i, t := range ss.srv.Tools {
			tools[i] = t.definition()
		}
		result := map[string]any{"tools": tools}
		if modern {
			result["ttlMs"], result["cacheScope"] = listTTL.Milliseconds(), "public"
		}
		ss.reply(m.ID, modern, result)
	case "tools/call":
		ss.call(m, modern)
	case "subscriptions/listen":
		if !modern {
			ss.fail(m.ID, codeMethodNotFound, "Method not found: subscriptions/listen", nil)
			return
		}
		ss.listen(m)
	default:
		ss.fail(m.ID, codeMethodNotFound, "Method not found: "+m.Method, nil)
	}
}

// reply answers a request with its result: of revision Latest, with the
// fields that revision adds.
func (ss *session) reply(id json.RawMessage, modern bool, result map[string]any) {
	if modern {
		result["resultType"] = "complete"
		result["_meta"] = map[string]any{metaServer: ss.srv.info()}
	}
	ss.send(response{ID: id, Result: result})
}

// listen opens a subscription, which tells nothing, as the tools don't
// change while MixMusic runs: it stays open until the assistant cancels
// it, or the session ends.
func (ss *session) listen(m message) {
	ss.mu.Lock()
	ss.subscriptions[idKey(m.ID)] = m.ID
	ss.mu.Unlock()
	ss.send(map[string]any{"jsonrpc": "2.0", "method": "notifications/subscriptions/acknowledged",
		"params": map[string]any{"_meta": map[string]any{metaSubscription: m.ID}, "notifications": map[string]any{}}})
}

// call runs a tool beside the session's reading, so that a slow one, as a
// search, doesn't hold up the others or a cancellation.
func (ss *session) call(m message, modern bool) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if json.Unmarshal(m.Params, &p) != nil || p.Name == "" {
		ss.fail(m.ID, codeInvalidParams, "Invalid params: tools/call names a tool", nil)
		return
	}
	i := slices.IndexFunc(ss.srv.Tools, func(t Tool) bool { return t.Name == p.Name })
	if i < 0 {
		ss.fail(m.ID, codeInvalidParams, "Unknown tool: "+p.Name, nil)
		return
	}
	tool := ss.srv.Tools[i]
	if !ss.paced() {
		ss.reply(m.ID, modern, toolError(errors.New("the assistant asks too much at once: wait a moment, and try again")))
		return
	}
	key := idKey(m.ID)
	ctx, cancel := context.WithTimeout(ss.ctx, callWait)
	ss.mu.Lock()
	ss.running[key] = cancel
	ss.mu.Unlock()
	go func() {
		defer cancel()
		// A call waits for one of those that run to end, past which the
		// pace holds the assistant back.
		select {
		case ss.calls <- struct{}{}:
			defer func() { <-ss.calls }()
		case <-ctx.Done():
		}
		var (
			res Result
			err = ctx.Err()
		)
		if err == nil {
			res, err = runTool(ctx, tool, p.Arguments)
		}
		ss.mu.Lock()
		_, waited := ss.running[key]
		delete(ss.running, key)
		ss.mu.Unlock()
		if !waited || ss.ctx.Err() != nil {
			return // cancelled: the assistant waits for no answer
		}
		if errors.Is(err, context.DeadlineExceeded) {
			err = fmt.Errorf("%s took longer than %v: try again", tool.Name, callWait)
		}
		if err != nil {
			ss.reply(m.ID, modern, toolError(err))
			return
		}
		result := map[string]any{"content": []map[string]any{{"type": "text", "text": res.Text}}, "isError": false}
		if res.Data != nil {
			result["structuredContent"] = res.Data
		}
		ss.reply(m.ID, modern, result)
	}()
}

// runTool runs tool, and turns a panic into an error, which the assistant
// reads, rather than stop MixMusic.
func runTool(ctx context.Context, tool Tool, args json.RawMessage) (res Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("mixmusic: AI assistants: %s panicked: %v", tool.Name, r)
			err = fmt.Errorf("%s failed inside MixMusic", tool.Name)
		}
	}()
	return tool.Call(ctx, args)
}

// toolError is the result of a tool that failed, which the assistant
// reads.
func toolError(err error) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": err.Error()}}, "isError": true}
}

// paced reports whether a tool call keeps to the pace, and counts it.
func (ss *session) paced() bool {
	now := time.Now()
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.tokens = min(ss.burst, ss.tokens+float64(now.Sub(ss.filled))/float64(ss.every))
	ss.filled = now
	if ss.tokens < 1 {
		return false
	}
	ss.tokens--
	return true
}
