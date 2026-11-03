package casttest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"
)

// The namespaces the fake device speaks.
const (
	NSConnection = "urn:x-cast:com.google.cast.tp.connection"
	NSHeartbeat  = "urn:x-cast:com.google.cast.tp.heartbeat"
	NSReceiver   = "urn:x-cast:com.google.cast.receiver"
	NSMedia      = "urn:x-cast:com.google.cast.media"
)

// ReceiverApp is the default media receiver's app ID.
const ReceiverApp = "CC1AD845"

// Message is a message the fake device got.
type Message struct {
	Namespace, Source, Destination string
	// Type is the payload's type, as "LOAD"; Payload is the payload.
	Type    string
	Payload map[string]any
}

// Float returns the payload's number at key, as currentTime.
func (m Message) Float(key string) float64 {
	v, _ := m.Payload[key].(float64)
	return v
}

// Media returns the payload's media of a LOAD.
func (m Message) Media() map[string]any {
	v, _ := m.Payload["media"].(map[string]any)
	return v
}

// Device is a fake cast device. Its methods are safe for concurrent use.
type Device struct {
	ID, Name, Model string

	ln net.Listener

	mu    sync.Mutex
	got   []Message
	more  chan struct{} // closed as a message comes
	conns []*peer
	// app is the receiver's session while it runs.
	app *app
	// level and muted are the volume.
	level float64
	muted bool
	// media is the media loaded, and session the last media session;
	// apps counts the receivers launched.
	media   *media
	session int
	apps    int
	// failLoads fails as many loads to come; silent stops all answers.
	failLoads int
	silent    bool
	// launchLate has the receiver run only once asked again after LAUNCH.
	launchLate bool
	// pings counts the pings the device got.
	pings int
}

type app struct {
	session, transport string
}

type media struct {
	session   int
	url       string
	meta      map[string]any
	state     string // IDLE, PLAYING, PAUSED, BUFFERING
	idle      string // the idle reason
	position  float64
	rate      float64
	duration  float64
	contentID string
}

// peer is a sender's connection.
type peer struct {
	c       net.Conn
	writing sync.Mutex
	// sender is the sender's ID, once it sent a message.
	sender string
}

// New starts a fake device named "Living Room", a Chromecast, with no app
// running and its volume at half, until the test ends.
func New(t testing.TB) *Device {
	t.Helper()
	cert, err := selfSigned()
	if err != nil {
		t.Fatal(err)
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatal(err)
	}
	d := &Device{ID: "0123456789abcdef0123456789abcdef", Name: "Living Room", Model: "Chromecast", ln: ln,
		more: make(chan struct{}), level: 0.5}
	go d.serve()
	t.Cleanup(d.Close)
	return d
}

// Close stops the device, and drops its connections.
func (d *Device) Close() {
	d.ln.Close()
	d.Drop()
}

// Drop drops the connections, as a device that went off the network.
func (d *Device) Drop() {
	d.mu.Lock()
	conns := d.conns
	d.conns = nil
	d.mu.Unlock()
	for _, p := range conns {
		p.c.Close()
	}
}

// Port is the port the device listens at, on 127.0.0.1.
func (d *Device) Port() int { return d.ln.Addr().(*net.TCPAddr).Port }

// Entry returns the device as a Responder announces it.
func (d *Device) Entry() Entry { return Entry{ID: d.ID, Name: d.Name, Model: d.Model, Port: d.Port()} }

// Addr is the device's address.
func (d *Device) Addr() string { return d.ln.Addr().String() }

// Messages returns the messages the device got, but the heartbeat's.
func (d *Device) Messages() []Message {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]Message(nil), d.got...)
}

// Count returns how many messages of type typ the device got.
func (d *Device) Count(typ string) int {
	n := 0
	for _, m := range d.Messages() {
		if m.Type == typ {
			n++
		}
	}
	return n
}

// Wait waits for the nth message (from 1) of type typ, and returns it.
func (d *Device) Wait(t testing.TB, typ string, nth int) Message {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		d.mu.Lock()
		seen := 0
		for _, m := range d.got {
			if m.Type == typ {
				seen++
				if seen == nth {
					d.mu.Unlock()
					return m
				}
			}
		}
		more := d.more
		d.mu.Unlock()
		select {
		case <-more:
		case <-deadline:
			t.Fatalf("the cast device got no %s number %d: it got %v", typ, nth, d.types())
			return Message{}
		}
	}
}

func (d *Device) types() []string {
	var out []string
	for _, m := range d.Messages() {
		out = append(out, m.Type)
	}
	return out
}

// Pings returns how many pings the device got.
func (d *Device) Pings() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.pings
}

// FailLoads has the next n loads fail, as LOAD_FAILED.
func (d *Device) FailLoads(n int) {
	d.mu.Lock()
	d.failLoads = n
	d.mu.Unlock()
}

// LaunchLate has the receiver run only once the sender asks again, after
// it answered LAUNCH, as a device that takes a moment to start it.
func (d *Device) LaunchLate() {
	d.mu.Lock()
	d.launchLate = true
	d.mu.Unlock()
}

// Silence has the device stop answering anything, pings too, as one that
// hangs.
func (d *Device) Silence() {
	d.mu.Lock()
	d.silent = true
	d.mu.Unlock()
}

// Media tells what the device plays: the URL of the media loaded, its
// player's state, its position in seconds, and its metadata.
func (d *Device) Media() (url, state string, position float64, meta map[string]any) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.media == nil {
		return "", "IDLE", 0, nil
	}
	return d.media.url, d.media.state, d.media.position, d.media.meta
}

// Volume returns the device's volume.
func (d *Device) Volume() (level float64, muted bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.level, d.muted
}

// Finish has the media play to its end, as senders hear.
func (d *Device) Finish() {
	d.change(func(m *media) {
		m.state, m.idle, m.position = "IDLE", "FINISHED", m.duration
	})
}

// FailMedia has the media fail as it plays.
func (d *Device) FailMedia() {
	d.change(func(m *media) { m.state, m.idle = "IDLE", "ERROR" })
}

// SetState sets the player's state, as "PAUSED" by the device's remote,
// and its position, as senders hear.
func (d *Device) SetState(state string, position float64) {
	d.change(func(m *media) { m.state, m.position = state, position })
}

// SetDuration has the device tell the media's length, in seconds.
func (d *Device) SetDuration(seconds float64) {
	d.change(func(m *media) { m.duration = seconds })
}

// change changes the media, and tells the senders.
func (d *Device) change(fn func(*media)) {
	d.mu.Lock()
	if d.media == nil {
		d.mu.Unlock()
		return
	}
	fn(d.media)
	status := d.mediaStatus(0)
	transport := d.transport()
	peers := d.peers()
	d.mu.Unlock()
	for _, t := range peers {
		t.p.send(NSMedia, transport, t.sender, status)
	}
}

// target is a sender to tell, at its connection.
type target struct {
	p      *peer
	sender string
}

// peers returns the senders that spoke. d.mu is held.
func (d *Device) peers() []target {
	var out []target
	for _, p := range d.conns {
		if p.sender != "" {
			out = append(out, target{p, p.sender})
		}
	}
	return out
}

// SetVolume sets the device's volume, as its own buttons do, and tells the
// senders.
func (d *Device) SetVolume(level float64, muted bool) {
	d.mu.Lock()
	d.level, d.muted = level, muted
	d.mu.Unlock()
	d.tellReceiver()
}

// StopApp stops the receiver, as another sender or the device does: other
// names the app that runs in its place, or is "" for the idle screen.
func (d *Device) StopApp(other string) {
	d.mu.Lock()
	transport := d.transport()
	d.app, d.media = nil, nil
	peers := d.peers()
	d.mu.Unlock()
	for _, t := range peers {
		if transport != "" {
			t.p.send(NSConnection, transport, t.sender, map[string]any{"type": "CLOSE"})
		}
	}
	if other == "" {
		d.tellReceiver()
		return
	}
	d.mu.Lock()
	status := d.receiverStatus(0)
	status["status"].(map[string]any)["applications"] = []map[string]any{{"appId": "0F5096E8", "displayName": other,
		"sessionId": "other-session", "transportId": "other-transport"}}
	peers = d.peers()
	d.mu.Unlock()
	for _, t := range peers {
		t.p.send(NSReceiver, "receiver-0", t.sender, status)
	}
}

func (d *Device) tellReceiver() {
	d.mu.Lock()
	status := d.receiverStatus(0)
	peers := d.peers()
	d.mu.Unlock()
	for _, t := range peers {
		t.p.send(NSReceiver, "receiver-0", t.sender, status)
	}
}

func (d *Device) transport() string {
	if d.app == nil {
		return ""
	}
	return d.app.transport
}

func (d *Device) serve() {
	for {
		c, err := d.ln.Accept()
		if err != nil {
			return
		}
		p := &peer{c: c}
		d.mu.Lock()
		d.conns = append(d.conns, p)
		d.mu.Unlock()
		go d.handle(p)
	}
}

func (d *Device) handle(p *peer) {
	defer p.c.Close()
	for {
		ns, src, dst, payload, err := readFrame(p.c)
		if err != nil {
			return
		}
		var body map[string]any
		if json.Unmarshal(payload, &body) != nil {
			continue
		}
		typ, _ := body["type"].(string)
		id := 0
		if v, ok := body["requestId"].(float64); ok {
			id = int(v)
		}
		d.mu.Lock()
		p.sender = src
		silent := d.silent
		if ns != NSHeartbeat {
			d.got = append(d.got, Message{Namespace: ns, Source: src, Destination: dst, Type: typ, Payload: body})
			close(d.more)
			d.more = make(chan struct{})
		} else if typ == "PING" {
			d.pings++
		}
		d.mu.Unlock()
		if silent {
			continue
		}
		switch ns {
		case NSHeartbeat:
			if typ == "PING" {
				p.send(NSHeartbeat, dst, src, map[string]any{"type": "PONG"})
			}
		case NSReceiver:
			d.receiver(p, src, typ, id, body)
		case NSMedia:
			d.mediaCommand(p, src, dst, typ, id, body)
		}
	}
}

func (d *Device) receiver(p *peer, src, typ string, id int, body map[string]any) {
	d.mu.Lock()
	switch typ {
	case "LAUNCH":
		if appID, _ := body["appId"].(string); appID != ReceiverApp {
			d.mu.Unlock()
			p.send(NSReceiver, "receiver-0", src, map[string]any{"type": "LAUNCH_ERROR", "requestId": id, "reason": "NOT_FOUND"})
			return
		}
		late := d.launchLate
		d.launchLate = false
		status := d.receiverStatus(id)
		d.apps++
		d.app = &app{session: "session-" + strconv.Itoa(d.apps), transport: "web-" + strconv.Itoa(d.apps)}
		if !late {
			status = d.receiverStatus(id)
		}
		d.mu.Unlock()
		p.send(NSReceiver, "receiver-0", src, status)
		return
	case "STOP":
		d.app, d.media = nil, nil
	case "SET_VOLUME":
		v, _ := body["volume"].(map[string]any)
		if l, ok := v["level"].(float64); ok {
			d.level = l
		}
		if m, ok := v["muted"].(bool); ok {
			d.muted = m
		}
	}
	status, others := d.receiverStatus(id), d.peers()
	unasked := d.receiverStatus(0)
	d.mu.Unlock()
	p.send(NSReceiver, "receiver-0", src, status)
	if typ != "GET_STATUS" {
		for _, t := range others {
			if t.p != p {
				t.p.send(NSReceiver, "receiver-0", t.sender, unasked)
			}
		}
	}
}

func (d *Device) mediaCommand(p *peer, src, dst, typ string, id int, body map[string]any) {
	d.mu.Lock()
	if d.app == nil || dst != d.app.transport {
		d.mu.Unlock()
		p.send(NSMedia, dst, src, map[string]any{"type": "INVALID_REQUEST", "requestId": id, "reason": "INVALID_COMMAND"})
		return
	}
	// The sender hears the answer, and the others the media's status, as
	// it changed.
	reply := func(v map[string]any) {
		var status map[string]any
		if v["type"] == "MEDIA_STATUS" && v["requestId"] != 0 {
			status = d.mediaStatus(0)
		}
		others := d.peers()
		d.mu.Unlock()
		p.send(NSMedia, dst, src, v)
		for _, t := range others {
			if status != nil && t.p != p {
				t.p.send(NSMedia, dst, t.sender, status)
			}
		}
	}
	if typ == "LOAD" {
		if d.failLoads > 0 {
			d.failLoads--
			reply(map[string]any{"type": "LOAD_FAILED", "requestId": id})
			return
		}
		m, _ := body["media"].(map[string]any)
		url, _ := m["contentId"].(string)
		meta, _ := m["metadata"].(map[string]any)
		duration, _ := m["duration"].(float64)
		start, _ := body["currentTime"].(float64)
		state := "PAUSED"
		if auto, _ := body["autoplay"].(bool); auto {
			state = "PLAYING"
		}
		d.session++
		d.media = &media{session: d.session, url: url, meta: meta, state: state, position: start, rate: 1, duration: duration,
			contentID: url}
		reply(d.mediaStatus(id))
		return
	}
	if typ == "GET_STATUS" {
		reply(d.mediaStatus(id))
		return
	}
	session, _ := body["mediaSessionId"].(float64)
	if d.media == nil || int(session) != d.media.session || d.media.state == "IDLE" {
		reply(map[string]any{"type": "INVALID_PLAYER_STATE", "requestId": id})
		return
	}
	switch typ {
	case "PLAY":
		d.media.state = "PLAYING"
	case "PAUSE":
		d.media.state = "PAUSED"
	case "SEEK":
		d.media.position, _ = body["currentTime"].(float64)
	case "STOP":
		d.media.state, d.media.idle = "IDLE", "CANCELLED"
	case "SET_PLAYBACK_RATE":
		d.media.rate, _ = body["playbackRate"].(float64)
	default:
		reply(map[string]any{"type": "INVALID_REQUEST", "requestId": id, "reason": "INVALID_COMMAND"})
		return
	}
	reply(d.mediaStatus(id))
}

// receiverStatus is a RECEIVER_STATUS of the device as it is, as an answer
// to the request id, or 0 to tell it unasked. d.mu is held.
func (d *Device) receiverStatus(id int) map[string]any {
	apps := []map[string]any{}
	if d.app != nil {
		apps = append(apps, map[string]any{"appId": ReceiverApp, "displayName": "Default Media Receiver",
			"sessionId": d.app.session, "transportId": d.app.transport,
			"namespaces": []map[string]string{{"name": NSMedia}}})
	}
	return map[string]any{"type": "RECEIVER_STATUS", "requestId": id, "status": map[string]any{
		"applications": apps,
		"volume":       map[string]any{"level": d.level, "muted": d.muted, "controlType": "attenuation"},
	}}
}

// mediaStatus is a MEDIA_STATUS of the media loaded. d.mu is held.
func (d *Device) mediaStatus(id int) map[string]any {
	status := []map[string]any{}
	if m := d.media; m != nil {
		st := map[string]any{"mediaSessionId": m.session, "playerState": m.state, "currentTime": m.position,
			"playbackRate": m.rate, "media": map[string]any{"contentId": m.contentID, "duration": m.duration}}
		if m.state == "IDLE" {
			st["idleReason"] = m.idle
		}
		status = append(status, st)
	}
	return map[string]any{"type": "MEDIA_STATUS", "requestId": id, "status": status}
}

func (p *peer) send(ns, src, dst string, payload any) {
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	p.writing.Lock()
	defer p.writing.Unlock()
	_ = writeFrame(p.c, ns, src, dst, b)
}

// writeFrame writes a CastMessage of a string payload, framed by its
// length.
func writeFrame(w io.Writer, ns, src, dst string, payload []byte) error {
	var body []byte
	field := func(n int, s []byte) {
		body = append(body, byte(n<<3|2))
		body = binary.AppendUvarint(body, uint64(len(s)))
		body = append(body, s...)
	}
	body = append(body, 1<<3, 0) // protocol_version
	field(2, []byte(src))
	field(3, []byte(dst))
	field(4, []byte(ns))
	body = append(body, 5<<3, 0) // payload_type: STRING
	field(6, payload)
	frame := binary.BigEndian.AppendUint32(nil, uint32(len(body)))
	_, err := w.Write(append(frame, body...))
	return err
}

// readFrame reads a CastMessage framed by its length.
func readFrame(r io.Reader) (ns, src, dst string, payload []byte, err error) {
	var size [4]byte
	if _, err = io.ReadFull(r, size[:]); err != nil {
		return
	}
	body := make([]byte, binary.BigEndian.Uint32(size[:]))
	if _, err = io.ReadFull(r, body); err != nil {
		return
	}
	var required int
	for len(body) > 0 {
		key, n := binary.Uvarint(body)
		if n <= 0 {
			return "", "", "", nil, errors.New("casttest: bad key")
		}
		body = body[n:]
		switch key & 7 {
		case 0:
			_, n := binary.Uvarint(body)
			if n <= 0 {
				return "", "", "", nil, errors.New("casttest: bad varint")
			}
			body = body[n:]
			if key>>3 == 1 || key>>3 == 5 {
				required++
			}
		case 2:
			l, n := binary.Uvarint(body)
			if n <= 0 || int(l) > len(body)-n {
				return "", "", "", nil, errors.New("casttest: bad length")
			}
			v := string(body[n : n+int(l)])
			body = body[n+int(l):]
			switch key >> 3 {
			case 2:
				src = v
			case 3:
				dst = v
			case 4:
				ns = v
			case 6:
				payload = []byte(v)
			}
			if k := key >> 3; k >= 2 && k <= 4 {
				required++
			}
		default:
			return "", "", "", nil, fmt.Errorf("casttest: wire type %d", key&7)
		}
	}
	if required != 5 {
		// A device's parser turns down a message without proto2's required
		// fields.
		return "", "", "", nil, fmt.Errorf("casttest: %d of the 5 required fields", required)
	}
	return ns, src, dst, payload, nil
}

// selfSigned makes a certificate of the device's own, as cast devices
// have.
func selfSigned() (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "fake cast device"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, nil
}
