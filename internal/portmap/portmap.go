// Package portmap asks the router of this computer's network to forward a
// TCP port of its internet address to this computer, as a room of Listen
// with friends that friends outside join needs (#562): by PCP (RFC 6887),
// NAT-PMP (RFC 6886) or UPnP IGD, whichever the router answers, and learns
// the router's internet address from the answer. It asks for one TCP port,
// for a while, renews it while it's needed (Keeper), and removes it at the
// end. On IPv6, which isn't translated, it asks the router by PCP to let
// the port of this computer's address in.
//
// Tests ask the fake routers of portmaptest, on the loopback: never a real
// one. docs/architecture/portmap.md tells why it is so.
package portmap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"
)

// Method is how a router was asked to forward the port.
type Method string

const (
	PCP    Method = "PCP"
	NATPMP Method = "NAT-PMP"
	UPnP   Method = "UPnP"
)

// Mapping is a TCP port a router forwards to this computer: from External,
// its internet address and the port it forwards, to the port Internal,
// for Lifetime, unless renewed, or until it's deleted when 0.
type Mapping struct {
	Method   Method
	External netip.AddrPort
	Internal uint16
	Lifetime time.Duration

	// server is the router asked by PCP or NAT-PMP, and local this
	// computer's address toward it; nonce names a mapping of PCP.
	server netip.AddrPort
	local  netip.Addr
	nonce  [12]byte
	// control is the URL of UPnP's WANIPConnection or WANPPPConnection,
	// and service its type.
	control, service string
}

// Client asks routers. Its zero value asks the routers of this computer's
// networks: tests point it at portmaptest's.
type Client struct {
	// Gateways returns the routers asked by PCP and NAT-PMP, of IPv4 and
	// of IPv6: the system's default gateways when nil.
	Gateways func() ([]netip.Addr, error)
	// ServerPort is PCP's and NAT-PMP's port on the router: 5351 when 0.
	ServerPort int
	// SSDP is where UPnP's search goes: 239.255.255.250:1900 when "".
	SSDP string
	// Retry is how long a question by UDP waits for its first answer: 250
	// ms when 0, then twice as long for each of 4 tries.
	Retry time.Duration
	// HTTP asks UPnP's descriptions and controls: a client of a few
	// seconds when nil.
	HTTP *http.Client
	// Description names the forward, on the routers that keep a name,
	// as UPnP's do: "MixMusic" when "".
	Description string
}

// Defaults of PCP and NAT-PMP, and of UPnP's search (UPnP Device
// Architecture 1.1, 1.3.2).
const (
	serverPort = 5351
	ssdpGroup  = "239.255.255.250:1900"
)

func (c *Client) serverPort() uint16 {
	if c.ServerPort > 0 && c.ServerPort < 65536 {
		return uint16(c.ServerPort)
	}
	return serverPort
}

func (c *Client) retry() time.Duration {
	if c.Retry > 0 {
		return c.Retry
	}
	return 250 * time.Millisecond
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 4 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func (c *Client) description() string {
	if c.Description != "" {
		return c.Description
	}
	return "MixMusic"
}

// gateways are the routers of IPv4, and of IPv6, PCP and NAT-PMP ask.
func (c *Client) gateways() (v4, v6 []netip.Addr) {
	find := c.Gateways
	if find == nil {
		find = DefaultGateways
	}
	all, _ := find()
	for _, gw := range all {
		if gw.Unmap().Is4() {
			v4 = append(v4, gw.Unmap())
		} else if gw.Is6() {
			v6 = append(v6, gw)
		}
	}
	return v4, v6
}

// ErrNoRouter: no router answered, by PCP, NAT-PMP or UPnP.
var ErrNoRouter = errors.New("no router answered: it may take no forwards from computers, or have UPnP, NAT-PMP and PCP off")

// errSilent: a router didn't answer by a protocol, or refused the
// connection, as one that doesn't speak it does.
var errSilent = errors.New("portmap: no answer")

// errNotPCP: the router speaks NAT-PMP, not PCP.
var errNotPCP = errors.New("portmap: the router speaks NAT-PMP, not PCP")

// RefusedError: a router answered, and refused the forward.
type RefusedError struct {
	Method Method
	Code   int
	Reason string
}

func (e *RefusedError) Error() string {
	return fmt.Sprintf("the router refused to forward the port (%s: %s)", e.Method, e.Reason)
}

// Map asks the routers to forward the TCP port of their internet address
// port, or another they choose, to the port of this computer, for
// lifetime: by PCP, then NAT-PMP, then UPnP IGD, until one does. A refusal
// is a *RefusedError, and no answer at all ErrNoRouter.
func (c *Client) Map(ctx context.Context, port uint16, lifetime time.Duration) (*Mapping, error) {
	v4, _ := c.gateways()
	var refused error
	for _, gw := range v4 {
		server := netip.AddrPortFrom(gw, c.serverPort())
		m, err := c.mapPCP(ctx, server, netip.Addr{}, port, port, lifetime, nil)
		if errors.Is(err, errNotPCP) || errors.Is(err, errSilent) {
			m, err = c.mapNATPMP(ctx, server, port, port, lifetime)
		}
		if err == nil {
			return m, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !errors.Is(err, errSilent) && refused == nil {
			refused = err
		}
	}
	m, err := c.mapUPnP(ctx, port, lifetime)
	if err == nil {
		return m, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if !errors.Is(err, errSilent) && refused == nil {
		refused = err
	}
	if refused != nil {
		return nil, refused
	}
	return nil, ErrNoRouter
}

// Open asks the router of IPv6, by PCP, to let the TCP port of addr, an
// IPv6 address of this computer that the internet reaches, in from the
// internet, for lifetime: IPv6 isn't translated, but most routers turn
// away what comes in unasked.
func (c *Client) Open(ctx context.Context, addr netip.Addr, port uint16, lifetime time.Duration) (*Mapping, error) {
	_, v6 := c.gateways()
	var refused error
	for _, gw := range v6 {
		m, err := c.mapPCP(ctx, netip.AddrPortFrom(gw, c.serverPort()), addr, port, port, lifetime, nil)
		if err == nil {
			return m, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !errors.Is(err, errSilent) && !errors.Is(err, errNotPCP) && refused == nil {
			refused = err
		}
	}
	if refused != nil {
		return nil, refused
	}
	return nil, ErrNoRouter
}

// Renew asks m's router again for lifetime, and returns the mapping as it
// is now, whose external address may have changed.
func (c *Client) Renew(ctx context.Context, m *Mapping, lifetime time.Duration) (*Mapping, error) {
	switch m.Method {
	case PCP:
		nonce := m.nonce
		return c.mapPCP(ctx, m.server, m.local, m.Internal, m.External.Port(), lifetime, &nonce)
	case NATPMP:
		return c.mapNATPMP(ctx, m.server, m.Internal, m.External.Port(), lifetime)
	case UPnP:
		return c.renewUPnP(ctx, m, lifetime)
	}
	return nil, fmt.Errorf("portmap: no method %q", m.Method)
}

// Delete asks m's router to forward its port no more.
func (c *Client) Delete(ctx context.Context, m *Mapping) error {
	switch m.Method {
	case PCP:
		nonce := m.nonce
		_, err := c.mapPCP(ctx, m.server, m.local, m.Internal, m.External.Port(), 0, &nonce)
		return err
	case NATPMP:
		_, err := c.mapNATPMP(ctx, m.server, m.Internal, 0, 0)
		return err
	case UPnP:
		return c.deleteUPnP(ctx, m)
	}
	return fmt.Errorf("portmap: no method %q", m.Method)
}

// Shared reports whether addr, a router's internet address, can't be
// reached from the internet: one of 100.64.0.0/10, which providers share
// among their customers (RFC 6598), or of a private range. Such a router
// sits behind another, most often the provider's.
func Shared(addr netip.Addr) bool {
	addr = addr.Unmap()
	return !addr.IsValid() || addr.IsUnspecified() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() ||
		netip.MustParsePrefix("100.64.0.0/10").Contains(addr)
}

// ask sends req to server from local, when valid, or the address the
// system picks, and returns the first answer ok takes: it sends req again
// after c.retry(), then twice as long, 4 times in all. No answer, or a
// refused connection, as from a router that doesn't speak the protocol,
// is errSilent.
func (c *Client) ask(ctx context.Context, local netip.Addr, server netip.AddrPort, req func(local netip.Addr) []byte, ok func([]byte) bool) ([]byte, netip.Addr, error) {
	var laddr *net.UDPAddr
	if local.IsValid() {
		laddr = net.UDPAddrFromAddrPort(netip.AddrPortFrom(local, 0))
	}
	conn, err := net.DialUDP("udp", laddr, net.UDPAddrFromAddrPort(server))
	if err != nil {
		return nil, netip.Addr{}, fmt.Errorf("%w: %v", errSilent, err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.SetReadDeadline(time.Now()) })
	defer stop()
	me := conn.LocalAddr().(*net.UDPAddr).AddrPort().Addr().Unmap()
	packet := req(me)
	buf := make([]byte, 1100)
	wait := c.retry()
	for range 4 {
		if _, err := conn.Write(packet); err != nil {
			return nil, me, fmt.Errorf("%w: %v", errSilent, err)
		}
		conn.SetReadDeadline(time.Now().Add(wait))
		for {
			n, err := conn.Read(buf)
			if err != nil {
				if ctx.Err() != nil {
					return nil, me, ctx.Err()
				}
				var ne net.Error
				if errors.As(err, &ne) && ne.Timeout() {
					break
				}
				return nil, me, fmt.Errorf("%w: %v", errSilent, err)
			}
			if ok(buf[:n]) {
				return append([]byte(nil), buf[:n]...), me, nil
			}
		}
		wait *= 2
	}
	return nil, me, errSilent
}
