package portmap

// UPnP IGD (Internet Gateway Device, v1 and v2): a search by SSDP, over UDP
// multicast, which the router answers with where its description is; the
// description, over HTTP, which names the control of its WANIPConnection
// or WANPPPConnection service; and SOAP actions of that service, over
// HTTP: GetExternalIPAddress, AddPortMapping and DeletePortMapping.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// What UPnP's search looks for, the device of IGD v2 and v1 and their
// services, and the services that forward ports, best first.
var (
	searchTargets = []string{
		"urn:schemas-upnp-org:device:InternetGatewayDevice:2",
		"urn:schemas-upnp-org:device:InternetGatewayDevice:1",
		"urn:schemas-upnp-org:service:WANIPConnection:1",
	}
	forwardServices = []string{
		"urn:schemas-upnp-org:service:WANIPConnection:2",
		"urn:schemas-upnp-org:service:WANIPConnection:1",
		"urn:schemas-upnp-org:service:WANPPPConnection:1",
	}
)

// maxDescription bounds a description, and an answer of a control.
const maxDescription = 256 << 10

// gateway is a router's service that forwards ports, as its description
// tells it, at the router's address host.
type gateway struct {
	host             netip.Addr
	control, service string
}

// search looks for the routers of UPnP IGD, by SSDP, for a few seconds,
// and returns those that answer whose description is at their own address.
func (c *Client) search(ctx context.Context) ([]netip.Addr, []string, error) {
	group := c.SSDP
	if group == "" {
		group = ssdpGroup
	}
	to, err := net.ResolveUDPAddr("udp4", group)
	if err != nil {
		return nil, nil, err
	}
	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", errSilent, err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.SetReadDeadline(time.Now()) })
	defer stop()
	for _, st := range searchTargets {
		msg := "M-SEARCH * HTTP/1.1\r\nHOST: " + ssdpGroup + "\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\nST: " + st + "\r\n\r\n"
		if _, err := conn.WriteToUDP([]byte(msg), to); err != nil {
			return nil, nil, fmt.Errorf("%w: %v", errSilent, err)
		}
	}
	var hosts []netip.Addr
	var locations []string
	seen := map[string]bool{}
	deadline := time.Now().Add(8 * c.retry())
	buf := make([]byte, 2048)
	for {
		conn.SetReadDeadline(deadline)
		n, from, err := conn.ReadFromUDPAddrPort(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil, nil, ctx.Err()
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() || len(hosts) > 0 {
				break
			}
			// Windows tells of an ICMP error on the next read: the others
			// may still answer.
			continue
		}
		resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(buf[:n])), nil)
		if err != nil || resp.StatusCode != http.StatusOK {
			continue
		}
		loc := strings.TrimSpace(resp.Header.Get("Location"))
		u, err := url.Parse(loc)
		if err != nil || u.Scheme != "http" || seen[loc] {
			continue
		}
		// Whatever answers the search can name any address: only a
		// description at the address of the one answering is asked.
		host, err := netip.ParseAddr(u.Hostname())
		if err != nil || host.Unmap() != from.Addr().Unmap() {
			continue
		}
		seen[loc] = true
		hosts = append(hosts, host.Unmap())
		locations = append(locations, loc)
		// One more moment for the others, once one answered.
		if d := time.Now().Add(c.retry()); d.Before(deadline) {
			deadline = d
		}
	}
	return hosts, locations, nil
}

// device is a device of a description, with its services and the devices
// it holds.
type device struct {
	Services []struct {
		Type    string `xml:"serviceType"`
		Control string `xml:"controlURL"`
	} `xml:"serviceList>service"`
	Devices []device `xml:"deviceList>device"`
}

// gatewayAt reads the description at loc, of the router host, and returns
// its best service that forwards ports.
func (c *Client) gatewayAt(ctx context.Context, host netip.Addr, loc string) (gateway, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, loc, nil)
	if err != nil {
		return gateway{}, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return gateway{}, fmt.Errorf("%w: %v", errSilent, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return gateway{}, fmt.Errorf("%w: the description answers %s", errSilent, resp.Status)
	}
	var root struct {
		URLBase string `xml:"URLBase"`
		Device  device `xml:"device"`
	}
	if err := xml.NewDecoder(io.LimitReader(resp.Body, maxDescription)).Decode(&root); err != nil {
		return gateway{}, fmt.Errorf("%w: its description: %v", errSilent, err)
	}
	base, err := url.Parse(loc)
	if err != nil {
		return gateway{}, err
	}
	if b := strings.TrimSpace(root.URLBase); b != "" {
		if u, err := url.Parse(b); err == nil {
			base = u
		}
	}
	found := map[string]string{}
	var walk func(d device)
	walk = func(d device) {
		for _, s := range d.Services {
			if _, ok := found[strings.TrimSpace(s.Type)]; !ok {
				found[strings.TrimSpace(s.Type)] = strings.TrimSpace(s.Control)
			}
		}
		for _, sub := range d.Devices {
			walk(sub)
		}
	}
	walk(root.Device)
	for _, typ := range forwardServices {
		ctl, ok := found[typ]
		if !ok {
			continue
		}
		u, err := base.Parse(ctl)
		if err != nil || u.Scheme != "http" {
			continue
		}
		// The control, too, is the router's own.
		if h, err := netip.ParseAddr(u.Hostname()); err != nil || h.Unmap() != host {
			continue
		}
		return gateway{host: host, control: u.String(), service: typ}, nil
	}
	return gateway{}, fmt.Errorf("%w: the router's description has no service that forwards ports", errSilent)
}

// mapUPnP looks for the routers of UPnP IGD, and asks the first that
// answers to forward port, or another when another computer has it, to the
// port of this computer, for lifetime, or for good when the router takes
// no lease.
func (c *Client) mapUPnP(ctx context.Context, port uint16, lifetime time.Duration) (*Mapping, error) {
	hosts, locations, err := c.search(ctx)
	if err != nil {
		return nil, err
	}
	err = errSilent
	for i, loc := range locations {
		gw, gerr := c.gatewayAt(ctx, hosts[i], loc)
		if gerr != nil {
			err = gerr
			continue
		}
		m := &Mapping{Method: UPnP, Internal: port, control: gw.control, service: gw.service, server: netip.AddrPortFrom(gw.host, 0)}
		if m.local, err = localToward(gw.host); err != nil {
			continue
		}
		external := port
		for try := 0; ; try++ {
			m.External = netip.AddrPortFrom(netip.Addr{}, external)
			var got *Mapping
			got, err = c.renewUPnP(ctx, m, lifetime)
			var re *RefusedError
			if err == nil {
				return got, nil
			}
			// Another computer has the port: another, a few times.
			if errors.As(err, &re) && re.Code == 718 && try < 3 {
				external = uint16(1024 + rand.IntN(65535-1024))
				continue
			}
			break
		}
	}
	return nil, err
}

// renewUPnP asks m's router to forward its port, as AddPortMapping does
// again of a forward it has, for lifetime, or for good when it takes no
// lease, and reads the router's internet address.
func (c *Client) renewUPnP(ctx context.Context, m *Mapping, lifetime time.Duration) (*Mapping, error) {
	lease := uint32(lifetime / time.Second)
	add := func(lease uint32) error {
		_, err := c.soap(ctx, m, "AddPortMapping", [][2]string{
			{"NewRemoteHost", ""},
			{"NewExternalPort", strconv.Itoa(int(m.External.Port()))},
			{"NewProtocol", "TCP"},
			{"NewInternalPort", strconv.Itoa(int(m.Internal))},
			{"NewInternalClient", m.local.String()},
			{"NewEnabled", "1"},
			{"NewPortMappingDescription", c.description()},
			{"NewLeaseDuration", strconv.FormatUint(uint64(lease), 10)},
		})
		return err
	}
	err := add(lease)
	var re *RefusedError
	if errors.As(err, &re) && re.Code == 725 {
		// OnlyPermanentLeasesSupported: one for good, which Delete
		// removes.
		lease = 0
		err = add(0)
	}
	if err != nil {
		return nil, err
	}
	out := *m
	out.Lifetime = time.Duration(lease) * time.Second
	ip, err := c.soap(ctx, m, "GetExternalIPAddress", nil)
	if err != nil {
		return &out, nil
	}
	addr, _ := netip.ParseAddr(strings.TrimSpace(ip["NewExternalIPAddress"]))
	out.External = netip.AddrPortFrom(addr.Unmap(), m.External.Port())
	return &out, nil
}

// deleteUPnP asks m's router to forward its port no more.
func (c *Client) deleteUPnP(ctx context.Context, m *Mapping) error {
	_, err := c.soap(ctx, m, "DeletePortMapping", [][2]string{
		{"NewRemoteHost", ""},
		{"NewExternalPort", strconv.Itoa(int(m.External.Port()))},
		{"NewProtocol", "TCP"},
	})
	return err
}

// upnpReason tells an error code of UPnP's actions.
func upnpReason(code int, text string) string {
	switch code {
	case 606:
		return "not authorized: the router may have forwards from computers off"
	case 718:
		return "another computer has the port"
	case 724:
		return "the router forwards a port to the same port alone"
	case 725:
		return "the router takes forwards for good alone"
	case 729:
		return "the router has no room for another forward"
	}
	if text != "" {
		return text + " (" + strconv.Itoa(code) + ")"
	}
	return "error " + strconv.Itoa(code)
}

// soap runs action of m's service, of the arguments args, in their order,
// and returns what it answers, by the names of its elements.
func (c *Client) soap(ctx context.Context, m *Mapping, action string, args [][2]string) (map[string]string, error) {
	var body strings.Builder
	body.WriteString(`<?xml version="1.0"?>` + "\r\n" + `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body>`)
	fmt.Fprintf(&body, `<u:%s xmlns:u="%s">`, action, html.EscapeString(m.service))
	for _, a := range args {
		fmt.Fprintf(&body, "<%s>%s</%s>", a[0], html.EscapeString(a[1]), a[0])
	}
	fmt.Fprintf(&body, `</u:%s></s:Body></s:Envelope>`, action)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.control, strings.NewReader(body.String()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPAction", `"`+m.service+"#"+action+`"`)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errSilent, err)
	}
	defer resp.Body.Close()
	out := map[string]string{}
	d := xml.NewDecoder(io.LimitReader(resp.Body, maxDescription))
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
				out[cur] += string(t)
			}
		case xml.EndElement:
			cur = ""
		}
	}
	if resp.StatusCode != http.StatusOK {
		code, _ := strconv.Atoi(strings.TrimSpace(out["errorCode"]))
		return nil, &RefusedError{Method: UPnP, Code: code, Reason: upnpReason(code, strings.TrimSpace(out["errorDescription"]))}
	}
	return out, nil
}

// localToward is this computer's address toward the router host: the one
// the system sends from, which the router forwards to.
func localToward(host netip.Addr) (netip.Addr, error) {
	conn, err := net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(netip.AddrPortFrom(host, 1900)))
	if err != nil {
		return netip.Addr{}, err
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).AddrPort().Addr().Unmap(), nil
}
