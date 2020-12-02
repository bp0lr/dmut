package resolver

import (
	"fmt"
	//"net"
	//"time"
	
	//"github.com/miekg/dns"
	//dns "github.com/projectdiscovery/retryabledns"	
	dns "github.com/bp0lr/dmut/dns"
)

//JobResponse desc
type JobResponse struct {
	Domain	string
	Status 	bool
	Data 	dns.DNSData
}

//GetDNSQueryResponse desc
func GetDNSQueryResponse(fqdn string, dnsServers []string, dnsTimeOut int, retries int) (JobResponse, error) {
	var res JobResponse
	res.Domain = fqdn

	dnsClient := dns.New(dnsServers, dnsTimeOut, retries)
	ips, err := dnsClient.Resolve(fqdn)
	if err != nil {
		fmt.Printf("err: %v\n", err)
		res.Status = false
		return res, err
	}

	res.Status = true
	res.Data = *ips

	return res, nil
}
