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
)

func TestPreviewExplainsRulesWithoutTouchingFilesOrResolvers(t *testing.T) {
	dir := t.TempDir()
	words, generated, output := filepath.Join(dir, "words"), filepath.Join(dir, "generated"), filepath.Join(dir, "output")
	for name, data := range map[string]string{words: "stage\n\ndev\n", generated: "old generated", output: "old results"} {
		if err := os.WriteFile(name, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, diag bytes.Buffer
	err := runCLI(context.Background(), []string{"-u", "test.example.com", "-d", words, "--preview", "--explain", "--preview-limit", "1", "--save-to", generated, "-o", output, "--dns-file", "missing"}, nil, &out, &diag)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"test-0.example.com", "numeric addition", "stage.test.example.com", "word insertion", "test-stage.example.com", "word concatenation / separator", `"stage"`} {
		if !strings.Contains(out.String(), text) {
			t.Errorf("missing %q: %s", text, &out)
		}
	}
	if strings.Count(out.String(), "\n") != 4 || !strings.Contains(diag.String(), "3 examples from the first 1 dictionary entries") {
		t.Fatalf("sample size wrong: %s; %s", &out, &diag)
	}
	for name, expected := range map[string]string{generated: "old generated", output: "old results"} {
		data, err := os.ReadFile(name)
		if err != nil || string(data) != expected {
			t.Fatalf("preview changed %s: %q; %v", name, data, err)
		}
	}
	assertOfflineTempsRemoved(t, dir)
}

func TestPreviewHonorsDisabledRulesAndSamplesOnlyPrefix(t *testing.T) {
	words := filepath.Join(t.TempDir(), "words")
	// A malformed record beyond the requested prefix must not be scanned:
	// preview is a sample, not validation or enumeration of the whole input.
	if err := os.WriteFile(words, []byte("stage\n"+strings.Repeat("x", 1<<20)), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := runCLI(context.Background(), []string{"-u", "test.example.com", "-d", words, "--preview", "--preview-limit", "1", "--disable-addnumbers", "--disable-addseparator"}, nil, &out, io.Discard)
	if err != nil || out.String() != "stage.test.example.com\n" {
		t.Fatalf("%q; %v", &out, err)
	}
	// The root domain has no separator rule, and repeated words do not repeat
	// examples within a rule.
	if err := os.WriteFile(words, []byte("stage\nstage\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	err = runCLI(context.Background(), []string{"-u", "example.com", "-d", words, "--preview", "--disable-addnumbers"}, nil, &out, io.Discard)
	if err != nil || out.String() != "stage.example.com\n" {
		t.Fatalf("%q; %v", &out, err)
	}
}

func TestPreviewConfigurationErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--preview", "-d", "words"},
		{"--explain", "-d", "words"},
		{"--preview-limit", "2", "-d", "words"},
		{"--preview", "-u", "example.com", "-d", "words", "--preview-limit", "0"},
		{"--preview", "-u", "example.com", "-d", "words", "--save-gen"},
		{"--preview", "-u", "example.com", "--update-files"},
		{"--completion", "unsupported"}, {"--completion="},
	} {
		var usage *usageError
		err := runCLI(context.Background(), args, nil, io.Discard, io.Discard)
		if !errors.As(err, &usage) {
			t.Errorf("%v: %v", args, err)
		}
	}
}

func TestPreviewOutputErrorAndEmptyDictionary(t *testing.T) {
	words := filepath.Join(t.TempDir(), "words")
	if err := os.WriteFile(words, []byte("stage\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"--preview", "--explain", "-u", "test.example.com", "-d", words}
	want := errors.New("broken output")
	if err := runCLI(context.Background(), args, nil, failingWriter{want}, io.Discard); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if err := os.WriteFile(words, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(context.Background(), args, nil, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "dictionary is empty") {
		t.Fatal(err)
	}
}

func TestCompletionDoesNotRequireDictionary(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		var out bytes.Buffer
		if err := runCLI(context.Background(), []string{"--completion", shell}, nil, &out, io.Discard); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"preview-limit", "dictionary", "show-stats"} {
			if !strings.Contains(out.String(), name) {
				t.Errorf("%s completion misses %s", shell, name)
			}
		}
	}
}
