package room

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"math/big"
	"net/url"
	"strings"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// codeAlphabet is what a room's code is made of: the digits and the
// letters but I, L and O, which read as 1, 1 and 0.
const codeAlphabet = "0123456789ABCDEFGHJKMNPQRSTUVWXYZ"

// codeLength is how many characters a code has: 33^6, about 1.3 billion
// codes.
const codeLength = 6

// secretSize is the size of an invite link's secret, in bytes.
const secretSize = 16

// NewCode returns a new room code, 6 characters from crypto/rand.
func NewCode() (string, error) {
	b := make([]byte, codeLength)
	n := big.NewInt(int64(len(codeAlphabet)))
	for i := range b {
		v, err := rand.Int(rand.Reader, n)
		if err != nil {
			return "", err
		}
		b[i] = codeAlphabet[v.Int64()]
	}
	return string(b), nil
}

// ParseCode reads a code as the user typed it: in either case, with spaces
// or dashes or not, O read as 0 and I and L as 1. It is false for what
// can't be a code.
func ParseCode(s string) (string, bool) {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		switch r {
		case ' ', '-', ' ':
			continue
		case 'O':
			r = '0'
		case 'I', 'L':
			r = '1'
		}
		if r > 0x7f || !strings.ContainsRune(codeAlphabet, r) {
			return "", false
		}
		b.WriteRune(r)
	}
	if b.Len() != codeLength {
		return "", false
	}
	return b.String(), true
}

// ShowCode is a code as the room's page shows it: in two halves.
func ShowCode(code string) string {
	if len(code) != codeLength {
		return code
	}
	return code[:3] + " " + code[3:]
}

// NewSecret returns a new secret for an invite link.
func NewSecret() ([]byte, error) {
	b := make([]byte, secretSize)
	_, err := rand.Read(b)
	return b, err
}

// Invite is what an invite link holds: the room's code, the key of the
// computer that hosts it, which the guest checks it connects to, and a
// secret, which proves the guest has the link as the code does.
type Invite struct {
	Code   string
	Host   remote.Fingerprint
	Secret []byte
}

// linkPrefix is how an invite link starts.
const linkPrefix = "mixmusic://join/"

// Link is the invite link: mixmusic://join/<code>#<key>.<secret>, the key
// in hex and the secret in base64url. What follows # stays in the app
// that opens it: a browser never sends it.
func (in Invite) Link() string {
	return linkPrefix + in.Code + "#" + in.Host.String() + "." + base64.RawURLEncoding.EncodeToString(in.Secret)
}

// ErrNotLink: what was given isn't an invite link.
var ErrNotLink = errors.New("that isn't a link to a room")

// ParseLink reads an invite link.
func ParseLink(link string) (Invite, error) {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || u.Scheme != "mixmusic" || u.Host != "join" {
		return Invite{}, ErrNotLink
	}
	return InviteOf(strings.Trim(u.Path, "/"), u.Fragment)
}

// InviteOf reads an invite link's code and what follows its #.
func InviteOf(code, fragment string) (Invite, error) {
	code, ok := ParseCode(code)
	key, secret, cut := strings.Cut(fragment, ".")
	if !ok || !cut {
		return Invite{}, ErrNotLink
	}
	fp, err := remote.ParseFingerprint(key)
	if err != nil {
		return Invite{}, ErrNotLink
	}
	s, err := base64.RawURLEncoding.DecodeString(secret)
	if err != nil || len(s) != secretSize {
		return Invite{}, ErrNotLink
	}
	return Invite{Code: code, Host: fp, Secret: s}, nil
}
