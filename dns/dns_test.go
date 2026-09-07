package dns

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	dnsmanager "github.com/bp0lr/dmut/dnsManager"
	mdns "github.com/miekg/dns"
)

func TestQueryCancellationAndMissingPool(t *testing.T) {
	c := New(nil, 500, 3, 25)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Query(ctx, "example.com", mdns.TypeA, ""); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := c.Query(context.Background(), "example.com", mdns.TypeA, ""); !errors.Is(err, dnsmanager.ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := New(nil, 0, 0, 0).Query(context.Background(), "example.com", mdns.TypeA, ""); err == nil {
		t.Fatal("invalid settings accepted")
	}
}

func TestLocalDNSErrorResponses(t *testing.T) {
	for _, code := range []int{mdns.RcodeNameError, mdns.RcodeServerFailure} {
		t.Run(mdns.RcodeToString[code], func(t *testing.T) {
			conn, err := net.ListenPacket("udp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			started := make(chan struct{})
			server := &mdns.Server{PacketConn: conn, NotifyStartedFunc: func() { close(started) }, Handler: mdns.HandlerFunc(func(w mdns.ResponseWriter, req *mdns.Msg) {
				resp := new(mdns.Msg)
				resp.SetRcode(req, code)
				_ = w.WriteMsg(resp)
			})}
			done := make(chan error, 1)
			go func() { done <- server.ActivateAndServe() }()
			select {
			case <-started:
			case err := <-done:
				t.Fatal(err)
			case <-time.After(5 * time.Second):
				t.Fatal("server did not start")
			}
			defer func() {
				_ = server.Shutdown()
				if err := <-done; err != nil {
					t.Error(err)
				}
			}()
			pool, err := dnsmanager.New([]string{conn.LocalAddr().String()})
			if err != nil {
				t.Fatal(err)
			}
			data, err := New(pool, 1000, 1, 25).Query(context.Background(), "example.com", mdns.TypeA, "")
			if code == mdns.RcodeServerFailure {
				if err == nil || !strings.Contains(err.Error(), "SERVFAIL") || data != nil {
					t.Fatalf("%v, %v", data, err)
				}
				var queryError *QueryError
				if !errors.As(err, &queryError) || queryError.Attempts != 1 || queryError.Server != conn.LocalAddr().String() || queryError.Type != mdns.TypeA {
					t.Fatalf("query context lost: %v", err)
				}
				if errors.Is(err, dnsmanager.ErrUnavailable) {
					t.Fatal("one failed query reported the whole pool unavailable")
				}
				// Two failures disable this resolver before a third attempt.
				pool, _ = dnsmanager.New([]string{conn.LocalAddr().String()})
				_, err = New(pool, 1000, 5, 1).Query(context.Background(), "example.com", mdns.TypeCNAME, "")
				if !errors.Is(err, dnsmanager.ErrUnavailable) || !errors.As(err, &queryError) || queryError.Attempts != 2 || !strings.Contains(err.Error(), "SERVFAIL") {
					t.Fatalf("pool exhaustion lost the original cause: %v", err)
				}
			} else if err != nil || data == nil || data.StatusCode != "NXDOMAIN" {
				t.Fatalf("%v, %v", data, err)
			}
		})
	}
}

func TestLocalTimeoutPreservesNetworkCause(t *testing.T) {
	// An open loopback socket that never replies gives a real timeout without
	// relying on an external resolver or an unused port's platform behavior.
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	server := conn.LocalAddr().String()
	_, err = New(nil, 30, 1, 25).Query(context.Background(), "test.example.com", mdns.TypeA, server)
	var queryError *QueryError
	var networkError net.Error
	if !errors.As(err, &queryError) || !errors.As(err, &networkError) || !networkError.Timeout() || queryError.Attempts != 1 || queryError.Server != server || queryError.TimeoutMS != 30 {
		t.Fatalf("timeout context missing: %v", err)
	}
}
