// Package resolver adapts DNS records to application responses.
package resolver

import (
	"context"
	"fmt"

	"github.com/bp0lr/dmut/dns"
)

// JobResponse describes a completed DNS response. Status does not imply a match;
// callers must still inspect the response code and the requested records.
type JobResponse struct {
	Domain string
	Status bool
	Data   dns.DNSData
}

// Querier allows tests to supply responses without network access.
type Querier interface {
	Query(context.Context, string, uint16, string) (*dns.DNSData, error)
}

// GetDNSQueryResponse preserves query errors for the caller.
func GetDNSQueryResponse(ctx context.Context, client Querier, fqdn string, qType uint16, server string) (JobResponse, error) {
	result := JobResponse{Domain: fqdn}
	data, err := client.Query(ctx, fqdn, qType, server)
	if err != nil {
		return result, fmt.Errorf("resolve %s: %w", fqdn, err)
	}
	if data == nil {
		return result, fmt.Errorf("resolve %s: empty response", fqdn)
	}
	result.Status = true
	result.Data = *data
	return result, nil
}
