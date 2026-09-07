package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/bp0lr/dmut/defines"
	"github.com/spf13/pflag"
)

type config struct {
	Workers, Retries, TimeoutMS, ErrorLimit                            int
	Dictionary, Domain, Output, DNSFile, DNSServers, SaveTo            string
	Verbose, UpdateDNS, UpdateFiles, ShowIP, Stats, Progress, SaveOnly bool
	Permutations                                                       defines.PermutationList
}

type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

func parseConfig(args []string, stdout, stderr io.Writer) (config, bool, error) {
	var c config
	var help, showVersion bool
	f := pflag.NewFlagSet("dmut", pflag.ContinueOnError)
	f.SetOutput(stderr)
	f.SetNormalizeFunc(func(_ *pflag.FlagSet, name string) pflag.NormalizedName {
		switch name {
		case "dnsFile":
			name = "dns-file"
		case "dnsServers":
			name = "dns-servers"
		case "dns-errorLimit":
			name = "dns-error-limit"
		}
		return pflag.NormalizedName(name)
	})
	f.IntVarP(&c.Workers, "workers", "w", 25, "DNS workers (1-150); not used by --save-gen")
	f.IntVar(&c.Retries, "dns-retries", 3, "Maximum attempts per DNS query (at least 1)")
	f.IntVar(&c.TimeoutMS, "dns-timeout", 500, "DNS timeout in milliseconds (1-10000)")
	f.IntVar(&c.ErrorLimit, "dns-error-limit", 25, "Disable a resolver after more than this many errors (at least 1)")
	f.StringVarP(&c.Dictionary, "dictionary", "d", "", "Mutation dictionary file (required except for help, version and updates)")
	f.StringVarP(&c.Domain, "url", "u", "", "Domain name; omit to read one domain per line from stdin")
	f.StringVarP(&c.Output, "output", "o", "", "Append results to this file")
	f.StringVarP(&c.DNSFile, "dns-file", "s", "", "Read DNS servers from this file")
	f.StringVarP(&c.DNSServers, "dns-servers", "l", "", "Comma-separated DNS servers; overrides --dns-file")
	f.StringVar(&c.SaveTo, "save-to", "generated.txt", "Generated file destination (used with --save-gen)")
	f.BoolVarP(&c.Verbose, "verbose", "v", false, "Write diagnostics to stderr")
	f.BoolVar(&c.UpdateDNS, "update-dnslist", false, "Download resolvers.txt and top20.txt to ~/.dmut")
	f.BoolVar(&c.UpdateFiles, "update-files", false, "Download resolver lists and words.txt to ~/.dmut")
	f.BoolVar(&c.ShowIP, "show-ip", false, "Include CNAME and IPv4 records in results")
	f.BoolVar(&c.Stats, "show-stats", false, "Write job statistics to stderr")
	f.BoolVar(&c.Progress, "use-pb", false, "Write a progress bar to stderr")
	f.BoolVar(&c.SaveOnly, "save-gen", false, "Save generated names and exit without DNS queries")
	f.BoolVar(&c.Permutations.AddToDomain, "disable-permutations", false, "Disable word insertion")
	f.BoolVar(&c.Permutations.AddNumbers, "disable-addnumbers", false, "Disable number additions")
	f.BoolVar(&c.Permutations.AddSeparator, "disable-addseparator", false, "Disable word concatenation and separators")
	f.BoolVarP(&help, "help", "h", false, "Show help")
	f.BoolVar(&showVersion, "version", false, "Show version")
	if err := f.Parse(args); err != nil {
		return c, false, &usageError{err}
	}
	if help {
		_, err := fmt.Fprintf(stdout, "Usage: dmut [options]\n\n%s\nLegacy aliases: --dnsFile, --dnsServers, --dns-errorLimit\n", f.FlagUsages())
		return c, true, err
	}
	if showVersion {
		_, err := fmt.Fprintln(stdout, "dmut", versionString())
		return c, true, err
	}
	var err error
	switch {
	case f.NArg() != 0:
		err = errors.New("unexpected positional arguments; use --url or stdin")
	case c.Workers < 1 || c.Workers > 150:
		err = errors.New("--workers must be between 1 and 150")
	case c.TimeoutMS < 1 || c.TimeoutMS > 10000:
		err = errors.New("--dns-timeout must be between 1 and 10000 milliseconds")
	case c.Retries < 1:
		err = errors.New("--dns-retries must be at least 1")
	case c.ErrorLimit < 1:
		err = errors.New("--dns-error-limit must be at least 1")
	case !c.UpdateDNS && !c.UpdateFiles && c.Dictionary == "":
		err = errors.New("--dictionary is required")
	case c.SaveOnly && c.SaveTo == "":
		err = errors.New("--save-to cannot be empty")
	}
	if err != nil {
		return c, false, &usageError{err}
	}
	return c, false, nil
}
