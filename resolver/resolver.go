package resolver

import (	
	dns "github.com/bp0lr/dmut/dns"
)

//JobResponse desc
type JobResponse struct {
	Domain	string
	Status 	bool
	Data 	dns.DNSData
}

//GetDNSQueryResponse desc
func GetDNSQueryResponse(fqdn string, qType uint16, dnsTimeOut int, retries int, errorLimit int, customDNSServer string) (JobResponse, error) {
	var res JobResponse
	res.Domain = fqdn

	dnsClient := dns.New(dnsTimeOut, retries, errorLimit)
	resp, err := dnsClient.Query(fqdn, qType, customDNSServer)
	if err == nil {
			res.Status = true
			res.Data = *resp

			return res, nil
	}
	
	res.Status = false
	return res, nil
}
