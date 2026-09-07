package util

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestReadLines(t *testing.T) {
	got, err := ReadLines(strings.NewReader(" first\r\n\r\n second \n"))
	if err != nil || !reflect.DeepEqual(got, []string{"first", "second"}) {
		t.Fatalf("%v, %v", got, err)
	}
	if _, err := ReadLines(strings.NewReader(strings.Repeat("x", 1<<20))); err == nil {
		t.Fatal("oversized line was accepted")
	}
	want := errors.New("input failed")
	if _, err := ReadLines(failingReader{want}); !errors.Is(err, want) {
		t.Fatalf("lost read error: %v", err)
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestReplaceFilePreservesPreviousOnFailure(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "list.txt")
	if err := os.WriteFile(name, []byte("previous"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := errors.New("disk full")
	err := ReplaceFile(name, func(w io.Writer) error { _, _ = io.WriteString(w, "partial"); return want })
	if !errors.Is(err, want) {
		t.Fatal(err)
	}
	assertFile(t, name, "previous")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary file leaked: %v, %v", entries, err)
	}
	if err := ReplaceFile(name, func(w io.Writer) error { _, err := io.WriteString(w, "complete"); return err }); err != nil {
		t.Fatal(err)
	}
	assertFile(t, name, "complete")
}

func TestReplaceFileCancellationBeforeCommit(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "list.txt")
	if err := os.WriteFile(name, []byte("previous"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := ReplaceFileContext(ctx, name, func(w io.Writer) error {
		if _, err := io.WriteString(w, "complete new file"); err != nil {
			return err
		}
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	assertFile(t, name, "previous")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary file leaked: %v, %v", entries, err)
	}
}

func TestDownload(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		status                      int
		body                        string
		chunked, truncated, wantErr bool
	}{
		{name: "success", status: 200, body: "new list"},
		{name: "status", status: 404, body: "not found", wantErr: true},
		{name: "empty", status: 200, wantErr: true},
		{name: "too large", status: 200, body: strings.Repeat("x", 65), wantErr: true},
		{name: "chunked too large", status: 200, body: strings.Repeat("x", 65), chunked: true, wantErr: true},
		{name: "interrupted", status: 200, body: "partial", truncated: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.truncated {
					w.Header().Set("Content-Length", "32")
				}
				w.WriteHeader(tc.status)
				if tc.chunked {
					w.(http.Flusher).Flush()
				}
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			name := filepath.Join(t.TempDir(), "list.txt")
			if err := os.WriteFile(name, []byte("previous"), 0o600); err != nil {
				t.Fatal(err)
			}
			err := download(context.Background(), server.Client(), server.URL, name, 64)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v", err)
			}
			want := tc.body
			if tc.wantErr {
				want = "previous"
			}
			assertFile(t, name, want)
			entries, err := os.ReadDir(filepath.Dir(name))
			if err != nil || len(entries) != 1 {
				t.Fatalf("temporary file leaked: %v, %v", entries, err)
			}
		})
	}
}

func TestDownloadCancellation(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	name := filepath.Join(t.TempDir(), "list.txt")
	if err := os.WriteFile(name, []byte("previous"), 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- download(ctx, server.Client(), server.URL, name, 64) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("download ignored cancellation")
	}
	assertFile(t, name, "previous")
}

func TestDownloadRejectsPaths(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../list", `..\list`, "/list"} {
		if _, err := DownloadFileContext(context.Background(), "unused", name); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
}

func assertFile(t *testing.T, name, want string) {
	t.Helper()
	got, err := os.ReadFile(name)
	if err != nil || string(got) != want {
		t.Fatalf("file = %q, %v; want %q", got, err, want)
	}
}
