# dmut

A Go command-line tool that generates subdomain name variations from a dictionary
and can resolve the resulting names. Use it for domains you own or are authorized
to assess.

dmut supports stdin, offline generation, text output files, optional progress
reporting, and configurable DNS resolvers.

## Requirements and installation

Download a binary for your operating system and architecture from
[GitHub Releases](https://github.com/bp0lr/dmut/releases/latest). No Go installation
is needed to run a release binary. Archives include the starter dictionary and
shell completion scripts; each archive has a separate SHA-256 checksum file.

To build from source:

Building this checkout requires **Go 1.27.1 or newer**. The Go command can select
the required toolchain automatically when toolchain downloads are enabled.

Install the latest published version:

~~~sh
go install github.com/bp0lr/dmut@latest
dmut --version
dmut --help
~~~

Make sure the Go binary installation directory is on your PATH. It is GOBIN when
configured, otherwise the bin directory under GOPATH.

To build the current checkout:

~~~sh
go build -o dmut .
~~~

On Windows, use:

~~~powershell
go build -o dmut.exe .
.\dmut.exe --help
~~~

The executable remains at the module root, so the installation path is unchanged.

## Preview the rules offline

Start with one example domain:

~~~sh
dmut -u test.example.com -d words.txt --preview --explain
~~~

This prints a small sample with columns for the name, rule family and dictionary
word. It makes no DNS queries and does not create or replace output files.
Omit **--explain** to print just the example names.

**--preview-limit N** reads the first N nonempty dictionary entries and shows at
most N distinct examples per enabled rule family (default 5). Duplicate names
from different families may appear so you can see the overlap. The sample follows
generation order, not the sorted order used by **--save-gen**. It is neither a
total count nor validation of the rest of the dictionary. Use **--url** to select
one domain; preview does not read domains from stdin.

For example, with `stage` as the first dictionary entry:

~~~text
NAME                    RULE                            WORD
test-0.example.com      numeric addition                -
stage.test.example.com  word insertion                  "stage"
test-stage.example.com  word concatenation / separator  "stage"
~~~

This example uses **--preview-limit 1**. The existing **--disable-...** options
also apply to previews. A rule explanation identifies the family and input word;
the legacy cumulative behavior for multi-label subdomains is retained.

## Quick start: generate a file offline

The repository includes [words.txt](words.txt), a small starter dictionary.
Provide one word per line in your own dictionary. Blank lines and surrounding
whitespace are ignored; both LF and CRLF files are accepted.

~~~sh
dmut -u test.example.com -d words.txt --save-gen --save-to generated.txt
~~~

This writes generated names without DNS queries. The destination is replaced
only after the new file has been written successfully. The output option and
resolver configuration are unused in this mode.

A summary on stderr reports the number of saved names, completed domains,
elapsed time and destination. On a reported failure, it states that the
destination was not replaced.

Offline generation reads domains one at a time and snapshots the normalized
dictionary to disk. Generated names are sorted and deduplicated in temporary
files, using a roughly 4 MiB sorting buffer and at most 32 input runs per merge.
These are buffer sizes, **not limits on the number of words, domains or results**.
Memory also includes I/O buffers, runtime overhead and the longest records.

Temporary files are created beside the destination selected by **--save-to**.
Allow additional disk space for the dictionary snapshot, sorting runs and the
new output while the previous output is still present. Temporary data is removed
after success, cancellation or a reported error. Forced termination or a machine
crash can leave temporary files behind.

Names are sorted within each input domain, and domains are processed in input
order. Deduplication remains per input domain: repeated input domains can produce
repeated output. **--workers** controls DNS resolution and is not used by the
offline mode; lowering it is no longer needed to address issue #15.

Input must be a **domain name**, such as test.example.com, without a URL scheme,
path or port. The historical flag name **--url** is retained for compatibility.

To read multiple domains from stdin on Linux or macOS:

~~~sh
cat domains.txt | dmut -d words.txt --save-gen --save-to generated.txt
~~~

In PowerShell:

~~~powershell
Get-Content domains.txt | dmut -d words.txt --save-gen --save-to generated.txt
~~~

## Resolve names

For an authorized domain:

~~~sh
dmut -u test.example.com -d words.txt -o results.txt
~~~

Results go to stdout and are appended to results.txt. Existing output is
preserved. An empty result file is retained, and output from interrupted or failed
runs may be partial. Check the exit code before treating a run as complete.

Diagnostics, statistics, progress and update messages go to stderr. **--use-pb**
does not suppress results or change the selected result format. Ordering is
unspecified because work runs concurrently.

**--show-ip** includes CNAME and IPv4 records. The legacy file and terminal layouts
differ; they are text formats, not JSON or CSV.

## Resolver configuration

Resolver sources are selected in this order:

1. **--dns-servers** / **-l**.
2. **--dns-file** / **-s**.
3. The resolvers.txt file in the configuration directory, if present.
4. The built-in Cloudflare, Google and Quad9 resolver addresses.

An explicitly selected empty or unreadable file is an error. Use one IPv4 address
per line, optionally followed by a colon and port; the default port is 53. Blank
lines and surrounding whitespace are ignored. **--dns-servers** takes a
comma-separated list and overrides the resolver file.

Update the two resolver lists:

~~~sh
dmut --update-dnslist
~~~

Update the resolver lists and starter dictionary:

~~~sh
dmut --update-files
~~~

Files are stored in **~/.dmut** on Unix and **%USERPROFILE%\.dmut** on Windows:

| File | Source | Used automatically? |
| --- | --- | --- |
| resolvers.txt | [dmut-resolvers](https://github.com/bp0lr/dmut-resolvers) | Yes, when no resolver flags are supplied |
| top20.txt | [dmut-resolvers](https://github.com/bp0lr/dmut-resolvers) | No; select it with --dns-file |
| words.txt | This repository | No; select it with --dictionary |

Downloads have a 30-second timeout and a 32 MiB limit per file. Failed, empty or
incomplete downloads preserve the previous file. Updates are applied one file at
a time, so a later failure does not roll back earlier successful files. Replacement
uses a temporary file in the destination directory; rename atomicity depends on
the operating system and filesystem.

## Options

Run **dmut --help** for the authoritative reference.

| Option | Default | Purpose |
| --- | --- | --- |
| -u, --url | stdin | Input domain name |
| -d, --dictionary | required | Dictionary path; not required for help, version or updates |
| -w, --workers | 25 | DNS workers, from 1 through 150; unused by --save-gen |
| --dns-timeout | 500 | Timeout in milliseconds, from 1 through 10000 |
| --dns-retries | 3 | Maximum attempts per query, at least 1 |
| --dns-error-limit | 25 | Disable a resolver after **more than** this many errors; at least 1 |
| -s, --dns-file | configuration lookup | Resolver list file |
| -l, --dns-servers | configuration lookup | Comma-separated resolvers; overrides --dns-file |
| -o, --output | none | Append results to a file |
| --save-gen | false | Write generated names without DNS queries, then exit |
| --save-to | generated.txt | Destination for --save-gen |
| --preview | false | Preview examples offline for one --url domain |
| --preview-limit | 5 | Dictionary entries sampled and maximum examples per rule |
| --explain | false | Show the rule family and word in preview output |
| --completion | | Print completion for bash, zsh, fish or powershell |
| --show-ip | false | Include CNAME and IPv4 records |
| --show-stats | false | Write job statistics to stderr |
| --use-pb | false | Write a progress bar to stderr |
| -v, --verbose | false | Write diagnostics to stderr |
| --update-dnslist | false | Update resolvers.txt and top20.txt, then exit |
| --update-files | false | Update both resolver lists and words.txt, then exit |
| --disable-permutations | false | Disable word insertion |
| --disable-addnumbers | false | Disable numeric additions |
| --disable-addseparator | false | Disable word concatenation and separators |
| --version | | Show the build version |
| -h, --help | | Show help |

The old spellings **--dnsFile**, **--dnsServers** and **--dns-errorLimit** remain
aliases. Invalid numeric arguments now produce an error instead of silently
reverting to defaults.

## Shell completion

Load completion for the current shell session:

~~~sh
# Bash
source <(dmut --completion bash)

# Fish
dmut --completion fish | source
~~~

~~~powershell
# PowerShell
dmut --completion powershell | Out-String | Invoke-Expression
~~~

For Zsh, save `dmut --completion zsh` as `_dmut` in a directory on your `fpath`,
then run `autoload -Uz compinit; compinit`. Release archives include the same
scripts in `completions/`. Bash and PowerShell suggest option names; Zsh and Fish
also describe options and complete file arguments. These commands do not change
your shell profile; add the appropriate loader there to enable completion in
future sessions.

## Exit codes and cancellation

| Code | Meaning |
| --- | --- |
| 0 | Completed successfully, including a run with no matches |
| 1 | Input, download, DNS or output failure |
| 2 | Invalid command-line arguments |
| 130 | Interrupted with Ctrl+C |

A DNS transport failure after the configured attempts now stops the run and
returns an error, rather than silently reporting success with missing work.
Workers are joined before exit; canceled downloads preserve the previous file.
Errors writing results or closing the output file are also reported.

Every DNS run reports checked/skipped domains, generated names, completed and
incomplete DNS jobs, matches and elapsed time on stderr. If generation stops
early, the generated count is explicitly marked partial. **--show-stats** adds
per-resolver error counts; failures always include a pool summary.

DNS errors identify the query type, domain, attempts, timeout and last resolver,
while preserving the underlying cause (for example a timeout or SERVFAIL).
Running out of attempts for one query is different from having no enabled
resolver. Resolver selection now checks the full pool before declaring it
unavailable, even when its initial random choices all hit disabled entries.

When diagnosing a failure, check the reported resolver and underlying error,
then your configured resolver file and network access. Wildcard checks and
confirmation identify their fixed resolver separately; it may not belong to the
configured pool. A genuine connectivity or server failure can still stop a run.
The changes for issue #20 improve correctness and diagnosis, not the availability
of external DNS servers.

## Existing behavior and limitations

- The original generation rules are retained. Offline generation uses disk-backed
  sorting with bounded buffers. The DNS resolution workflow still holds its
  generated work in memory; this change addresses the offline case in issue #15.
- Duplicate names are removed within each generation job, not across all input
  domains. Offline output follows input-domain order and lexical order within
  each domain; DNS result order is not stable.
- Text inputs keep the existing 1 MiB limit per line. There is no total input or
  output count limit in offline mode. Memory usage can still depend on individual
  word/domain lengths, and disk space must be sufficient for temporary files.
- The resolution workflow checks A and CNAME records.
- Wildcard checking uses a fixed probe and Google DNS (8.8.8.8:53). The existing
  policy skips a job when a completed probe response is not NXDOMAIN; it can
  discard domains in situations other than a wildcard.
- Result confirmation also uses Google DNS with fixed settings. Selecting a
  custom resolver list does **not** keep all queries on that list, which matters
  for private or split-horizon DNS.
- Resolver lists change over time. Their source does not guarantee that every
  server is available or returns correct answers.
- Historical comparisons with altdns used different environments and resolver
  lists. They are not a current performance guarantee.

## Development and releases

See [CONTRIBUTING.md](CONTRIBUTING.md) for checks, package responsibilities and the
release procedure. CI runs tests and builds on Linux, Windows and macOS, checks
formatting and dependencies, and runs the race detector on Linux.

The release workflow prepares archives and SHA-256 checksums for Linux, macOS and
Windows on amd64 and arm64. It uploads workflow artifacts for review; publication
as a GitHub Release is a separate maintainer action.

## Credits and license

Inspired by [altdns](https://github.com/infosec-au/altdns), originally created by
**infosec-au**. Thanks to its authors and contributors.

[MIT license](LICENSE).
