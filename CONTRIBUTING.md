# Contributing to encrypten

Thank you for your interest in contributing to encrypten!

## Development Setup

```sh
git clone https://github.com/keiritz/encrypten.git
cd encrypten
go build ./cmd/encrypten/
```

**Requirements:**

- Go 1.25+
- git
- git-crypt (for running compatibility tests)

## Running Tests

```sh
go test ./...              # Run all tests
go test -race ./...        # With race condition detection
go tool golangci-lint run  # Lint
```

## Pull Request Guidelines

### Branch Naming

```
<type>/<issue-number>-<short-description>
```

Types: `feat`, `fix`, `test`, `infra`, `docs`, `refactor`

### Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

Refs #<issue-number>
```

Scopes: `keyfile`, `header`, `crypto`, `gitutil`, `cli`, `worktree`, `compat`

### Merge Strategy

All PRs are merged via **merge commit** (no squash or rebase).
