package room

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// TestNewCodeIsOfTheAlphabet makes codes: 6 characters each, none of I, L
// or O, and all of them read back as they are.
func TestNewCodeIsOfTheAlphabet(t *testing.T) {
	seen := map[string]bool{}
	for range 500 {
		code, err := NewCode()
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != codeLength || strings.ContainsAny(code, "ILO") {
			t.Fatalf("code %q", code)
		}
		if got, ok := ParseCode(code); !ok || got != code {
			t.Fatalf("%q reads as %q, %v", code, got, ok)
		}
		seen[code] = true
	}
	if len(seen) < 495 {
		t.Fatalf("500 codes, of which %d differ", len(seen))
	}
}

// TestParseCodeReadsWhatPeopleType reads codes in either case, with spaces
// and dashes, O as 0 and I and L as 1.
func TestParseCodeReadsWhatPeopleType(t *testing.T) {
	for in, want := range map[string]string{
		"abc123":        "ABC123",
		"ABC 123":       "ABC123",
		"abc-123":       "ABC123",
		" a b c 1 2 3 ": "ABC123",
		"OIL0il":        "011011",
		"xyz9oo":        "XYZ900",
	} {
		if got, ok := ParseCode(in); !ok || got != want {
			t.Errorf("%q reads as %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "ABC12", "ABC1234", "ABC12!", "ÀBC123", "ABC12U3"} {
		if got, ok := ParseCode(bad); ok {
			t.Errorf("%q reads as %q", bad, got)
		}
	}
	if got := ShowCode("ABC123"); got != "ABC 123" {
		t.Errorf("ShowCode = %q", got)
	}
}

// TestInviteLinkRoundTrips writes an invite link and reads it back, and
// refuses links that aren't one.
func TestInviteLinkRoundTrips(t *testing.T) {
	id, _ := remote.NewIdentity()
	secret, _ := NewSecret()
	in := Invite{Code: "ABC123", Host: id.Fingerprint(), Secret: secret}
	link := in.Link()
	if !strings.HasPrefix(link, "mixmusic://join/ABC123#") {
		t.Fatalf("link %q", link)
	}
	got, err := ParseLink(link)
	if err != nil || got.Code != in.Code || got.Host != in.Host || !bytes.Equal(got.Secret, in.Secret) {
		t.Fatalf("%q reads as %+v, %v", link, got, err)
	}
	// Typed, the code may be in lower case.
	if got, err := ParseLink(strings.Replace(link, "ABC123", "abc-123", 1)); err != nil || got.Code != "ABC123" {
		t.Fatalf("lower case: %+v, %v", got, err)
	}
	for _, bad := range []string{
		"", "https://join/ABC123#x", "mixmusic://play/ABC123#" + in.Host.String() + ".AAAA",
		"mixmusic://join/ABC123", "mixmusic://join/ABC123#" + in.Host.String(),
		"mixmusic://join/ABC123#nothex.AAAAAAAAAAAAAAAAAAAAAA",
		"mixmusic://join/ABC123#" + in.Host.String() + ".short",
		"mixmusic://join/ABC12#" + in.Host.String() + ".AAAAAAAAAAAAAAAAAAAAAA",
	} {
		if got, err := ParseLink(bad); err == nil {
			t.Errorf("%q reads as %+v", bad, got)
		}
	}
}
