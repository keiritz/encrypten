# encrypten

A [git-crypt](https://github.com/AGWA/git-crypt) compatible encryption tool written in Go with full worktree support.

> **Note:** This project is not affiliated with or endorsed by git-crypt.

## Features

- **git-crypt compatible** — encrypts and decrypts files using the same format (AES-256-CTR + HMAC-SHA1)
- **Full worktree support** — lock/unlock worktrees independently without affecting other worktrees
- **Static binary** — single binary with no runtime dependencies (`CGO_ENABLED=0`)
- **Go stdlib crypto only** — no external cryptographic dependencies
- **Cross-platform** — Linux, macOS, and Windows

## Installation

### From source

```sh
go install github.com/keiritz/encrypten/cmd/encrypten@latest
```

### From release binaries

Download pre-built binaries from the [Releases](https://github.com/keiritz/encrypten/releases) page.

## Quick Start

```sh
# Generate a key
encrypten keygen my-key

# Initialize encryption in a repository
cd my-repo
encrypten init

# Define files to encrypt in .gitattributes
echo 'secrets/** filter=git-crypt diff=git-crypt' >> .gitattributes

# Unlock (decrypt) files with a key
encrypten unlock my-key

# Lock (encrypt) files
encrypten lock

# Check encryption status
encrypten status
```

## Commands

| Command | Description |
|---|---|
| `init` | Initialize encryption in a repository (generates key, sets up filters) |
| `unlock <KEY_FILE>` | Decrypt files using the specified key file |
| `lock` | Encrypt files |
| `status` | Show encryption status of tracked files |
| `keygen <FILE>` | Generate a new encryption key |
| `export-key <FILE\|->` | Export the current key to a file or stdout |
| `clean` | Git clean filter (encrypts on commit) |
| `smudge` | Git smudge filter (decrypts on checkout) |
| `diff` | Git diff textconv filter |
| `version` | Display version |

### Flags

- `lock --force` / `lock -f` — Lock even with uncommitted changes
- `unlock --force` / `unlock -f` — Unlock even with uncommitted changes

## Worktree Support

encrypten supports git worktrees natively. Each worktree can be locked/unlocked independently:

```sh
# Main worktree
encrypten unlock my-key

# Create a worktree
git worktree add ../my-feature feature-branch

# Unlock the worktree (uses the shared key automatically)
cd ../my-feature
encrypten unlock

# Lock this worktree without affecting main
encrypten lock
```

## Differences from git-crypt

| | encrypten | git-crypt |
|---|---|---|
| Worktree support | Full per-worktree lock/unlock | [Not supported](https://github.com/AGWA/git-crypt/issues/105) |
| Language | Go | C++ |
| Binary | Static, single file | Requires OpenSSL |
| GPG key sharing | Not supported | Supported |
| Crypto | Go stdlib only | OpenSSL |

## License

[MIT](LICENSE)
