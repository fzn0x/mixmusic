//go:build !windows

package app

import "errors"

// userSealer is nil: macOS keeps secrets in the Keychain, and Linux,
// without a Secret Service, in files readable by the user alone.
var userSealer sealer

// unavailable reports whether err means there is no credential store.
func unavailable(err error) bool { return errors.Is(err, errors.ErrUnsupported) }
