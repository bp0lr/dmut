package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bp0lr/dmut/internal/linesort"
	"github.com/bp0lr/dmut/tables"
	"github.com/bp0lr/dmut/util"
)

func (a *application) saveGenerated(ctx context.Context, stdin io.Reader) error {
	started := time.Now()
	if err := util.ReplaceFileContext(ctx, a.cfg.SaveTo, func(out io.Writer) error {
		return a.generateOffline(ctx, stdin, out)
	}); err != nil {
		fmt.Fprintf(a.diag, "Offline generation stopped after %d complete domain(s); destination %s was not replaced.\n", a.offlineDomains, a.cfg.SaveTo)
		return fmt.Errorf("save generated names: %w", err)
	}
	_, err := fmt.Fprintf(a.diag, "Generated %d names from %d domain(s) in %.2fs; saved to %s\n", a.offlineNames, a.offlineDomains, time.Since(started).Seconds(), a.cfg.SaveTo)
	return err
}

func (a *application) generateOffline(ctx context.Context, stdin io.Reader, out io.Writer) (err error) {
	parent, err := filepath.Abs(filepath.Dir(a.cfg.SaveTo))
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp(parent, ".dmut-work-*")
	if err != nil {
		return fmt.Errorf("create temporary workspace: %w", err)
	}
	// Cleanup happens before the ReplaceFileContext callback returns, so a
	// cleanup failure also leaves the previous destination in place.
	defer func() { err = errors.Join(err, os.RemoveAll(dir)) }()
	dictionary := filepath.Join(dir, "dictionary.txt")
	if err := snapshotDictionary(ctx, a.cfg.Dictionary, dictionary); err != nil {
		return err
	}
	w := bufio.NewWriterSize(out, 64<<10)
	var domains uint64
	visit := func(domain string) error {
		if err := a.writeOfflineDomain(ctx, domain, dictionary, dir, w); err != nil {
			return fmt.Errorf("process %q: %w", domain, err)
		}
		domains++
		a.offlineDomains++
		return nil
	}
	if a.cfg.Domain != "" {
		err = visit(strings.TrimSpace(a.cfg.Domain))
	} else {
		err = util.ForEachLine(ctx, stdin, visit)
	}
	if err != nil {
		return err
	}
	if domains == 0 {
		return errors.New("no domains supplied; use --url or stdin")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return w.Flush()
}

// snapshotDictionary keeps a normalized snapshot on disk. Each domain can read
// it without retaining the dictionary or depending on a seekable source file.
func snapshotDictionary(ctx context.Context, source, destination string) (err error) {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("read dictionary: %w", err)
	}
	defer func() { err = errors.Join(err, in.Close()) }()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("snapshot dictionary: %w", err)
	}
	defer func() { err = errors.Join(err, out.Close()) }()
	w := bufio.NewWriterSize(out, 64<<10)
	nonempty := false
	if err := util.ForEachLine(ctx, in, func(word string) error {
		nonempty = true
		_, err := fmt.Fprintln(w, word)
		return err
	}); err != nil {
		return fmt.Errorf("snapshot dictionary: %w", err)
	}
	if !nonempty {
		return errors.New("dictionary is empty")
	}
	return w.Flush()
}

func (a *application) writeOfflineDomain(ctx context.Context, domain, dictionary, dir string, out io.Writer) (err error) {
	job, err := parseDomain(domain)
	if err != nil {
		return err
	}
	sorter, err := linesort.New(dir)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, sorter.Close()) }()
	words := func(visit func(string) error) (err error) {
		f, err := os.Open(dictionary)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, f.Close()) }()
		return util.ForEachLine(ctx, f, visit)
	}
	if err := tables.GenerateTo(ctx, job, words, a.cfg.Permutations, func(name string) error {
		return sorter.Add(ctx, name)
	}); err != nil {
		return err
	}
	// A fresh sorter per input domain preserves the legacy deduplication scope.
	count, err := sorter.WriteTo(ctx, out)
	if err == nil {
		a.offlineNames += count
	}
	return err
}
