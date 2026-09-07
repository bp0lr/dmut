package linesort

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestDeduplicateAcrossMultipleMergePasses(t *testing.T) {
	s, err := newSorter(t.TempDir(), 90, 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	var expected []string
	for i := 0; i < 160; i++ {
		line := fmt.Sprintf("record-%03d", (i*17)%43)
		expected = append(expected, line)
		if err := s.Add(context.Background(), line); err != nil {
			t.Fatal(err)
		}
		if s.bytes > 90 || len(s.chunk) > 3 {
			t.Fatal("chunk grew past its budget")
		}
	}
	if s.runs <= uint64(s.fanIn*s.fanIn) {
		t.Fatal("fixture must require multiple merge passes")
	}
	slices.Sort(expected)
	expected = slices.Compact(expected)
	var out bytes.Buffer
	count, err := s.WriteTo(context.Background(), &out)
	if err != nil {
		t.Fatal(err)
	}
	if count != uint64(len(expected)) || out.String() != strings.Join(expected, "\n")+"\n" {
		t.Fatalf("incorrect unique output: %d, %q", count, out.String())
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("old runs were retained: %v, %v", entries, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary directory remains: %v", err)
	}
}

func TestEmptyInputAndRecordsLargerThanBuffer(t *testing.T) {
	for _, records := range [][]string{nil, {""}, {strings.Repeat("x", 200000), "a", "", "a", strings.Repeat("x", 200000), "ends-in-\r"}} {
		s, err := newSorter(t.TempDir(), 32, 2, 2)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := s.Close(); err != nil {
				t.Error(err)
			}
		})
		for _, record := range records {
			if err := s.Add(context.Background(), record); err != nil {
				t.Fatal(err)
			}
		}
		var out bytes.Buffer
		count, err := s.WriteTo(context.Background(), &out)
		if err != nil {
			t.Fatal(err)
		}
		want := slices.Clone(records)
		slices.Sort(want)
		want = slices.Compact(want)
		expected := strings.Join(want, "\n")
		if len(want) > 0 {
			expected += "\n"
		}
		if count != uint64(len(want)) || out.String() != expected {
			t.Fatal("lost or truncated records")
		}
	}
}

type errorWriter struct{ err error }

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return 0, nil }

func TestSortReportsIOFailures(t *testing.T) {
	want := errors.New("disk full")
	for _, writer := range []io.Writer{errorWriter{want}, shortWriter{}} {
		s, err := newSorter(t.TempDir(), 1, 1, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range []string{"z", "a", "z"} {
			if err := s.Add(context.Background(), line); err != nil {
				t.Fatal(err)
			}
		}
		_, err = s.WriteTo(context.Background(), writer)
		if !errors.Is(err, want) && !errors.Is(err, io.ErrShortWrite) {
			t.Fatal(err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
	s, err := newSorter(t.TempDir(), 1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate an unavailable temporary destination without platform-specific
	// permissions or filling the actual disk.
	if err := os.Remove(s.dir); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(context.Background(), "record"); err == nil {
		t.Fatal("spill failure ignored")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTruncatedRunIsAnError(t *testing.T) {
	s, err := newSorter(t.TempDir(), 1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Add(context.Background(), "record"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.path(0, 0), []byte("incomplete"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.WriteTo(context.Background(), io.Discard); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal(err)
	}
}

type cancelWriter struct{ cancel context.CancelFunc }

func (w cancelWriter) Write(p []byte) (int, error) { w.cancel(); return len(p), nil }

func TestSortCancellation(t *testing.T) {
	s, err := newSorter(t.TempDir(), 1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"b", "a", "c"} {
		if err := s.Add(context.Background(), line); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := s.WriteTo(ctx, cancelWriter{cancel}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	s, err = New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Add(ctx, "a"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := s.WriteTo(ctx, io.Discard); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestCancellationDuringMerge(t *testing.T) {
	s, err := newSorter(t.TempDir(), 1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"b", "a"} {
		if err := s.Add(context.Background(), line); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = merge(ctx, []string{s.path(0, 0), s.path(0, 1)}, cancelWriter{cancel})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}
