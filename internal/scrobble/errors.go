package scrobble

import "errors"

// ErrKeyRefused is the error of a call the service made with a key it no
// longer accepts: the session or the token the user gave. A service's own
// error tells it through errors.Is. The plays wait until SetSession gives
// another key.
var ErrKeyRefused = errors.New("scrobble: the service refused the key")

// IsRejected reports whether err tells that the service refused the plays
// sent for what they hold, as the Rejected method of an error of err's
// chain says: such plays are not sent again. Any other error but
// ErrKeyRefused keeps them for later.
func IsRejected(err error) bool {
	var r interface{ Rejected() bool }
	return errors.As(err, &r) && r.Rejected()
}
