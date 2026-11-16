package room

import "time"

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
				var m message
				if err := c.f.Receive(&m, wait); err != nil {
					return
				}
				select {
				case in <- m:
				case <-c.gone:
					return
				}
			}
		}()
	})
	return c.in
}
