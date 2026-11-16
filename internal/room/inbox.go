package room

import (
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// sent is a message as a guest sends it once it proved who it is: of the
// fields of the messages a guest has, and none of the host's, so that a
// guest's message, of a field it has no business with, builds nothing of
// it. A field a guest's message takes is added here too, as in message.
type sent struct {
	Type   string             `json:"type"`
	Seq    int64              `json:"seq,omitempty"`
	Track  *remote.QueueTrack `json:"track,omitempty"`
	Sent   time.Duration      `json:"sent,omitempty"`
	Do     string             `json:"do,omitempty"`
	Port   int                `json:"port,omitempty"`
	Reason string             `json:"reason,omitempty"`
	OK     bool               `json:"ok,omitempty"`
}

func (m sent) message() message {
	return message{Type: m.Type, Seq: m.Seq, Track: m.Track, Sent: m.Sent, Do: m.Do, Port: m.Port, Reason: m.Reason, OK: m.OK}
}

// messages are what c sends, as they come, read on a goroutine of their
// own from the first call on, until c goes quiet for wait or closes: the
// server waits on them and on what its user answers at once. Call it once
// c proved who it is, as its limit is set by then.
func (c *conn) messages(wait time.Duration) <-chan message {
	c.inOnce.Do(func() {
		in := make(chan message)
		c.in = in
		go func() {
			defer close(in)
			for {
				var m sent
				if err := c.f.Receive(&m, wait); err != nil {
					return
				}
				select {
				case in <- m.message():
				case <-c.gone:
					return
				}
			}
		}()
	})
	return c.in
}
