package browser

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// devtools speaks the DevTools protocol of a browser made from Chromium
// over its pipe (--remote-debugging-pipe): each message is a JSON object
// that a zero byte ends. It talks to the browser itself, never to a page,
// so that nothing runs in the pages the user signs in on.
type devtools struct {
	w io.WriteCloser

	writing sync.Mutex
	mu      sync.Mutex
	next    int
	waiting map[int]chan devtoolsReply
	// broken is closed once the pipe broke, as the browser exited, and
	// err tells why.
	broken chan struct{}
	err    error
}

// devtoolsReply is the browser's answer to a command.
type devtoolsReply struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// errPipeBroken is the error of a command the browser can no longer answer.
var errPipeBroken = errors.New("the browser's DevTools pipe closed")

func newDevtools(r io.Reader, w io.WriteCloser) *devtools {
	d := &devtools{w: w, waiting: map[int]chan devtoolsReply{}, broken: make(chan struct{})}
	go d.read(r)
	return d
}

// read hands each answer to the command that waits for it, until the pipe
// breaks. Events, which have no ID, are none of MixMusic's.
func (d *devtools) read(r io.Reader) {
	br := bufio.NewReaderSize(r, 64<<10)
	for {
		msg, err := br.ReadBytes(0)
		if err != nil {
			d.mu.Lock()
			d.err = errors.Join(errPipeBroken, err)
			d.mu.Unlock()
			close(d.broken)
			return
		}
		var reply devtoolsReply
		if json.Unmarshal(msg[:len(msg)-1], &reply) != nil || reply.ID == 0 {
			continue
		}
		d.mu.Lock()
		ch := d.waiting[reply.ID]
		delete(d.waiting, reply.ID)
		d.mu.Unlock()
		if ch != nil {
			ch <- reply
		}
	}
}

// call sends the command method with params, and reads its answer into
// result, unless result is nil.
func (d *devtools) call(ctx context.Context, method string, params, result any) error {
	select {
	case <-d.broken:
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.err
	default:
	}
	ch := make(chan devtoolsReply, 1)
	d.mu.Lock()
	d.next++
	id := d.next
	d.waiting[id] = ch
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		delete(d.waiting, id)
		d.mu.Unlock()
	}()
	if params == nil {
		params = struct{}{}
	}
	msg, err := json.Marshal(struct {
		ID     int    `json:"id"`
		Method string `json:"method"`
		Params any    `json:"params"`
	}{id, method, params})
	if err != nil {
		return err
	}
	d.writing.Lock()
	_, err = d.w.Write(append(msg, 0))
	d.writing.Unlock()
	if err != nil {
		return errors.Join(errPipeBroken, err)
	}
	select {
	case reply := <-ch:
		if reply.Error != nil {
			return fmt.Errorf("%s: %s", method, reply.Error.Message)
		}
		if result == nil {
			return nil
		}
		return json.Unmarshal(reply.Result, result)
	case <-d.broken:
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// close closes MixMusic's end of the pipe it writes to.
func (d *devtools) close() { d.w.Close() }
