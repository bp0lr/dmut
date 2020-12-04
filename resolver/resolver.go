package resolver

import (	
	//"fmt"

	miekg "github.com/miekg/dns"
	dns "github.com/bp0lr/dmut/dns"
)

//JobResponse desc
type JobResponse struct {
	Domain	string
	Status 	bool
	Data 	dns.DNSData
}

//GetDNSQueryResponse desc
func GetDNSQueryResponse(fqdn string, dnsTimeOut int, retries int, errorLimit int) (JobResponse, error) {
	var res JobResponse
	res.Domain = fqdn

	qtype:= []uint16 {miekg.TypeCNAME, miekg.TypeA}

	// I prefer to make a single query for each type, stop it if a have found something to win some milliseconds.
	for _, value := range qtype{		
		dnsClient := dns.New(dnsTimeOut, retries, errorLimit)
		ips, err := dnsClient.Query(fqdn, value)
		if err == nil {
			res.Status = true
			res.Data = *ips

			return res, nil
		}
	}

	/*
	dnsClient := dns.New(dnsServers, dnsTimeOut, retries)
	ips, err := dnsClient.QueryMultiple(fqdn, qtype)
	if err != nil {
		fmt.Printf("err: %v\n", err)
		res.Status = false
		return res, err
	}
	*/

	res.Status = false
	return res, nil
}
