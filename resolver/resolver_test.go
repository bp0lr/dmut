package resolver

import (
	"context"
	"errors"
	"testing"

	"github.com/bp0lr/dmut/dns"
)

type stub struct {
	data *dns.DNSData
	err  error
}

func (s stub) Query(context.Context, string, uint16, string) (*dns.DNSData, error) {
	return s.data, s.err
}

func TestQueryErrorsArePreserved(t *testing.T) {
	want := errors.New("transport failed")
	res, err := GetDNSQueryResponse(context.Background(), stub{err: want}, "example.com", 1, "")
	if !errors.Is(err, want) || res.Status {
		t.Fatalf("%+v, %v", res, err)
	}
	if _, err := GetDNSQueryResponse(context.Background(), stub{}, "example.com", 1, ""); err == nil {
		t.Fatal("nil response accepted")
	}
	res, err = GetDNSQueryResponse(context.Background(), stub{data: &dns.DNSData{StatusCode: "NXDOMAIN"}}, "example.com", 1, "")
	if err != nil || !res.Status || res.Data.StatusCode != "NXDOMAIN" {
		t.Fatalf("%+v, %v", res, err)
	}
}
