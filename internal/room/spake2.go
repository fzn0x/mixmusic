package room

// SPAKE2 (RFC 9382), of the ciphersuite SPAKE2-P256-SHA256-HKDF-HMAC: a
// guest and the host prove to each other that they know the room's code,
// or its invite link's secret, without sending it, nor anything an
// eavesdropper, or someone who answers in place of either, could try codes
// against offline. Each try checks one code, online: the host counts the
// wrong ones (host.go). The guest is A, the host B.
//
// The low-level point arithmetic of crypto/elliptic, which Go keeps
// though it deprecated it for ECDH and ECDSA, is what SPAKE2 needs: the
// sum of two points. Its scalar multiplication is constant-time, as the
// nistec it runs on.

import (
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math/big"
)

var curve = elliptic.P256()

// The points M and N of RFC 9382, 6, for P-256, compressed as SEC1 does.
const (
	pointMHex = "02886e2f97ace46e55ba9dd7242579f2993b64e16ef3dcab95afd497333d8fa12f"
	pointNHex = "03d8bbd6c639c62937b04d997f38c3770719c629d7014d49a24b4f98baa1292b49"
)

// point is a point of P-256 in affine coordinates.
type point struct{ x, y *big.Int }

var pointM, pointN = mustPoint(pointMHex), mustPoint(pointNHex)

func mustPoint(s string) point {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	x, y := elliptic.UnmarshalCompressed(curve, b)
	if x == nil {
		panic("room: a point of RFC 9382 isn't on P-256")
	}
	return point{x, y}
}

// ErrSpake: the other side's SPAKE2 message isn't a point it may send.
var ErrSpake = errors.New("room: a SPAKE2 message that isn't a point of P-256")

// passwordScalar is w of a password: SHA-512 of it, apart from anything
// else hashed, modulo the order of P-256, 64 bytes for 32 so that the
// modulo doesn't lean, as RFC 9382, 3.2, has it. It isn't hashed by a
// memory-hard function: nothing keeps it, and each try is online.
func passwordScalar(password []byte) *big.Int {
	h := sha512.New()
	h.Write([]byte("MixMusic room password v1"))
	h.Write([]byte{0})
	h.Write(password)
	w := new(big.Int).SetBytes(h.Sum(nil))
	return w.Mod(w, curve.Params().N)
}

// spake is one side of an exchange.
type spake struct {
	// guest is set on A's side.
	guest bool
	// idA and idB are the guest's and the host's identities, as the
	// transcript names them.
	idA, idB []byte
	w        *big.Int
	// own is the side's secret scalar, x or y, and msg its message, pA or
	// pB, uncompressed.
	own *big.Int
	msg []byte
}

// newSpake starts one side of an exchange of the password scalar w,
// between the guest idA and the host idB, with a new secret scalar.
func newSpake(guest bool, w *big.Int, idA, idB []byte) (*spake, error) {
	own, err := rand.Int(rand.Reader, curve.Params().N)
	if err != nil {
		return nil, err
	}
	return spakeOf(guest, w, own, idA, idB)
}

// spakeOf is newSpake with the secret scalar own, as the test vectors
// give it.
func spakeOf(guest bool, w, own *big.Int, idA, idB []byte) (*spake, error) {
	if w.Sign() == 0 {
		return nil, errors.New("room: a password scalar of 0")
	}
	blind := pointN
	if guest {
		blind = pointM
	}
	wx, wy := curve.ScalarMult(blind.x, blind.y, scalarBytes(w))
	ox, oy := curve.ScalarBaseMult(scalarBytes(own))
	x, y := curve.Add(wx, wy, ox, oy)
	if x.Sign() == 0 && y.Sign() == 0 {
		return nil, errors.New("room: a SPAKE2 message at infinity")
	}
	return &spake{guest: guest, idA: idA, idB: idB, w: w, own: own, msg: elliptic.Marshal(curve, x, y)}, nil
}

// scalarBytes is k in the 32 bytes of P-256's scalars.
func scalarBytes(k *big.Int) []byte { return k.FillBytes(make([]byte, 32)) }

// spakeKeys are what an exchange gives both sides: the shared key Ke, and
// the confirmations each sends, cA from the guest and cB from the host.
type spakeKeys struct {
	ke, ca, cb []byte
}

// finish takes the other side's message, and returns the keys of the
// exchange, bound to aad as the confirmation keys are.
func (s *spake) finish(theirs, aad []byte) (spakeKeys, error) {
	tx, ty := elliptic.Unmarshal(curve, theirs)
	if tx == nil {
		return spakeKeys{}, ErrSpake
	}
	// K = own * (theirs - w * their blind): P-256's cofactor is 1.
	blind := pointM
	if s.guest {
		blind = pointN
	}
	n := curve.Params().N
	neg := new(big.Int).Sub(n, s.w)
	bx, by := curve.ScalarMult(blind.x, blind.y, scalarBytes(neg))
	ux, uy := curve.Add(tx, ty, bx, by)
	if ux.Sign() == 0 && uy.Sign() == 0 {
		return spakeKeys{}, ErrSpake
	}
	kx, ky := curve.ScalarMult(ux, uy, scalarBytes(s.own))
	if kx.Sign() == 0 && ky.Sign() == 0 {
		return spakeKeys{}, ErrSpake
	}
	pa, pb := s.msg, theirs
	if !s.guest {
		pa, pb = theirs, s.msg
	}
	tt := transcriptOf(s.idA, s.idB, pa, pb, elliptic.Marshal(curve, kx, ky), scalarBytes(s.w))
	return keysOf(tt, aad)
}

// transcriptOf is TT of RFC 9382, 3.3: each part after its length, in 8
// bytes, little-endian.
func transcriptOf(parts ...[]byte) []byte {
	var tt []byte
	for _, p := range parts {
		tt = binary.LittleEndian.AppendUint64(tt, uint64(len(p)))
		tt = append(tt, p...)
	}
	return tt
}

// keysOf is the key schedule of RFC 9382, 4: Ke || Ka = Hash(TT), KcA ||
// KcB = HKDF(Ka, nil, "ConfirmationKeys" || AAD), and each confirmation
// the HMAC of TT by its key.
func keysOf(tt, aad []byte) (spakeKeys, error) {
	sum := sha256.Sum256(tt)
	ke, ka := sum[:16], sum[16:]
	kc, err := hkdf.Key(sha256.New, ka, nil, "ConfirmationKeys"+string(aad), 32)
	if err != nil {
		return spakeKeys{}, err
	}
	mac := func(key []byte) []byte {
		m := hmac.New(sha256.New, key)
		m.Write(tt)
		return m.Sum(nil)
	}
	return spakeKeys{ke: ke, ca: mac(kc[:16]), cb: mac(kc[16:])}, nil
}
