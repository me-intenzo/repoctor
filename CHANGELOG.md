# Changelog

All notable changes to Repoctor are documented here.

## [v1.0.1] - 2026-09-27

### Added

- Added dependency checks for exact versions pinned in `package-lock.json` and
  `go.mod`, including support for Go module replacements and a curated set of
  known CVEs.
- Added repository scanning checks for oversized historical blobs, committed
  environment files, secrets, `.gitignore` gaps, and stale branches.
- Added text and JSON reporting with severity-aware exit codes.
- Added release automation for Linux, macOS, and Windows binaries.

### Changed

- Organized scanner, Git, finding, reporting, and version code under the
  `internal/` packages.
- Reduced false positives from test fixtures during environment-file and
  secret scans.

### Fixed

- Corrected severity marker rendering in the repository scan showcase.
