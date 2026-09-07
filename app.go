package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bp0lr/dmut/dns"
	dnsmanager "github.com/bp0lr/dmut/dnsManager"
	"github.com/bp0lr/dmut/util"
	"github.com/cheggaaa/pb/v3"
)

// checkedWriter serializes writes and retains the first error, including errors
// from progress rendering, whose API does not return write failures.
type checkedWriter struct {
	mu  sync.Mutex
	w   io.Writer
	err error
}

func (w *checkedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.w.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	w.err = err
	return n, err
}

func (w *checkedWriter) Err() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}

type application struct {
	cfg                   config
	out, diag             *checkedWriter
	pool                  *dnsmanager.Pool
	client, confirmation  *dns.Client
	mu                    sync.Mutex
	works                 []string
	valid, skipped, found int
}

func newApplication(cfg config, stdout, stderr io.Writer) *application {
	return &application{cfg: cfg, out: &checkedWriter{w: stdout}, diag: &checkedWriter{w: stderr}}
}

func (a *application) run(ctx context.Context, stdin io.Reader) (err error) {
	defer func() { err = errors.Join(err, a.out.Err(), a.diag.Err()) }()
	if err := ctx.Err(); err != nil {
		return err
	}
	if a.cfg.UpdateDNS || a.cfg.UpdateFiles {
		return a.updateFiles(ctx)
	}
	if a.cfg.SaveOnly {
		return a.saveGenerated(ctx, stdin)
	}
	words, err := util.ReadFileLines(a.cfg.Dictionary)
	if err != nil {
		return fmt.Errorf("read dictionary: %w", err)
	}
	if len(words) == 0 {
		return errors.New("dictionary is empty")
	}
	var domains []string
	if a.cfg.Domain != "" {
		domains = []string{strings.TrimSpace(a.cfg.Domain)}
	} else {
		domains, err = util.ReadLines(stdin)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
	}
	if len(domains) == 0 {
		return errors.New("no domains supplied; use --url or stdin")
	}
	if !a.cfg.SaveOnly {
		hosts, err := a.resolverHosts()
		if err != nil {
			return err
		}
		a.pool, err = dnsmanager.New(hosts)
		if err != nil {
			return err
		}
		a.client = dns.New(a.pool, a.cfg.TimeoutMS, a.cfg.Retries, a.cfg.ErrorLimit)
		// Preserve the existing confirmation settings.
		a.confirmation = dns.New(a.pool, 500, 3, 10)
	}
	if err := runJobs(ctx, 30, domains, func(ctx context.Context, domain string) error {
		names, skipped, err := a.generateTable(ctx, domain, words)
		if err != nil {
			return fmt.Errorf("process %q: %w", domain, err)
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if skipped {
			a.skipped++
		} else {
			a.valid++
		}
		a.works = append(a.works, names...)
		return nil
	}); err != nil {
		return err
	}
	var output *os.File
	if a.cfg.Output != "" {
		output, err = os.OpenFile(a.cfg.Output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("open output: %w", err)
		}
		defer func() { err = errors.Join(err, output.Close()) }()
	}
	var bar *pb.ProgressBar
	if a.cfg.Progress {
		bar = pb.New(len(a.works)).SetWriter(a.diag).Start()
		defer bar.Finish()
	}
	if a.cfg.Verbose {
		fmt.Fprintf(a.diag, "Domains: %d; valid: %d; skipped: %d; generated: %d\n", len(domains), a.valid, a.skipped, len(a.works))
	}
	if err := runJobs(ctx, a.cfg.Workers, a.works, func(ctx context.Context, domain string) error {
		if bar != nil {
			defer bar.Increment()
		}
		return a.processDNS(ctx, domain, output)
	}); err != nil {
		return err
	}
	if a.cfg.Stats {
		for _, entry := range a.pool.Snapshot() {
			fmt.Fprintf(a.diag, "Resolver: %s; errors: %d; enabled: %t\n", entry.Host, entry.Errors, entry.Status)
		}
		fmt.Fprintf(a.diag, "Domains: %d; words: %d; generated: %d; found: %d\n", len(domains), len(words), len(a.works), a.found)
	}
	return ctx.Err()
}

func (a *application) resolverHosts() ([]string, error) {
	if a.cfg.DNSServers != "" {
		return strings.Split(a.cfg.DNSServers, ","), nil
	}
	if a.cfg.DNSFile != "" {
		hosts, err := util.ReadFileLines(a.cfg.DNSFile)
		if err != nil {
			return nil, fmt.Errorf("read resolver file: %w", err)
		}
		return hosts, nil
	}
	dir, err := util.GetDir()
	if err != nil {
		return nil, err
	}
	hosts, err := util.ReadFileLines(filepath.Join(dir, "resolvers.txt"))
	if err == nil {
		return hosts, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read default resolver file: %w", err)
	}
	return []string{"1.1.1.1:53", "1.0.0.1:53", "8.8.8.8:53", "8.8.4.4:53", "9.9.9.9:53"}, nil
}

func (a *application) updateFiles(ctx context.Context) error {
	files := []struct{ url, name string }{
		{"https://raw.githubusercontent.com/bp0lr/dmut-resolvers/main/resolvers.txt", "resolvers.txt"},
		{"https://raw.githubusercontent.com/bp0lr/dmut-resolvers/main/top20.txt", "top20.txt"},
	}
	if a.cfg.UpdateFiles {
		files = append(files, struct{ url, name string }{"https://raw.githubusercontent.com/bp0lr/dmut/main/words.txt", "words.txt"})
	}
	for _, file := range files {
		name, err := util.DownloadFileContext(ctx, file.url, file.name)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(a.diag, "Updated %s\n", name); err != nil {
			return err
		}
	}
	return nil
}

// runJobs cancels outstanding work on the first error and joins every worker.
func runJobs(ctx context.Context, workers int, jobs []string, fn func(context.Context, string) error) error {
	if workers < 1 {
		return errors.New("worker count must be positive")
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	tasks := make(chan string)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case task, ok := <-tasks:
					if !ok || ctx.Err() != nil {
						return
					}
					if err := fn(ctx, task); err != nil {
						cancel(err)
						return
					}
				}
			}
		}()
	}
send:
	for _, job := range jobs {
		select {
		case <-ctx.Done():
			break send
		case tasks <- job:
		}
	}
	close(tasks)
	wg.Wait()
	return context.Cause(ctx)
}
