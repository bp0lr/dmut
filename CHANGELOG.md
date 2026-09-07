# Changelog

## v1.0.0 - 2026-09-07

### Changed

- Require Go 1.27.1 and refresh the dependencies used by the CLI.
- Keep the root installation path and retain legacy DNS flag aliases alongside
  the normalized spellings `--dns-file`, `--dns-servers`, `--dns-error-limit`.
- Validate numeric arguments instead of silently replacing invalid values.
- Send diagnostics, progress and update messages to stderr; keep results on
  stdout even when progress is enabled.
- Report input, output and DNS failures with nonzero exit codes. A DNS transport
  failure after the configured attempts now cancels the remaining work.
- Keep empty output files, and leave `--output` untouched in offline generation
  mode. Existing result files are still opened in append mode.
- Make `--update-dnslist` update only resolver lists; use `--update-files` to also
  download the starter dictionary.
- Replace global execution state with per-invocation configuration, synchronized
  statistics and an independently owned resolver pool.
- Remove unused dependencies and helpers, and replace reflection-based string
  deduplication with standard-library slice operations.

### Fixed

- Address #20's misleading resolver failures: check the whole pool before
  reporting it unavailable, retain query type/resolver/attempts/cause, and show
  completion and resolver summaries on failed or canceled runs. Real DNS
  connectivity failures still stop the run; the original report's external
  network conditions could not be reproduced from the supplied information.
- Fix #15's offline `--save-gen` memory growth: stream domain input, snapshot the
  dictionary on disk, emit names individually and deduplicate with an external
  merge sort. Large inputs complete without retaining the whole job in RAM or
  introducing a total word/domain/result limit. Preserve the existing rule output
  and per-domain deduplication scope; offline ordering is now deterministic.
- Preserve the previous generated file and remove temporary data on reported
  input, output and cancellation failures. Check cancellation before replacement.
- Preserve DNS errors instead of returning a successful error value on failure.
- Handle absent DNS responses before inspecting them.
- Synchronize resolver state and result counters used by concurrent workers.
- Preserve previous files on failed downloads or generated-file writes.
- Check download HTTP status, nonempty content, size and timeout.
- Check text scanner errors and result-file writes and closes.
- Use appropriate directory creation permissions and platform-aware file paths.
- Accept CRLF text lists while ignoring blank lines and surrounding whitespace.

### Added

- Offline `--preview` and `--explain`, sampling dictionary entries and examples
  per existing rule family with `--preview-limit` (default 5).
- Final DNS and offline summaries, examples in help, and generated Bash, Zsh,
  Fish and PowerShell completion scripts included in release archives.
- `--version`, explicit exit statuses and cancellation with worker cleanup.
- Local regression tests for file preservation, errors, CLI compatibility and
  concurrent state.
- Captured legacy-output fixtures for all generation flags, multi-pass disk sort
  tests and isolated large-file heap regressions for offline generation.
- CI across Linux, Windows and macOS, Linux race checks, dependency verification
  and vulnerability checks.
- Weekly dependency update configuration and release artifacts with checksums.
- Updated README and contributor documentation.

The existing generation, wildcard and confirmation policies remain in place.
The README documents their current limitations. Go package APIs were reorganized;
compatibility is maintained for CLI flag names, not the previous Go APIs.
