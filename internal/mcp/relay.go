package mcp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

// Flag is the argument that runs MixMusic as the command an assistant
// starts: MixMusic --mcp.
const Flag = "--mcp"

// ErrUnreachable is why Relay can't reach MixMusic: it doesn't run, or AI
// assistants are off.
var ErrUnreachable = errors.New("MixMusic isn't open, or Let AI assistants control MixMusic is off in its Settings › Desktop")

// dialWait is how long Relay waits for the server to answer.
const dialWait = 5 * time.Second

// Relay connects the assistant that started this process, which speaks
// on in and out, to the server of the MixMusic whose data folder is dir.
// It passes on each side's lines to the other until either closes: the
// assistant, by closing in; MixMusic, by ending the session, as when AI
// assistants are turned off, which ends this process, and the session on
// the assistant's side.
func Relay(dir string, in io.Reader, out io.Writer) error {
	r, err := readReach(dir)
	if err != nil {
		return ErrUnreachable
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(r.Port)), dialWait)
	if err != nil {
		return ErrUnreachable
	}
	defer conn.Close()
	data, _ := json.Marshal(hello{Key: r.Key})
	_ = conn.SetDeadline(time.Now().Add(dialWait))
	if _, err := conn.Write(append(data, '\n')); err != nil {
		return ErrUnreachable
	}
	br := bufio.NewReader(conn)
	line, err := br.ReadSlice('\n')
	var w welcome
	if err != nil || json.Unmarshal(line, &w) != nil {
		// Another program took the port MixMusic left.
		return ErrUnreachable
	}
	if !w.OK {
		return fmt.Errorf("MixMusic refused the assistant: %s", w.Error)
	}
	_ = conn.SetDeadline(time.Time{})
	go func() {
		_, _ = io.Copy(conn, in)
		// The assistant is done: MixMusic ends the session, which ends
		// the copy below.
		if tc, ok := conn.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		} else {
			conn.Close()
		}
	}()
	// The session ends as MixMusic closes the connection, which the
	// system may tell as a reset rather than its end.
	_, _ = io.Copy(out, br)
	return nil
}
