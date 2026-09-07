package util

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxDownloadBytes int64 = 32 << 20

// ReadLines trims whitespace and ignores blank lines. Oversized lines are errors.
func ReadLines(r io.Reader) ([]string, error) {
	var lines []string
	err := ForEachLine(context.Background(), r, func(line string) error {
		lines = append(lines, line)
		return nil
	})
	return lines, err
}

// ForEachLine visits trimmed, nonempty lines without retaining the input.
// The scanner keeps the existing 1 MiB per-line limit. Cancellation is checked
// between reads; callers own interrupting a reader that blocks inside Read.
func ForEachLine(ctx context.Context, r io.Reader, visit func(string) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 4096), 1<<20)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !sc.Scan() {
			if err := ctx.Err(); err != nil {
				return err
			}
			return sc.Err()
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if line := strings.TrimSpace(sc.Text()); line != "" {
			if err := visit(line); err != nil {
				return err
			}
		}
	}
}

// ReadFileLines reads a text list and reports read and close errors.
func ReadFileLines(name string) (lines []string, err error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	return ReadLines(f)
}

// ReplaceFile writes to a temporary file in the destination directory, then
// renames it into place. A failed write leaves the previous destination intact.
// Rename atomicity depends on the operating system and filesystem.
func ReplaceFile(name string, write func(io.Writer) error) (err error) {
	return ReplaceFileContext(context.Background(), name, write)
}

// ReplaceFileContext also checks cancellation before committing the new file.
func ReplaceFileContext(ctx context.Context, name string, write func(io.Writer) error) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(name), ".dmut-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		_ = f.Close()
		if err != nil {
			_ = os.Remove(tmp)
		}
	}()
	if err = write(f); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return os.Rename(tmp, name)
}

// DownloadFile downloads a configuration file using the default timeout.
func DownloadFile(url, saveName string) (string, error) {
	return DownloadFileContext(context.Background(), url, saveName)
}

// DownloadFileContext replaces a configuration file only after a complete,
// nonempty successful HTTP response has been saved within the size limit.
func DownloadFileContext(ctx context.Context, url, saveName string) (string, error) {
	if saveName == "." || saveName == ".." || saveName == "" || filepath.Base(saveName) != saveName || strings.ContainsAny(saveName, `/\`) {
		return "", errors.New("download name must be a file name without directories")
	}
	dir, err := GetDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := filepath.Join(dir, saveName)
	client := &http.Client{Timeout: 30 * time.Second}
	if err := download(ctx, client, url, name, maxDownloadBytes); err != nil {
		return "", fmt.Errorf("download %s: %w", saveName, err)
	}
	return name, nil
}

func download(ctx context.Context, client *http.Client, url, name string, limit int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP status %s", resp.Status)
	}
	if resp.ContentLength > limit {
		return fmt.Errorf("response exceeds %d bytes", limit)
	}
	return ReplaceFile(name, func(w io.Writer) error {
		n, err := io.Copy(w, io.LimitReader(resp.Body, limit+1))
		if err != nil {
			return err
		}
		if n == 0 {
			return errors.New("empty response")
		}
		if n > limit {
			return fmt.Errorf("response exceeds %d bytes", limit)
		}
		if err := resp.Body.Close(); err != nil {
			return err
		}
		return ctx.Err()
	})
}
