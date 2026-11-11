package mcp

import (
	"bufio"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// FileName is the file of the data folder that tells the command how to
// reach the server: its port, and the key it asks for. It is there only
// while the server runs.
const FileName = "mcp.json"

// helloWait is how long a connection may take to give the key.
const helloWait = 5 * time.Second

// maxSessions bounds the assistants connected at once.
const maxSessions = 16

// reach is what FileName holds.
type reach struct {
	Port int    `json:"port"`
	Key  string `json:"key"`
}

// hello is the first line of a connection, which gives the key, and
// welcome the server's answer: OK, or why not.
type (
	hello struct {
		Key string `json:"key"`
	}
	welcome struct {
		OK    bool   `json:"ok,omitempty"`
		Error string `json:"error,omitempty"`
	}
)

// Server serves the tools to the assistants connected. The zero value
// with its fields set is ready: Start has it listen.
type Server struct {
	// Name and Version are the server's, Title its name for people, which
	// assistants show.
	Name, Title, Version string
	// Instructions tell the assistant how to use the tools.
	Instructions string
	// Tools are what assistants may call, in the order tools/list tells
	// them.
	Tools []Tool
	// Dir is the folder FileName goes in.
	Dir string
	// Listen opens the listener: on 127.0.0.1, at a port the system
	// picks, when nil.
	Listen func() (net.Listener, error)
	// Changed is told, from any goroutine, as the server starts and stops,
	// and as assistants connect and leave.
	Changed func()
	// Burst and Every are the pace of the tool calls of a session: Burst
	// at once, then one each Every; 10 and 200 ms when zero.
	Burst int
	Every time.Duration

	mu       sync.Mutex
	ln       net.Listener
	key      string
	sessions map[*session]struct{}
}

// Session is an assistant connected: its name, as it tells it, and since
// when.
type Session struct {
	Client string
	Since  time.Time
}

// Running reports whether the server listens.
func (s *Server) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ln != nil
}

// Start has the server listen on 127.0.0.1, with a new key, and writes
// FileName, unless it listens already.
func (s *Server) Start() error {
	s.mu.Lock()
	if s.ln != nil {
		s.mu.Unlock()
		return nil
	}
	listen := s.Listen
	if listen == nil {
		listen = func() (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") }
	}
	ln, err := listen()
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("MixMusic can't listen for AI assistants on this computer: %w", err)
	}
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok || !addr.IP.IsLoopback() {
		ln.Close()
		s.mu.Unlock()
		return fmt.Errorf("MixMusic listens for AI assistants on 127.0.0.1 alone, not %s", ln.Addr())
	}
	key, err := newKey()
	if err == nil {
		err = writeReach(s.Dir, reach{Port: addr.Port, Key: key})
	}
	if err != nil {
		ln.Close()
		s.mu.Unlock()
		return fmt.Errorf("MixMusic can't tell AI assistants how to reach it: %w", err)
	}
	s.ln, s.key = ln, key
	s.sessions = map[*session]struct{}{}
	s.mu.Unlock()
	go s.accept(ln, key)
	s.changed()
	return nil
}

// Stop stops listening, ends every session at once, and removes FileName.
func (s *Server) Stop() {
	s.mu.Lock()
	ln, sessions := s.ln, s.sessions
	s.ln, s.key, s.sessions = nil, "", nil
	s.mu.Unlock()
	if ln == nil {
		return
	}
	ln.Close()
	Forget(s.Dir)
	// Side by side, as a command that doesn't read holds the last answer
	// of its subscriptions up to endWait.
	var wg sync.WaitGroup
	for ss := range sessions {
		wg.Go(ss.end)
	}
	wg.Wait()
	s.changed()
}

// Forget removes FileName from dir, which a MixMusic that didn't stop its
// server, as one that crashed, left.
func Forget(dir string) {
	// One that can't go names a port where no server listens, or another
	// key, which the command tells as such.
	_ = os.Remove(filepath.Join(dir, FileName))
}

// Sessions returns the assistants connected, the first to connect first.
func (s *Server) Sessions() []Session {
	s.mu.Lock()
	var out []Session
	for ss := range s.sessions {
		out = append(out, ss.info())
	}
	s.mu.Unlock()
	slices.SortFunc(out, func(a, b Session) int { return a.Since.Compare(b.Since) })
	return out
}

func (s *Server) changed() {
	if s.Changed != nil {
		s.Changed()
	}
}

// accept takes the connections of ln, which gives way as the server stops.
func (s *Server) accept(ln net.Listener, key string) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go s.greet(conn, key)
	}
}

// greet reads the key a connection gives, and serves it as a session when
// it is the server's.
func (s *Server) greet(conn net.Conn, key string) {
	_ = conn.SetReadDeadline(time.Now().Add(helloWait))
	r := bufio.NewReaderSize(conn, 4096)
	line, err := r.ReadSlice('\n')
	var h hello
	if err != nil || json.Unmarshal(line, &h) != nil ||
		subtle.ConstantTimeCompare([]byte(h.Key), []byte(key)) != 1 {
		answer(conn, welcome{Error: "the key isn't MixMusic's: turn AI assistants off and on again in MixMusic's Settings › Desktop, and start the command again"})
		conn.Close()
		return
	}
	_ = conn.SetReadDeadline(time.Time{})
	ss := newSession(s, conn, r)
	s.mu.Lock()
	switch {
	case s.key != key:
		s.mu.Unlock()
		answer(conn, welcome{Error: "AI assistants are off in MixMusic's Settings › Desktop"})
		conn.Close()
		return
	case len(s.sessions) >= maxSessions:
		s.mu.Unlock()
		answer(conn, welcome{Error: fmt.Sprintf("%d assistants are connected to MixMusic already", maxSessions)})
		conn.Close()
		return
	}
	s.sessions[ss] = struct{}{}
	s.mu.Unlock()
	if !answer(conn, welcome{OK: true}) {
		s.drop(ss)
		ss.end()
		return
	}
	s.changed()
	ss.run()
	s.drop(ss)
	s.changed()
}

// drop forgets a session that ended.
func (s *Server) drop(ss *session) {
	s.mu.Lock()
	delete(s.sessions, ss)
	s.mu.Unlock()
}

// answer writes a line of the greeting, and reports whether it went.
func answer(conn net.Conn, w welcome) bool {
	data, _ := json.Marshal(w)
	_ = conn.SetWriteDeadline(time.Now().Add(helloWait))
	_, err := conn.Write(append(data, '\n'))
	_ = conn.SetWriteDeadline(time.Time{})
	return err == nil
}

// newKey is a key no one can guess: 256 random bits.
func newKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// writeReach writes FileName in dir, which the user alone may read, in
// place of the one there.
func writeReach(dir string, r reach) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, FileName+".*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp, 0o600)
	}
	if err == nil {
		err = os.Rename(tmp, filepath.Join(dir, FileName))
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// readReach reads FileName in dir.
func readReach(dir string) (reach, error) {
	var r reach
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return r, err
	}
	if r.Port <= 0 || r.Port > 65535 || r.Key == "" {
		return r, errors.New("it names no port or no key")
	}
	return r, nil
}
