package remote

import (
	"crypto/tls"
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"
)

// TestFramesCarryMessages sends messages of other protocols over Frames,
// as rooms do, and refuses one longer than MaxMessage.
func TestFramesCarryMessages(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	type note struct {
		Type string `json:"type"`
		N    int    `json:"n"`
	}
	go func() { _ = NewFrames(a).Send(note{Type: "ping", N: 7}, time.Second) }()
	var got note
	if err := NewFrames(b).Receive(&got, time.Second); err != nil || got != (note{Type: "ping", N: 7}) {
		t.Fatalf("got %+v, %v", got, err)
	}
	go func() {
		var head [4]byte
		binary.BigEndian.PutUint32(head[:], MaxMessage+1)
		a.Write(head[:])
	}()
	if err := NewFrames(b).Receive(&got, time.Second); err == nil {
		t.Fatal("a message longer than MaxMessage was read")
	}
	if err := NewFrames(b).Receive(&got, 20*time.Millisecond); !errors.Is(err, ErrLost) {
		t.Fatalf("a silence is %v, not ErrLost", err)
	}
}

// TestClientTLSPinsTheServersKey connects with the configurations other
// protocols share: the key pinned answers, another key is ErrNotSame.
func TestClientTLSPinsTheServersKey(t *testing.T) {
	server, _ := NewIdentity()
	client, _ := NewIdentity()
	other, _ := NewIdentity()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				tc := tls.Server(c, server.ServerTLS("mixmusic-test/1"))
				_ = tc.Handshake()
				time.Sleep(50 * time.Millisecond)
				tc.Close()
			}()
		}
	}()
	dial := func(pinned *Fingerprint) (Fingerprint, error) {
		c, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			return Fingerprint{}, err
		}
		defer c.Close()
		var peer Fingerprint
		tc := tls.Client(c, client.ClientTLS("mixmusic-test/1", pinned, &peer))
		err = tc.Handshake()
		if err == nil {
			key, kerr := PeerKey(tc.ConnectionState())
			if kerr != nil || key != server.Fingerprint() {
				t.Errorf("PeerKey is %v, %v", key, kerr)
			}
		}
		return peer, err
	}
	want := server.Fingerprint()
	if peer, err := dial(&want); err != nil || peer != want {
		t.Fatalf("pinned to the server's key: %v, %v", peer, err)
	}
	if peer, err := dial(nil); err != nil || peer != want {
		t.Fatalf("pinned to none: %v, %v", peer, err)
	}
	wrong := other.Fingerprint()
	if _, err := dial(&wrong); !errors.Is(err, ErrNotSame) {
		t.Fatalf("pinned to another key: %v, not ErrNotSame", err)
	}
}

// TestFramesLimitRefusesALongMessageFromItsLength refuses a message longer
// than the limit from its length alone: it reads none of it, so a sender
// that sends only the length gets the refusal at once, not after the wait.
func TestFramesLimitRefusesALongMessageFromItsLength(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	go func() {
		var head [4]byte
		binary.BigEndian.PutUint32(head[:], 64<<10)
		a.Write(head[:])
	}()
	f := NewFrames(b)
	f.Limit(4 << 10)
	start := time.Now()
	var v map[string]any
	if err := f.Receive(&v, 5*time.Second); err == nil || errors.Is(err, ErrLost) {
		t.Fatalf("a message over the limit: %v", err)
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("the refusal took %v: the message was waited for", took)
	}
	// Without a limit, MaxMessage holds, as for remote control.
	f.Limit(0)
	go func() { _ = NewFrames(a).Send(map[string]string{"type": "ping"}, time.Second) }()
	if err := f.Receive(&v, time.Second); err != nil || v["type"] != "ping" {
		t.Fatalf("got %v, %v", v, err)
	}
}
