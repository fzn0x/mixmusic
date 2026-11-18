// Package upnptest has a fake DLNA media renderer for tests: an HTTP
// server on the loopback address that serves its description, and its
// AVTransport, RenderingControl and ConnectionManager actions over SOAP,
// as a renderer does, keeping the actions it gets; and an SSDP responder
// on the loopback address that answers a search for it. It reads and
// writes its XML by itself, apart from package upnp, so that a test checks
// one against the other. Nothing of it reaches the network.
package upnptest

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Options configure a renderer. Their zero value is a renderer of Opus in
// WebM and AAC in MP4, with gapless playback and one speed.
type Options struct {
	// Name and Model are its friendlyName and modelName; UDN its ID.
	Name, Model, UDN string
	// Sink is what GetProtocolInfo tells it plays, comma-separated.
	Sink string
	// NoNext has it turn SetNextAVTransportURI down, as a renderer
	// without gapless playback does.
	NoNext bool
	// Speeds has it play at other speeds than 1.
	Speeds bool
	// NoFetch has it play without fetching the media.
	NoFetch bool
}

// Action is an action the renderer got: its service, name and arguments.
type Action struct {
	Service, Name string
	Args          map[string]string
}

// Fetch is a request of media the renderer made, as Play has it fetch the
// media: what it fetched, how it was answered, with which headers.
type Fetch struct {
	URL    string
	Status int
	Header http.Header
}

// Renderer is a fake media renderer.
type Renderer struct {
	t    *testing.T
	opts Options
	srv  *httptest.Server

	mu      sync.Mutex
	actions []Action
	fetches []Fetch
	state   string
	status  string
	uri     string
	meta    string
	next    string
	pos     time.Duration
	posAt   time.Time
	length  time.Duration
	volume  int
	muted   bool
	speed   string
	broken  bool
}

// renderers counts the renderers made, which their UDNs tell apart.
var renderers atomic.Int64

// New starts a renderer, which stops as the test ends.
func New(t *testing.T, opts Options) *Renderer {
	t.Helper()
	if opts.Name == "" {
		opts.Name = "Living Room TV"
	}
	if opts.UDN == "" {
		opts.UDN = fmt.Sprintf("uuid:5a6b7c8d-0000-1111-2222-%012d", renderers.Add(1))
	}
	if opts.Sink == "" {
		opts.Sink = "http-get:*:audio/mpeg:*,http-get:*:audio/mp4:*,http-get:*:audio/webm:*"
	}
	r := &Renderer{t: t, opts: opts, state: "NO_MEDIA_PRESENT", status: "OK", volume: 40, speed: "1"}
	r.srv = httptest.NewServer(http.HandlerFunc(r.serve))
	t.Cleanup(r.srv.Close)
	return r
}

// Location is the address of its description.
func (r *Renderer) Location() string { return r.srv.URL + "/description.xml" }

// UDN is its ID.
func (r *Renderer) UDN() string { return r.opts.UDN }

// Close stops it answering, as a renderer switched off.
func (r *Renderer) Close() { r.srv.Close() }

// Break has its actions fail, or work again.
func (r *Renderer) Break(broken bool) {
	r.mu.Lock()
	r.broken = broken
	r.mu.Unlock()
}

func (r *Renderer) serve(w http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/description.xml":
		w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
		fmt.Fprintf(w, description, xmlText(r.opts.Name), xmlText(r.opts.Model), r.opts.UDN)
	case "/AVTransport/control", "/RenderingControl/control", "/ConnectionManager/control":
		r.control(w, req)
	default:
		http.NotFound(w, req)
	}
}

const description = `<?xml version="1.0"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
<specVersion><major>1</major><minor>0</minor></specVersion>
<device>
<deviceType>urn:schemas-upnp-org:device:MediaRenderer:1</deviceType>
<friendlyName>%s</friendlyName>
<manufacturer>Test</manufacturer>
<modelName>%s</modelName>
<UDN>%s</UDN>
<serviceList>
<service><serviceType>urn:schemas-upnp-org:service:RenderingControl:1</serviceType><serviceId>urn:upnp-org:serviceId:RenderingControl</serviceId><SCPDURL>/RenderingControl/scpd.xml</SCPDURL><controlURL>/RenderingControl/control</controlURL><eventSubURL>/RenderingControl/event</eventSubURL></service>
<service><serviceType>urn:schemas-upnp-org:service:ConnectionManager:1</serviceType><serviceId>urn:upnp-org:serviceId:ConnectionManager</serviceId><SCPDURL>/ConnectionManager/scpd.xml</SCPDURL><controlURL>/ConnectionManager/control</controlURL><eventSubURL>/ConnectionManager/event</eventSubURL></service>
<service><serviceType>urn:schemas-upnp-org:service:AVTransport:2</serviceType><serviceId>urn:upnp-org:serviceId:AVTransport</serviceId><SCPDURL>/AVTransport/scpd.xml</SCPDURL><controlURL>AVTransport/control</controlURL><eventSubURL>/AVTransport/event</eventSubURL></service>
</serviceList>
</device>
</root>`

func xmlText(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

// control answers an action of a service.
func (r *Renderer) control(w http.ResponseWriter, req *http.Request) {
	soapAction := strings.Trim(req.Header.Get("SOAPAction"), `"`)
	typ, name, ok := strings.Cut(soapAction, "#")
	body, _ := io.ReadAll(req.Body)
	args, err := readArgs(body, name)
	if !ok || err != nil || req.Method != http.MethodPost {
		r.t.Errorf("upnptest: a bad action %q: %v", soapAction, err)
		fault(w, 401, "Invalid Action")
		return
	}
	service := strings.TrimPrefix(strings.TrimSuffix(req.URL.Path, "/control"), "/")
	if !strings.HasPrefix(typ, "urn:schemas-upnp-org:service:"+service+":") {
		r.t.Errorf("upnptest: %s sent to %s", typ, service)
	}
	r.mu.Lock()
	r.actions = append(r.actions, Action{Service: service, Name: name, Args: args})
	broken := r.broken
	r.mu.Unlock()
	if broken {
		fault(w, 501, "Action Failed")
		return
	}
	out, code := r.act(name, args)
	if code != 0 {
		fault(w, code, "refused")
		return
	}
	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body>`)
	b.WriteString(`<u:` + name + `Response xmlns:u="` + typ + `">`)
	for i := 0; i+1 < len(out); i += 2 {
		b.WriteString("<" + out[i] + ">" + xmlText(out[i+1]) + "</" + out[i] + ">")
	}
	b.WriteString(`</u:` + name + `Response></s:Body></s:Envelope>`)
	io.WriteString(w, b.String())
}

// readArgs reads the arguments of the action name in a SOAP body.
func readArgs(body []byte, name string) (map[string]string, error) {
	d := xml.NewDecoder(bytes.NewReader(body))
	args := map[string]string{}
	in := false
	var cur string
	var text strings.Builder
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == name {
				in = true
			} else if in {
				cur = t.Name.Local
				text.Reset()
			}
		case xml.CharData:
			if cur != "" {
				text.Write(t)
			}
		case xml.EndElement:
			if in && t.Name.Local == cur {
				args[cur] = text.String()
				cur = ""
			}
		}
	}
	if !in {
		return nil, fmt.Errorf("no %s element", name)
	}
	return args, nil
}

func fault(w http.ResponseWriter, code int, desc string) {
	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><s:Fault>`+
		`<faultcode>s:Client</faultcode><faultstring>UPnPError</faultstring><detail><UPnPError xmlns="urn:schemas-upnp-org:control-1-0">`+
		`<errorCode>%d</errorCode><errorDescription>%s</errorDescription></UPnPError></detail></s:Fault></s:Body></s:Envelope>`, code, desc)
}

// act does an action, and returns its answer's arguments, as name, value
// pairs, or the code of the UPnP error it turns it down with.
func (r *Renderer) act(name string, args map[string]string) ([]string, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.advance()
	switch name {
	case "GetProtocolInfo":
		return []string{"Source", "", "Sink", r.opts.Sink}, 0
	case "GetTransportInfo":
		return []string{"CurrentTransportState", r.state, "CurrentTransportStatus", r.status, "CurrentSpeed", r.speed}, 0
	case "GetPositionInfo":
		return []string{"Track", "1", "TrackDuration", clock(r.length), "TrackMetaData", r.meta, "TrackURI", r.uri,
			"RelTime", clock(r.pos), "AbsTime", clock(r.pos), "RelCount", "0", "AbsCount", "0"}, 0
	case "SetAVTransportURI":
		r.uri, r.meta, r.next = args["CurrentURI"], args["CurrentURIMetaData"], ""
		r.state, r.status, r.pos, r.length = "STOPPED", "OK", 0, durationOf(r.meta)
		return nil, 0
	case "SetNextAVTransportURI":
		if r.opts.NoNext {
			return nil, 401
		}
		r.next = args["NextURI"]
		return nil, 0
	case "Play":
		if args["Speed"] != "1" && !r.opts.Speeds {
			return nil, 717
		}
		r.speed = args["Speed"]
		if r.uri == "" {
			return nil, 701
		}
		if r.state == "STOPPED" && !r.opts.NoFetch {
			r.state = "TRANSITIONING"
			go r.fetch(r.uri)
			return nil, 0
		}
		r.state, r.posAt = "PLAYING", time.Now()
		return nil, 0
	case "Pause":
		if r.state == "PLAYING" {
			r.state = "PAUSED_PLAYBACK"
		}
		return nil, 0
	case "Stop":
		r.state, r.pos = "STOPPED", 0
		return nil, 0
	case "Seek":
		d, ok := parseClock(args["Target"])
		if !ok || args["Unit"] != "REL_TIME" {
			return nil, 710
		}
		r.pos, r.posAt = d, time.Now()
		return nil, 0
	case "GetVolume":
		return []string{"CurrentVolume", strconv.Itoa(r.volume)}, 0
	case "SetVolume":
		v, err := strconv.Atoi(args["DesiredVolume"])
		if err != nil || v < 0 || v > 100 || args["Channel"] != "Master" {
			return nil, 402
		}
		r.volume = v
		return nil, 0
	case "GetMute":
		m := "0"
		if r.muted {
			m = "1"
		}
		return []string{"CurrentMute", m}, 0
	case "SetMute":
		r.muted = args["DesiredMute"] == "1" || args["DesiredMute"] == "true"
		return nil, 0
	}
	return nil, 401
}

// fetch fetches the media at uri, as the renderer plays it, and plays it
// once it came, unless another came meanwhile.
func (r *Renderer) fetch(uri string) {
	req, _ := http.NewRequest(http.MethodGet, uri, nil)
	req.Header.Set("getcontentFeatures.dlna.org", "1")
	f := Fetch{URL: uri}
	res, err := http.DefaultClient.Do(req)
	if err == nil {
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		f.Status, f.Header = res.StatusCode, res.Header
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fetches = append(r.fetches, f)
	if r.uri != uri || r.state != "TRANSITIONING" {
		return
	}
	if err != nil || res.StatusCode != http.StatusOK && res.StatusCode != http.StatusPartialContent {
		r.state, r.status = "STOPPED", "ERROR_OCCURRED"
		return
	}
	r.state, r.posAt = "PLAYING", time.Now()
}

// advance moves the position on while it plays. r.mu is held.
func (r *Renderer) advance() {
	if r.state != "PLAYING" {
		return
	}
	now := time.Now()
	r.pos += now.Sub(r.posAt)
	r.posAt = now
	if r.length > 0 && r.pos > r.length {
		r.pos = r.length
	}
}

// durationOf reads the duration of the res of DIDL-Lite metadata.
func durationOf(meta string) time.Duration {
	_, rest, ok := strings.Cut(meta, `duration="`)
	if !ok {
		return 0
	}
	v, _, _ := strings.Cut(rest, `"`)
	d, _ := parseClock(v)
	return d
}

// Finish plays the media to its end: the renderer moves on to the media
// queued after it, or stops.
func (r *Renderer) Finish() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.next != "" {
		r.uri, r.next, r.pos, r.posAt, r.state = r.next, "", 0, time.Now(), "PLAYING"
		return
	}
	r.pos, r.state = 0, "STOPPED"
}

// TakeOver has the renderer play uri, as another app has it.
func (r *Renderer) TakeOver(uri string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.uri, r.next, r.state, r.pos, r.posAt = uri, "", "PLAYING", 0, time.Now()
}

// SetVolume changes its volume, as its own remote does.
func (r *Renderer) SetVolume(level int, muted bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.volume, r.muted = level, muted
}

// Volume returns its volume, from 0 to 100, and whether it's muted.
func (r *Renderer) Volume() (int, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.volume, r.muted
}

// State returns its transport state, the address it plays, and the one
// queued after it.
func (r *Renderer) State() (state, uri, next string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state, r.uri, r.next
}

// Actions returns the actions it got.
func (r *Renderer) Actions() []Action {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.actions)
}

// Fetches returns the requests of media it made.
func (r *Renderer) Fetches() []Fetch {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.fetches)
}

// Wait waits for the nth action of name, and returns it.
func (r *Renderer) Wait(t *testing.T, name string, n int) Action {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		seen := 0
		for _, a := range r.Actions() {
			if a.Name == name {
				if seen++; seen == n {
					return a
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no %s #%d: %v", name, n, names(r.Actions()))
		}
	}
}

func names(as []Action) []string {
	var out []string
	for _, a := range as {
		out = append(out, a.Name)
	}
	return out
}

// Responder answers SSDP's searches for media renderers, on the loopback
// address, with the locations of its renderers.
type Responder struct {
	conn      *net.UDPConn
	locations []string
}

// NewResponder starts a responder of the descriptions at locations, which
// stops as the test ends.
func NewResponder(t *testing.T, locations ...string) *Responder {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	r := &Responder{conn: conn, locations: locations}
	t.Cleanup(func() { conn.Close() })
	go r.serve()
	return r
}

// Addr is where searches go to it.
func (r *Responder) Addr() string { return r.conn.LocalAddr().String() }

func (r *Responder) serve() {
	buf := make([]byte, 2048)
	for {
		n, from, err := r.conn.ReadFromUDP(buf)
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return
			}
			continue
		}
		msg := string(buf[:n])
		if !strings.HasPrefix(msg, "M-SEARCH * HTTP/1.1\r\n") || !strings.Contains(msg, "\r\nMAN: \"ssdp:discover\"\r\n") ||
			!strings.Contains(msg, "\r\nST: urn:schemas-upnp-org:device:MediaRenderer:1\r\n") {
			continue
		}
		for _, loc := range r.locations {
			reply := "HTTP/1.1 200 OK\r\nCACHE-CONTROL: max-age=1800\r\nEXT:\r\nLOCATION: " + loc +
				"\r\nSERVER: Test/1 UPnP/1.0 Fake/1\r\nST: urn:schemas-upnp-org:device:MediaRenderer:1\r\nUSN: uuid:x::urn:schemas-upnp-org:device:MediaRenderer:1\r\n\r\n"
			r.conn.WriteToUDP([]byte(reply), from)
		}
	}
}

// clock writes d as H:MM:SS.
func clock(d time.Duration) string {
	s := int64(d / time.Second)
	return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
}

// parseClock reads H:MM:SS, with an optional fraction.
func parseClock(v string) (time.Duration, bool) {
	parts := strings.Split(strings.TrimSpace(v), ":")
	if len(parts) != 3 {
		return 0, false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	s, err3 := strconv.ParseFloat(parts[2], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, false
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(s*float64(time.Second)), true
}
