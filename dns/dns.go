package retryablednstest

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"math/rand"
	"net"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
	"fmt"

	dnsManager	"github.com/bp0lr/dmut/dnsManager"

	"github.com/miekg/dns"
)

// Client is a DNS resolver client to resolve hostnames.
type Client struct {
	resolvers  	[]string
	maxRetries 	int
	dnsTimeOut	int
	errorLimit	int
	mutex      	*sync.Mutex
}

// New creates a new dns client
func New(dnsTimeOut int, maxRetries int, errorLimit int) *Client {
	client := Client{
		mutex:      &sync.Mutex{},
		dnsTimeOut: dnsTimeOut,
		maxRetries: maxRetries,
		errorLimit: errorLimit,
	}
	return &client
}

// Resolve is the underlying resolve function that actually resolves a host
// and gets the ip records for that host.
func (c *Client) Resolve(host string) (*DNSData, error) {
	return c.Query(host, dns.TypeA, "")
}

// Do sends a provided dns request and return the raw native response
func (c *Client) Do(msg *dns.Msg) (resp *dns.Msg, err error) {

	cli := dns.Client{Net: "udp", Timeout: time.Duration(c.dnsTimeOut) * time.Millisecond}

	for i := 0; i < c.maxRetries; i++ {
		resolver := c.resolvers[rand.Intn(len(c.resolvers))]
		resp, _, err = cli.Exchange(msg, resolver)
		if err != nil {
			//fmt.Printf("err: %v\n", err)
			continue
		}

		// In case we get a non empty answer stop retrying
		if resp != nil {
			return
		}
	}

	return
}

// Query sends a provided dns request and return enriched response
func (c *Client) Query(host string, requestType uint16, customDNSServer string) (*DNSData, error) {
	return c.QueryMultiple(host, []uint16{requestType}, customDNSServer)
}

// QueryMultiple sends a provided dns request and return the data
func (c *Client) QueryMultiple(host string, requestTypes []uint16, customDNSServer string) (*DNSData, error) {
	
	var (
		dnsdata 	DNSData
		err     	error
		msg     	dns.Msg
	)

	cliUDP := dns.Client{Net: "udp", Timeout: time.Duration(c.dnsTimeOut) * time.Millisecond}		
	for _, requestType := range requestTypes {

		msg.Id = dns.Id()
		msg.RecursionDesired = true
		msg.Question = make([]dns.Question, 1)

		name := dns.Fqdn(host)

		// In case of PTR adjust the domain name
		if requestType == dns.TypePTR {
			var err error
			if net.ParseIP(host) != nil {
				name, err = dns.ReverseAddr(host)
				if err != nil {
					return nil, err
				}
			}
			msg.SetEdns0(dns.DefaultMsgSize, false)
		}

		question := dns.Question{
			Name:   name,
			Qtype:  requestType,
			Qclass: dns.ClassINET,
		}
		
		msg.Question[0] = question
		
		var dnsServer string
		for i := 0; i < c.maxRetries; i++ {			
			
			if(len(customDNSServer) == 0){
				val:=dnsManager.ReturnRandomDNSServerEntry();						
				dnsServer = val.Host
			}else{
				dnsServer = customDNSServer
			}
			
			var resp *dns.Msg
			resp, _, err = cliUDP.Exchange(&msg, dnsServer)
			
			if err != nil {
				dnsManager.ReportDNSError(dnsServer, c.errorLimit)
				//fmt.Printf("err: %v\n", err)
				continue;
			}

			if resp != nil && resp.Truncated {
				fmt.Printf("[truncate]: %v\n", msg.Question[0].String())
				
				//We have a truncated response, lets retry the query using TCP
				cliTCP := dns.Client{Net: "tcp", Timeout: time.Duration(c.dnsTimeOut) * time.Millisecond}
				resp, _, err = cliTCP.Exchange(&msg, dnsServer)
			}

			if err != nil && len(customDNSServer) == 0 {
				dnsManager.ReportDNSError(dnsServer, c.errorLimit)
				//fmt.Printf("err: %v\n", err)
				continue;
			}

			if(dns.RcodeToString[resp.Rcode] == "SERVFAIL"){
				dnsManager.ReportDNSError(dnsServer, c.errorLimit)
				continue;
			}

			dnsdata.Host = host
			dnsdata.Raw += resp.String()
			dnsdata.StatusCode = dns.RcodeToString[resp.Rcode]
			
			if(dnsdata.StatusCode != "NXDOMAIN"){
				fmt.Printf("[%v] : %v\n", host, dnsdata.StatusCode)
			}
			
			dnsdata.Resolver = append(dnsdata.Resolver, dnsServer)
			dnsdata.OriReq = msg.String()
			dnsdata.OriRes = resp.String()

			dnsdata.ParseFromMsg(resp)
			break
		}
	}

	dnsdata.dedupe()
	return &dnsdata, err
}

func parse(answer *dns.Msg, requestType uint16) (results []string) {
	for _, record := range answer.Answer {
		switch requestType {
		case dns.TypeA:
			if t, ok := record.(*dns.A); ok {
				results = append(results, t.A.String())
			}
		case dns.TypeNS:
			if t, ok := record.(*dns.NS); ok {
				results = append(results, t.Ns)
			}
		case dns.TypeCNAME:
			if t, ok := record.(*dns.CNAME); ok {
				results = append(results, t.Target)
			}
		case dns.TypeSOA:
			if t, ok := record.(*dns.SOA); ok {
				results = append(results, t.Mbox)
			}
		case dns.TypePTR:
			if t, ok := record.(*dns.PTR); ok {
				results = append(results, t.Ptr)
			}
		case dns.TypeMX:
			if t, ok := record.(*dns.MX); ok {
				results = append(results, t.Mx)
			}
		case dns.TypeTXT:
			if t, ok := record.(*dns.TXT); ok {
				results = append(results, t.Txt...)
			}
		case dns.TypeAAAA:
			if t, ok := record.(*dns.AAAA); ok {
				results = append(results, t.AAAA.String())
			}
		}
	}

	return
}

//DNSData desc
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
	OriRes		string
	OriReq		string
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
	dedupeSlice(&d.Resolver, less(&d.Resolver))
	dedupeSlice(&d.A, less(&d.A))
	dedupeSlice(&d.AAAA, less(&d.AAAA))
	dedupeSlice(&d.CNAME, less(&d.CNAME))
	dedupeSlice(&d.MX, less(&d.MX))
	dedupeSlice(&d.PTR, less(&d.PTR))
	dedupeSlice(&d.SOA, less(&d.SOA))
	dedupeSlice(&d.NS, less(&d.NS))
	dedupeSlice(&d.TXT, less(&d.TXT))
}

//Marshal desc
func (d *DNSData) Marshal() ([]byte, error) {
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	err := enc.Encode(d)
	if err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

//Unmarshal desc
func (d *DNSData) Unmarshal(b []byte) error {
	dec := gob.NewDecoder(bytes.NewBuffer(b))
	err := dec.Decode(&d)
	if err != nil {
		return err
	}
	return nil
}

func less(v interface{}) func(i, j int) bool {
	s := *v.(*[]string)
	return func(i, j int) bool { return s[i] < s[j] }
}

func dedupeSlice(slicePtr interface{}, less func(i, j int) bool) {
	v := reflect.ValueOf(slicePtr).Elem()
	if v.Len() <= 1 {
		return
	}
	sort.Slice(v.Interface(), less)

	i := 0
	for j := 1; j < v.Len(); j++ {
		if !less(i, j) {
			continue
		}
		i++
		v.Index(i).Set(v.Index(j))
	}
	i++
	v.SetLen(i)
}