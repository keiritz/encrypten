# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [0.1.0] - 2026-03-15

### Added

- AES-256-CTR + HMAC-SHA1 encryption/decryption compatible with git-crypt
- Key file read/write (FORMAT_VERSION 2)
- Git filter support (clean, smudge, diff textconv)
- Commands: `init`, `lock`, `unlock`, `status`, `keygen`, `export-key`
- Full git worktree support with per-worktree lock/unlock
- `--force` flag for lock/unlock with uncommitted changes
- Cross-platform support (Linux, macOS, Windows)
