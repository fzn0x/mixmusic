package remote

// Pairing proves that both computers know the code the one controlled
// shows, bit by bit, as Bluetooth's passkey entry does: in each round,
// each commits to a bit of the code before either shows it, with an HMAC
// keyed by a nonce of its own, of the TLS connection's exported bytes and
// both keys. docs/architecture/remote.md tells why.

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// codeBits is how many bits a code has, one round each: 999,999 fits in 20.
const codeBits = 20

// nonceSize is the size of a commitment's nonce, in bytes.
const nonceSize = 16

// exportLabel is the label of the bytes a pairing's TLS connection
// exports (RFC 8446, 7.5).
const exportLabel = "EXPORTER-MixMusic-pairing"

// The roles a commitment is of.
const (
	roleController = "controller"
	roleHost       = "host"
)

// NewCode returns a new code, 6 digits from crypto/rand.
func NewCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// CodeValue reads a code as the user typed it, with spaces or not: 6
// digits.
func CodeValue(code string) (uint32, bool) {
	code = strings.Map(func(r rune) rune {
		if r == ' ' || r == '-' || r == ' ' {
			return -1
		}
		return r
	}, code)
	if len(code) != 6 {
		return 0, false
	}
	var v uint32
	for _, c := range code {
		if c < '0' || c > '9' {
			return 0, false
		}
		v = v*10 + uint32(c-'0')
	}
	return v, true
}

// transcript is what each commitment of a pairing is bound to: the bytes
// its TLS connection exports, the same on both of its ends and nowhere
// else, and both computers' keys.
type transcript struct {
	exported         []byte
	controller, host Fingerprint
}

// transcriptOf binds a pairing to the TLS connection cs.
func transcriptOf(cs tls.ConnectionState, controller, host Fingerprint) (transcript, error) {
	exported, err := cs.ExportKeyingMaterial(exportLabel, nil, 32)
	if err != nil {
		return transcript{}, err
	}
	return transcript{exported: exported, controller: controller, host: host}, nil
}

// commitment is the HMAC-SHA256 keyed by nonce of role's bit of the code
// in round.
func commitment(nonce []byte, role string, t transcript, round int, bit uint32) []byte {
	m := hmac.New(sha256.New, nonce)
	m.Write([]byte("MixMusic pairing v1 " + role))
	m.Write([]byte{0})
	m.Write(t.exported)
	m.Write(t.controller[:])
	m.Write(t.host[:])
	m.Write([]byte{byte(round), byte(bit)})
	return m.Sum(nil)
}

func newNonce() ([]byte, error) {
	b := make([]byte, nonceSize)
	_, err := rand.Read(b)
	return b, err
}

// pairing runs the rounds of a pairing over w: first waits for the first
// message of the other computer, as long as its user may take to type the
// code; wait for each after.
type pairing struct {
	w           *wire
	t           transcript
	code        uint32
	first, wait time.Duration
}

// controller runs the rounds as the computer that pairs, which commits
// and opens first in each.
func (p pairing) controller() error {
	for i := range codeBits {
		bit := p.code >> i & 1
		nonce, err := newNonce()
		if err != nil {
			return err
		}
		if err := p.w.send(message{Type: "commit", Round: i, MAC: commitment(nonce, roleController, p.t, i, bit)}, p.wait); err != nil {
			return err
		}
		theirs, err := p.w.expect("commit", p.wait)
		if err != nil {
			return err
		}
		if theirs.Round != i || len(theirs.MAC) != sha256.Size {
			return p.fail()
		}
		if err := p.w.send(message{Type: "open", Round: i, Nonce: nonce}, p.wait); err != nil {
			return err
		}
		open, err := p.w.expect("open", p.wait)
		if err != nil {
			return err
		}
		if open.Round != i || len(open.Nonce) != nonceSize || !hmac.Equal(commitment(open.Nonce, roleHost, p.t, i, bit), theirs.MAC) {
			return p.fail()
		}
	}
	return nil
}

// host runs the rounds as the computer paired with, which commits and
// opens second in each, once the other's commitment of the round came, and
// checked.
func (p pairing) host() error {
	for i := range codeBits {
		bit := p.code >> i & 1
		wait := p.wait
		if i == 0 {
			wait = p.first
		}
		theirs, err := p.w.expect("commit", wait)
		if err != nil {
			return err
		}
		if theirs.Round != i || len(theirs.MAC) != sha256.Size {
			return p.fail()
		}
		nonce, err := newNonce()
		if err != nil {
			return err
		}
		if err := p.w.send(message{Type: "commit", Round: i, MAC: commitment(nonce, roleHost, p.t, i, bit)}, p.wait); err != nil {
			return err
		}
		open, err := p.w.expect("open", p.wait)
		if err != nil {
			return err
		}
		if open.Round != i || len(open.Nonce) != nonceSize || !hmac.Equal(commitment(open.Nonce, roleController, p.t, i, bit), theirs.MAC) {
			return p.fail()
		}
		if err := p.w.send(message{Type: "open", Round: i, Nonce: nonce}, p.wait); err != nil {
			return err
		}
	}
	return nil
}

// fail tells the other computer that the code didn't match, and returns
// ErrWrongCode.
func (p pairing) fail() error {
	_ = p.w.send(message{Type: "failed", Reason: reasonCode}, p.wait)
	return ErrWrongCode
}
