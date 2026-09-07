package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bp0lr/dmut/dns"
	dnsmanager "github.com/bp0lr/dmut/dnsManager"
)

// Called only after workers and output files have been joined/closed.
func (a *application) reportDNSRun(err error, phase string, domains, words int, outputOpened bool, elapsed time.Duration) {
	status := "Completed"
	if err != nil {
		status = "Failed"
		if errors.Is(err, context.Canceled) {
			status = "Canceled"
		}
	}
	fmt.Fprintf(a.diag, "%s in %.2fs; phase: %s\n", status, elapsed.Seconds(), phase)
	fmt.Fprintf(a.diag, "Domains checked: %d/%d; skipped: %d; words: %d; generated: %d\n", a.valid+a.skipped, domains, a.skipped, words, len(a.works))
	fmt.Fprintf(a.diag, "DNS jobs completed: %d/%d; incomplete: %d; found: %d\n", a.completed, len(a.works), len(a.works)-a.completed, a.found)
	if phase == "domain and wildcard checks" {
		fmt.Fprintln(a.diag, "Generation did not finish; generated names are a partial count.")
	}
	if outputOpened {
		fmt.Fprintf(a.diag, "Results appended to %s", a.cfg.Output)
		if err != nil {
			fmt.Fprint(a.diag, " (run incomplete; existing results preserved)")
		}
		fmt.Fprintln(a.diag)
	}
	if a.cfg.Stats || err != nil {
		enabled, totalErrors := 0, 0
		entries := a.pool.Snapshot()
		for _, entry := range entries {
			if entry.Status {
				enabled++
			}
			totalErrors += entry.Errors
			if a.cfg.Stats {
				fmt.Fprintf(a.diag, "Resolver: %s; errors: %d; enabled: %t\n", entry.Host, entry.Errors, entry.Status)
			}
		}
		fmt.Fprintf(a.diag, "Configured resolvers: %d/%d enabled; recorded errors: %d\n", enabled, len(entries), totalErrors)
	}
	var queryError *dns.QueryError
	if errors.Is(err, dnsmanager.ErrUnavailable) {
		fmt.Fprintln(a.diag, "No configured resolver remains enabled. Check the resolver list and network access; --show-stats lists resolver errors.")
	} else if errors.As(err, &queryError) {
		fmt.Fprintln(a.diag, "A DNS query exhausted its attempts. Check the reported resolver, DNS response or network error; this does not mean every resolver is unavailable.")
	}
}
