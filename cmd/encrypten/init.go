package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/keiritz/encrypten/internal/gitutil"
	"github.com/keiritz/encrypten/internal/keyfile"
)

func cmdInit(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: encrypten init")
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	keyDir, err := gitutil.KeyDir(dir)
	if err != nil {
		return fmt.Errorf("determining key directory: %w", err)
	}

	keyPath := filepath.Join(keyDir, "default")

	if _, err := os.Stat(keyPath); err != nil {
		// Key does not exist — generate it.
		if err := os.MkdirAll(keyDir, 0700); err != nil {
			return fmt.Errorf("creating key directory: %w", err)
		}

		key, err := keyfile.Generate()
		if err != nil {
			return fmt.Errorf("generating key: %w", err)
		}

		f, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600) //nolint:gosec // path derived from git dir
		if err != nil {
			return fmt.Errorf("creating key file: %w", err)
		}

		writeErr := keyfile.Write(f, key)
		closeErr := f.Close()

		if writeErr != nil {
			_ = os.Remove(keyPath) //nolint:gosec // cleanup of file we just created
			return fmt.Errorf("writing key: %w", writeErr)
		}
		if closeErr != nil {
			_ = os.Remove(keyPath) //nolint:gosec // cleanup of file we just created
			return fmt.Errorf("closing key file: %w", closeErr)
		}
	}

	// Warn if encrypten is not in $PATH.
	if err := gitutil.CheckInPath(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: %v\n", err)
	}

	// Always set filter and diff configuration.
	if err := gitutil.SetFilter(dir); err != nil {
		return fmt.Errorf("setting filter config: %w", err)
	}
	if err := gitutil.SetDiffTextconv(dir); err != nil {
		return fmt.Errorf("setting diff textconv config: %w", err)
	}

	// Enable worktree-specific config so secondary worktrees can
	// override filter settings independently.
	if err := gitutil.EnableWorktreeConfig(dir); err != nil {
		return fmt.Errorf("enabling worktree config: %w", err)
	}

	return nil
}
