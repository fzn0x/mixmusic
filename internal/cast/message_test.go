package cast

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"testing"
)

// TestMessageBytes writes a CastMessage as protocol buffers have it, the
// required fields that are zero too, and reads it back.
func TestMessageBytes(t *testing.T) {
	m := message{Source: "sender-0", Destination: "receiver-0", Namespace: nsHeartbeat, Payload: []byte(`{"type":"PING"}`)}
	want := "0800" + // protocol_version CASTV2_1_0
		"1208" + hex.EncodeToString([]byte("sender-0")) +
		"1a0a" + hex.EncodeToString([]byte("receiver-0")) +
		"2227" + hex.EncodeToString([]byte(nsHeartbeat)) + // 39 bytes
		"2800" + // payload_type STRING
		"320f" + hex.EncodeToString([]byte(`{"type":"PING"}`))
	if got := hex.EncodeToString(m.marshal()); got != want {
		t.Fatalf("marshal\n got %s\nwant %s", got, want)
	}
	back, err := unmarshal(m.marshal())
	if err != nil || back.Source != m.Source || back.Destination != m.Destination || back.Namespace != m.Namespace ||
		back.Binary || string(back.Payload) != string(m.Payload) {
		t.Fatalf("unmarshal: %+v, %v", back, err)
	}
	bin := message{Source: "a", Destination: "b", Namespace: "c", Binary: true, Payload: []byte{0, 1, 2}}
	if back, err := unmarshal(bin.marshal()); err != nil || !back.Binary || !bytes.Equal(back.Payload, bin.Payload) {
		t.Fatalf("binary: %+v, %v", back, err)
	}
}

// TestMessageUnknownFields skips the fields a newer device may add, of
// every wire type, and turns down a message cut short.
func TestMessageUnknownFields(t *testing.T) {
	b := message{Source: "s", Destination: "d", Namespace: "n", Payload: []byte("{}")}.marshal()
	b = append(b, 8<<3|wireVarint, 0x96, 0x01)         // field 8, varint 150
	b = append(b, 9<<3|wire64, 1, 2, 3, 4, 5, 6, 7, 8) // field 9, fixed64
	b = append(b, 10<<3|wire32, 1, 2, 3, 4)            // field 10, fixed32
	b = append(b, 11<<3|wireBytes, 2, 'h', 'i')        // field 11, bytes
	m, err := unmarshal(b)
	if err != nil || m.Source != "s" || string(m.Payload) != "{}" {
		t.Fatalf("with unknown fields: %+v, %v", m, err)
	}
	if _, err := unmarshal(b[:len(b)-1]); err == nil {
		t.Error("a message cut short read")
	}
}

// TestFrames frames messages by their length, and turns down a frame too
// big for the protocol.
func TestFrames(t *testing.T) {
	var buf bytes.Buffer
	for _, p := range []string{`{"type":"PING"}`, `{"type":"PONG"}`} {
		if err := writeMessage(&buf, message{Source: "s", Destination: "d", Namespace: nsHeartbeat, Payload: []byte(p)}); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []string{`{"type":"PING"}`, `{"type":"PONG"}`} {
		m, err := readMessage(&buf)
		if err != nil || string(m.Payload) != want {
			t.Fatalf("read %q, %v; want %q", m.Payload, err, want)
		}
	}
	if _, err := readMessage(&buf); !errors.Is(err, io.EOF) {
		t.Errorf("after the last frame: %v", err)
	}
	if _, err := readMessage(bytes.NewReader([]byte{0x10, 0, 0, 0})); err == nil {
		t.Error("a frame of 256 MiB read")
	}
}
