// Package portmaptest is a router in memory, for tests: it answers PCP
// (RFC 6887) and NAT-PMP (RFC 6886) on a UDP port of the loopback, and UPnP
// IGD's search on another, with its description and its control over HTTP,
// as their specifications have them, and keeps the forwards asked of it.
// Tests ask it, and never a real router.
package portmaptest

import (
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Forward is a forward the router keeps: a TCP port of its internet
// address, External, to the port Internal of the computer Client, as
// Method asked it, for Lifetime, or for good when 0.
type Forward struct {
	Method   string
	External uint16
	Internal uint16
	Client   netip.Addr
	Lifetime time.Duration
}

// Router is the router. Its exported fields say how it answers, and may
// change while it runs.
type Router struct {
	// External is the router's internet address, which it tells.
	External netip.Addr
	// PCP, NATPMP and UPnP say which protocols it answers: one it doesn't
	// is silent, as a router with it off.
	PCP, NATPMP, UPnP bool
	// Permanent has UPnP take only forwards without a lease, as older
	// routers do (error 725).
	Permanent bool
	// Taken are the external ports another computer has: UPnP refuses
	// them (error 718), and PCP and NAT-PMP give another port.
	Taken []uint16
	// Refuse has every protocol refuse the forward, as a router that
	// takes none from this computer.
	Refuse bool

	v4, v6 *net.UDPConn
	ssdp   *net.UDPConn
	web    *httptest.Server
	start  time.Time

	mu       sync.Mutex
	forwards map[string]Forward
	asked    []string
}

// New starts a router of the internet address external, on the loopback,
// which answers every protocol. Close stops it.
func New(external string) (*Router, error) {
	r := &Router{External: netip.MustParseAddr(external), PCP: true, NATPMP: true, UPnP: true, start: time.Now(), forwards: map[string]Forward{}}
	var err error
	if r.v4, err = net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}); err != nil {
		return nil, err
	}
	// The same port on the IPv6 loopback, for PCP's questions of IPv6,
	// where it has one.
	port := r.v4.LocalAddr().(*net.UDPAddr).Port
	r.v6, _ = net.ListenUDP("udp6", &net.UDPAddr{IP: net.IPv6loopback, Port: port})
	if r.ssdp, err = net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}); err != nil {
		r.Close()
		return nil, err
	}
	r.web = httptest.NewServer(http.HandlerFunc(r.serveHTTP))
	go r.servePMP(r.v4)
	if r.v6 != nil {
		go r.servePMP(r.v6)
	}
	go r.serveSSDP()
	return r, nil
}

// Close stops the router.
func (r *Router) Close() {
	for _, c := range []*net.UDPConn{r.v4, r.v6, r.ssdp} {
		if c != nil {
			c.Close()
		}
	}
	if r.web != nil {
		r.web.Close()
	}
}

// Port is the UDP port PCP and NAT-PMP answer on, of 127.0.0.1 and of ::1
// when HasIPv6.
func (r *Router) Port() int { return r.v4.LocalAddr().(*net.UDPAddr).Port }

// HasIPv6 reports whether the router answers PCP on ::1 too.
func (r *Router) HasIPv6() bool { return r.v6 != nil }

// SSDP is the address UPnP's search goes to, in place of its multicast
// group's.
func (r *Router) SSDP() string { return r.ssdp.LocalAddr().String() }

// Forwards returns the forwards the router keeps.
func (r *Router) Forwards() []Forward {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Forward, 0, len(r.forwards))
	for _, f := range r.forwards {
		out = append(out, f)
	}
	return out
}

// Asked returns what was asked of the router, in order: "PCP map", "PCP
// delete", "NAT-PMP address", "NAT-PMP map", "NAT-PMP delete", "UPnP
// search", "UPnP description", and UPnP's actions by their names.
func (r *Router) Asked() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.asked...)
}

func (r *Router) note(what string) {
	r.mu.Lock()
	r.asked = append(r.asked, what)
	r.mu.Unlock()
}

func (r *Router) settings() (external netip.Addr, pcp, pmp, upnp, permanent, refuse bool, taken []uint16) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.External, r.PCP, r.NATPMP, r.UPnP, r.Permanent, r.Refuse, append([]uint16(nil), r.Taken...)
}

// Set changes how the router answers while it runs.
func (r *Router) Set(fn func(r *Router)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fn(r)
}

// key names a forward by its external port.
func key(external uint16) string { return strconv.Itoa(int(external)) }

// place gives the forward of client's internal port a free external port,
// suggested when free, and keeps it for lifetime, or deletes it when 0,
// for PCP and NAT-PMP. It returns the external port.
func (r *Router) place(method string, client netip.Addr, internal, suggested uint16, lifetime time.Duration, taken []uint16) uint16 {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, f := range r.forwards {
		if f.Client == client && f.Internal == internal && f.Method == method {
			if lifetime == 0 {
				delete(r.forwards, k)
				return 0
			}
			f.Lifetime = lifetime
			r.forwards[k] = f
			return f.External
		}
	}
	if lifetime == 0 {
		return 0
	}
	free := func(p uint16) bool {
		for _, t := range taken {
			if t == p {
				return false
			}
		}
		_, used := r.forwards[key(p)]
		return p != 0 && !used
	}
	ext := suggested
	for p := uint16(40000); !free(ext); p++ {
		ext = p
	}
	r.forwards[key(ext)] = Forward{Method: method, External: ext, Internal: internal, Client: client, Lifetime: lifetime}
	return ext
}

func (r *Router) epoch() uint32 { return uint32(time.Since(r.start) / time.Second) }

// servePMP answers PCP and NAT-PMP on c, as a router that speaks both
// does: by the version of each question.
func (r *Router) servePMP(c *net.UDPConn) {
	buf := make([]byte, 1100)
	for {
		n, from, err := c.ReadFromUDPAddrPort(buf)
		if err != nil {
			return
		}
		external, pcp, pmp, _, _, refuse, taken := r.settings()
		req := buf[:n]
		var out []byte
		switch {
		case n >= 2 && req[0] == 0 && pmp:
			out = r.natpmp(req, from, external, refuse, taken)
		case n >= 2 && req[0] == 0 && pcp:
			// A router of PCP alone tells a NAT-PMP client the version it
			// speaks (RFC 6887, 9).
			out = pcpAnswer(req, 1, 0, r.epoch())
		case n >= 2 && req[0] == 2 && pcp:
			out = r.pcp(req, from, external, refuse, taken)
		case n >= 2 && req[0] == 2 && pmp:
			// A router of NAT-PMP alone: Unsupported Version (RFC 6886,
			// 3.5).
			out = binary.BigEndian.AppendUint16([]byte{0, 128 + req[1]&0x7f}, 1)
			out = binary.BigEndian.AppendUint32(out, r.epoch())
		}
		if out != nil {
			c.WriteToUDPAddrPort(out, from)
		}
	}
}

// natpmp answers a question of NAT-PMP (RFC 6886).
func (r *Router) natpmp(req []byte, from netip.AddrPort, external netip.Addr, refuse bool, taken []uint16) []byte {
	op := req[1]
	head := func(result uint16) []byte {
		b := binary.BigEndian.AppendUint16([]byte{0, 128 + op}, result)
		return binary.BigEndian.AppendUint32(b, r.epoch())
	}
	switch {
	case op == 0 && len(req) == 2:
		r.note("NAT-PMP address")
		return append(head(0), external.AsSlice()...)
	case (op == 1 || op == 2) && len(req) == 12:
		internal := binary.BigEndian.Uint16(req[4:])
		suggested := binary.BigEndian.Uint16(req[6:])
		life := binary.BigEndian.Uint32(req[8:])
		if refuse {
			r.note("NAT-PMP map")
			b := head(2)
			b = binary.BigEndian.AppendUint16(b, internal)
			b = binary.BigEndian.AppendUint16(b, 0)
			return binary.BigEndian.AppendUint32(b, 0)
		}
		if life == 0 {
			r.note("NAT-PMP delete")
		} else {
			r.note("NAT-PMP map")
		}
		ext := r.place("NAT-PMP", from.Addr().Unmap(), internal, suggested, time.Duration(life)*time.Second, taken)
		b := head(0)
		b = binary.BigEndian.AppendUint16(b, internal)
		b = binary.BigEndian.AppendUint16(b, ext)
		return binary.BigEndian.AppendUint32(b, life)
	}
	return head(5)
}

// pcp answers a question of PCP (RFC 6887): MAP alone.
func (r *Router) pcp(req []byte, from netip.AddrPort, external netip.Addr, refuse bool, taken []uint16) []byte {
	if len(req) < 24 || req[1]&0x80 != 0 {
		return nil
	}
	if req[1]&0x7f != 1 {
		return pcpAnswer(req, 4, 0, r.epoch())
	}
	if len(req) < 60 {
		return pcpAnswer(req, 3, 0, r.epoch())
	}
	client := netip.AddrFrom16([16]byte(req[8:24])).Unmap()
	if client != from.Addr().Unmap().WithZone("") {
		return pcpAnswer(req, 12, 0, r.epoch())
	}
	life := binary.BigEndian.Uint32(req[4:])
	if refuse {
		r.note("PCP map")
		return pcpAnswer(req, 2, 0, r.epoch())
	}
	if req[36] != 6 {
		return pcpAnswer(req, 9, 0, r.epoch())
	}
	if life == 0 {
		r.note("PCP delete")
	} else {
		r.note("PCP map")
	}
	internal := binary.BigEndian.Uint16(req[40:])
	suggested := binary.BigEndian.Uint16(req[42:])
	ext := internal
	addr := client
	if client.Is4() {
		ext = r.place("PCP", client, internal, suggested, time.Duration(life)*time.Second, taken)
		addr = external
	} else {
		// IPv6 isn't translated: the router lets the port of the address
		// in, as itself.
		r.place("PCP", client, internal, internal, time.Duration(life)*time.Second, nil)
	}
	out := pcpAnswer(req, 0, life, r.epoch())
	out = append(out, req[24:36]...) // the nonce
	out = append(out, 6, 0, 0, 0)
	out = binary.BigEndian.AppendUint16(out, internal)
	out = binary.BigEndian.AppendUint16(out, ext)
	a16 := addr.As16()
	return append(out, a16[:]...)
}

// pcpAnswer is the head of PCP's answer to req, of result, lifetime and
// epoch (RFC 6887, 7.2).
func pcpAnswer(req []byte, result byte, lifetime, epoch uint32) []byte {
	b := []byte{2, 0x80 | req[1]&0x7f, 0, result}
	b = binary.BigEndian.AppendUint32(b, lifetime)
	b = binary.BigEndian.AppendUint32(b, epoch)
	return append(b, make([]byte, 12)...)
}

// The device and services of the router's UPnP.
const (
	igdDevice = "urn:schemas-upnp-org:device:InternetGatewayDevice:1"
	ipService = "urn:schemas-upnp-org:service:WANIPConnection:1"
)

// serveSSDP answers UPnP's searches for the router's device or its
// service, from the description's address.
func (r *Router) serveSSDP() {
	buf := make([]byte, 2048)
	for {
		n, from, err := r.ssdp.ReadFromUDPAddrPort(buf)
		if err != nil {
			return
		}
		_, _, _, upnp, _, _, _ := r.settings()
		msg := string(buf[:n])
		if !upnp || !strings.HasPrefix(msg, "M-SEARCH * HTTP/1.1\r\n") || !strings.Contains(strings.ToLower(msg), `man: "ssdp:discover"`) {
			continue
		}
		st := ""
		for _, line := range strings.Split(msg, "\r\n") {
			if k, v, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(k), "ST") {
				st = strings.TrimSpace(v)
			}
		}
		if st != igdDevice && st != ipService && st != "ssdp:all" {
			continue
		}
		r.note("UPnP search")
		answer := "HTTP/1.1 200 OK\r\nCACHE-CONTROL: max-age=120\r\nST: " + st + "\r\nUSN: uuid:fake-router::" + st +
			"\r\nEXT:\r\nSERVER: portmaptest UPnP/1.1\r\nLOCATION: " + r.web.URL + "/rootDesc.xml\r\n\r\n"
		r.ssdp.WriteToUDPAddrPort([]byte(answer), from)
	}
}

// description is the router's device description, of the nesting IGD's
// specification has.
const description = `<?xml version="1.0"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
<specVersion><major>1</major><minor>0</minor></specVersion>
<device>
<deviceType>` + igdDevice + `</deviceType>
<friendlyName>Fake router</friendlyName>
<serviceList><service><serviceType>urn:schemas-upnp-org:service:Layer3Forwarding:1</serviceType><controlURL>/ctl/L3F</controlURL></service></serviceList>
<deviceList><device>
<deviceType>urn:schemas-upnp-org:device:WANDevice:1</deviceType>
<deviceList><device>
<deviceType>urn:schemas-upnp-org:device:WANConnectionDevice:1</deviceType>
<serviceList><service>
<serviceType>` + ipService + `</serviceType>
<serviceId>urn:upnp-org:serviceId:WANIPConn1</serviceId>
<controlURL>/ctl/IPConn</controlURL>
<eventSubURL>/evt/IPConn</eventSubURL>
<SCPDURL>/WANIPCn.xml</SCPDURL>
</service></serviceList>
</device></deviceList>
</device></deviceList>
</device>
</root>`

func (r *Router) serveHTTP(w http.ResponseWriter, req *http.Request) {
	switch {
	case req.Method == http.MethodGet && req.URL.Path == "/rootDesc.xml":
		r.note("UPnP description")
		w.Header().Set("Content-Type", "text/xml")
		io.WriteString(w, description)
	case req.Method == http.MethodPost && req.URL.Path == "/ctl/IPConn":
		r.control(w, req)
	default:
		http.NotFound(w, req)
	}
}

// control runs an action of WANIPConnection, by SOAP.
func (r *Router) control(w http.ResponseWriter, req *http.Request) {
	action := strings.Trim(req.Header.Get("SOAPAction"), `"`)
	service, name, ok := strings.Cut(action, "#")
	if !ok || service != ipService {
		fault(w, 401, "Invalid Action")
		return
	}
	args := map[string]string{}
	d := xml.NewDecoder(io.LimitReader(req.Body, 64<<10))
	var cur string
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			cur = t.Name.Local
		case xml.CharData:
			if cur != "" {
				args[cur] += string(t)
			}
		case xml.EndElement:
			cur = ""
		}
	}
	r.note(name)
	external, _, _, _, permanent, refuse, taken := r.settings()
	switch name {
	case "GetExternalIPAddress":
		reply(w, name, "<NewExternalIPAddress>"+external.String()+"</NewExternalIPAddress>")
	case "AddPortMapping":
		ext, err1 := strconv.Atoi(args["NewExternalPort"])
		in, err2 := strconv.Atoi(args["NewInternalPort"])
		lease, err3 := strconv.Atoi(args["NewLeaseDuration"])
		client, err4 := netip.ParseAddr(args["NewInternalClient"])
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil || args["NewProtocol"] != "TCP" || ext <= 0 || ext > 65535 {
			fault(w, 402, "Invalid Args")
			return
		}
		if refuse {
			fault(w, 606, "Action not authorized")
			return
		}
		if permanent && lease != 0 {
			fault(w, 725, "OnlyPermanentLeasesSupported")
			return
		}
		for _, t := range taken {
			if int(t) == ext {
				fault(w, 718, "ConflictInMappingEntry")
				return
			}
		}
		r.mu.Lock()
		if f, used := r.forwards[key(uint16(ext))]; used && f.Client != client {
			r.mu.Unlock()
			fault(w, 718, "ConflictInMappingEntry")
			return
		}
		r.forwards[key(uint16(ext))] = Forward{Method: "UPnP", External: uint16(ext), Internal: uint16(in), Client: client, Lifetime: time.Duration(lease) * time.Second}
		r.mu.Unlock()
		reply(w, name, "")
	case "DeletePortMapping":
		ext, _ := strconv.Atoi(args["NewExternalPort"])
		r.mu.Lock()
		_, had := r.forwards[key(uint16(ext))]
		delete(r.forwards, key(uint16(ext)))
		r.mu.Unlock()
		if !had {
			fault(w, 714, "NoSuchEntryInArray")
			return
		}
		reply(w, name, "")
	default:
		fault(w, 401, "Invalid Action")
	}
}

func reply(w http.ResponseWriter, action, body string) {
	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	fmt.Fprintf(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body><u:%sResponse xmlns:u="%s">%s</u:%sResponse></s:Body></s:Envelope>`,
		action, ipService, body, action)
}

// fault is UPnP's error, as a SOAP fault (UPnP Device Architecture,
// 3.2.2).
func fault(w http.ResponseWriter, code int, text string) {
	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body><s:Fault><faultcode>s:Client</faultcode><faultstring>UPnPError</faultstring><detail><UPnPError xmlns="urn:schemas-upnp-org:control-1-0"><errorCode>%d</errorCode><errorDescription>%s</errorDescription></UPnPError></detail></s:Fault></s:Body></s:Envelope>`,
		code, text)
}
