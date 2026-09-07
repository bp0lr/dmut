# Contributing

Use Go 1.27.1 or newer. Keep changes focused and describe the problem, resulting
behavior, compatibility impact and validation in the pull request.

## Local checks

```sh
go test -count=1 ./...
go vet ./...
go build ./...
go mod verify
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

Format changed Go files with `gofmt -w`. If dependencies change, run `go mod tidy`
and commit both `go.mod` and `go.sum`. Review direct dependencies individually;
avoid unrelated upgrades in a bug-fix PR.

Run the race detector in a supported environment with CGO and a C compiler:

```sh
go test -race -count=1 ./...
```

CI runs this check on Linux. A Windows installation with `CGO_ENABLED=0` needs a
compatible C compiler or a Linux environment before it can run `-race`.

Tests use temporary files, injected responses and HTTP/DNS servers bound to
loopback. Do not make tests depend on public DNS servers or downloaded lists.

The offline memory regression uses a five-million-entry dictionary and a separate
workload with over 192 MiB of generated output. It runs in isolated subprocesses
and checks sampled Go heap usage with a fixed GC setting, rather than imposing a
memory limit on the application. Allow several hundred MiB of temporary disk
space. CI runs it on Linux separately from the race detector:

```sh
DMUT_TEST_LARGE_FILES=1 go test -run '^TestOfflineMemoryBound$' -count=1 -v .
```

In PowerShell:

```powershell
$env:DMUT_TEST_LARGE_FILES = '1'
go test -run '^TestOfflineMemoryBound$' -count=1 -v .
Remove-Item Env:DMUT_TEST_LARGE_FILES
```

## Code organization

| Location | Responsibility |
| --- | --- |
| `main.go` | Process signals, version and exit status |
| `config.go` | Flags, legacy aliases and argument validation |
| `app.go` | Invocation state, cancellation, file inputs and worker lifecycle |
| `offline.go` | Streaming offline input, on-disk dictionary and output transaction |
| `preview.go`, `completion.go` | Offline examples, rule explanations and shell completion |
| `diagnostics.go` | Completion/error summaries after workers and output cleanup |
| `internal/linesort/` | Bounded-buffer disk sorting and per-domain deduplication |
| `processing.go` | Existing domain workflow and result presentation |
| `dns/` | Context-aware DNS client and response data |
| `dnsManager/` | Resolver pool with synchronized health state |
| `resolver/` | Query response adapter and error propagation |
| `tables/`, `defines/` | Existing generation rules and their input types |
| `util/` | Text lists, temporary-file replacement and downloads |

The CLI is the supported user interface. Keep the executable at the module root
to preserve `go install github.com/bp0lr/dmut@latest`. Go package APIs are currently
implementation details and changed during this modernization.

Avoid package-global execution state. Pass configuration and dependencies
explicitly, return errors to the caller, and let `main` choose the exit code.
Protect shared state or give it a single owner. Keep diagnostics on stderr and
results on stdout.

When changing flags, update both `--help` and the README and test any retained
aliases. When changing file handling, test failed writes and preservation of
existing data. New tests should cover behavior and meaningful failure cases.

Shell completion is generated from the flag definitions. Check emitted scripts
in their target shells when changing the generator. Preview uses the same rule
implementation as normal generation; keep the captured legacy-output fixtures
passing and preserve the distinction between a sample and a full job.

## Release procedure

1. Update the changelog and confirm that CI passes for the intended commit.
2. Run **Release artifacts** manually on that commit's branch with a version such
   as `v1.2.3-rc.1`, or push a matching version tag when ready. Manual builds do
   not create a tag.
3. Download the artifacts for each platform. Each contains a platform archive
   and its `.sha256` file. Verify the archive with `sha256sum -c FILE.sha256` on
   Linux, `shasum -a 256 -c FILE.sha256` on macOS, or compare the hash from
   `Get-FileHash FILE.zip -Algorithm SHA256` in PowerShell.
4. Extract the archive and check `dmut --version` and `dmut --help` on the target
   platform. Archives include the binary, README, changelog, license, starter
   dictionary and shell completion scripts.
5. Publish a GitHub Release separately, attached to the reviewed commit's version
   tag, and upload the archives and checksum files. Include migration notes.

The workflow runs CI before packaging, disables CGO for release binaries and
embeds the version with `-ldflags "-X main.version=VERSION"`. It has read-only
repository permissions and does not publish releases automatically. Checksums
detect file corruption; they are not signatures.

Dependabot checks Go modules and GitHub Actions weekly. Keep the minimum Go version
and the CI configuration in sync when updating the toolchain.
