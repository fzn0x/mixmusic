package upnp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/dnssd"
)

// searchTarget is what Search asks for: renderers of any version answer
// for version 1 of the device type.
const searchTarget = "urn:schemas-upnp-org:device:MediaRenderer:1"

// ssdpGroup is the address SSDP's searches go to.
var ssdpGroup = &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}

// searches is how many times Search asks, within its wait: a renderer may
// miss one, as UDP goes.
const searches = 3

// SearchOptions configure Search. Their zero value asks the network.
type SearchOptions struct {
	// Wait is how long Search waits for answers: 2.5 s when 0. It asks
	// three times in the first two thirds of it.
	Wait time.Duration
	// To, when set, is where the searches go, host:port, in place of
	// SSDP's group on each of the computer's networks: a test's renderer,
	// which may answer from the loopback address then.
	To string
	// From, when set, is the address Search asks from, as
	// "127.0.0.1:0": a test's, which mustn't reach the network.
	From string
	// HTTP fetches the renderers' descriptions: a client of its own when
	// nil.
	HTTP *http.Client
}

// Search finds the media renderers on the computer's networks: it sends
// SSDP's M-SEARCH for them, from a port of its own, never SSDP's 1900,
// which Windows' own SSDP service holds, and gathers what answers for
// opts.Wait. It keeps the renderers at a private or link-local IPv4
// address that serve their description and the control addresses of
// their services themselves, with an AVTransport service of any version.
// They come sorted by name.
func Search(ctx context.Context, opts SearchOptions) ([]Renderer, error) {
	wait := opts.Wait
	if wait <= 0 {
		wait = 2500 * time.Millisecond
	}
	client := opts.HTTP
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	from := &net.UDPAddr{}
	if opts.From != "" {
		a, err := net.ResolveUDPAddr("udp4", opts.From)
		if err != nil {
			return nil, err
		}
		from = a
	}
	conn, err := net.ListenUDP("udp4", from)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	var to *net.UDPAddr
	if opts.To != "" {
		if to, err = net.ResolveUDPAddr("udp4", opts.To); err != nil {
			return nil, err
		}
	}
	msg := searchMessage(2)
	send := func() error {
		if to != nil {
			_, err := conn.WriteToUDP(msg, to)
			return err
		}
		return dnssd.SendToGroup(conn, msg, ssdpGroup)
	}
	if err := send(); err != nil {
		return nil, err
	}

	f := &finder{client: client, loopback: to != nil, seen: map[string]bool{}}
	start := time.Now()
	deadline := start.Add(wait)
	sent := 1
	buf := make([]byte, 4096)
	for {
		// The next search goes at its third of two thirds of the wait.
		until := deadline
		if sent < searches {
			until = start.Add(wait * 2 / 3 * time.Duration(sent) / (searches - 1))
		}
		conn.SetReadDeadline(until)
		n, addr, err := conn.ReadFromUDP(buf)
		if ctx.Err() != nil {
			f.wg.Wait()
			return nil, ctx.Err()
		}
		if err != nil {
			var ne net.Error
			if !errors.As(err, &ne) || !ne.Timeout() {
				if isReset(err) {
					continue
				}
				f.wg.Wait()
				return nil, err
			}
			if sent < searches {
				sent++
				_ = send()
				continue
			}
			break
		}
		f.answer(ctx, buf[:n], addr.IP)
	}
	f.wg.Wait()
	return f.renderers(), nil
}

// searchMessage is SSDP's M-SEARCH for media renderers, which answer
// within mx seconds.
func searchMessage(mx int) []byte {
	var b bytes.Buffer
	b.WriteString("M-SEARCH * HTTP/1.1\r\n")
	b.WriteString("HOST: 239.255.255.250:1900\r\n")
	b.WriteString("MAN: \"ssdp:discover\"\r\n")
	b.WriteString("MX: " + string(rune('0'+mx)) + "\r\n")
	b.WriteString("ST: " + searchTarget + "\r\n")
	b.WriteString("USER-AGENT: MixMusic UPnP/1.1 MixMusic/1\r\n")
	b.WriteString("\r\n")
	return b.Bytes()
}

// finder fetches the description of each renderer that answers, once.
type finder struct {
	client   *http.Client
	loopback bool
	wg       sync.WaitGroup

	mu    sync.Mutex
	seen  map[string]bool
	found []Renderer
}

// answer takes in an answer to a search from the address from, and
// fetches the description it points to, unless it was fetched.
func (f *finder) answer(ctx context.Context, msg []byte, from net.IP) {
	if !localAddress(from) && !(f.loopback && from.IsLoopback()) {
		return
	}
	res, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(msg)), nil)
	if err != nil || res.StatusCode != http.StatusOK {
		return
	}
	res.Body.Close()
	location := res.Header.Get("Location")
	host := from.To4().String()
	if !sameHost(location, host) {
		return
	}
	f.mu.Lock()
	seen := f.seen[location]
	f.seen[location] = true
	f.mu.Unlock()
	if seen {
		return
	}
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		r, err := describe(ctx, f.client, location, host)
		if err != nil {
			return
		}
		f.mu.Lock()
		if !slices.ContainsFunc(f.found, func(o Renderer) bool { return o.ID == r.ID }) {
			f.found = append(f.found, r)
		}
		f.mu.Unlock()
	}()
}

// renderers returns the renderers found, sorted by name.
func (f *finder) renderers() []Renderer {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := slices.Clone(f.found)
	slices.SortFunc(out, func(a, b Renderer) int {
		if n := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); n != 0 {
			return n
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}
