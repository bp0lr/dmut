package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bp0lr/dmut/defines"
	"github.com/bp0lr/dmut/tables"
	"github.com/bp0lr/dmut/util"
)

func TestOfflinePreservesPerDomainResults(t *testing.T) {
	dir := t.TempDir()
	words, output := filepath.Join(dir, "words.txt"), filepath.Join(dir, "generated.txt")
	if err := os.WriteFile(words, []byte(" stage \r\ndev\r\n\r\nstage\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Repeated domains must remain repeated: legacy deduplication is per input
	// domain, not global. Domain order is now input order, with sorted names.
	domains := []string{"test.example.com", "a.b.example.co.uk", "example.com", "test.example.com"}
	for mask := 0; mask < 8; mask++ {
		flags := defines.PermutationList{AddToDomain: mask&1 != 0, AddNumbers: mask&2 != 0, AddSeparator: mask&4 != 0}
		a := newApplication(config{Dictionary: words, SaveTo: output, SaveOnly: true, Permutations: flags}, io.Discard, io.Discard)
		if err := a.run(context.Background(), strings.NewReader(strings.Join(domains, "\r\n"))); err != nil {
			t.Fatal(err)
		}
		var expected strings.Builder
		for _, domain := range domains {
			job, err := parseDomain(domain)
			if err != nil {
				t.Fatal(err)
			}
			names := tables.GenerateTables(job, []string{"stage", "dev", "stage"}, flags)
			slices.Sort(names)
			for _, name := range names {
				fmt.Fprintln(&expected, name)
			}
		}
		got, err := os.ReadFile(output)
		if err != nil || string(got) != expected.String() {
			t.Fatalf("mask %d: results changed: %v", mask, err)
		}
		if len(a.works) != 0 {
			t.Fatal("offline run retained generated work")
		}
		assertOfflineTempsRemoved(t, dir)
	}
}

func TestOfflineLateInputFailurePreservesDestination(t *testing.T) {
	dir := t.TempDir()
	words, output := filepath.Join(dir, "words"), filepath.Join(dir, "generated")
	if err := os.WriteFile(words, []byte("stage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("previous"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := newApplication(config{Dictionary: words, SaveTo: output, SaveOnly: true}, io.Discard, io.Discard)
	err := a.run(context.Background(), strings.NewReader("test.example.com\nhttps://invalid.example/path\n"))
	if err == nil {
		t.Fatal("late input failure was ignored")
	}
	got, err := os.ReadFile(output)
	if err != nil || string(got) != "previous" {
		t.Fatalf("previous file was replaced: %q, %v", got, err)
	}
	assertOfflineTempsRemoved(t, dir)
}

type cancelOutput struct {
	w      io.Writer
	cancel context.CancelFunc
}

func (w cancelOutput) Write(p []byte) (int, error) { n, err := w.w.Write(p); w.cancel(); return n, err }

func TestOfflineOutputFailureAndCancellationPreserveDestination(t *testing.T) {
	dir := t.TempDir()
	words, output := filepath.Join(dir, "words"), filepath.Join(dir, "generated")
	var dictionary strings.Builder
	for i := 0; i < 1500; i++ {
		fmt.Fprintf(&dictionary, "word%04d\n", i)
	}
	if err := os.WriteFile(words, []byte(dictionary.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	want := errors.New("disk full")
	for _, cancelled := range []bool{false, true} {
		if err := os.WriteFile(output, []byte("previous"), 0o600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		a := newApplication(config{Dictionary: words, SaveTo: output, Domain: "test.example.com", SaveOnly: true}, io.Discard, io.Discard)
		err := util.ReplaceFileContext(ctx, output, func(out io.Writer) error {
			if cancelled {
				return a.generateOffline(ctx, nil, cancelOutput{out, cancel})
			}
			return a.generateOffline(ctx, nil, failingWriter{want})
		})
		cancel()
		if cancelled && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if !cancelled && !errors.Is(err, want) {
			t.Fatal(err)
		}
		got, err := os.ReadFile(output)
		if err != nil || string(got) != "previous" {
			t.Fatalf("previous file was replaced: %q, %v", got, err)
		}
		assertOfflineTempsRemoved(t, dir)
	}
}

type stagedDomains struct {
	readFirst bool
	out       *bytes.Buffer
}

func (r *stagedDomains) Read(p []byte) (int, error) {
	if !r.readFirst {
		r.readFirst = true
		return copy(p, "test.example.com\n"), nil
	}
	if r.out.Len() == 0 {
		return 0, errors.New("read next domain before processing the previous one")
	}
	return 0, io.EOF
}

func TestOfflineProcessesDomainsBeforeReadingWholeInput(t *testing.T) {
	dir := t.TempDir()
	words := filepath.Join(dir, "words")
	var dictionary strings.Builder
	for i := 0; i < 1500; i++ {
		fmt.Fprintf(&dictionary, "word%04d\n", i)
	}
	if err := os.WriteFile(words, []byte(dictionary.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	a := newApplication(config{Dictionary: words, SaveTo: filepath.Join(dir, "generated"), SaveOnly: true}, io.Discard, io.Discard)
	if err := a.generateOffline(context.Background(), &stagedDomains{out: &out}, &out); err != nil {
		t.Fatal(err)
	}
	assertOfflineTempsRemoved(t, dir)
}

func assertOfflineTempsRemoved(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".dmut-") {
			t.Errorf("temporary data remains: %s", entry.Name())
		}
	}
}
