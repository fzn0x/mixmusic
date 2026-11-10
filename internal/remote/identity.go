// Package remote is how MixMusic on one computer controls MixMusic on
// another in the same network (#353): each computer's key, pairing by a
// short code, the encrypted connection, and the messages of a session.
// docs/architecture/remote.md tells why it is so.
package remote

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// An Identity is a computer's key, and the self-signed certificate of it
// that TLS presents. Only the key counts: the certificate is made anew
// each time.
type Identity struct {
	key  ed25519.PrivateKey
	cert tls.Certificate
	fp   Fingerprint
}

// NewIdentity makes a new key.
func NewIdentity() (*Identity, error) {
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}
	return IdentityOf(seed)
}

// IdentityOf is the identity of the key of seed, which Seed returns.
func IdentityOf(seed []byte) (*Identity, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, errors.New("remote: a key's seed is 32 bytes")
	}
	key := ed25519.NewKeyFromSeed(seed)
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return nil, err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "MixMusic"},
		// No one checks the times: a computer is its key.
		NotBefore:   now.Add(-24 * time.Hour),
		NotAfter:    now.Add(20 * 365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		return nil, err
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &Identity{key: key, cert: tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf},
		fp: FingerprintOf(leaf)}, nil
}

// Seed returns the seed the key is made of, to keep it.
func (id *Identity) Seed() []byte { return id.key.Seed() }

// Fingerprint returns the fingerprint of the key.
func (id *Identity) Fingerprint() Fingerprint { return id.fp }

// A Fingerprint is a key's: the SHA-256 of its certificate's
// SubjectPublicKeyInfo.
type Fingerprint [32]byte

// FingerprintOf returns the fingerprint of the key of c.
func FingerprintOf(c *x509.Certificate) Fingerprint { return sha256.Sum256(c.RawSubjectPublicKeyInfo) }

// String is the fingerprint in hex, as remote.json keeps it.
func (f Fingerprint) String() string { return hex.EncodeToString(f[:]) }

// ID is the computer's ID, which discovery tells: the first 80 bits of the
// fingerprint, as 16 letters and digits.
func (f Fingerprint) ID() string {
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(f[:10]))
}

// ParseFingerprint reads a fingerprint String wrote.
func ParseFingerprint(s string) (Fingerprint, error) {
	var f Fingerprint
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != len(f) {
		return f, fmt.Errorf("remote: %q isn't a key's fingerprint", s)
	}
	copy(f[:], b)
	return f, nil
}

// peerKey returns the fingerprint of the certificate a TLS connection's
// other end presented, which must be of an Ed25519 key.
func peerKey(cs tls.ConnectionState) (Fingerprint, error) {
	if len(cs.PeerCertificates) == 0 {
		return Fingerprint{}, errors.New("remote: the other computer presented no certificate")
	}
	c := cs.PeerCertificates[0]
	if _, ok := c.PublicKey.(ed25519.PublicKey); !ok {
		return Fingerprint{}, errors.New("remote: the other computer's key isn't an Ed25519 key")
	}
	return FingerprintOf(c), nil
}
