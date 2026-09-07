package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bp0lr/dmut/dns"
	mdns "github.com/miekg/dns"
)

type queryFunc func(context.Context, string, uint16, string) (*dns.DNSData, error)

func (f queryFunc) Query(ctx context.Context, host string, kind uint16, server string) (*dns.DNSData, error) {
	return f(ctx, host, kind, server)
}

func TestDNSFailureReportsProgressWithoutStatsFlag(t *testing.T) {
	for _, phase := range []string{"wildcard", "resolution"} {
		t.Run(phase, func(t *testing.T) {
			dir := t.TempDir()
			words, output := filepath.Join(dir, "words"), filepath.Join(dir, "results")
			if err := os.WriteFile(words, []byte("stage\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(output, []byte("previous\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			var out, diag bytes.Buffer
			a := newApplication(config{Dictionary: words, Domain: "test.example.com", Output: output, DNSServers: "127.0.0.1", Workers: 1, TimeoutMS: 10, Retries: 2, ErrorLimit: 25}, &out, &diag)
			cause := errors.New("test timeout")
			a.client = queryFunc(func(_ context.Context, host string, kind uint16, server string) (*dns.DNSData, error) {
				if phase == "resolution" && server != "" {
					return &dns.DNSData{StatusCode: "NXDOMAIN"}, nil
				}
				return nil, &dns.QueryError{Host: host, Type: kind, Server: "127.0.0.1:53", Attempts: 2, TimeoutMS: 10, Err: cause}
			})
			err := a.run(context.Background(), nil)
			if !errors.Is(err, cause) {
				t.Fatal(err)
			}
			for _, text := range []string{"Failed", "DNS jobs completed: 0/", "incomplete:", "Configured resolvers: 1/1 enabled", "exhausted its attempts"} {
				if !strings.Contains(diag.String(), text) {
					t.Errorf("missing %q: %s", text, &diag)
				}
			}
			if phase == "wildcard" && (!strings.Contains(err.Error(), "wildcard check (fixed resolver") || !strings.Contains(diag.String(), "partial count")) {
				t.Fatalf("wildcard context missing: %v; %s", err, &diag)
			}
			if phase == "resolution" && !strings.Contains(diag.String(), "run incomplete; existing results preserved") {
				t.Fatal(diag.String())
			}
			got, err := os.ReadFile(output)
			if err != nil || string(got) != "previous\n" || out.Len() != 0 {
				t.Fatalf("error polluted results: %q; stdout %q; %v", got, &out, err)
			}
		})
	}
}

func TestDNSSuccessAndCancellationSummaries(t *testing.T) {
	for _, cancelRun := range []bool{false, true} {
		words := filepath.Join(t.TempDir(), "words")
		if err := os.WriteFile(words, []byte("stage\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		var diag bytes.Buffer
		a := newApplication(config{Dictionary: words, Domain: "test.example.com", DNSServers: "127.0.0.1", Workers: 1, TimeoutMS: 10, Retries: 1, ErrorLimit: 25}, io.Discard, &diag)
		ctx, cancel := context.WithCancel(context.Background())
		a.client = queryFunc(func(_ context.Context, _ string, _ uint16, server string) (*dns.DNSData, error) {
			if server == "" && cancelRun {
				cancel()
				return nil, context.Canceled
			}
			return &dns.DNSData{StatusCode: mdns.RcodeToString[mdns.RcodeNameError]}, nil
		})
		err := a.run(ctx, nil)
		cancel()
		if cancelRun {
			if !errors.Is(err, context.Canceled) || !strings.Contains(diag.String(), "Canceled") || a.completed != 0 {
				t.Fatalf("%v; %s", err, &diag)
			}
		} else if err != nil || a.completed == 0 || !strings.Contains(diag.String(), "incomplete: 0") {
			t.Fatalf("%v; %s", err, &diag)
		}
	}
}
