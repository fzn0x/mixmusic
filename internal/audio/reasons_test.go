package audio

import (
	"errors"
	"fmt"
	"testing"
)

// TestReason names each failure of a deck, as the decoders wrap it, with
// the words of it, for the windows to say it in their language (#800).
func TestReason(t *testing.T) {
	for _, c := range []struct {
		err          error
		reason, part string
	}{
		{errNetwork, ReasonNetwork, errNetwork.Error()},
		{fmt.Errorf("%w (Vorbis: bad packet)", errDecode), ReasonDecode, errDecode.Error()},
		{fmt.Errorf("%w (an Ogg file of another codec)", errUnsupported), ReasonUnsupported, errUnsupported.Error()},
		{errFile, ReasonFile, errFile.Error()},
		{ErrNoOutput, "", ""},
		{errors.New("the network failed while loading the audio"), "", ""},
	} {
		if reason, part := Reason(c.err); reason != c.reason || part != c.part {
			t.Errorf("Reason(%q) = %q, %q, want %q, %q", c.err, reason, part, c.reason, c.part)
		}
	}
}
