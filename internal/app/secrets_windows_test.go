package app

import (
	"bytes"
	"errors"
	"fmt"
	"syscall"
	"testing"

	"github.com/egoist/mygo"
)

// errInvalidParameter is ERROR_INVALID_PARAMETER, DPAPI's answer to a call
// MixMusic got wrong.
const errInvalidParameter = syscall.Errno(87)

// sealOrSkip seals data with DPAPI, and skips the test where the session
// has no DPAPI keys, as a service's may not.
func sealOrSkip(t *testing.T, data, purpose []byte) []byte {
	t.Helper()
	sealed, err := dpapi{}.protect(data, purpose)
	var errno syscall.Errno
	if errors.As(err, &errno) && errno != errInvalidParameter {
		t.Skipf("DPAPI doesn't seal in this session: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	return sealed
}

// TestDPAPI seals a secret for the user: it opens for its purpose alone,
// and not once damaged.
func TestDPAPI(t *testing.T) {
	purpose := []byte("MixMusic " + tokenSecret)
	sealed := sealOrSkip(t, secret, purpose)
	if bytes.Contains(sealed, []byte("apiKey")) {
		t.Fatal("the secret is readable in what DPAPI sealed")
	}
	got, err := dpapi{}.unprotect(sealed, purpose)
	if err != nil || !bytes.Equal(got, secret) {
		t.Fatalf("unprotect = %q, %v", got, err)
	}
	if _, err := (dpapi{}).unprotect(sealed, []byte("MixMusic "+lastfmSecret)); err == nil {
		t.Error("another purpose opened it")
	}
	damaged := bytes.Clone(sealed)
	damaged[len(damaged)-5] ^= 0xff
	if _, err := (dpapi{}).unprotect(damaged, purpose); err == nil {
		t.Error("a damaged file opened")
	}
	if _, err := (dpapi{}).unprotect(secret, purpose); err == nil {
		t.Error("a plain file opened")
	}
	if _, err := (dpapi{}).protect(nil, purpose); err == nil {
		t.Error("nothing was sealed")
	}
}

// TestUnavailable falls back to the sealed file where the Credential
// Manager isn't available to the session, and only then.
func TestUnavailable(t *testing.T) {
	for err, want := range map[error]bool{
		errNoStore: true,
		fmt.Errorf("mygo: secrets: credential manager: write: %w", errNoLogonSession): true,
		errRefused:             false,
		mygo.ErrSecretNotFound: false,
	} {
		if got := unavailable(err); got != want {
			t.Errorf("unavailable(%v) = %v", err, got)
		}
	}
}

// TestSecretSealedWithDPAPI keeps a secret the Credential Manager refuses
// for the logon session sealed with DPAPI.
func TestSecretSealedWithDPAPI(t *testing.T) {
	sealOrSkip(t, secret, []byte("probe"))
	k := newKeeper()
	refused := fmt.Errorf("mygo: secrets: credential manager: write: %w", errNoLogonSession)
	k.getErr, k.setErr, k.deleteErr = refused, refused, refused
	s := testSecret(t, k, userSealer)
	if err := s.Save(secret); err != nil {
		t.Fatal(err)
	}
	if exists(s.plain) || !exists(s.sealed) {
		t.Errorf("plain %v, sealed %v", exists(s.plain), exists(s.sealed))
	}
	if got := load(t, s); !bytes.Equal(got, secret) {
		t.Errorf("loaded %q", got)
	}
	if err := s.Clear(); err != nil || exists(s.sealed) {
		t.Errorf("clearing: %v, the sealed file left: %v", err, exists(s.sealed))
	}
}
