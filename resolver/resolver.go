package resolver

import (
	"fmt"
	//"net"
	"time"
	"sort"
	"strings"

	"github.com/miekg/dns"
)

//GetDNSQueryResponse desc
func GetDNSQueryResponse(queryType string, fqdn string, dnsServer string) ([]string, error) {
	qt, ok := dns.StringToType[queryType]
	if !ok {
		return nil, fmt.Errorf("Query type '%s' is an unknown type", queryType)
	}
	
	udp := &dns.Client{
			Net: "udp", 
			Timeout: time.Millisecond * time.Duration(2500),
			DialTimeout:  5 * time.Second,
			ReadTimeout:  20 * time.Second,
			WriteTimeout: 20 * time.Second,
	}
	
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(fqdn), qt)
	
	in, _ , err := udp.Exchange(m, dnsServer)
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return nil, err
	}

	responses := []string{}

	switch dns.RcodeToString[in.Rcode] {
	// TODO: Catch more error codes (https://github.com/miekg/dns/blob/master/msg.go#L127)
	case "NXDOMAIN":
		return nil, fmt.Errorf("Domain was not found. (NXDOMAIN)")
	}

	for _, a := range in.Answer {
		r, err := FormatDNSAnswer(a)
		if err != nil {
			return nil, err
		}
		for _, rp := range r {
			responses = append(responses, ":"+strings.TrimSpace(rp))
		}
	}

	sort.Strings(responses)

	return responses, nil
}

//FormatDNSAnswer desc
func FormatDNSAnswer(a interface{}) (r []string, err error) {
	switch a.(type) {
	case *dns.A:
		r = []string{a.(*dns.A).A.String()}
	case *dns.AAAA:
		r = []string{a.(*dns.AAAA).AAAA.String()}
	case *dns.CNAME:
		r = []string{a.(*dns.CNAME).Target}
	case *dns.MX:
		r = []string{fmt.Sprintf("%d %s", a.(*dns.MX).Preference, a.(*dns.MX).Mx)}
	case *dns.NS:
		r = []string{a.(*dns.NS).Ns}
	case *dns.PTR:
		r = []string{a.(*dns.PTR).Ptr}
	case *dns.TXT:
		r = a.(*dns.TXT).Txt
	case *dns.SRV:
		r = []string{fmt.Sprintf("%d %d %d %s",
			a.(*dns.SRV).Priority,
			a.(*dns.SRV).Weight,
			a.(*dns.SRV).Port,
			a.(*dns.SRV).Target,
		)}
	default:
		err = fmt.Errorf("Got an unexpected answer type: %s", a.(dns.RR).String())
	}

	return
}