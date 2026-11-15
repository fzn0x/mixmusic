package room

import (
	"bytes"
	"crypto/elliptic"
	"encoding/hex"
	"math/big"
	"testing"
)

// rfc9382Vectors are the test vectors of RFC 9382, Appendix B, of
// SPAKE2-P256-SHA256-HKDF-HMAC, with no AAD: w, x and y as they give them,
// what each side sends, the shared key Ke and the confirmations.
var rfc9382Vectors = []struct {
	a, b                        string
	w, x, y, pa, pb, ke, ca, cb string
}{
	{
		a: "server", b: "client",
		w:  "2ee57912099d31560b3a44b1184b9b4866e904c49d12ac5042c97dca461b1a5f",
		x:  "43dd0fd7215bdcb482879fca3220c6a968e66d70b1356cac18bb26c84a78d729",
		y:  "dcb60106f276b02606d8ef0a328c02e4b629f84f89786af5befb0bc75b6e66be",
		pa: "04a56fa807caaa53a4d28dbb9853b9815c61a411118a6fe516a8798434751470f9010153ac33d0d5f2047ffdb1a3e42c9b4e6be662766e1eeb4116988ede5f912c",
		pb: "0406557e482bd03097ad0cbaa5df82115460d951e3451962f1eaf4367a420676d09857ccbc522686c83d1852abfa8ed6e4a1155cf8f1543ceca528afb591a1e0b7",
		ke: "0e0672dc86f8e45565d338b0540abe69",
		ca: "58ad4aa88e0b60d5061eb6b5dd93e80d9c4f00d127c65b3b35b1b5281fee38f0",
		cb: "d3e2e547f1ae04f2dbdbf0fc4b79f8ecff2dff314b5d32fe9fcef2fb26dc459b",
	},
	{
		a: "", b: "client",
		w:  "0548d8729f730589e579b0475a582c1608138ddf7054b73b5381c7e883e2efae",
		x:  "403abbe3b1b4b9ba17e3032849759d723939a27a27b9d921c500edde18ed654b",
		y:  "903023b6598908936ea7c929bd761af6039577a9c3f9581064187c3049d87065",
		pa: "04a897b769e681c62ac1c2357319a3d363f610839c4477720d24cbe32f5fd85f44fb92ba966578c1b712be6962498834078262caa5b441ecfa9d4a9485720e918a",
		pb: "04e0f816fd1c35e22065d5556215c097e799390d16661c386e0ecc84593974a61b881a8c82327687d0501862970c64565560cb5671f696048050ca66ca5f8cc7fc",
		ke: "642f05c473c2cd79909f9a841e2f30a7",
		ca: "47d29e6666af1b7dd450d571233085d7a9866e4d49d2645e2df975489521232b",
		cb: "3313c5cefc361d27fb16847a91c2a73b766ffa90a4839122a9b70a2f6bd1d6df",
	},
	{
		a: "server", b: "",
		w:  "626e0cdc7b14c9db3e52a0b1b3a768c98e37852d5db30febe0497b14eae8c254",
		x:  "07adb3db6bc623d3399726bfdbfd3d15a58ea776ab8a308b00392621291f9633",
		y:  "b6a4fc8dbb629d4ba51d6f91ed1532cf87adec98f25dd153a75accafafedec16",
		pa: "04f88fb71c99bfffaea370966b7eb99cd4be0ff1a7d335caac4211c4afd855e2e15a873b298503ad8ba1d9cbb9a392d2ba309b48bfd7879aefd0f2cea6009763b0",
		pb: "040c269d6be017dccb15182ac6bfcd9e2a14de019dd587eaf4bdfd353f031101e7cca177f8eb362a6e83e7d5e729c0732e1b528879c086f39ba0f31a9661bd34db",
		ke: "005184ff460da2ce59062c87733c299c",
		ca: "bc9f9bbe99f26d0b2260e6456e05a86196a3307ec6663a18bf6ac825736533b2",
		cb: "c2370e1bf813b086dff0d834e74425a06e6390f48f5411900276dcccc5a297ec",
	},
	{
		a: "", b: "",
		w:  "7bf46c454b4c1b25799527d896508afd5fc62ef4ec59db1efb49113063d70cca",
		x:  "8cef65df64bb2d0f83540c53632de911b5b24b3eab6cc74a97609fd659e95473",
		y:  "d7a66f64074a84652d8d623a92e20c9675c61cb5b4f6a0063e4648a2fdc02d53",
		pa: "04a65b367a3f613cf9f0654b1b28a1e3a8a40387956c8ba6063e8658563890f46ca1ef6a676598889fc28de2950ab8120b79a5ef1ea4c9f44bc98f585634b46d66",
		pb: "04589f13218822710d98d8b2123a079041052d9941b9cf88c6617ddb2fcc0494662eea8ba6b64692dc318250030c6af045cb738bc81ba35b043c3dcb46adf6f58d",
		ke: "fc6374762ba5cf11f4b2caa08b2cd1b9",
		ca: "dfb4db8d48ae5a675963ea5e6c19d98d4ea028d8e898dad96ea19a80ade95dca",
		cb: "d0f0609d1613138d354f7e95f19fb556bf52d751947241e8c7118df5ef0ae175",
	},
}

func unhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestSpakeMatchesRFC9382 runs both sides of each exchange of RFC 9382's
// test vectors: each sends what the RFC has it send, and both get its Ke
// and its confirmations.
func TestSpakeMatchesRFC9382(t *testing.T) {
	for _, v := range rfc9382Vectors {
		w := new(big.Int).SetBytes(unhex(t, v.w))
		x := new(big.Int).SetBytes(unhex(t, v.x))
		y := new(big.Int).SetBytes(unhex(t, v.y))
		a, err := spakeOf(true, w, x, []byte(v.a), []byte(v.b))
		if err != nil {
			t.Fatal(err)
		}
		b, err := spakeOf(false, w, y, []byte(v.a), []byte(v.b))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a.msg, unhex(t, v.pa)) || !bytes.Equal(b.msg, unhex(t, v.pb)) {
			t.Fatalf("A=%q B=%q: pA %x, pB %x", v.a, v.b, a.msg, b.msg)
		}
		ka, err := a.finish(b.msg, nil)
		if err != nil {
			t.Fatal(err)
		}
		kb, err := b.finish(a.msg, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range []spakeKeys{ka, kb} {
			if !bytes.Equal(k.ke, unhex(t, v.ke)) || !bytes.Equal(k.ca, unhex(t, v.ca)) || !bytes.Equal(k.cb, unhex(t, v.cb)) {
				t.Fatalf("A=%q B=%q: Ke %x, cA %x, cB %x", v.a, v.b, k.ke, k.ca, k.cb)
			}
		}
	}
}

// TestSpakeAgreesOnTheCodeAlone exchanges with new scalars: the same code
// gives both sides the same confirmations, another code or other AAD, as
// another TLS connection's, doesn't.
func TestSpakeAgreesOnTheCodeAlone(t *testing.T) {
	exchange := func(guestCode, hostCode, guestAAD, hostAAD string) (spakeKeys, spakeKeys) {
		t.Helper()
		g, err := newSpake(true, passwordScalar([]byte(guestCode)), []byte("guest"), []byte("host"))
		if err != nil {
			t.Fatal(err)
		}
		h, err := newSpake(false, passwordScalar([]byte(hostCode)), []byte("guest"), []byte("host"))
		if err != nil {
			t.Fatal(err)
		}
		kg, err := g.finish(h.msg, []byte(guestAAD))
		if err != nil {
			t.Fatal(err)
		}
		kh, err := h.finish(g.msg, []byte(hostAAD))
		if err != nil {
			t.Fatal(err)
		}
		return kg, kh
	}
	kg, kh := exchange("ABC123", "ABC123", "tls", "tls")
	if !bytes.Equal(kg.ke, kh.ke) || !bytes.Equal(kg.ca, kh.ca) || !bytes.Equal(kg.cb, kh.cb) {
		t.Fatal("the same code gave other keys")
	}
	kg, kh = exchange("ABC124", "ABC123", "tls", "tls")
	if bytes.Equal(kg.ca, kh.ca) || bytes.Equal(kg.cb, kh.cb) {
		t.Fatal("another code gave the same confirmations")
	}
	kg, kh = exchange("ABC123", "ABC123", "tls one", "tls two")
	if bytes.Equal(kg.ca, kh.ca) || bytes.Equal(kg.cb, kh.cb) {
		t.Fatal("other AAD gave the same confirmations")
	}
}

// TestSpakeRefusesWhatIsntAPoint refuses messages that aren't points of
// P-256, and the one that would make K the point at infinity.
func TestSpakeRefusesWhatIsntAPoint(t *testing.T) {
	w := passwordScalar([]byte("ABC123"))
	h, err := newSpake(false, w, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	bad := [][]byte{nil, {4}, bytes.Repeat([]byte{1}, 65), append([]byte{4}, bytes.Repeat([]byte{0xff}, 64)...)}
	// w*M, which a guest that guessed w could send to make pA - w*M
	// nothing.
	wx, wy := curve.ScalarMult(pointM.x, pointM.y, scalarBytes(w))
	bad = append(bad, elliptic.Marshal(curve, wx, wy))
	for _, m := range bad {
		if _, err := h.finish(m, nil); err == nil {
			t.Errorf("%x was taken", m)
		}
	}
}
