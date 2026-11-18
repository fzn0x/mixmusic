// Package fcasttest has a fake FCast receiver for tests: a TCP server on
// the loopback address that speaks protocol v3, or v4 over TLS 1.3 with a
// key of its own, as a receiver does, and keeps the messages it gets; and
// an mDNS responder that answers for it. It frames, encodes and checks its
// messages by itself, apart from package fcast, so that a test checks one
// against the other: what it reads it checks as FlatBuffers' verifier
// does. Nothing of it reaches the network.
package fcasttest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/dnssd"
)

// Options configure a receiver. Their zero value speaks v4, of WebM and
// MP4 with Opus and AAC.
type Options struct {
	// Name is its instance name.
	Name string
	// Version is the version it speaks, and announces unless Announce is
	// set: 4 when 0.
	Version, Announce int
	// Containers and Audio are the media it tells it plays, in v4.
	Containers, Audio []string
	// WrongKey has it announce the fingerprint of another key than its
	// own.
	WrongKey bool
	// Silent has it answer nothing once it's connected, Ping either.
	Silent bool
}

// Message is a message the receiver got: its kind, as "Load",
// "PlaybackStateChanged" or v3's "Pause", and what it tells.
type Message struct {
	Kind                                        string
	URL, Container, Title, Image, Artist, Album string
	Start                                       time.Duration
	Speed, Volume                               float64
	State                                       int
	App                                         string
}

// Receiver is a fake FCast receiver.
type Receiver struct {
	t    *testing.T
	opts Options
	ln   net.Listener
	cert tls.Certificate
	fp   string

	mu   sync.Mutex
	msgs []Message
	conn net.Conn
	wmu  sync.Mutex
	url  string
}

// New starts a receiver, which stops as the test ends.
func New(t *testing.T, opts Options) *Receiver {
	t.Helper()
	if opts.Name == "" {
		opts.Name = "Bedroom TV"
	}
	if opts.Version == 0 {
		opts.Version = 4
	}
	if opts.Announce == 0 {
		opts.Announce = opts.Version
	}
	if opts.Containers == nil {
		opts.Containers = []string{"webm", "mp4"}
	}
	if opts.Audio == nil {
		opts.Audio = []string{"opus", "aac"}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	r := &Receiver{t: t, opts: opts, ln: ln}
	r.cert, r.fp = selfSigned(t)
	if opts.WrongKey {
		_, r.fp = selfSigned(t)
	}
	t.Cleanup(r.Close)
	go r.accept()
	return r
}

// selfSigned makes a self-signed certificate, as a receiver's, and the
// fingerprint of its key: the SHA-256 of its SubjectPublicKeyInfo, in
// base64.
func selfSigned(t *testing.T) (tls.Certificate, string) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(c.RawSubjectPublicKeyInfo)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, base64.StdEncoding.EncodeToString(sum[:])
}

// Close stops it.
func (r *Receiver) Close() {
	r.ln.Close()
	r.mu.Lock()
	if r.conn != nil {
		r.conn.Close()
	}
	r.mu.Unlock()
}

// Host and Port are where it listens; Fingerprint the fingerprint it
// announces.
func (r *Receiver) Host() string { return "127.0.0.1" }
func (r *Receiver) Port() int    { return r.ln.Addr().(*net.TCPAddr).Port }
func (r *Receiver) Fingerprint() string {
	return r.fp
}

// Text is its TXT record, as it announces it.
func (r *Receiver) Text() []string {
	return []string{"v=" + strconv.Itoa(r.opts.Announce), "fp=" + r.fp}
}

// Responder answers mDNS's questions about the receiver, on the loopback
// address.
func (r *Receiver) Responder(t *testing.T) string {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	resp := &dnssd.Responder{Service: "_fcast._tcp.local.", Instance: r.opts.Name, Port: r.Port(), Text: r.Text(),
		AddrFor: func(net.IP) net.IP { return net.IPv4(127, 0, 0, 1) }}
	go resp.Serve(conn)
	return conn.LocalAddr().String()
}

func (r *Receiver) accept() {
	for {
		c, err := r.ln.Accept()
		if err != nil {
			return
		}
		go r.serve(c)
	}
}

func (r *Receiver) record(m Message) {
	r.mu.Lock()
	r.msgs = append(r.msgs, m)
	r.mu.Unlock()
}

func writePacket(w io.Writer, op byte, body []byte) error {
	b := binary.LittleEndian.AppendUint32(nil, uint32(1+len(body)))
	b = append(b, op)
	_, err := w.Write(append(b, body...))
	return err
}

func readPacket(rd io.Reader) (byte, []byte, error) {
	var h [4]byte
	if _, err := io.ReadFull(rd, h[:]); err != nil {
		return 0, nil, err
	}
	n := binary.LittleEndian.Uint32(h[:])
	if n == 0 || n > 512<<10 {
		return 0, nil, fmt.Errorf("fcasttest: a packet of %d bytes", n)
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(rd, b); err != nil {
		return 0, nil, err
	}
	return b[0], b[1:], nil
}

// send writes a packet to the sender connected.
func (r *Receiver) send(op byte, body []byte) {
	r.mu.Lock()
	c := r.conn
	r.mu.Unlock()
	if c == nil {
		return
	}
	r.wmu.Lock()
	defer r.wmu.Unlock()
	_ = writePacket(c, op, body)
}

func (r *Receiver) sendJSON(op byte, v any) {
	b, _ := json.Marshal(v)
	r.send(op, b)
}

// sendV4 sends the Message of type typ.
func (r *Receiver) sendV4(typ int, msg fb) { r.send(20, build(fb{0: fu8(typ), 1: msg})) }

func (r *Receiver) serve(c net.Conn) {
	defer c.Close()
	op, body, err := readPacket(c)
	if err != nil || op != 11 {
		r.t.Errorf("fcasttest: the sender began with %d: %v", op, err)
		return
	}
	var v struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		r.t.Errorf("fcasttest: a version of %q", body)
		return
	}
	r.record(Message{Kind: "Version", State: v.Version})
	b, _ := json.Marshal(map[string]int{"version": r.opts.Version})
	if err := writePacket(c, 11, b); err != nil {
		return
	}
	v4 := v.Version >= 4 && r.opts.Version >= 4
	if v4 {
		tc := tls.Server(c, &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{r.cert}})
		if err := tc.Handshake(); err != nil {
			return
		}
		c = tc
	}
	r.mu.Lock()
	r.conn = c
	r.mu.Unlock()
	if !v4 && v.Version >= 3 {
		r.sendJSON(14, map[string]any{"displayName": r.opts.Name, "appName": "Fake", "appVersion": "1"})
	}
	introduced := !v4
	for {
		op, body, err := readPacket(c)
		if err != nil {
			return
		}
		if r.opts.Silent && introduced {
			continue
		}
		introduced = true
		if op == 12 {
			r.send(13, nil)
			continue
		}
		if op == 13 {
			continue
		}
		if v4 {
			if op != 20 {
				r.t.Errorf("fcasttest: a v4 packet of opcode %d", op)
				continue
			}
			r.onV4(body)
			continue
		}
		r.onV3(op, body)
	}
}

// fetch fetches the media, as the receiver plays it, and tells whether it
// came.
func fetch(url string) bool {
	res, err := http.Get(url)
	if err != nil {
		return false
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	return res.StatusCode == http.StatusOK || res.StatusCode == http.StatusPartialContent
}

// The v4 Message types the fake reads and sends.
const (
	tLoad, tProgress, tVolume, tState, tSpeed, tSenderIntro, tReceiverIntro, tStop = 1, 2, 3, 4, 5, 6, 7, 8
	tError                                                                         = 23
)

func (r *Receiver) onV4(body []byte) {
	root, err := readRoot(body)
	if err != nil {
		r.t.Errorf("fcasttest: %v", err)
		return
	}
	typ, msg, err := root.union(0)
	if err != nil {
		r.t.Errorf("fcasttest: %v", err)
		return
	}
	fail := func(err error) bool {
		if err != nil {
			r.t.Errorf("fcasttest: message %d: %v", typ, err)
		}
		return err != nil
	}
	switch typ {
	case tSenderIntro:
		info, ok, err := msg.table(0)
		if fail(err) || !ok {
			r.t.Error("fcasttest: an introduction without device_info")
			return
		}
		app, err := info.str(1)
		if fail(err) {
			return
		}
		r.record(Message{Kind: "SenderIntroduction", App: app})
		r.sendV4(tReceiverIntro, fb{0: fb{0: r.opts.Name, 1: "Fake receiver", 2: "1"},
			1: fb{0: fb{0: []string{"http", "https"}, 1: r.opts.Containers, 3: r.opts.Audio}}})
	case tLoad:
		kind, item, err := msg.union(0)
		if fail(err) || kind != 1 {
			r.t.Errorf("fcasttest: a load of kind %d", kind)
			return
		}
		m := Message{Kind: "Load"}
		var e [6]error
		m.Container, e[0] = item.str(0)
		m.URL, e[1] = item.str(1)
		us, _, e2 := item.micros(2)
		m.Start = time.Duration(us) * time.Microsecond
		m.Speed, _, e[2] = item.f32(4)
		m.Title, e[3] = item.str(6)
		m.Image, e[4] = item.str(7)
		if mt, meta, err := item.union(8); err == nil && mt == 2 {
			m.Artist, _ = meta.str(0)
			m.Album, _ = meta.str(1)
		} else {
			e[5] = err
		}
		for _, err := range append(e[:], e2) {
			if fail(err) {
				return
			}
		}
		if m.Container == "" || m.URL == "" {
			r.t.Error("fcasttest: a MediaItem without its required container and source_url")
		}
		r.record(m)
		r.sendV4(tLoad, fb{0: fu8(1), 1: fb{0: m.Container, 1: m.URL}})
		r.mu.Lock()
		r.url = m.URL
		r.mu.Unlock()
		go func() {
			r.sendV4(tState, fb{0: fu8(1)})
			if !fetch(m.URL) {
				r.sendV4(tError, fb{0: fu8(1)}) // ResourceNotFound
				return
			}
			r.sendV4(tState, fb{0: fu8(2)})
			r.sendV4(tProgress, fb{0: ftime(m.Start.Microseconds()), 1: ftime(200e6)})
		}()
	case tState:
		s, err := msg.u8(0)
		if fail(err) {
			return
		}
		r.record(Message{Kind: "PlaybackStateChanged", State: s})
		r.sendV4(tState, fb{0: fu8(s)})
	case tProgress:
		us, _, err := msg.micros(0)
		if fail(err) {
			return
		}
		r.record(Message{Kind: "ProgressChanged", Start: time.Duration(us) * time.Microsecond})
		r.sendV4(tProgress, fb{0: ftime(us), 1: ftime(200e6)})
	case tVolume:
		v, _, err := msg.f32(0)
		if fail(err) {
			return
		}
		r.record(Message{Kind: "VolumeChanged", Volume: v})
		r.sendV4(tVolume, fb{0: ff32(v)})
	case tSpeed:
		v, _, err := msg.f32(0)
		if fail(err) {
			return
		}
		r.record(Message{Kind: "SpeedChanged", Speed: v})
		r.sendV4(tSpeed, fb{0: ff32(v)})
	case tStop:
		r.record(Message{Kind: "StopPlayback"})
		r.sendV4(tState, fb{0: fu8(0)})
	default:
		r.t.Errorf("fcasttest: a message of type %d", typ)
	}
}

func (r *Receiver) onV3(op byte, body []byte) {
	var m map[string]any
	if len(body) > 0 {
		if err := json.Unmarshal(body, &m); err != nil {
			r.t.Errorf("fcasttest: a v3 body of %q", body)
			return
		}
	}
	str := func(v any) string { s, _ := v.(string); return s }
	num := func(v any) float64 { f, _ := v.(float64); return f }
	switch op {
	case 1: // Play
		msg := Message{Kind: "Play", URL: str(m["url"]), Container: str(m["container"]), Start: time.Duration(num(m["time"]) * float64(time.Second)),
			Speed: num(m["speed"])}
		if meta, ok := m["metadata"].(map[string]any); ok {
			msg.Title, msg.Image = str(meta["title"]), str(meta["thumbnailUrl"])
		}
		r.record(msg)
		r.mu.Lock()
		r.url = msg.URL
		r.mu.Unlock()
		go func() {
			if !fetch(msg.URL) {
				r.sendJSON(9, map[string]string{"message": "couldn't load"})
				return
			}
			r.sendJSON(6, map[string]any{"generationTime": time.Now().UnixMilli(), "state": 1, "time": msg.Start.Seconds(), "duration": 200})
		}()
	case 2:
		r.record(Message{Kind: "Pause"})
		r.sendJSON(6, map[string]any{"generationTime": time.Now().UnixMilli(), "state": 2, "time": 10.0, "duration": 200})
	case 3:
		r.record(Message{Kind: "Resume"})
		r.sendJSON(6, map[string]any{"generationTime": time.Now().UnixMilli(), "state": 1, "time": 10.0, "duration": 200})
	case 4:
		r.record(Message{Kind: "Stop"})
		r.sendJSON(6, map[string]any{"generationTime": time.Now().UnixMilli(), "state": 0})
	case 5:
		r.record(Message{Kind: "Seek", Start: time.Duration(num(m["time"]) * float64(time.Second))})
		r.sendJSON(6, map[string]any{"generationTime": time.Now().UnixMilli(), "state": 1, "time": num(m["time"]), "duration": 200})
	case 8:
		r.record(Message{Kind: "SetVolume", Volume: num(m["volume"])})
		r.sendJSON(7, map[string]any{"generationTime": time.Now().UnixMilli(), "volume": num(m["volume"])})
	case 10:
		r.record(Message{Kind: "SetSpeed", Speed: num(m["speed"])})
	case 14:
		r.record(Message{Kind: "Initial", App: str(m["appName"])})
	case 17:
		ev, _ := m["event"].(map[string]any)
		r.record(Message{Kind: "SubscribeEvent", State: int(num(ev["type"]))})
	default:
		r.t.Errorf("fcasttest: a v3 packet of opcode %d", op)
	}
}

// Finish plays the media to its end.
func (r *Receiver) Finish() {
	if r.opts.Version >= 4 {
		r.sendV4(tState, fb{0: fu8(4)})
		return
	}
	r.sendJSON(19, map[string]any{"generationTime": time.Now().UnixMilli(), "event": map[string]any{"type": 1}})
	r.sendJSON(6, map[string]any{"generationTime": time.Now().UnixMilli(), "state": 0, "time": 200, "duration": 200})
}

// TakeOver loads url, as another app does.
func (r *Receiver) TakeOver(url string) {
	if r.opts.Version >= 4 {
		r.sendV4(tLoad, fb{0: fu8(1), 1: fb{0: "video/mp4", 1: url}})
		return
	}
	r.sendJSON(15, map[string]any{"generationTime": time.Now().UnixMilli(), "playData": map[string]any{"container": "video/mp4", "url": url}})
}

// SetVolume changes its volume, as its own remote does.
func (r *Receiver) SetVolume(v float64) {
	if r.opts.Version >= 4 {
		r.sendV4(tVolume, fb{0: ff32(v)})
		return
	}
	r.sendJSON(7, map[string]any{"generationTime": time.Now().UnixMilli(), "volume": v})
}

// Messages returns the messages it got.
func (r *Receiver) Messages() []Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.msgs)
}

// Wait waits for the nth message of kind, and returns it.
func (r *Receiver) Wait(t *testing.T, kind string, n int) Message {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		seen := 0
		for _, m := range r.Messages() {
			if m.Kind == kind {
				if seen++; seen == n {
					return m
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no %s #%d: %+v", kind, n, r.Messages())
		}
	}
}
