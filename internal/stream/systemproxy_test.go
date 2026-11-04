package stream

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/sysproxy"
)

// connectProxy is a proxy server that tunnels CONNECT requests to the one
// address it reaches, and remembers what they asked for.
type connectProxy struct {
	*httptest.Server
	mu      sync.Mutex
	tunnels []string
}

func newConnectProxy(t *testing.T, to string) *connectProxy {
	p := &connectProxy{}
	p.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "CONNECT only", http.StatusMethodNotAllowed)
			return
		}
		p.mu.Lock()
		p.tunnels = append(p.tunnels, r.Host)
		p.mu.Unlock()
		upstream, err := net.Dial("tcp", to)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		conn, buf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			upstream.Close()
			return
		}
		io.WriteString(conn, "HTTP/1.1 200 Connection established\r\n\r\n")
		go func() {
			io.Copy(upstream, buf)
			upstream.Close()
		}()
		io.Copy(conn, upstream)
		conn.Close()
	}))
	t.Cleanup(p.Close)
	return p
}

func (p *connectProxy) asked() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.tunnels...)
}

// TestProxyFetchesThroughTheSystemProxy fetches a stream from YouTube's
// audio servers, over HTTPS, through the proxy server of Windows' settings,
// as MixMusic does on a network that lets traffic out only through it; and
// directly when the bypass list names the server.
func TestProxyFetchesThroughTheSystemProxy(t *testing.T) {
	size := chunkSize + 4321
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i * 13)
	}
	// The audio server, which httptest's certificate names example.com.
	audio := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	}))
	t.Cleanup(audio.Close)
	roots := x509.NewCertPool()
	roots.AddCert(audio.Certificate())

	for _, tc := range []struct {
		name    string
		bypass  string
		through bool
	}{
		{"through the proxy", "", true},
		{"bypassed", "*.example.com;example.com", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			connect := newConnectProxy(t, audio.Listener.Addr().String())
			r := NewResolver(nil, "")
			r.Run = func(context.Context, ...string) ([]byte, error) {
				return fmt.Appendf(nil, `{"url": "https://example.com/videoplayback?expire=%d", "ext": "webm", "acodec": "opus", "filesize": %d}`,
					time.Now().Add(6*time.Hour).Unix(), size), nil
			}
			p := NewProxy(r)
			tr := p.HTTP.Transport.(*http.Transport)
			tr.Proxy = sysproxy.NewPicker(sysproxy.Settings{Server: connect.Listener.Addr().String(), Bypass: tc.bypass}).Proxy
			tr.TLSClientConfig = &tls.Config{RootCAs: roots}
			// Nothing leaves this computer: example.com, reached
			// directly, is the audio server.
			tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				switch addr {
				case connect.Listener.Addr().String():
				case "example.com:443":
					addr = audio.Listener.Addr().String()
				default:
					return nil, fmt.Errorf("dialed %s", addr)
				}
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			}

			rec := get(t, p, "")
			if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), data) {
				t.Fatalf("status %d, %d bytes of %d; failure %q", rec.Code, rec.Body.Len(), size, p.Failure(testID))
			}
			asked := connect.asked()
			if len(asked) > 0 != tc.through {
				t.Fatalf("tunnels: %q", asked)
			}
			for _, host := range asked {
				if host != "example.com:443" {
					t.Errorf("tunnelled to %q", host)
				}
			}
		})
	}
}
