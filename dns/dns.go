// Package dns provides the DNS client used by dmut.
package dns

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	dnsmanager "github.com/bp0lr/dmut/dnsManager"
	"github.com/miekg/dns"
)

// Client holds immutable query settings and a synchronized resolver pool.
type Client struct {
	pool       *dnsmanager.Pool
	maxRetries int
	dnsTimeOut int
	errorLimit int
}

// New creates a client with an explicitly owned resolver pool.
func New(pool *dnsmanager.Pool, timeout, retries, errorLimit int) *Client {
	return &Client{pool: pool, dnsTimeOut: timeout, maxRetries: retries, errorLimit: errorLimit}
}

// QueryError preserves the failed query's settings and underlying cause.
type QueryError struct {
	Host, Server string
	Type         uint16
	Attempts     int
	TimeoutMS    int
	Err          error
}

func (e *QueryError) Error() string {
	kind := dns.TypeToString[e.Type]
	if kind == "" {
		kind = fmt.Sprintf("TYPE%d", e.Type)
	}
	server := e.Server
	if server == "" {
		server = "none selected"
	}
	return fmt.Sprintf("DNS %s query for %s failed after %d attempt(s) (timeout %dms; last resolver %s): %v", kind, e.Host, e.Attempts, e.TimeoutMS, server, e.Err)
}

func (e *QueryError) Unwrap() error { return e.Err }

// Query sends one record query with cancellation support.
func (c *Client) Query(ctx context.Context, host string, requestType uint16, customDNSServer string) (*DNSData, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.maxRetries < 1 || c.dnsTimeOut < 1 || c.errorLimit < 1 {
		return nil, errors.New("invalid DNS client settings")
	}
	msg := new(dns.Msg)
	name := dns.Fqdn(host)
	if requestType == dns.TypePTR && net.ParseIP(host) != nil {
		var err error
		name, err = dns.ReverseAddr(host)
		if err != nil {
			return nil, err
		}
		msg.SetEdns0(dns.DefaultMsgSize, false)
	}
	msg.SetQuestion(name, requestType)
	udp := dns.Client{Net: "udp", Timeout: time.Duration(c.dnsTimeOut) * time.Millisecond}
	var lastErr error
	var lastServer string
	failure := func(attempts int, err error) error {
		return &QueryError{Host: host, Server: lastServer, Type: requestType, Attempts: attempts, TimeoutMS: c.dnsTimeOut, Err: err}
	}
	for i := 0; i < c.maxRetries; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		server := customDNSServer
		if server == "" {
			if c.pool == nil {
				return nil, failure(i, dnsmanager.ErrUnavailable)
			}
			entry, err := c.pool.Pick()
			if err != nil {
				return nil, failure(i, errors.Join(err, lastErr))
			}
			server = entry.Host
		}
		lastServer = server
		resp, _, err := udp.ExchangeContext(ctx, msg, server)
		if err == nil && resp != nil && resp.Truncated {
			tcp := dns.Client{Net: "tcp", Timeout: udp.Timeout}
			resp, _, err = tcp.ExchangeContext(ctx, msg, server)
		}
		if err == nil && resp == nil {
			err = errors.New("empty DNS response")
		}
		if err == nil && resp.Rcode == dns.RcodeServerFailure {
			err = errors.New("DNS server returned SERVFAIL")
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = err
			if c.pool != nil {
				c.pool.ReportError(server, c.errorLimit)
			}
			continue
		}
		data := &DNSData{
			Host: host, Raw: resp.String(), StatusCode: dns.RcodeToString[resp.Rcode],
			Resolver: []string{server}, OriReq: msg.String(), OriRes: resp.String(),
		}
		if err := data.ParseFromMsg(resp); err != nil {
			return nil, err
		}
		data.dedupe()
		return data, nil
	}
	return nil, failure(c.maxRetries, lastErr)
}

// DNSData contains the records and metadata returned by a DNS query.
type DNSData struct {
	Host       string   `json:"host,omitempty"`
	TTL        int      `json:"ttl,omitempty"`
	Resolver   []string `json:"resolver,omitempty"`
	A          []string `json:"a,omitempty"`
	AAAA       []string `json:"aaaa,omitempty"`
	CNAME      []string `json:"cname,omitempty"`
	MX         []string `json:"mx,omitempty"`
	PTR        []string `json:"ptr,omitempty"`
	SOA        []string `json:"soa,omitempty"`
	NS         []string `json:"ns,omitempty"`
	TXT        []string `json:"txt,omitempty"`
	Raw        string   `json:"raw,omitempty"`
	StatusCode string   `json:"status_code,omitempty"`
	OriRes     string
	OriReq     string
}

// ParseFromMsg and enrich data
func (d *DNSData) ParseFromMsg(msg *dns.Msg) error {
	for _, record := range msg.Answer {
		switch record.(type) {
		case *dns.A:
			d.A = append(d.A, trimChars(record.(*dns.A).A.String()))
		case *dns.NS:
			d.NS = append(d.NS, trimChars(record.(*dns.NS).Ns))
		case *dns.CNAME:
			d.CNAME = append(d.CNAME, trimChars(record.(*dns.CNAME).Target))
		case *dns.SOA:
			d.SOA = append(d.SOA, trimChars(record.(*dns.SOA).Mbox))
		case *dns.PTR:
			d.PTR = append(d.PTR, trimChars(record.(*dns.PTR).Ptr))
		case *dns.MX:
			d.MX = append(d.MX, trimChars(record.(*dns.MX).Mx))
		case *dns.TXT:
			for _, txt := range record.(*dns.TXT).Txt {
				d.TXT = append(d.TXT, trimChars(txt))
			}
		case *dns.AAAA:
			d.AAAA = append(d.AAAA, trimChars(record.(*dns.AAAA).AAAA.String()))
		}
	}

	return nil
}

// JSON returns the object as json string
func (d *DNSData) JSON() (string, error) {
	b, err := json.Marshal(&d)
	return string(b), err
}

func trimChars(s string) string {
	return strings.TrimRight(s, ".")
}

func (d *DNSData) dedupe() {
	// dedupe all records
	slices.Sort(d.Resolver)
	d.Resolver = slices.Compact(d.Resolver)
	slices.Sort(d.A)
	d.A = slices.Compact(d.A)
	slices.Sort(d.AAAA)
	d.AAAA = slices.Compact(d.AAAA)
	slices.Sort(d.CNAME)
	d.CNAME = slices.Compact(d.CNAME)
	slices.Sort(d.MX)
	d.MX = slices.Compact(d.MX)
	slices.Sort(d.PTR)
	d.PTR = slices.Compact(d.PTR)
	slices.Sort(d.SOA)
	d.SOA = slices.Compact(d.SOA)
	slices.Sort(d.NS)
	d.NS = slices.Compact(d.NS)
	slices.Sort(d.TXT)
	d.TXT = slices.Compact(d.TXT)
}

// Marshal encodes DNS data using gob.
func (d *DNSData) Marshal() ([]byte, error) {
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	err := enc.Encode(d)
	if err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

// Unmarshal decodes DNS data from gob.
func (d *DNSData) Unmarshal(b []byte) error {
	dec := gob.NewDecoder(bytes.NewBuffer(b))
	err := dec.Decode(&d)
	if err != nil {
		return err
	}
	return nil
}
