package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bp0lr/dmut/resolver"
)

func TestConfigCompatibility(t *testing.T) {
	legacy, _, err := parseConfig([]string{"-d", "words.txt", "--dnsFile", "servers.txt", "--dnsServers", "127.0.0.1", "--dns-errorLimit", "4"}, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	modern, _, err := parseConfig([]string{"-d", "words.txt", "--dns-file", "servers.txt", "--dns-servers", "127.0.0.1", "--dns-error-limit", "4"}, io.Discard, io.Discard)
	if err != nil || legacy != modern {
		t.Fatalf("alias mismatch: %v", err)
	}
	for _, args := range [][]string{
		{}, {"-d", "words", "-w", "0"}, {"-d", "words", "-w", "151"},
		{"-d", "words", "--dns-timeout", "-1"}, {"-d", "words", "--dns-timeout", "10001"},
		{"-d", "words", "--dns-retries", "0"}, {"-d", "words", "--dns-error-limit", "0"},
		{"-d", "words", "unexpected"}, {"--unknown"},
	} {
		_, _, err := parseConfig(args, io.Discard, io.Discard)
		var usage *usageError
		if !errors.As(err, &usage) {
			t.Errorf("%v: expected usage error, got %v", args, err)
		}
	}
}

func TestHelpAndVersion(t *testing.T) {
	for _, flag := range []string{"--help", "-h", "--version"} {
		var out bytes.Buffer
		if err := runCLI(context.Background(), []string{flag}, nil, &out, io.Discard); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "dmut") {
			t.Fatal(out.String())
		}
	}
}

func TestSaveOnlyDoesNotLoadResolversOrTouchOutput(t *testing.T) {
	dir := t.TempDir()
	words, generated, output := filepath.Join(dir, "words.txt"), filepath.Join(dir, "generated.txt"), filepath.Join(dir, "results.txt")
	if err := os.WriteFile(words, []byte("stage\r\n\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("previous"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, diag bytes.Buffer
	err := runCLI(context.Background(), []string{"-d", words, "--save-gen", "--save-to", generated, "-o", output, "--dns-file", "missing-file"}, strings.NewReader(" test.example.com\r\n\r\n"), &out, &diag)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(generated)
	if err != nil || len(data) == 0 {
		t.Fatalf("generated = %q, %v", data, err)
	}
	if strings.Contains(string(data), "\r") {
		t.Fatal("CRLF leaked into a domain")
	}
	data, err = os.ReadFile(output)
	if err != nil || string(data) != "previous" {
		t.Fatalf("output changed: %q, %v", data, err)
	}
	if out.Len() != 0 || diag.Len() == 0 {
		t.Fatalf("incorrect output streams: %q, %q", out.String(), diag.String())
	}
}

func TestInvalidInputPreservesGeneratedFile(t *testing.T) {
	dir := t.TempDir()
	words, generated := filepath.Join(dir, "words"), filepath.Join(dir, "generated")
	if err := os.WriteFile(words, []byte("stage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(generated, []byte("previous"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"", "https://example.com/path", strings.Repeat("x", 1<<20)} {
		err := runCLI(context.Background(), []string{"-d", words, "--save-gen", "--save-to", generated}, strings.NewReader(input), io.Discard, io.Discard)
		if err == nil {
			t.Fatal("invalid input accepted")
		}
		data, err := os.ReadFile(generated)
		if err != nil || string(data) != "previous" {
			t.Fatalf("file changed: %q, %v", data, err)
		}
	}
}

func TestEmptyDictionary(t *testing.T) {
	name := filepath.Join(t.TempDir(), "words")
	if err := os.WriteFile(name, []byte("\r\n  \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := runCLI(context.Background(), []string{"-d", name, "--save-gen"}, strings.NewReader("example.com"), io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "dictionary is empty") {
		t.Fatal(err)
	}
}

func TestResolverPrecedence(t *testing.T) {
	a := newApplication(config{DNSFile: "missing-file", DNSServers: "127.0.0.1:53"}, io.Discard, io.Discard)
	hosts, err := a.resolverHosts()
	if err != nil || len(hosts) != 1 || hosts[0] != "127.0.0.1:53" {
		t.Fatalf("%v, %v", hosts, err)
	}
}

func TestRunJobsCancellationJoinsWorkers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{}, 2)
	var running atomic.Int32
	done := make(chan error, 1)
	go func() {
		done <- runJobs(ctx, 2, []string{"a", "b", "c"}, func(ctx context.Context, _ string) error {
			running.Add(1)
			defer running.Add(-1)
			started <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		})
	}()
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("worker did not start")
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || running.Load() != 0 {
			t.Fatalf("%v; workers = %d", err, running.Load())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("workers did not stop")
	}
	want := errors.New("job failed")
	if err := runJobs(context.Background(), 2, []string{"a", "b"}, func(context.Context, string) error { return want }); !errors.Is(err, want) {
		t.Fatal(err)
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestOutputFailureAndConcurrentResults(t *testing.T) {
	want := errors.New("output failed")
	a := newApplication(config{}, failingWriter{want}, io.Discard)
	if err := a.writeResult("example.com", resolver.JobResponse{}, nil); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if a.found != 0 {
		t.Fatal("failed result counted")
	}
	var out bytes.Buffer
	a = newApplication(config{}, &out, io.Discard)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := a.writeResult("example.com", resolver.JobResponse{}, nil); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if a.found != 32 || strings.Count(out.String(), "example.com\n") != 32 {
		t.Fatal("concurrent results lost or interleaved")
	}
}
