package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/bp0lr/dmut/defines"
	"github.com/bp0lr/dmut/resolver"
	"github.com/bp0lr/dmut/tables"
	"github.com/bp0lr/dmut/util"
	"github.com/miekg/dns"
	tld "github.com/weppos/publicsuffix-go/publicsuffix"
)

func (a *application) generateTable(ctx context.Context, domain string, words []string) ([]string, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	job, err := parseDomain(domain)
	if err != nil {
		return nil, false, err
	}
	if !a.cfg.SaveOnly {
		wildcard, err := a.checkWildcard(ctx, job)
		if err != nil {
			return nil, false, fmt.Errorf("wildcard check (fixed resolver 8.8.8.8:53): %w", err)
		}
		if wildcard {
			if a.cfg.Verbose {
				fmt.Fprintf(a.diag, "Skipped %s: wildcard check matched\n", domain)
			}
			return nil, true, nil
		}
	}
	names := tables.GenerateTables(job, words, a.cfg.Permutations)
	return names, false, ctx.Err()
}

func parseDomain(domain string) (defines.DmutJob, error) {
	if strings.ContainsAny(domain, "/:@ \\?#\t\r\n") {
		return defines.DmutJob{}, fmt.Errorf("expected a domain name without scheme, path or port")
	}
	parts, err := tld.Parse(domain)
	if err != nil {
		return defines.DmutJob{}, err
	}
	return defines.DmutJob{Domain: domain, Sld: parts.SLD, Tld: parts.TLD, Trd: parts.TRD}, nil
}

func (a *application) processDNS(ctx context.Context, domain string, output *os.File) error {
	if a.cfg.Verbose {
		fmt.Fprintf(a.diag, "Testing: %s\n", domain)
	}
	for _, kind := range []uint16{dns.TypeA, dns.TypeCNAME} {
		response, err := resolver.GetDNSQueryResponse(ctx, a.client, domain, kind, "")
		if err != nil {
			return err
		}
		found, err := a.processResponse(ctx, domain, response, output, kind)
		if err != nil {
			return err
		}
		if found {
			break
		}
	}
	return nil
}

func (a *application) processResponse(ctx context.Context, domain string, result resolver.JobResponse, output *os.File, kind uint16) (bool, error) {
	if !result.Status || result.Data.StatusCode != "NOERROR" {
		return false, nil
	}
	if kind == dns.TypeA && len(result.Data.A) == 0 {
		return false, nil
	}
	if kind == dns.TypeCNAME && len(result.Data.CNAME) == 0 {
		return false, nil
	}
	retest, err := resolver.GetDNSQueryResponse(ctx, a.confirmation, domain, kind, "8.8.8.8:53")
	if err != nil {
		return false, fmt.Errorf("result confirmation (fixed resolver 8.8.8.8:53): %w", err)
	}
	if !retest.Status || retest.Data.StatusCode != "NOERROR" {
		return false, nil
	}
	return true, a.writeResult(domain, result, output)
}

func (a *application) writeResult(domain string, result resolver.JobResponse, output *os.File) error {
	// Keep the existing text formats while ensuring each result is emitted whole.
	a.mu.Lock()
	defer a.mu.Unlock()
	name := strings.TrimSuffix(domain, ".")
	cnames := util.TrimChars(strings.Join(result.Data.CNAME, ","))
	addresses := util.TrimChars(strings.Join(result.Data.A, ","))
	if output != nil {
		line := name
		if a.cfg.ShowIP {
			line += ":" + cnames + addresses
		}
		if _, err := fmt.Fprintln(output, line); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
	}
	line := name
	if a.cfg.ShowIP {
		line = fmt.Sprintf("%s : [%s]:[%s]", name, cnames, addresses)
	}
	if _, err := fmt.Fprintln(a.out, line); err != nil {
		return fmt.Errorf("write stdout: %w", err)
	}
	a.found++
	return nil
}

func (a *application) checkWildcard(ctx context.Context, job defines.DmutJob) (bool, error) {
	var mutations []string
	tables.AddToDomain(job, []string{"supposedtonotexistmyfriend"}, &mutations)
	for _, mutation := range mutations {
		for _, kind := range []uint16{dns.TypeA, dns.TypeCNAME} {
			response, err := resolver.GetDNSQueryResponse(ctx, a.client, mutation, kind, "8.8.8.8:53")
			if err != nil {
				return false, err
			}
			// Retain the legacy wildcard policy; document its limitations.
			if response.Status && response.Data.StatusCode != "NXDOMAIN" {
				return true, nil
			}
		}
	}
	return false, nil
}
