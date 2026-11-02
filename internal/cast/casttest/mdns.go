package casttest

import (
	"net"
	"strings"
	"sync"
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

// Entry is a cast device as the responder announces it.
type Entry struct {
	ID, Name, Model string
	// Host is the address of the device's A record: 127.0.0.1 when "",
	// none when "-", which leaves the device's address to the answer's.
	Host string
	Port int
}

// Responder answers mDNS queries for cast devices at a loopback address,
// as devices answer one-shot queriers: directly, to the query's address,
// with its ID and its questions.
type Responder struct {
	conn *net.UDPConn

	mu      sync.Mutex
	entries []Entry
	// sparse has it answer only what each question asks, with nothing
	// beside it.
	sparse bool
	// queries holds the questions of each query it got.
	queries [][]dnsmessage.Question
}

// NewResponder starts a responder of entries until the test ends.
func NewResponder(t testing.TB, entries ...Entry) *Responder {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	r := &Responder{conn: conn, entries: entries}
	go r.serve()
	t.Cleanup(func() { conn.Close() })
	return r
}

// Addr is where the responder takes queries.
func (r *Responder) Addr() string { return r.conn.LocalAddr().String() }

// Sparse has the responder answer only what each question asks: the
// pointers to the devices alone, at first.
func (r *Responder) Sparse() {
	r.mu.Lock()
	r.sparse = true
	r.mu.Unlock()
}

// Queries returns how many queries the responder got.
func (r *Responder) Queries() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.queries)
}

const service = "_googlecast._tcp.local."

func instance(e Entry) string { return "Chromecast-" + e.ID + "." + service }
func hostOf(e Entry) string   { return e.ID + ".local." }

func (r *Responder) serve() {
	buf := make([]byte, 9000)
	for {
		n, from, err := r.conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		var p dnsmessage.Parser
		h, err := p.Start(buf[:n])
		if err != nil || h.Response {
			continue
		}
		qs, err := p.AllQuestions()
		if err != nil {
			continue
		}
		r.mu.Lock()
		r.queries = append(r.queries, qs)
		entries, sparse := append([]Entry(nil), r.entries...), r.sparse
		r.mu.Unlock()
		if msg := answer(h.ID, qs, entries, sparse); msg != nil {
			r.conn.WriteToUDP(msg, from)
		}
	}
}

// answer builds the answer to the questions qs of the query id.
func answer(id uint16, qs []dnsmessage.Question, entries []Entry, sparse bool) []byte {
	type record struct {
		name string
		kind dnsmessage.Type
		e    Entry
	}
	var answers, extra []record
	for _, q := range qs {
		name := strings.ToLower(q.Name.String())
		for _, e := range entries {
			switch {
			case q.Type == dnsmessage.TypePTR && name == service:
				answers = append(answers, record{name, dnsmessage.TypePTR, e})
				if !sparse {
					extra = append(extra, record{instance(e), dnsmessage.TypeSRV, e}, record{instance(e), dnsmessage.TypeTXT, e})
					if e.Host != "-" {
						extra = append(extra, record{hostOf(e), dnsmessage.TypeA, e})
					}
				}
			case q.Type == dnsmessage.TypeSRV && name == strings.ToLower(instance(e)):
				answers = append(answers, record{instance(e), dnsmessage.TypeSRV, e})
			case q.Type == dnsmessage.TypeTXT && name == strings.ToLower(instance(e)):
				answers = append(answers, record{instance(e), dnsmessage.TypeTXT, e})
			case q.Type == dnsmessage.TypeA && name == strings.ToLower(hostOf(e)) && e.Host != "-":
				answers = append(answers, record{hostOf(e), dnsmessage.TypeA, e})
			}
		}
	}
	if len(answers) == 0 {
		return nil
	}
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{ID: id, Response: true, Authoritative: true})
	b.EnableCompression()
	b.StartQuestions()
	for _, q := range qs {
		b.Question(q)
	}
	put := func(rec record) {
		// The cache-flush bit, as devices set it on their unique records.
		class := dnsmessage.ClassINET
		if rec.kind != dnsmessage.TypePTR {
			class |= 1 << 15
		}
		h := dnsmessage.ResourceHeader{Name: dnsmessage.MustNewName(rec.name), Class: class, TTL: 120}
		switch rec.kind {
		case dnsmessage.TypePTR:
			b.PTRResource(h, dnsmessage.PTRResource{PTR: dnsmessage.MustNewName(instance(rec.e))})
		case dnsmessage.TypeSRV:
			b.SRVResource(h, dnsmessage.SRVResource{Port: uint16(rec.e.Port), Target: dnsmessage.MustNewName(hostOf(rec.e))})
		case dnsmessage.TypeTXT:
			b.TXTResource(h, dnsmessage.TXTResource{TXT: []string{"id=" + rec.e.ID, "cd=0123", "rm=", "ve=05",
				"md=" + rec.e.Model, "ic=/setup/icon.png", "fn=" + rec.e.Name, "ca=4101", "st=0", "rs="}})
		case dnsmessage.TypeA:
			ip := net.ParseIP(rec.e.Host).To4()
			if ip == nil {
				ip = net.IPv4(127, 0, 0, 1).To4()
			}
			b.AResource(h, dnsmessage.AResource{A: [4]byte(ip)})
		}
	}
	b.StartAnswers()
	for _, rec := range answers {
		put(rec)
	}
	b.StartAuthorities()
	b.StartAdditionals()
	for _, rec := range extra {
		put(rec)
	}
	msg, err := b.Finish()
	if err != nil {
		return nil
	}
	return msg
}
