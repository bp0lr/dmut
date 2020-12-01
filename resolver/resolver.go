package resolver

import (
	"fmt"
	"net"
	"time"
	//"sort"
	//"strings"

	"github.com/miekg/dns"
)

func exchangeWithRetry(c *dns.Client, m *dns.Msg, server string) (*dns.Msg, error) {
	r, _, err := c.Exchange(m, server)
	if err != nil {
		for i := 0; i < 3; i++ {			
			fmt.Printf("Retry %v: %v\n", i, server)
			r, _, err = c.Exchange(m, server)
			if err == nil {
				return r, err				
			}
		}
		
	}
	return r, err
}


//GetDNSQueryResponse desc
func GetDNSQueryResponse(queryType string, fqdn string, dnsServer string, dnsTimeout int) (string, error) {
	qt, ok := dns.StringToType[queryType]
	if !ok {
		return "", fmt.Errorf("Query type '%s' is an unknown type", queryType)
	}
	
	c := new(dns.Client)	
	c.Dialer = &net.Dialer{
		Timeout: time.Duration(dnsTimeout) * time.Millisecond,
	}

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(fqdn), qt)
	
	//in, _ , err := c.Exchange(m, dnsServer)
	in, err := exchangeWithRetry(c, m, dnsServer)
	if err != nil {
		fmt.Printf("err0: %v\n", err)
		return "", err
	}

	switch dns.RcodeToString[in.Rcode] {
	// TODO: Catch more error codes (https://github.com/miekg/dns/blob/master/msg.go#L127)
	case "NXDOMAIN":
		return "", fmt.Errorf("%v was not found. (NXDOMAIN)", fqdn)
	case "NOERROR":
		break
	default: 
		fmt.Printf("[%v] %v\n", fqdn, dns.RcodeToString[in.Rcode])	
		break
	}

	//confirm CNAME
	for _, record := range in.Answer {
		if _, ok := record.(*dns.CNAME); ok {
			fmt.Printf("CNAME found: %v\n", string(record.(*dns.CNAME).Target))
			return string(record.(*dns.CNAME).Target), nil
		}
	}

	//confirm A
	for _, record := range in.Answer {
		if _, ok := record.(*dns.A); ok {		
			fmt.Printf("A found: %v\n", record.(*dns.A).A.String())
			return record.(*dns.A).A.String(), nil
		}
	}

	//confirm AAAA
	for _, record := range in.Answer {
		if _, ok := record.(*dns.AAAA); ok {
			fmt.Printf("AAAA found: %v\n", record.(*dns.AAAA).AAAA.String())
			return record.(*dns.AAAA).AAAA.String(), nil
		}
	}

	return "", nil
}

/*
//FormatDNSAnswer desc
func FormatDNSAnswer(a interface{}) (r []string, err error) {
	fmt.Printf("res: %v\n", r)
	switch a.(type) {
	case *dns.A:
		r = []string{a.(*dns.A).A.String()}
	case *dns.AAAA:
		r = []string{a.(*dns.AAAA).AAAA.String()}
	case *dns.CNAME:
		r = []string{a.(*dns.CNAME).Target}
	//case *dns.MX:
	//	r = []string{fmt.Sprintf("%d %s", a.(*dns.MX).Preference, a.(*dns.MX).Mx)}
	case *dns.NS:
		r = []string{a.(*dns.NS).Ns}
	//case *dns.PTR:
	//	r = []string{a.(*dns.PTR).Ptr}
	//case *dns.TXT:
	//	r = a.(*dns.TXT).Txt
	//case *dns.SRV:
	//	r = []string{fmt.Sprintf("%d %d %d %s",
	//		a.(*dns.SRV).Priority,
	//		a.(*dns.SRV).Weight,
	//		a.(*dns.SRV).Port,
	//		a.(*dns.SRV).Target,
	//	)}
	default:
		err = fmt.Errorf("Got an unexpected answer type: %s", a.(dns.RR).String())
	}

	return
}
*/